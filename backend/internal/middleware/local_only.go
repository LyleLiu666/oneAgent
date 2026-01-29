package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequireLoopback blocks requests that do not originate from a loopback address,
// unless allowRemote is true.
func RequireLoopback(allowRemote bool, feature string, allowRemoteHint string) gin.HandlerFunc {
	feature = strings.TrimSpace(feature)
	if feature == "" {
		feature = "endpoint"
	}
	allowRemoteHint = strings.TrimSpace(allowRemoteHint)
	if allowRemoteHint == "" {
		allowRemoteHint = "ALLOW_REMOTE"
	}

	return func(c *gin.Context) {
		if allowRemote {
			c.Next()
			return
		}

		host := strings.TrimSpace(c.Request.RemoteAddr)
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		ip := net.ParseIP(strings.TrimSpace(host))
		if ip != nil && ip.IsLoopback() {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": fmt.Sprintf("%s is local-only by default (set %s=1 to allow remote access)", feature, allowRemoteHint),
		})
	}
}

