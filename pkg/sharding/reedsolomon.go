package sharding

import (
	"bytes"
	"fmt"
	"io"

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
	// Data shards = Threshold (T)
	// Parity shards = Total (N) - Threshold (T)
	enc, err := reedsolomon.New(s.Threshold, s.Total-s.Threshold)
	if err != nil {
		return nil, err
	}

	// Split the data into equal parts. The library handles padding if necessary.
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

// Join reverses the Split process. It requires a map where key=index, value=data.
// It can reconstruct the original data as long as len(shards) >= Threshold.
func (s *Splitter) Join(shards map[int][]byte, originalSize int) ([]byte, error) {
	enc, err := reedsolomon.New(s.Threshold, s.Total-s.Threshold)
	if err != nil {
		return nil, err
	}

	// Prepare the slice for the library.
	// We need a slice of size Total, where missing shards are nil.
	reconstructShards := make([][]byte, s.Total)
	validCount := 0

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
	// Note: We verify=true to ensure integrity of the data we DO have
	if err := enc.Reconstruct(reconstructShards); err != nil {
		return nil, fmt.Errorf("reconstruction failed: %w", err)
	}

	// Join the data shards back into one buffer
	var buf bytes.Buffer
	if err := enc.Join(&buf, reconstructShards, originalSize); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}