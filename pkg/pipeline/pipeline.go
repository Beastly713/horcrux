package pipeline

import (
	"fmt"
	"io"

	"github.com/Beastly713/horcrux/pkg/compression"
	"github.com/Beastly713/horcrux/pkg/crypto/encryptor"
	"github.com/Beastly713/horcrux/pkg/sharding"
)

// PipelineConfig holds the parameters for the split operation
type PipelineConfig struct {
	Total     int
	Threshold int
}

// SplitPipeline orchestrates the flow: Read -> Compress -> Encrypt -> Shard
// NOTE: Return type is now []sharding.Shard (single list of shards), not [][]sharding.Shard
func SplitPipeline(input io.Reader, key []byte, config PipelineConfig) ([]sharding.Shard, error) {
	// 1. Read Input
	plainBytes, err := io.ReadAll(input)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	// 2. Compress
	compressor := compression.NewGzipCompressor()
	compressedBytes, err := compressor.Compress(plainBytes)
	if err != nil {
		return nil, fmt.Errorf("compression failed: %w", err)
	}

	// 3. Encrypt (Authenticated AES-GCM)
	// NOTE: Use package-level Encrypt function. 
	// Note argument order in your encryptor.go: (plaintext, key)
	cipherText, err := encryptor.Encrypt(compressedBytes, key)
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	// 4. Shard (Reed-Solomon)
	splitter, err := sharding.NewSplitter(config.Total, config.Threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize splitter: %w", err)
	}

	shards, err := splitter.Split(cipherText)
	if err != nil {
		return nil, fmt.Errorf("sharding failed: %w", err)
	}

	// Flatten the slice of slice of shards
	flatShards := make([]sharding.Shard, len(shards))
	for i, s := range shards {
		flatShards[i] = s[0]
	}

	return flatShards, nil
}

// JoinPipeline orchestrates the reverse: Unshard -> Decrypt -> Decompress
func JoinPipeline(shards map[int][]byte, key []byte, total, threshold int) ([]byte, error) {
	// 1. Unshard (Reed-Solomon Join)
	splitter, err := sharding.NewSplitter(total, threshold)
	if err != nil {
		return nil, err
	}

	// We pass 0 as size for now (addressed in next Phase)
	joinedBytes, err := splitter.Join(shards, 0)
	if err != nil {
		return nil, fmt.Errorf("reconstruction failed: %w", err)
	}

	// 2. Decrypt
	// CHANGED: Use package-level Decrypt function. Argument order: (ciphertext, key)
	decryptedBytes, err := encryptor.Decrypt(joinedBytes, key)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (integrity check): %w", err)
	}

	// 3. Decompress
	compressor := compression.NewGzipCompressor()
	plainBytes, err := compressor.Decompress(decryptedBytes)
	if err != nil {
		return nil, fmt.Errorf("decompression failed: %w", err)
	}

	return plainBytes, nil
}