package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestETag_returns304WhenIfNoneMatchMatches(t *testing.T) {
	h := ETag(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Error("expected 304, got", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Error("expected empty body for 304")
	}
}

func TestETag_returns200WhenIfNoneMatchDoesNotMatch(t *testing.T) {
	h := ETag(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", `"wrong-etag"`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Error("expected 200")
	}
	if rec.Body.String() != `{"id":1}` {
		t.Error("expected full body")
	}
}

func TestETag_passesThroughNonGet(t *testing.T) {
	h := ETag(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Error("expected 201")
	}
	if rec.Header().Get("ETag") != "" {
		t.Error("expected no ETag for POST")
	}
}
