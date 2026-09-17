package config

import "fmt"

// The current gRPC transport cannot honor HTTP signing or RSA encryption.
// Reject mixed settings instead of silently sending configured secrets in plaintext.
func validateGRPCSecurity(address, key, cryptoKey string) error {
	if address != "" && (key != "" || cryptoKey != "") {
		return fmt.Errorf("gRPC does not support KEY/-k or CRYPTO_KEY/-crypto-key; use HTTP for signed/encrypted requests or explicitly clear these settings")
	}
	return nil
}
