---
summary: 'Release checklist for camsnap (GitHub release + Homebrew tap)'
---

# Releasing camsnap

Follow these steps for each release. Title GitHub releases as `camsnap <version>`.

## Checklist
- Update code version in `cmd/camsnap/main.go`.
- Update `CHANGELOG.md` with the new version section.
- Tag the release: `git tag -a v<version> -m "Release <version>"` and push tags after commits.
- Build source archive for Homebrew: `git archive --format=tar.gz --output /tmp/camsnap-<version>.tar.gz v<version>`.
- Build prebuilt macOS ARM64 binary bundle:
  - `GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w -X main.version=<version>" -o /tmp/camsnap-arm64 ./cmd/camsnap`
  - Package: copy binary + README + LICENSE into `/tmp/camsnap-arm64-dist` and `tar -czf /tmp/camsnap-macos-arm64.tar.gz camsnap-arm64-dist`
- Compute checksums: `shasum -a 256 /tmp/camsnap-<version>.tar.gz /tmp/camsnap-macos-arm64.tar.gz`.
- Update `homebrew-tap/Formula/camsnap.rb` to point to the new tag + revision and ensure `ffmpeg` dependency.
- Update tap README with the new version/date if needed.
- Commit and push changes in camsnap and the tap; push tags: `git push origin main --tags` then `git push` in `../homebrew-tap`.
- Create GitHub release for `v<version>`:
  - Title: `camsnap <version>`
  - Body: bullets from `CHANGELOG.md` for that version
  - Assets: attach `/tmp/camsnap-<version>.tar.gz` and `/tmp/camsnap-macos-arm64.tar.gz` with their SHA256 sums in the body
- Verify Homebrew install: `brew update && brew reinstall steipete/tap/camsnap && camsnap --version`.
- Smoke-test CLI: `camsnap --help`, `camsnap discover --info` (should not crash), and a sample `snap` against a known camera if available.
