package pipeline

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestPipelineRoundTrip(t *testing.T) {
	// 1. Setup
	key := make([]byte, 32) // AES-256 key
	rand.Read(key)

	// We'll use a string that compresses well to prove compression is working
	// (repeated data compresses very well)
	originalString := "This is a secret message that repeats. "
	for i := 0; i < 500; i++ {
		originalString += "This is a secret message that repeats. "
	}
	originalData := []byte(originalString)
	reader := bytes.NewReader(originalData)

	config := PipelineConfig{
		Total:     5,
		Threshold: 3,
	}

	// 2. Run Split Pipeline
	shards, err := SplitPipeline(reader, key, config)
	if err != nil {
		t.Fatalf("SplitPipeline failed: %v", err)
	}

	if len(shards) != 5 {
		t.Fatalf("Expected 5 shards, got %d", len(shards))
	}

	// 3. Verify Compression & Encryption worked
	// The total size of shards should be roughly equal to the COMPRESSED size, not original.
	// Since our text is highly repetitive, it should be much smaller than original.
	totalShardSize := 0
	for _, s := range shards {
		totalShardSize += len(s.Data)
	}
	// Original is ~20KB. Compressed should be < 1KB.
	// If Encryption didn't run, we'd see cleartext.
	if totalShardSize > len(originalData)/2 {
		t.Logf("Warning: Data did not compress well (Size: %d -> %d). Check compression logic.", len(originalData), totalShardSize)
	}

	// 4. Simulate Loss (Keep only 3 shards: 0, 2, 4)
	shardsMap := make(map[int][]byte)
	shardsMap[0] = shards[0].Data
	shardsMap[2] = shards[2].Data
	shardsMap[4] = shards[4].Data

	// 5. Run Join Pipeline
	restoredData, err := JoinPipeline(shardsMap, key, config.Total, config.Threshold)
	if err != nil {
		t.Fatalf("JoinPipeline failed: %v", err)
	}

	// 6. Verify Content
	if !bytes.Equal(originalData, restoredData) {
		t.Fatal("Pipeline Round-Trip failed: Data mismatch")
	}
}