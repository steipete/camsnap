# AVFoundation spike

This Darwin/cgo package uses ARC for the Objective-C bridge. The cgo C sources
are compiled as Objective-C so `-fobjc-arc` applies consistently to both the
bridge and generated glue.

Build the probe with an embedded `Info.plist` from the repository root:

```sh
PLIST_PATH="$(pwd)/internal/avf/Info.plist"
go build -o /tmp/avfprobe \
  -ldflags "-linkmode external -extldflags -Wl,-sectcreate,__TEXT,__info_plist,$PLIST_PATH" \
  ./cmd/avfprobe
otool -s __TEXT __info_plist /tmp/avfprobe
```

For the ad-hoc signing experiment, embed the plist before signing:

```sh
codesign -s - --force /tmp/avfprobe
codesign --verify --verbose /tmp/avfprobe
```
