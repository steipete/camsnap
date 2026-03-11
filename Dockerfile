FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /camsnap ./cmd/camsnap

FROM alpine:3.23
RUN apk add --no-cache ffmpeg
RUN adduser -D -h /home/camsnap camsnap \
 && mkdir -p /config /output \
 && chown camsnap:camsnap /config /output
USER camsnap
ENV XDG_CONFIG_HOME=/config
VOLUME ["/config", "/output"]
WORKDIR /output
COPY --from=build /camsnap /usr/local/bin/camsnap
ENTRYPOINT ["camsnap"]
