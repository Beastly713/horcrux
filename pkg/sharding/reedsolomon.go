package sharding

import (
	"bytes"
	"fmt"

	"github.com/klauspost/reedsolomon"
)

// Shard represents a single fragment of the split file
type Shard struct {
	Index int    // 0-based index
	Data  []byte // The actual binary content (encrypted part)
}

// Splitter handles erasure coding (Reed-Solomon)
type Splitter struct {
	Total     int
	Threshold int
}

func NewSplitter(total, threshold int) (*Splitter, error) {
	if threshold > total {
		return nil, fmt.Errorf("threshold cannot exceed total shards")
	}
	return &Splitter{
		Total:     total,
		Threshold: threshold,
	}, nil
}

// Split takes a contiguous byte slice (encrypted data) and splits it into shards
// using Reed-Solomon erasure coding.
func (s *Splitter) Split(data []byte) ([][]Shard, error) {
	// Create the encoder
	enc, err := reedsolomon.New(s.Threshold, s.Total-s.Threshold)
	if err != nil {
		return nil, err
	}

	// Split the data into equal parts.
	shardsBytes, err := enc.Split(data)
	if err != nil {
		return nil, err
	}

	// Generate parity shards
	if err := enc.Encode(shardsBytes); err != nil {
		return nil, err
	}

	// Wrap in our Shard struct
	result := make([][]Shard, s.Total)
	for i, data := range shardsBytes {
		result[i] = []Shard{
			{Index: i, Data: data},
		}
	}

	return result, nil
}

// Join reverses the Split process.
func (s *Splitter) Join(shards map[int][]byte, originalSize int) ([]byte, error) {
	enc, err := reedsolomon.New(s.Threshold, s.Total-s.Threshold)
	if err != nil {
		return nil, err
	}

	// Prepare the slice for the library.
	reconstructShards := make([][]byte, s.Total)
	validCount := 0

	// Populate the shards we have
	for i := 0; i < s.Total; i++ {
		if data, ok := shards[i]; ok {
			reconstructShards[i] = data
			validCount++
		}
	}

	if validCount < s.Threshold {
		return nil, fmt.Errorf("not enough shards to reconstruct: have %d, need %d", validCount, s.Threshold)
	}

	// Reconstruct the missing data shards
	if err := enc.Reconstruct(reconstructShards); err != nil {
		return nil, fmt.Errorf("reconstruction failed: %w", err)
	}

	// MANUAL JOIN: Concatenate the data shards directly.
	// This avoids ambiguity with the library's Join function when size is unknown.
	var buf bytes.Buffer
	for i := 0; i < s.Threshold; i++ {
		if len(reconstructShards[i]) == 0 {
			return nil, fmt.Errorf("unexpected empty shard at index %d", i)
		}
		buf.Write(reconstructShards[i])
	}

	joined := buf.Bytes()

	// If originalSize is provided (non-zero), we strip the padding.
	// In the Pipeline logic, we pass 0, so we return the full padded data.
	// The Pipeline then uses the Length Prefix to strip it accurately.
	if originalSize > 0 {
		if len(joined) < originalSize {
			return nil, fmt.Errorf("reconstructed data shorter than expected size")
		}
		joined = joined[:originalSize]
	}

	return joined, nil
}