package httputil

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
)

// ETag wraps a handler to support conditional requests (304 Not Modified).
// It buffers the response, computes an ETag from the body hash, and returns
// 304 when the client sends a matching If-None-Match header.
// Only applies to successful (2xx) JSON responses.
func ETag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		buf := &bytes.Buffer{}
		rw := &etagResponseWriter{
			ResponseWriter: w,
			body:           buf,
			status:         http.StatusOK,
		}
		next.ServeHTTP(rw, r)
		if rw.status < 200 || rw.status >= 300 {
			for k, v := range rw.header {
				for _, vv := range v {
					w.Header().Add(k, vv)
				}
			}
			w.WriteHeader(rw.status)
			_, _ = w.Write(buf.Bytes())
			return
		}
		body := buf.Bytes()
		etag := `"` + base64.RawURLEncoding.EncodeToString(sha256Hash(body))[:22] + `"`
		for k, v := range rw.header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
		if match := r.Header.Get("If-None-Match"); match != "" && etagMatch(match, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.WriteHeader(rw.status)
		_, _ = w.Write(body)
	})
}

func sha256Hash(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

func etagMatch(ifNoneMatch, etag string) bool {
	for _, s := range strings.Split(ifNoneMatch, ",") {
		s = strings.TrimSpace(s)
		if s == "*" || s == etag || s == "W/"+etag {
			return true
		}
	}
	return false
}

type etagResponseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	header     http.Header
	status     int
	wroteHeader bool
}

func (e *etagResponseWriter) Header() http.Header {
	if e.header == nil {
		e.header = make(http.Header)
	}
	return e.header
}

func (e *etagResponseWriter) WriteHeader(code int) {
	if e.wroteHeader {
		return
	}
	e.wroteHeader = true
	e.status = code
}

func (e *etagResponseWriter) Write(b []byte) (int, error) {
	return e.body.Write(b)
}
