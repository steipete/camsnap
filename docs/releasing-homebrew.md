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

## 2) Bump the version in code
Edit `cmd/camsnap/main.go` and set `var version = "x.y.z"`.

## 3) Tag & push
```sh
git commit -am "release: vX.Y.Z"
git tag vX.Y.Z
git push origin main --tags
```

## 4) Update the Homebrew tap formula
In the tap repo (assumed sibling at `../homebrew-tap`), update `Formula/camsnap.rb`:

1. Set `version "X.Y.Z"`.
2. Point `url` to the new tag source tarball, e.g.:
   ```
   url "https://github.com/steipete/camsnap/archive/refs/tags/vX.Y.Z.tar.gz"
   ```
3. Update `sha256` for that tarball:
   ```sh
   curl -L -o /tmp/camsnap.tar.gz https://github.com/steipete/camsnap/archive/refs/tags/vX.Y.Z.tar.gz
   shasum -a 256 /tmp/camsnap.tar.gz
   ```
   Paste the hash into the formula.
4. Ensure `depends_on "go" => :build` is present and build step uses:
   ```ruby
   system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/camsnap"
   ```

Commit and push in the tap repo:
```sh
git commit -am "camsnap vX.Y.Z"
git push origin main
```

## 5) Sanity-check install from tap
```sh
brew uninstall camsnap || true
brew untap steipete/tap || true
brew tap steipete/tap
brew install steipete/tap/camsnap
brew test steipete/tap/camsnap
camsnap --version
```

## 6) Announce
- Create GitHub Release for tag `vX.Y.Z` (link changelog).
- Optionally post in team channel with upgrade command: `brew update && brew upgrade steipete/tap/camsnap`.

## Notes
- We build from source in the formula; no binary assets required.
- Keep the tap formula small: version, url, sha256, license, dependencies, `std_go_args`.
