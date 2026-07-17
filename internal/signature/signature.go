package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// Header is the HTTP header used to pass SHA256 request and response signatures.
const Header = "HashSHA256"

// Calculate returns a hex-encoded HMAC-SHA256 signature for data and key.
func Calculate(data []byte, key string) string {
	return hex.EncodeToString(calculate(data, key))
}

// Verify reports whether expected matches the HMAC-SHA256 signature for data and key.
func Verify(data []byte, key, expected string) bool {
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}
	return hmac.Equal(calculate(data, key), expectedBytes)
}

func calculate(data []byte, key string) []byte {
	hash := hmac.New(sha256.New, []byte(key))
	_, _ = hash.Write(data)
	return hash.Sum(nil)
}
