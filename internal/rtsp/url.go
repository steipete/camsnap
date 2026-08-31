// Package rtsp builds RTSP URLs for cameras.
package rtsp

import (
	"fmt"
	"net"
	"net/netip"
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

	// Released configs can contain bracketed authorities with URI-escaped
	// scopes. Decode those once; bare address literals always use raw scopes.
	if strings.HasPrefix(host, "[") {
		if parsed, err := url.Parse("rtsp://" + host); err == nil {
			if addr, err := netip.ParseAddr(parsed.Hostname()); err == nil && addr.Zone() != "" {
				host = parsed.Host
			}
		}
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}
	// Address colons and scope IDs do not mean the host already has a port.
	if _, err := netip.ParseAddr(host); err == nil || !strings.Contains(host, ":") {
		port := cam.Port
		if port == 0 {
			port = defaultPort
		}
		host = net.JoinHostPort(host, fmt.Sprintf("%d", port))
	}

	var user *url.Userinfo
	if cam.Username != "" {
		if cam.Password != "" {
			user = url.UserPassword(cam.Username, cam.Password)
		} else {
			user = url.User(cam.Username)
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

	path := cam.Path
	if path == "" {
		// Default to /stream1 as the main Tapo stream.
		path = "/stream1"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Escape the authority's raw IPv6 scope, but preserve released custom-path
	// bytes, including query separators and existing percent escapes.
	return (&url.URL{
		Scheme: proto,
		User:   user,
		Host:   host,
	}).String() + path, nil
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
