package middleware

import (
	"bytes"
	"crypto/rsa"
	"io"
	"net/http"

	"github.com/Dja-tiger/metrics-service/internal/encryption"
)

// Decrypt decrypts marked request bodies before gzip and signature processing.
// Unmarked requests remain supported for compatibility with existing clients.
func Decrypt(key *rsa.PrivateKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			algorithm := r.Header.Get(encryption.Header)
			if algorithm == "" {
				next.ServeHTTP(w, r)
				return
			}
			if algorithm != encryption.Algorithm || key == nil {
				http.Error(w, "invalid encrypted request", http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
			_ = r.Body.Close()
			if err != nil {
				http.Error(w, "invalid encrypted request", http.StatusBadRequest)
				return
			}
			plain, err := encryption.Decrypt(key, body)
			if err != nil {
				http.Error(w, "invalid encrypted request", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(plain))
			r.ContentLength = int64(len(plain))
			r.Header.Del("Content-Length")
			r.Header.Del(encryption.Header)
			next.ServeHTTP(w, r)
		})
	}
}
