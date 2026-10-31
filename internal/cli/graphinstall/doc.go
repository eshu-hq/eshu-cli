// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package graphinstall verifies and installs a local NornicDB binary for
// `eshu install nornicdb`.
//
// This is destination-authored code, not a moved copy: the core
// implementation (`eshu/go/internal/cli/graphinstall`) closes over the
// `query` root (for one backend-name constant) and `eshulocal` (for home
// resolution), and both packages fail the destination census by
// construction, so a byte-identical move is impossible. The behavior
// contract is pinned instead: the managed-home filesystem seam, the
// fail-closed source policy, the version-probe protocol, and every
// user-visible message are recorded in the #8874 contract record
// (eshu#9484, "Install filesystem and source contract") and in
// docs/install-pin.md, and the parity gates compare the installed CLI
// against the core binary behavior instead of diffing sources.
//
// Install resolves a source -- a local binary, a local archive/package, a
// download URL, or (when From is empty) the pinned release manifest for the
// running CLI version -- verifies its checksum and reported version, and
// copies it into the CLI's managed home (<home>/bin/nornicdb-headless and
// <home>/graph-backends/nornicdb/manifest.json), writing an install
// manifest alongside it. An install that already matches the managed
// binary's version and checksum is reported as reused rather than
// rewritten, unless Options.Force is set.
//
// The package reads no cobra flags. The host state it does touch is
// install-scoped rather than process wiring, and this is the complete list.
//
// Deliberate environment reads: ESHU_HOME selects the managed home root
// (verbatim, leading ~ expanded, no "eshu" segment appended); XDG_DATA_HOME
// (Linux), LOCALAPPDATA (Windows), and HOME/USERPROFILE (via os.UserHomeDir)
// resolve the platform default; ESHU_NORNICDB_INSTALL_TIMEOUT bounds an
// install-source download. Three stdlib boundaries also consult the
// environment: the download's http.Client leaves Transport nil, so
// http.DefaultTransport honours HTTP_PROXY/HTTPS_PROXY/NO_PROXY;
// os.MkdirTemp reads TMPDIR; exec.Command resolves through PATH.
//
// graphinstall never executes the candidate NornicDB binary itself.
// Verifying that a candidate really is NornicDB requires running
// `<binary> version`, and the caller threads its VersionReader
// implementation through Options.ReadVersion instead; a nil reader is
// rejected before any work. The package does run one other subprocess:
// exec.Command("pkgutil", "--expand-full", ...) to expand a darwin .pkg
// install source. That is archive extraction, not version verification.
package graphinstall
