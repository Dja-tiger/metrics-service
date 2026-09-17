package middleware

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Dja-tiger/metrics-service/internal/encryption"
)

func TestDecrypt(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"id":"Alloc"}`)
	encrypted, err := encryption.Encrypt(&key.PublicKey, body)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := bytes.Clone(encrypted)
	corrupt[len(corrupt)-1] ^= 1
	for _, tc := range []struct {
		name, header string
		key          *rsa.PrivateKey
		body         []byte
		status       int
	}{
		{"encrypted", encryption.Algorithm, key, encrypted, 200},
		{"legacy", "", key, body, 200},
		{"disabled", "", nil, body, 200},
		{"missing key", encryption.Algorithm, nil, encrypted, 400},
		{"unknown algorithm", "unknown", key, encrypted, 400},
		{"corrupted", encryption.Algorithm, key, corrupt, 400},
		{"empty", encryption.Algorithm, key, nil, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := Decrypt(tc.key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				got, err := io.ReadAll(r.Body)
				if err != nil || !bytes.Equal(got, body) {
					t.Errorf("body mismatch: %v", err)
				}
				w.WriteHeader(http.StatusOK)
			}))
			r := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(tc.body))
			r.Header.Set(encryption.Header, tc.header)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || called != (tc.status == 200) {
				t.Fatalf("status=%d called=%v", w.Code, called)
			}
		})
	}
}
