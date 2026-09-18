package encryption

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// LoadPublicKey reads a PKIX or PKCS#1 PEM RSA public key; an empty path disables encryption.
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	if path == "" {
		return nil, nil
	}
	block, err := readPEM(path)
	if err != nil {
		return nil, err
	}
	var key *rsa.PublicKey
	switch block.Type {
	case "PUBLIC KEY":
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse public key: %w", err)
		}
		var ok bool
		key, ok = parsed.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key must be RSA")
		}
	case "RSA PUBLIC KEY":
		key, err = x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse RSA public key: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported public key PEM type %q", block.Type)
	}
	if err := validatePublicKey(key); err != nil {
		return nil, err
	}
	return key, nil
}

// LoadPrivateKey reads an unencrypted PKCS#8 or PKCS#1 PEM RSA private key.
// An empty path disables decryption.
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	if path == "" {
		return nil, nil
	}
	block, err := readPEM(path)
	if err != nil {
		return nil, err
	}
	var key *rsa.PrivateKey
	switch block.Type {
	case "PRIVATE KEY":
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		var ok bool
		key, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key must be RSA")
		}
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse RSA private key: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported private key PEM type %q", block.Type)
	}
	if err := validatePublicKey(&key.PublicKey); err != nil {
		return nil, err
	}
	if err := key.Validate(); err != nil {
		return nil, fmt.Errorf("validate private key: %w", err)
	}
	key.Precompute()
	return key, nil
}

func readPEM(path string) (*pem.Block, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("key file does not contain PEM data")
	}
	return block, nil
}
