package shamir

import (
	"bytes"
	"testing"
)

func TestSplitAndCombine(t *testing.T) {
	secret := []byte("I solemnly swear that I am up to no good")
	n := 5
	k := 3

	// 1. Split
	shares, err := Split(secret, n, k)
	if err != nil {
		t.Fatalf("Failed to split: %v", err)
	}
	if len(shares) != n {
		t.Errorf("Expected %d shares, got %d", n, len(shares))
	}

	// 2. Combine with Exact Threshold (k=3)
	subset := shares[:k]
	reconstructed, err := Combine(subset)
	if err != nil {
		t.Fatalf("Failed to combine: %v", err)
	}
	if !bytes.Equal(secret, reconstructed) {
		t.Errorf("Reconstructed secret mismatch.\nExpected: %s\nGot: %s", secret, reconstructed)
	}

	// 3. Combine with All Shares
	reconstructedAll, err := Combine(shares)
	if err != nil {
		t.Fatalf("Failed to combine all: %v", err)
	}
	if !bytes.Equal(secret, reconstructedAll) {
		t.Error("Failed to reconstruct with all shares")
	}

	// 4. Fail with Less than Threshold (k-1)
	// Theoretically, this should produce garbage data, not the original secret.
	wrongResult, _ := Combine(shares[:k-1])
	if bytes.Equal(secret, wrongResult) {
		t.Error("Security failure: Reconstructed secret with less than threshold shares")
	}
}