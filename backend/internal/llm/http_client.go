package llm

import (
	"net"
	"net/http"
	"time"
)

func newHTTPClient(timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// Important: For streaming (SSE) requests, http.Client.Timeout would enforce a whole-request
		// deadline and will terminate long-running streams with:
		//   context deadline exceeded (Client.Timeout or context cancellation while reading body)
		// Instead, cap only the time to receive response headers.
		ResponseHeaderTimeout: timeout,
	}

	return &http.Client{
		Transport: transport,
	}
}
