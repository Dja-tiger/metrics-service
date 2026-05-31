package middleware

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGzipRequestDecompression(t *testing.T) {
	body := []byte(`{"id":"Alloc","type":"gauge","value":12.34}`)
	compressedBody := mustGzip(t, body)

	handler := Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		if payload["id"] != "Alloc" {
			t.Fatalf("unexpected payload id: %#v", payload["id"])
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(compressedBody))
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code mismatch: got %d want %d", rec.Code, http.StatusOK)
	}
}

func TestGzipResponseCompressionJSON(t *testing.T) {
	handler := Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/value", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip content encoding, got %s", rec.Header().Get("Content-Encoding"))
	}

	decompressed := mustGunzip(t, rec.Body.Bytes())
	if string(decompressed) != `{"status":"ok"}` {
		t.Fatalf("unexpected response body: %s", string(decompressed))
	}
}

func TestGzipResponseCompressionHTML(t *testing.T) {
	handler := Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip content encoding, got %s", rec.Header().Get("Content-Encoding"))
	}

	decompressed := mustGunzip(t, rec.Body.Bytes())
	if string(decompressed) != "<html><body>ok</body></html>" {
		t.Fatalf("unexpected response body: %s", string(decompressed))
	}
}

func TestGzipResponseSkipUnsupportedContentType(t *testing.T) {
	handler := Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/Alloc", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("did not expect content encoding for text/plain, got %s", rec.Header().Get("Content-Encoding"))
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func mustGzip(t *testing.T, body []byte) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(body); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buffer.Bytes()
}

func mustGunzip(t *testing.T, body []byte) []byte {
	t.Helper()

	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gunzip: %v", err)
	}
	return data
}
