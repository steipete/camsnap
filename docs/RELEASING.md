---
summary: 'Release checklist for camsnap (GitHub release + Homebrew tap)'
---

# Releasing camsnap

Follow these steps for each release. Title GitHub releases as `camsnap <version>`.

## Checklist
- The binary version is injected from the git tag via GoReleaser ldflags (`main.version` stays `"dev"` in code); no code version bump is needed.
- Update `CHANGELOG.md` with the new version section; mirror the version in `package.json`.
- Land the release-preparation PR after exact-head CI passes. Dispatch `gh workflow run release.yml --ref main -f tag=v<version> -f dry_run=true` on that merge commit and require success.
- Download `dry-run-archives` from that run, verify its checksums, and inspect both extracted Darwin binaries with `otool -arch all -l`; each `LC_BUILD_VERSION` must show `minos 14.0` before publication.
- Create an SSH-signed annotated `v<version>` tag at the verified merge commit using the fleet release key, never the local default key. Verify it with `git -c gpg.format=ssh -c gpg.ssh.allowedSignersFile=.github/release-allowed-signers verify-tag v<version>` before pushing; stop on any failure. Never move or delete an existing version tag.
- If a tag's release run fails, workflow/config fixes on `main` can be applied with `gh workflow run release.yml --ref main -f tag=v<version> -f dry_run=false`; the workflow uses `main`'s GoReleaser config against the tag's source. Signing scripts come from the tag, so script fixes require a new patch release containing the fix.
- GoReleaser builds target-specific macOS, Linux, and Windows archives plus `checksums.txt`.
- Darwin builds explicitly target macOS 14.0, the minimum required by the native AVFoundation device APIs. Verify both architectures with `otool -l camsnap` (`LC_BUILD_VERSION` must show `minos 14.0`) and `codesign --verify --strict camsnap`; retain successful notarization results from the signing job.
- Docker builds receive the release version through `VERSION`; the release workflow verifies the published image's `--version`. Local Docker builds default to `dev` unless `--build-arg VERSION=<version>` is supplied.
- Darwin release binaries embed the camera usage-description plist and are always signed. CI uses an ad-hoc signature when Developer ID secrets are absent.
- Developer ID signing uses `MACOS_SIGN_P12_BASE64`, `MACOS_SIGN_P12_PASSWORD`, and `MACOS_SIGN_IDENTITY`.
- The signing hook validates Apple's Developer ID G2 intermediate by subject, issuer, and SHA-256 fingerprint before importing it, adds its temporary keychain to the user search list, and checks for the configured valid identity before signing. Cleanup restores the original search list. Release builds run serially so signing hooks cannot race over that shared list.
- Optional App Store Connect notarization uses `APP_STORE_CONNECT_KEY_ID`, `APP_STORE_CONNECT_ISSUER_ID`, and `APP_STORE_CONNECT_API_KEY_P8`.
- Confirm `update-homebrew-tap` finished. It dispatches `update-formula.yml` in `steipete/homebrew-tap` with `artifact_template={formula}_{version}_{target}.tar.gz`.
- Verify the tap formula contains matching URLs and checksums for `darwin_amd64`, `darwin_arm64`, `linux_amd64`, and `linux_arm64`.
- Update tap README with the new version/date if needed.
- Push only the verified release tag: `git push origin refs/tags/v<version>`. The tag starts the GitHub Actions release workflow. Keep the next patch version as an empty `Unreleased` section at the top of the changelog.
- GoReleaser creates the GitHub release titled `camsnap <version>` with an empty body. Fill in the body:
  - Bullets from `CHANGELOG.md` for that version plus a note to use `checksums.txt`
  - `gh release edit v<version> --notes-file <file>`
- Verify Homebrew install (one-line tap+install): `git -C "$(brew --repo steipete/tap)" pull --ff-only && brew upgrade steipete/tap/camsnap && brew test steipete/tap/camsnap && camsnap --version`.
- Smoke-test CLI: `camsnap --help`, `camsnap discover --info` (should not crash), and a sample `snap` against a known camera if available.
