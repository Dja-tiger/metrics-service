package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/Dja-tiger/metrics-service/internal/signature"
)

// HashSHA256 verifies request signatures and signs response bodies when a key is configured.
func HashSHA256(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if key == "" {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			responseWriter := newHashResponseWriter(w)

			if requestRequiresSignature(r.Method) && r.Header.Get(signature.Header) != "" {
				if !verifyRequestSignature(r, key) {
					responseWriter.WriteHeader(http.StatusBadRequest)
					responseWriter.flush(key)
					return
				}
			}

			next.ServeHTTP(responseWriter, r)
			responseWriter.flush(key)
		})
	}
}

func verifyRequestSignature(r *http.Request, key string) bool {
	body, err := io.ReadAll(r.Body)
	closeErr := r.Body.Close()
	if err != nil || closeErr != nil {
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	return signature.Verify(body, key, r.Header.Get(signature.Header))
}

func requestRequiresSignature(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

type hashResponseWriter struct {
	http.ResponseWriter
	body        bytes.Buffer
	statusCode  int
	wroteHeader bool
}

func newHashResponseWriter(w http.ResponseWriter) *hashResponseWriter {
	return &hashResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (w *hashResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.statusCode = statusCode
}

func (w *hashResponseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.statusCode = http.StatusOK
	}
	return w.body.Write(body)
}

func (w *hashResponseWriter) flush(key string) {
	body := w.body.Bytes()
	w.Header().Set(signature.Header, signature.Calculate(body, key))
	w.ResponseWriter.WriteHeader(w.statusCode)
	_, _ = w.ResponseWriter.Write(body)
}
