// Package delivery defines identifiers for retry-safe metric batches.
package delivery

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	models "github.com/Dja-tiger/metrics-service/internal/model"
)

// Header is shared by HTTP headers and lowercase gRPC metadata.
const Header = "Idempotency-Key"

// ErrConflict means an ID was reused for a different batch.
var ErrConflict = errors.New("idempotency key reused with different metrics")

// Batch retains its ID and immutable metrics until acknowledged.
type Batch struct {
	ID      string
	Metrics []models.Metrics
}

// New creates an ID independent of metric values and process restarts.
// The caller transfers ownership of metrics to the batch.
func New(metrics []models.Metrics) Batch {
	return Batch{ID: rand.Text(), Metrics: metrics}
}

// ValidKey bounds untrusted identifiers before storage.
func ValidKey(key string) bool {
	if len(key) == 0 || len(key) > 128 {
		return false
	}
	for _, c := range key {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

// Fingerprint compares decoded payloads independently of compression/encryption.
func Fingerprint(metrics []models.Metrics) (string, error) {
	data, err := json.Marshal(metrics)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
