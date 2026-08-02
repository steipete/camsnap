# AVFoundation backend

This Darwin/cgo package uses ARC for the Objective-C bridge. The cgo C sources
are compiled as Objective-C so `-fobjc-arc` applies consistently to both the
bridge and generated glue.

Build camsnap with an embedded `Info.plist` and ad-hoc signature from the repository root:

```sh
make build
otool -s __TEXT __info_plist ./camsnap
codesign --verify --verbose ./camsnap
```
