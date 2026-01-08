package secrets

import (
	"crypto/rand"
	"fmt"
)

// Secret wraps a byte slice that contains sensitive data (e.g., encryption keys).
// It provides a mechanism to zero out the memory when no longer needed.
type Secret struct {
	data []byte
}

// NewSecret generates a cryptographically secure random key of the specified size.
func NewSecret(size int) (*Secret, error) {
	key := make([]byte, size)
	_, err := rand.Read(key)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random secret: %w", err)
	}
	return &Secret{data: key}, nil
}

// WrapSecret creates a Secret from an existing byte slice.
// WARNING: The original slice is still accessible; use this only when necessary.
func WrapSecret(data []byte) *Secret {
	return &Secret{data: data}
}

// Bytes returns the raw bytes of the secret.
// Use with caution and ensure the Secret is destroyed after use.
func (s *Secret) Bytes() []byte {
	return s.data
}

// Destroy overwrites the secret data with zeros to prevent memory leaks.
// It is idempotent.
func (s *Secret) Destroy() {
	if s.data != nil {
		for i := range s.data {
			s.data[i] = 0
		}
		s.data = nil
	}
}