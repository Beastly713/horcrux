package sharding

import (
	"bytes"
	"math/rand"
	"testing"
	"time"
)

func TestSplitAndJoin(t *testing.T) {
	// Setup random seed
	rand.Seed(time.Now().UnixNano())

	// Config: 5 shards total, 3 required to recover
	total := 5
	threshold := 3

	splitter, err := NewSplitter(total, threshold)
	if err != nil {
		t.Fatalf("Failed to create splitter: %v", err)
	}

	// 1. Generate Random Data
	// Note: Reed-Solomon often pads data to be divisible by the number of data shards.
	originalData := make([]byte, 1024*10) // 10KB
	rand.Read(originalData)

	// 2. Split
	shards, err := splitter.Split(originalData)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(shards) != total {
		t.Errorf("Expected %d shards, got %d", total, len(shards))
	}

	// 3. Simulate Loss (Delete 2 shards, keep 3)
	// We put them in a map to simulate having "some" files available
	availableShards := make(map[int][]byte)
	
	// We deliberately skip index 0 and 4 to simulate loss
	// We only keep indices 1, 2, 3 (which is exactly threshold)
	availableShards[1] = shards[1][0].Data
	availableShards[2] = shards[2][0].Data
	availableShards[3] = shards[3][0].Data

	// 4. Join
	// Note: We pass original size to trim padding accurately
	restoredData, err := splitter.Join(availableShards, len(originalData))
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}

	// 5. Verify
	if !bytes.Equal(originalData, restoredData) {
		t.Fatal("Restored data does not match original data")
	}
}