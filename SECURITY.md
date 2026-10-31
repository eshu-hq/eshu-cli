# Security Policy

## Supported Versions

Security fixes target the latest release only.

| Version | Supported |
| ------- | --------- |
| Latest  | Yes       |
| Older   | No        |

The first versioned release is `v0.1.0` (Refs `eshu-hq/eshu#9738`). Before
that tag, the tree is pre-release with no cutover authority: report issues
against `main` and name the commit SHA.

## Reporting a Vulnerability

Do not open a public GitHub issue for security problems.

Instead:

1. Use [GitHub private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability) if enabled for this repository.
2. If private reporting is unavailable, contact the maintainers privately before disclosure.
3. Include the impact, reproduction steps, affected components, and any suggested mitigation.

Every tag-triggered release runs `govulncheck ./...` in the `vuln` job and
attaches the report (`govulncheck.txt`) plus its SHA256 to the GitHub
Release, so a published artifact always names the scan it passed. A
`govulncheck` failure blocks the release. Weekly scheduled CI re-runs the
scan on `main`.

## Disclosure Guidelines

- Do not publicly disclose an issue before a fix is released.
- Avoid testing in ways that could harm users, systems, or data.
- Report in good faith with enough detail to reproduce the issue.

## Operational Notes

When running eshu-cli:

- Keep dependencies up to date (`go mod tidy`, Dependabot actions-only scope).
- Never commit credentials or `.env` files. The `~/.eshu/.env` (0600)
  filesystem contract the CLI reads is local-only.
- The `install nornicdb` family refuses remote `--from` sources without an
  independently obtained `--sha256`; checksum mismatch fails closed.

## Maintainer

Eshu is maintained by Allen Sanabria. GitHub: [@linuxdynasty](https://github.com/linuxdynasty)
