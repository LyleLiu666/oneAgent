package netutil

import (
	"net"
	"strings"
)

// IsLoopbackBind returns true if the bind address is loopback/localhost.
// It treats empty bind as loopback because many configs default to loopback.
func IsLoopbackBind(bind string) bool {
	b := strings.TrimSpace(bind)
	if b == "" {
		return true
	}
	if strings.EqualFold(b, "localhost") {
		return true
	}
	if ip := net.ParseIP(b); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// IsAnyBind returns true if the bind address likely means "all interfaces".
func IsAnyBind(bind string) bool {
	b := strings.TrimSpace(bind)
	if b == "" {
		return false
	}
	if b == "0.0.0.0" || b == "::" {
		return true
	}
	if ip := net.ParseIP(b); ip != nil {
		return ip.IsUnspecified()
	}
	return false
}
