// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type installSourceKind string

const (
	sourceLocalBinary       installSourceKind = "local-binary"
	sourceLocalArchive      installSourceKind = "local-archive"
	sourceLocalPackage      installSourceKind = "local-package"
	sourceDownloadedBinary  installSourceKind = "downloaded-binary"
	sourceDownloadedArchive installSourceKind = "downloaded-archive"
	sourceDownloadedPackage installSourceKind = "downloaded-package"
	installTimeoutEnv                         = "ESHU_NORNICDB_INSTALL_TIMEOUT"
)

// nornicDBBinaryMaxBytes caps any staged NornicDB binary at 512 MiB.
// Archives enforce it during streaming; the darwin package path enforces it
// on the expanded Payload binary and, since pkgutil expansion itself is
// unbounded, on the whole expansion tree while pkgutil runs
// (runBoundedExpansion), and refuses before expansion when the artifact
// itself already meets the cap or the temp volume lacks cap-sized room.
const nornicDBBinaryMaxBytes = 512 << 20

type preparedInstallSource struct {
	SourcePath      string
	SourceKind      installSourceKind
	SourceSHA256    string
	LocalBinaryPath string
	BinarySHA256    string
	Version         string
	Headless        bool
	cleanup         func() error
}

func (s preparedInstallSource) Close() error {
	if s.cleanup == nil {
		return nil
	}
	return s.cleanup()
}

// prepareInstallSource verifies the source reference and stages a local
// binary: checksum first (fail closed on mismatch or on a remote source
// without --sha256), then version probe, then extraction for archives and
// packages. The caller owns Close.
func prepareInstallSource(ctx context.Context, sourceRef, expectedSHA string, readVersion VersionReader) (preparedInstallSource, error) {
	sourceRef = strings.TrimSpace(sourceRef)
	remote := isRemoteInstallSource(sourceRef)
	expectedSHA, err := normalizeInstallSHA256(expectedSHA, remote)
	if err != nil {
		return preparedInstallSource{}, err
	}
	localPath, kind, cleanup, err := materializeInstallSource(ctx, sourceRef)
	if err != nil {
		return preparedInstallSource{}, err
	}

	sourceSHA, err := sha256File(localPath)
	if err != nil {
		_ = cleanup()
		return preparedInstallSource{}, err
	}

	if expectedSHA != "" && expectedSHA != sourceSHA {
		_ = cleanup()
		return preparedInstallSource{}, fmt.Errorf("sha256 mismatch for %q: got %s, want %s", sourceRef, sourceSHA, expectedSHA)
	}

	prepared, err := inspectInstallSource(localPath, kind, sourceRef, readVersion)
	if err != nil {
		_ = cleanup()
		return preparedInstallSource{}, err
	}
	prepared.SourceSHA256 = sourceSHA
	prepared.cleanup = combineInstallCleanups(prepared.cleanup, cleanup)
	return prepared, nil
}

func isRemoteInstallSource(sourceRef string) bool {
	parsed, err := url.Parse(sourceRef)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
}

// normalizeInstallSHA256 enforces the fail-closed source policy the install
// contract pins: remote http/https sources require the independently
// verified artifact checksum, and any supplied checksum must be 64 hex
// characters. Uppercase input normalizes to lowercase before comparison.
func normalizeInstallSHA256(raw string, required bool) (string, error) {
	checksum := strings.ToLower(strings.TrimSpace(raw))
	if checksum == "" {
		if required {
			return "", fmt.Errorf("remote nornicdb sources require --sha256 with the independently verified artifact checksum")
		}
		return "", nil
	}
	if len(checksum) != 64 {
		return "", fmt.Errorf("invalid nornicdb source sha256: expected 64 hexadecimal characters")
	}
	for _, character := range checksum {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return "", fmt.Errorf("invalid nornicdb source sha256: expected 64 hexadecimal characters")
		}
	}
	return checksum, nil
}

// combineInstallCleanups preserves independent source and extraction cleanup
// callbacks so archive/package installs do not leak temporary trees.
func combineInstallCleanups(cleanups ...func() error) func() error {
	return func() error {
		var errs []error
		for _, cleanup := range cleanups {
			if cleanup == nil {
				continue
			}
			if err := cleanup(); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
}

func materializeInstallSource(ctx context.Context, sourceRef string) (string, installSourceKind, func() error, error) {
	parsed, err := url.Parse(sourceRef)
	if err == nil && parsed.Scheme != "" && parsed.Scheme != "file" {
		if !isRemoteInstallSource(sourceRef) {
			return "", "", nil, fmt.Errorf("unsupported nornicdb source URL scheme %q; use http, https, or file", parsed.Scheme)
		}
		path, err := downloadInstallSource(ctx, sourceRef)
		if err != nil {
			return "", "", nil, err
		}
		kind := sourceDownloadedBinary
		if looksLikeArchive(sourceRef) {
			kind = sourceDownloadedArchive
		} else if looksLikePackage(sourceRef) {
			kind = sourceDownloadedPackage
		}
		return path, kind, func() error { return os.RemoveAll(filepath.Dir(path)) }, nil
	}

	if err == nil && parsed.Scheme == "file" {
		sourceRef = parsed.Path
	}

	path, err := filepath.Abs(sourceRef)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve nornicdb source path: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", "", nil, fmt.Errorf("stat nornicdb source %q: %w", path, err)
	}
	if info.IsDir() {
		return "", "", nil, fmt.Errorf("nornicdb source path %q is a directory; pass the binary or tarball path", path)
	}
	kind := sourceLocalBinary
	if looksLikeArchive(path) {
		kind = sourceLocalArchive
	} else if looksLikePackage(path) {
		kind = sourceLocalPackage
	}
	return path, kind, func() error { return nil }, nil
}

func inspectInstallSource(localPath string, kind installSourceKind, sourceRef string, readVersion VersionReader) (preparedInstallSource, error) {
	switch kind {
	case sourceLocalArchive, sourceDownloadedArchive:
		extractedBinary, extractedName, cleanup, err := extractBinaryFromArchive(localPath)
		if err != nil {
			return preparedInstallSource{}, err
		}
		version, err := readVersion(extractedBinary)
		if err != nil {
			_ = cleanup()
			return preparedInstallSource{}, fmt.Errorf("verify nornicdb source binary %q: %w", sourceRef, err)
		}
		binarySHA, err := sha256File(extractedBinary)
		if err != nil {
			_ = cleanup()
			return preparedInstallSource{}, err
		}
		return preparedInstallSource{
			SourcePath:      sourceRef,
			SourceKind:      kind,
			LocalBinaryPath: extractedBinary,
			BinarySHA256:    binarySHA,
			Version:         version,
			Headless:        extractedName == managedBinaryName,
			cleanup:         cleanup,
		}, nil
	case sourceLocalPackage, sourceDownloadedPackage:
		extractedBinary, extractedName, cleanup, err := extractBinaryFromPackage(localPath)
		if err != nil {
			return preparedInstallSource{}, err
		}
		version, err := readVersion(extractedBinary)
		if err != nil {
			_ = cleanup()
			return preparedInstallSource{}, fmt.Errorf("verify nornicdb source binary %q: %w", sourceRef, err)
		}
		binarySHA, err := sha256File(extractedBinary)
		if err != nil {
			_ = cleanup()
			return preparedInstallSource{}, err
		}
		return preparedInstallSource{
			SourcePath:      sourceRef,
			SourceKind:      kind,
			LocalBinaryPath: extractedBinary,
			BinarySHA256:    binarySHA,
			Version:         version,
			Headless:        extractedName == managedBinaryName,
			cleanup:         cleanup,
		}, nil
	default:
		version, err := readVersion(localPath)
		if err != nil {
			return preparedInstallSource{}, fmt.Errorf("verify nornicdb source binary %q: %w", sourceRef, err)
		}
		binarySHA, err := sha256File(localPath)
		if err != nil {
			return preparedInstallSource{}, err
		}
		return preparedInstallSource{
			SourcePath:      localPath,
			SourceKind:      kind,
			LocalBinaryPath: localPath,
			BinarySHA256:    binarySHA,
			Version:         version,
			Headless:        filepath.Base(localPath) == managedBinaryName,
			cleanup:         nil,
		}, nil
	}
}

func downloadInstallSource(ctx context.Context, sourceURL string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout, err := installDownloadTimeout()
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("build nornicdb download request: %w", err)
	}
	client := &http.Client{Timeout: timeout}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("download nornicdb source %q: %w", sourceURL, err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download nornicdb source %q: unexpected status %s", sourceURL, response.Status)
	}

	tempDir, err := os.MkdirTemp("", "eshu-nornicdb-install-*")
	if err != nil {
		return "", fmt.Errorf("create nornicdb source temp directory: %w", err)
	}
	name := filepath.Base(request.URL.Path)
	if strings.TrimSpace(name) == "" || name == "." || name == "/" {
		name = "nornicdb-source"
	}
	targetPath := filepath.Join(tempDir, name)
	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- targetPath is filepath.Join of an os.MkdirTemp dir and a filepath.Base-cleaned URL filename; contained within the temp dir
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("create downloaded nornicdb source file: %w", err)
	}
	if _, err := io.Copy(file, response.Body); err != nil {
		_ = file.Close()
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("download nornicdb source body: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("close downloaded nornicdb source file: %w", err)
	}
	return targetPath, nil
}

func installDownloadTimeout() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(installTimeoutEnv))
	if raw == "" {
		return 30 * time.Second, nil
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s=%q: %w", installTimeoutEnv, raw, err)
	}
	if timeout <= 0 {
		return 0, fmt.Errorf("parse %s=%q: must be greater than zero", installTimeoutEnv, raw)
	}
	return timeout, nil
}

func extractBinaryFromArchive(archivePath string) (string, string, func() error, error) {
	file, err := os.Open(archivePath) // #nosec G304 -- archivePath is the explicitly selected local artifact or a downloaded temp file; extraction writes only fixed allowlisted basenames under a fresh temp directory
	if err != nil {
		return "", "", nil, fmt.Errorf("open nornicdb archive %q: %w", archivePath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	var tarReader *tar.Reader
	switch {
	case strings.HasSuffix(strings.ToLower(archivePath), ".tar.gz"), strings.HasSuffix(strings.ToLower(archivePath), ".tgz"):
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return "", "", nil, fmt.Errorf("open gzip nornicdb archive %q: %w", archivePath, err)
		}
		defer func() {
			_ = gzipReader.Close()
		}()
		tarReader = tar.NewReader(gzipReader)
	case strings.HasSuffix(strings.ToLower(archivePath), ".tar"):
		tarReader = tar.NewReader(file)
	default:
		return "", "", nil, fmt.Errorf("nornicdb source %q is not a supported archive; use .tar, .tar.gz, or .tgz", archivePath)
	}

	tempDir, err := os.MkdirTemp("", "eshu-nornicdb-archive-*")
	if err != nil {
		return "", "", nil, fmt.Errorf("create archive extraction directory: %w", err)
	}

	var extractedPath string
	var extractedName string
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = os.RemoveAll(tempDir)
			return "", "", nil, fmt.Errorf("read nornicdb archive %q: %w", archivePath, err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.Base(header.Name)
		if name != managedBinaryName && name != "nornicdb" {
			continue
		}
		targetPath := filepath.Join(tempDir, name)
		targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o750) // #nosec G302 G304 -- 0o750 is intentional: extracted NornicDB binary requires owner execute; targetPath is filepath.Join of os.MkdirTemp dir and filepath.Base(header.Name) validated against a fixed allowlist
		if err != nil {
			_ = os.RemoveAll(tempDir)
			return "", "", nil, fmt.Errorf("create extracted nornicdb binary: %w", err)
		}
		written, err := io.Copy(targetFile, io.LimitReader(tarReader, nornicDBBinaryMaxBytes))
		if err != nil {
			_ = targetFile.Close()
			_ = os.RemoveAll(tempDir)
			return "", "", nil, fmt.Errorf("extract nornicdb binary from archive: %w", err)
		}
		if written >= nornicDBBinaryMaxBytes {
			// A member at exactly the cap is indistinguishable from a
			// truncated longer member (LimitReader reports no error for
			// either), so meets-or-exceeds refuses: only the verified
			// artifact bytes may become the managed binary.
			_ = targetFile.Close()
			_ = os.RemoveAll(tempDir)
			return "", "", nil, fmt.Errorf("nornicdb archive member %q meets or exceeds the 512 MiB extraction cap (stopped after %d bytes at the cap); refusing truncated install", header.Name, written)
		}
		if err := targetFile.Close(); err != nil {
			_ = os.RemoveAll(tempDir)
			return "", "", nil, fmt.Errorf("close extracted nornicdb binary: %w", err)
		}
		extractedPath = targetPath
		extractedName = name
		if name == managedBinaryName {
			break
		}
	}
	if extractedPath == "" {
		_ = os.RemoveAll(tempDir)
		return "", "", nil, fmt.Errorf("nornicdb archive %q did not contain a usable NornicDB binary", archivePath)
	}
	return extractedPath, extractedName, func() error { return os.RemoveAll(tempDir) }, nil
}

func looksLikeArchive(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar")
}

func looksLikePackage(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".pkg")
}
