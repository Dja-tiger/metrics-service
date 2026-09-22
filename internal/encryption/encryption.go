// Package encryption protects request bodies using RSA-OAEP and AES-256-GCM.
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
)

// Header identifies encrypted request bodies.
const Header = "X-Metrics-Encryption"

// Algorithm identifies the versioned wire format used by agent and server.
const Algorithm = "rsa-oaep-aes256-gcm-v1"

// Encrypt encrypts a body with a fresh AES key protected by the public RSA key.
// The wire format is version byte, RSA-encrypted key, nonce, and GCM ciphertext.
func Encrypt(key *rsa.PublicKey, body []byte) ([]byte, error) {
	if err := validatePublicKey(key); err != nil {
		return nil, err
	}
	sessionKey := make([]byte, 32)
	if _, err := rand.Read(sessionKey); err != nil {
		return nil, fmt.Errorf("generate session key: %w", err)
	}
	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, key, sessionKey, []byte(Algorithm))
	if err != nil {
		return nil, fmt.Errorf("encrypt session key: %w", err)
	}
	gcm, err := newGCM(sessionKey)
	if err != nil {
		return nil, err
	}
	header := append([]byte{1}, wrapped...)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	output := append(header, nonce...)
	return gcm.Seal(output, nonce, body, header), nil
}

// Decrypt authenticates and decrypts an encrypted request body.
func Decrypt(key *rsa.PrivateKey, body []byte) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf("private key is required")
	}
	if err := validatePublicKey(&key.PublicKey); err != nil {
		return nil, err
	}
	headerSize := 1 + key.Size()
	if len(body) < headerSize || body[0] != 1 {
		return nil, fmt.Errorf("invalid encrypted body")
	}
	sessionKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, key, body[1:headerSize], []byte(Algorithm))
	if err != nil {
		return nil, fmt.Errorf("decrypt session key: %w", err)
	}
	if len(sessionKey) != 32 {
		return nil, fmt.Errorf("invalid session key length")
	}
	gcm, err := newGCM(sessionKey)
	if err != nil {
		return nil, err
	}
	if len(body) < headerSize+gcm.NonceSize()+gcm.Overhead() {
		return nil, fmt.Errorf("truncated encrypted body")
	}
	nonceEnd := headerSize + gcm.NonceSize()
	plain, err := gcm.Open(nil, body[headerSize:nonceEnd], body[nonceEnd:], body[:headerSize])
	if err != nil {
		return nil, fmt.Errorf("decrypt body: %w", err)
	}
	return plain, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	return gcm, nil
}

func validatePublicKey(key *rsa.PublicKey) error {
	if key == nil || key.N == nil || key.N.BitLen() < 2048 || key.E < 3 || key.E%2 == 0 {
		return fmt.Errorf("RSA key must have at least 2048 bits and a valid public exponent")
	}
	return nil
}
