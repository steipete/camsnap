# Docker

The camsnap container includes ffmpeg and is published for `linux/amd64` and `linux/arm64`.

```sh
docker run --rm ghcr.io/steipete/camsnap --help
```

Mount one volume for persistent configuration and another for output files:

```sh
docker run --rm \
  -v camsnap-config:/config \
  -v "$PWD":/output \
  ghcr.io/steipete/camsnap snap kitchen --out kitchen.jpg
```

The image sets `XDG_CONFIG_HOME=/config`, runs as an unprivileged user, and writes relative output paths under `/output`. Add a camera through the same mounted configuration volume before capturing:

```sh
docker run --rm -v camsnap-config:/config ghcr.io/steipete/camsnap \
  add --name kitchen --host 192.168.1.50 \
  --user camera --pass 'camera-password'
```

The container supports network cameras only; it does not expose direct local-webcam capture.
