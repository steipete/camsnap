# RTSP camera setup

camsnap stores connection defaults per camera so capture commands can stay short. Command flags override saved values when you need a one-off change.

## Connection controls

The main compatibility controls are:

| Setting | Values | Purpose |
| --- | --- | --- |
| `--rtsp-transport` | `tcp`, `udp` | Select the RTSP transport. |
| `--stream` | `stream1`, `stream2` | Select a conventional stream path. |
| `--rtsp-client` | `ffmpeg`, `gortsplib` | Select the snapshot client. |
| `--rtsp-auth` | `auto`, `basic`, `digest` | Override authentication preference. |
| `--path` | camera-specific path | Replace the conventional stream path. |
| `--no-audio` | flag | Remove audio from clips. |
| `--audio-codec` | for example, `aac` | Transcode clip audio. |

Save recurring controls with the camera:

```sh
camsnap add --name kitchen --host 192.168.1.50 \
  --user camera --pass 'camera-password' \
  --rtsp-transport udp --stream stream2 --rtsp-client gortsplib
```

Use the same controls directly with `snap`, `clip`, and `watch` to override a saved camera.

## Tokenized stream paths

Custom paths retain query strings and percent escapes. Quote paths containing shell characters, for example `--path '/cam/realmonitor?channel=1&subtype=0'`.

Some cameras use an opaque RTSP path instead of `stream1` or `stream2`. Save the path from a tokenized RTSP URL, such as a UniFi Protect stream, with `--path`:

```sh
camsnap add --name living-room --host 192.168.1.1 --port 7447 \
  --protocol rtsp --path Bfy47SNWz9n2WRrw
camsnap snap living-room --out living-room.jpg
```

The command-specific `--path` flag overrides the stored path:

```sh
camsnap snap living-room --path Bfy47SNWz9n2WRrw --out living-room.jpg
camsnap clip living-room --path Bfy47SNWz9n2WRrw --dur 5s \
  --out living-room.mp4
```

Increase `--timeout` when a stream needs longer to deliver its first keyframe.

## Diagnose a camera

`doctor` checks whether ffmpeg is available, tests each saved camera's network endpoint, and can run a short ffmpeg probe:

```sh
camsnap doctor --probe --rtsp-transport udp
```

Capture failures redact RTSP credentials before printing ffmpeg diagnostics.

## IPv6 hosts and discovery

`discover` suggestions omit the ONVIF service port because it is independent of the RTSP port. `add` defaults to RTSP port 554; pass `--port` when the camera uses a different one.

Use a bare IPv6 address with a separate port, such as `--host '2001:db8::1' --port 8554`. Link-local addresses need the local interface scope, for example `--host 'fe80::1%en0'`. Store the raw scope delimiter `%` in `--host`; camsnap escapes it when building the RTSP URL.

Previously saved bracketed authorities with URI-escaped scopes, such as `[fe80::1%25en0]:8554`, retain their meaning. Prefer bare addresses with a separate `--port` for new entries, including numeric scope IDs.
