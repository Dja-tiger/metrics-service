package encryption

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestEncryption(t *testing.T) {
	key := testKey(t)
	wrongKey := testKey(t)
	for _, body := range [][]byte{nil, []byte("metrics"), bytes.Repeat([]byte("large batch"), 10000)} {
		encoded, err := Encrypt(&key.PublicKey, body)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := Decrypt(key, encoded)
		if err != nil || !bytes.Equal(decoded, body) {
			t.Fatalf("round trip: %v", err)
		}
		second, err := Encrypt(&key.PublicKey, body)
		if err != nil || bytes.Equal(encoded, second) {
			t.Fatal("encryption must be randomized")
		}
		if _, err := Decrypt(wrongKey, encoded); err == nil {
			t.Fatal("wrong key accepted")
		}
		for _, pos := range []int{0, 1, 1 + key.Size(), len(encoded) - 1} {
			corrupt := bytes.Clone(encoded)
			corrupt[pos] ^= 1
			if _, err := Decrypt(key, corrupt); err == nil {
				t.Fatalf("corruption accepted at %d", pos)
			}
		}
		for _, length := range []int{0, 1, key.Size(), 1 + key.Size(), len(encoded) - 1} {
			if _, err := Decrypt(key, encoded[:length]); err == nil {
				t.Fatalf("truncation accepted: %d", length)
			}
		}
	}
	if _, err := Encrypt(nil, nil); err == nil {
		t.Fatal("nil public key accepted")
	}
	if _, err := Decrypt(nil, nil); err == nil {
		t.Fatal("nil private key accepted")
	}
}

func TestKeyFiles(t *testing.T) {
	key := testKey(t)
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		data   []byte
		public bool
	}{
		{"PUBLIC KEY", pubDER, true},
		{"RSA PUBLIC KEY", x509.MarshalPKCS1PublicKey(&key.PublicKey), true},
		{"PRIVATE KEY", privateDER, false},
		{"RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "key.pem")
			if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: tc.name, Bytes: tc.data}), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.public {
				loaded, err := LoadPublicKey(path)
				if err != nil || !loaded.Equal(&key.PublicKey) {
					t.Fatalf("load public: %v", err)
				}
				if _, err := LoadPrivateKey(path); err == nil {
					t.Fatal("public key accepted as private")
				}
			} else {
				loaded, err := LoadPrivateKey(path)
				if err != nil || !loaded.Equal(key) {
					t.Fatalf("load private: %v", err)
				}
				if _, err := LoadPublicKey(path); err == nil {
					t.Fatal("private key accepted as public")
				}
			}
		})
	}
	if key, err := LoadPublicKey(""); key != nil || err != nil {
		t.Fatal("empty path must disable encryption")
	}
	if key, err := LoadPrivateKey(""); key != nil || err != nil {
		t.Fatal("empty path must disable decryption")
	}
	path := filepath.Join(t.TempDir(), "invalid.pem")
	for _, data := range [][]byte{nil, []byte("invalid"), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("invalid")}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("invalid")})} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadPublicKey(path); err == nil {
			t.Fatal("invalid public key accepted")
		}
		if _, err := LoadPrivateKey(path); err == nil {
			t.Fatal("invalid private key accepted")
		}
	}
	if _, err := LoadPublicKey(path + "missing"); err == nil {
		t.Fatal("missing public key accepted")
	}
	if _, err := LoadPrivateKey(path + "missing"); err == nil {
		t.Fatal("missing private key accepted")
	}
}
