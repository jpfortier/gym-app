package httputil

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGzip_compressesWhenClientAccepts(t *testing.T) {
	h := Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"foo":"bar"}`))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Error("expected gzip encoding")
	}
	gr, _ := gzip.NewReader(rec.Body)
	body, _ := io.ReadAll(gr)
	if string(body) != `{"foo":"bar"}` {
		t.Error("expected decompressed body")
	}
}

func TestGzip_passesThroughWhenNoAcceptEncoding(t *testing.T) {
	h := Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"foo":"bar"}`))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "" {
		t.Error("expected no content-encoding")
	}
	if !strings.Contains(rec.Body.String(), `{"foo":"bar"}`) {
		t.Error("expected uncompressed body")
	}
}
