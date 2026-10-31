// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed nornicdb_release_manifest.json
var embeddedReleaseManifest []byte

// pinnedReleaseManifest is the decoded shape of the embedded
// nornicdb_release_manifest.json: the NornicDB release assets pinned per CLI
// version, OS, and architecture. The manifest ships with zero releases while
// the project tracks the NornicDB main branch, so resolution always falls
// through to its "--from" guidance error -- the no-argument installer stays
// unavailable by recorded policy, exactly as the core seam pins it.
type pinnedReleaseManifest struct {
	Version  int                 `json:"version"`
	Backend  string              `json:"backend"`
	Releases []pinnedReleaseSpec `json:"releases"`
}

type pinnedReleaseSpec struct {
	CLIVersion string               `json:"eshu_version"`
	ReleaseTag string               `json:"release_tag"`
	Assets     []pinnedReleaseAsset `json:"assets"`
}

type pinnedReleaseAsset struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Format   string `json:"format"`
	Headless bool   `json:"headless"`
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
}

// defaultReleaseManifest decodes the embedded manifest. Tests pass their own
// bytes through decodeReleaseManifest so the embedded empty manifest is not
// the only shape ever exercised.
func defaultReleaseManifest() (pinnedReleaseManifest, error) {
	return decodeReleaseManifest(embeddedReleaseManifest)
}

func decodeReleaseManifest(raw []byte) (pinnedReleaseManifest, error) {
	var manifest pinnedReleaseManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return pinnedReleaseManifest{}, fmt.Errorf("decode pinned NornicDB release manifest: %w", err)
	}
	if manifest.Version <= 0 {
		return pinnedReleaseManifest{}, fmt.Errorf("decode pinned NornicDB release manifest: missing version")
	}
	if manifest.Backend != "nornicdb" {
		return pinnedReleaseManifest{}, fmt.Errorf("decode pinned NornicDB release manifest: unexpected backend %q", manifest.Backend)
	}
	return manifest, nil
}

// resolvePinnedReleaseSource maps an empty --from to the pinned release
// asset for this CLI version on this platform. preferHeadless selects the
// headless artifact (the default laptop binary); the full binary resolves
// only through an explicit --from source.
func resolvePinnedReleaseSource(manifest pinnedReleaseManifest, cliVersion, hostOS, hostArch string, preferHeadless bool) (string, string, error) {
	cliVersion = strings.TrimSpace(cliVersion)
	if cliVersion == "" {
		cliVersion = "dev"
	}
	hostOS = strings.TrimSpace(hostOS)
	hostArch = strings.TrimSpace(hostArch)
	for _, release := range manifest.Releases {
		if strings.TrimSpace(release.CLIVersion) != cliVersion {
			continue
		}
		for _, asset := range release.Assets {
			if asset.OS == hostOS && asset.Arch == hostArch && asset.Headless == preferHeadless {
				if strings.TrimSpace(asset.URL) == "" || strings.TrimSpace(asset.SHA256) == "" {
					return "", "", fmt.Errorf("pinned NornicDB release asset for %s/%s is incomplete", hostOS, hostArch)
				}
				return asset.URL, strings.ToLower(strings.TrimSpace(asset.SHA256)), nil
			}
		}
		if preferHeadless {
			return "", "", missingPinnedReleaseAssetError("headless", cliVersion, hostOS, hostArch)
		}
		return "", "", missingPinnedReleaseAssetError("full", cliVersion, hostOS, hostArch)
	}
	if preferHeadless {
		return "", "", missingPinnedReleaseAssetError("headless", cliVersion, hostOS, hostArch)
	}
	return "", "", missingPinnedReleaseAssetError("full", cliVersion, hostOS, hostArch)
}

func missingPinnedReleaseAssetError(kind, cliVersion, hostOS, hostArch string) error {
	sourceHint := "<path-to-nornicdb-headless>"
	if kind == "full" {
		sourceHint = "<path-to-nornicdb>"
	}
	return fmt.Errorf("no embedded %s NornicDB release asset for Eshu %s on %s/%s; Eshu currently tracks the latest NornicDB main branch, so build NornicDB from main and run eshu install nornicdb --from %s", kind, cliVersion, hostOS, hostArch, sourceHint)
}
