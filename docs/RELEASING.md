---
summary: 'Release checklist for camsnap (GitHub release + Homebrew tap)'
---

# Releasing camsnap

Follow these steps for each release. Title GitHub releases as `camsnap <version>`.

## Checklist
- Update code version in `cmd/camsnap/main.go`.
- Update `CHANGELOG.md` with the new version section.
- Tag the release: `git tag -a v<version> -m "Release <version>"` and push tags after commits.
- GoReleaser builds target-specific macOS, Linux, and Windows archives plus `checksums.txt`.
- Darwin release binaries embed the camera usage-description plist and are always signed. CI uses an ad-hoc signature when Developer ID secrets are absent.
- Developer ID signing uses `MACOS_SIGN_P12_BASE64`, `MACOS_SIGN_P12_PASSWORD`, and `MACOS_SIGN_IDENTITY`.
- Optional App Store Connect notarization uses `APP_STORE_CONNECT_KEY_ID`, `APP_STORE_CONNECT_ISSUER_ID`, and `APP_STORE_CONNECT_API_KEY_P8`.
- Confirm `update-homebrew-tap` finished. It dispatches `update-formula.yml` in `steipete/homebrew-tap` with `artifact_template={formula}_{version}_{target}.tar.gz`.
- Verify the tap formula contains matching URLs and checksums for `darwin_amd64`, `darwin_arm64`, `linux_amd64`, and `linux_arm64`.
- Update tap README with the new version/date if needed.
- Commit and push changes in camsnap, then push tags: `git push origin main --tags`.
- Create GitHub release for `v<version>`:
  - Title: `camsnap <version>`
  - Body: bullets from `CHANGELOG.md` for that version plus a note to use `checksums.txt`
- Verify Homebrew install (one-line tap+install): `brew update && brew reinstall steipete/tap/camsnap && camsnap --version`.
- Smoke-test CLI: `camsnap --help`, `camsnap discover --info` (should not crash), and a sample `snap` against a known camera if available.
