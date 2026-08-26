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

	// A bare IP literal (v4 or v6) always needs JoinHostPort -- IPv6
	// literals contain internal colons (e.g. "fe80::1"), which the old
	// `!strings.Contains(host, ":")` check mistook for an already-complete
	// "host:port" (or "[host]:port") authority and left untouched,
	// producing an unbracketed, portless RTSP authority. net.ParseIP
	// distinguishes a bare literal from an authority that already carries
	// a port (net.ParseIP fails on "1.2.3.4:554" and on "[fe80::1]:554"
	// alike, since neither is a valid bare IP string), and
	// net.JoinHostPort brackets IPv6 automatically.
	if net.ParseIP(host) != nil || !strings.Contains(host, ":") {
		port := cam.Port
		if port == 0 {
			port = defaultPort
		}
		host = net.JoinHostPort(host, fmt.Sprintf("%d", port))
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

// ReplacePath replaces the trailing path segment of an RTSP URL.
func ReplacePath(baseURL, path string) string {
	if path == "" {
		return baseURL
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if index := strings.LastIndex(baseURL, "/"); index >= 0 {
		return baseURL[:index] + path
	}
	return baseURL + path
}
