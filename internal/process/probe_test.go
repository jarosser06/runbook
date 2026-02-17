package process

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbeHTTPSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if !ProbeHTTP(srv.URL) {
		t.Errorf("ProbeHTTP(%s) = false, want true (server is up)", srv.URL)
	}
}

func TestProbeHTTPNotFound(t *testing.T) {
	// A 404 response still means the port is listening, so probe should return true.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if !ProbeHTTP(srv.URL) {
		t.Errorf("ProbeHTTP(%s) = false for 404, want true (server is listening)", srv.URL)
	}
}

func TestProbeHTTPNoServer(t *testing.T) {
	// Nothing listening on this port.
	if ProbeHTTP("http://127.0.0.1:19743") {
		t.Error("ProbeHTTP = true for unbound port, want false")
	}
}
