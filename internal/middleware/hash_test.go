package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Dja-tiger/metrics-service/internal/signature"
)

func TestHashSHA256WithGzip(t *testing.T) {
	key := "secret"
	requestBody := []byte(`{"id":"Alloc","type":"gauge","value":42}`)
	responseBody := []byte(`{"status":"ok"}`)

	handler := Gzip(HashSHA256(key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if !bytes.Equal(body, requestBody) {
			t.Fatalf("unexpected request body: got %q want %q", body, requestBody)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(responseBody)
	})))

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(mustGzip(t, requestBody)))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set(signature.Header, signature.Calculate(requestBody, key))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code mismatch: got %d want %d", rec.Code, http.StatusOK)
	}

	decompressedResponse := mustGunzip(t, rec.Body.Bytes())
	if !bytes.Equal(decompressedResponse, responseBody) {
		t.Fatalf("unexpected response body: got %q want %q", decompressedResponse, responseBody)
	}
	if !signature.Verify(decompressedResponse, key, rec.Header().Get(signature.Header)) {
		t.Fatal("response signature is invalid")
	}
}

func TestHashSHA256RejectsInvalidRequest(t *testing.T) {
	key := "secret"
	handlerCalled := false
	handler := HashSHA256(key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader([]byte("payload")))
	req.Header.Set(signature.Header, "invalid")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code mismatch: got %d want %d", rec.Code, http.StatusBadRequest)
	}
	if handlerCalled {
		t.Fatal("handler must not be called for invalid signature")
	}
	if !signature.Verify(rec.Body.Bytes(), key, rec.Header().Get(signature.Header)) {
		t.Fatal("error response signature is invalid")
	}
}

func TestHashSHA256AllowsUnsignedRequest(t *testing.T) {
	key := "secret"
	responseBody := []byte(`{"status":"ok"}`)

	handler := HashSHA256(key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(responseBody)
	}))

	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader([]byte(`{"id":"Alloc","type":"gauge"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code mismatch: got %d want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected content-type: %q", rec.Header().Get("Content-Type"))
	}
	if !signature.Verify(responseBody, key, rec.Header().Get(signature.Header)) {
		t.Fatal("response signature is invalid")
	}
}

func TestHashSHA256DisabledWithoutKey(t *testing.T) {
	handler := HashSHA256("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/update", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code mismatch: got %d want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get(signature.Header) != "" {
		t.Fatal("signature header must be absent without a key")
	}
}
