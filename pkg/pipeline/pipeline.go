package pipeline

import (
	"encoding/binary"
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

// SplitPipeline orchestrates the flow: Read -> Compress -> Encrypt -> LengthPrefix -> Shard
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
	cipherText, err := encryptor.Encrypt(compressedBytes, key)
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	// 4. Prepend Length (8 bytes)
	// We must store the exact length of the ciphertext to strip padding after reconstruction.
	lengthBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(lengthBuf, uint64(len(cipherText)))
	
	// payload = [Length (8 bytes) | CipherText]
	payload := append(lengthBuf, cipherText...)

	// 5. Shard (Reed-Solomon)
	splitter, err := sharding.NewSplitter(config.Total, config.Threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize splitter: %w", err)
	}

	shards, err := splitter.Split(payload)
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

// JoinPipeline orchestrates the reverse: Unshard -> StripPadding -> Decrypt -> Decompress
func JoinPipeline(shards map[int][]byte, key []byte, total, threshold int) ([]byte, error) {
	// 1. Unshard (Reed-Solomon Join)
	splitter, err := sharding.NewSplitter(total, threshold)
	if err != nil {
		return nil, err
	}

	// Pass 0 as size to recover full data + padding
	joinedBytes, err := splitter.Join(shards, 0)
	if err != nil {
		return nil, fmt.Errorf("reconstruction failed: %w", err)
	}

	// 2. Strip Padding using Prefix Length
	if len(joinedBytes) < 8 {
		return nil, fmt.Errorf("reconstructed data is too short to contain length prefix")
	}

	// Read the original length
	originalLen := binary.LittleEndian.Uint64(joinedBytes[:8])
	
	// Safety check: ensure the buffer actually has enough bytes
	if uint64(len(joinedBytes)-8) < originalLen {
		return nil, fmt.Errorf("reconstructed data is shorter than expected length")
	}

	// Extract the exact ciphertext (Slice: start at 8, end at 8+length)
	cipherText := joinedBytes[8 : 8+originalLen]

	// 3. Decrypt
	decryptedBytes, err := encryptor.Decrypt(cipherText, key)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (integrity check): %w", err)
	}

	// 4. Decompress
	compressor := compression.NewGzipCompressor()
	plainBytes, err := compressor.Decompress(decryptedBytes)
	if err != nil {
		return nil, fmt.Errorf("decompression failed: %w", err)
	}

	return plainBytes, nil
}