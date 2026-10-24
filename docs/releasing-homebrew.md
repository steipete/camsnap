# camsnap Homebrew Release Playbook

This mirrors the lightweight Homebrew flow we use in other CLIs (e.g., `peekaboo`), but only targets our tap (no npm).

## 0) Prereqs
- macOS with Homebrew installed.
- Clean git working tree on `main`.
- Go toolchain installed (Go version is read from `go.mod`).
- Access to the tap repo (e.g., `steipete/homebrew-tap`).

## 1) Verify build is green
```sh
make fmt
golangci-lint run ./...
go test ./...
```

## 2) Prepare release metadata
Follow [the release checklist](RELEASING.md): update `CHANGELOG.md` and mirror the version in `package.json`. Leave `main.version` as `"dev"`; GoReleaser injects the tagged version through ldflags.

## 3) Tag & push
```sh
# First land the reviewed release preparation and pass the release dry run.
# Sign vX.Y.Z at that verified commit with the fleet release key.
git -c gpg.format=ssh -c gpg.ssh.allowedSignersFile=.github/release-allowed-signers verify-tag vX.Y.Z
git push origin refs/tags/vX.Y.Z
```

## 4) Verify the Homebrew tap formula
The release workflow dispatches `update-formula.yml` in `steipete/homebrew-tap` with:
```sh
artifact_template='{formula}_{version}_{target}.tar.gz'
```

Confirm the workflow succeeds and `Formula/camsnap.rb` contains matching URLs and checksums for:
- `darwin_amd64`
- `darwin_arm64`
- `linux_amd64`
- `linux_arm64`

If the automatic dispatch fails, rerun `update-formula.yml` with `formula=camsnap`, `tag=vX.Y.Z`, `repository=steipete/camsnap`, and the artifact template above.

## 5) Sanity-check install from tap
```sh
brew tap steipete/tap
git -C "$(brew --repo steipete/tap)" pull --ff-only
brew upgrade steipete/tap/camsnap # use brew install if not installed
brew test steipete/tap/camsnap
camsnap --version
```

## 6) Announce
- GoReleaser creates `camsnap X.Y.Z` for tag `vX.Y.Z`; fill its body with that version's changelog bullets and a note to verify downloads against `checksums.txt`.
- Optionally post in team channel with upgrade command: `brew update && brew upgrade steipete/tap/camsnap`.

## Notes
- Release automation updates the tap from the matching GoReleaser asset for each supported macOS and Linux architecture.
- Keep the tap formula small: version, url, sha256, license, dependencies.
