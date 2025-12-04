// Package rtsp builds RTSP URLs for cameras.
package rtsp

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/steipete/camsnap/internal/config"
)

const defaultPort = 554

// BuildURL returns an RTSP URL for a camera, including auth if available.
func BuildURL(cam config.Camera) (string, error) {
	host := cam.Host
	if host == "" {
		return "", fmt.Errorf("host is required")
	}

	if !strings.Contains(host, ":") && cam.Port != 0 {
		host = net.JoinHostPort(host, fmt.Sprintf("%d", cam.Port))
	} else if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, fmt.Sprintf("%d", defaultPort))
	}

	userInfo := ""
	if cam.Username != "" {
		if cam.Password != "" {
			userInfo = url.UserPassword(cam.Username, cam.Password).String()
		} else {
			userInfo = url.User(cam.Username).String()
		}
	}

	proto := cam.Protocol
	if proto == "" {
		proto = "rtsp"
	}

	switch strings.ToLower(proto) {
	case "rtsp", "rtsps":
	default:
		return "", fmt.Errorf("unsupported protocol %q", proto)
	}

	authority := host
	if userInfo != "" {
		authority = userInfo + "@" + host
	}

	path := cam.Path
	if path == "" {
		// Default to /stream1 as the main Tapo stream.
		path = "/stream1"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return fmt.Sprintf("%s://%s%s", proto, authority, path), nil
}
