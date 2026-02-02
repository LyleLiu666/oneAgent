package llm

import "net/http"

const defaultAPIRetryAttempts = 3

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusTooManyRequests:
		return true
	default:
		return status >= 500 && status <= 599
	}
}
