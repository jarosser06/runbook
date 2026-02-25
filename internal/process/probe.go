package process

import (
	"context"
	"net/http"
	"time"
)

// ProbeHTTP returns true if addr responds to an HTTP GET request within 5 seconds.
// Any HTTP response (even 404/405) means the server is listening.
func ProbeHTTP(addr string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", addr, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}
