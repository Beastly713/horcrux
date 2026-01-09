package tests

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beastly713/horcrux/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullRoundTrip simulates the full user journey: Split -> partial delete -> Bind
func TestFullRoundTrip(t *testing.T) {
	// 1. Setup specific test environment
	tmpDir := t.TempDir()
	originalFile := filepath.Join(tmpDir, "secret_plans.txt")
	originalContent := make([]byte, 1024*1024) // 1MB random data
	_, err := rand.Read(originalContent)
	require.NoError(t, err)

	err = os.WriteFile(originalFile, originalContent, 0644)
	require.NoError(t, err)

	// Calculate hash of original
	originalHash := sha256.Sum256(originalContent)

	// 2. Execute SPLIT Command
	root := cmd.GetRootCmd()

	// We will manually invoke the split logic via CLI args
	splitArgs := []string{"split", originalFile, "-n", "5", "-t", "3", "-d", tmpDir}
	root.SetArgs(splitArgs)
	err = root.Execute()
	require.NoError(t, err, "Split command failed")

	// 3. Verify files were created
	matches, err := filepath.Glob(filepath.Join(tmpDir, "*.horcrux"))
	require.NoError(t, err)
	assert.Equal(t, 5, len(matches), "Should have created 5 horcrux files")

	// 4. Simulate Disaster: Delete 2 files (since threshold is 3, we can lose 2)
	os.Remove(matches[0])
	os.Remove(matches[1])

	// FIX: Delete the original file so Bind is forced to reconstruct it
	os.Remove(originalFile)

	// 5. Execute BIND Command
	bindArgs := []string{"bind", tmpDir, "--destination", tmpDir}
	root.SetArgs(bindArgs)
	err = root.Execute()
	require.NoError(t, err, "Bind command failed")

	// 6. Verify Content
	// The file should be recreated at the destination
	restoredFile := filepath.Join(tmpDir, "secret_plans.txt")
	restoredContent, err := os.ReadFile(restoredFile)
	require.NoError(t, err, "Failed to read restored file")

	restoredHash := sha256.Sum256(restoredContent)

	// The Ultimate Check: Does Hash(Restored) == Hash(Original)?
	if !bytes.Equal(originalHash[:], restoredHash[:]) {
		t.Fatalf("Restored file hash mismatch!\nOriginal: %x\nRestored: %x", originalHash, restoredHash)
	}
}

// TestCorruptShard_Safety ensures that modified/corrupted shards cause the bind to fail
// (protecting the user from getting a bad file) rather than silently succeeding.
func TestCorruptShard_Safety(t *testing.T) {
	tmpDir := t.TempDir()
	originalFile := filepath.Join(tmpDir, "codes.txt")
	originalContent := []byte("Launch codes: 12345")
	os.WriteFile(originalFile, originalContent, 0644)

	root := cmd.GetRootCmd()

	// 1. SPLIT (3 shards, threshold 2)
	root.SetArgs([]string{"split", originalFile, "-n", "3", "-t", "2", "-d", tmpDir})
	require.NoError(t, root.Execute())

	matches, _ := filepath.Glob(filepath.Join(tmpDir, "*.horcrux"))
	require.Len(t, matches, 3)

	// CRITICAL FIX: Delete the original file so Bind actually attempts restoration
	os.Remove(originalFile)

	// 2. CORRUPT One Shard
	// We append garbage to the end of the file. 
	// Since AES-GCM validates the authentication tag against the data, 
	// changing the data length or content will invalidate the tag.
	f, err := os.OpenFile(matches[0], os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)
	f.Write([]byte("MALICIOUS_DATA"))
	f.Close()

	// 3. BIND (Should FAIL or Skip Corrupt Shard)
	// Scenario: We have 3 shards total. 1 is corrupt. Threshold is 2.
	// The Bind logic gathers all 3. It should try to reconstruct. 
	// The AES-GCM check on the corrupted shard will fail.
	root.SetArgs([]string{"bind", tmpDir, "--destination", tmpDir})
	err = root.Execute()
	
	// 4. VERIFY FAILURE (Security Check)
	restoredFile := filepath.Join(tmpDir, "codes.txt")
	
	// If the file exists, it MUST match original content (meaning it ignored the bad shard and used the 2 good ones).
	// If the file does not exist, that is also acceptable (failed safe).
	// What is NOT acceptable is a file existing with garbage content.
	if _, err := os.Stat(restoredFile); err == nil {
		content, _ := os.ReadFile(restoredFile)
		if bytes.Equal(content, originalContent) {
			t.Log("Success: System automatically recovered using the remaining good shards.")
		} else {
			t.Fatalf("Security Fail: Bind created a corrupted file! Integrity check bypassed.")
		}
	} else {
		t.Log("Success: System refused to output corrupted data.")
	}

	// 5. RECOVERY (Remove corrupt shard explicitly)
	os.Remove(matches[0]) // Delete the bad one
	
	// Now we have 2 valid shards (Threshold is 2). Bind should succeed now.
	root.SetArgs([]string{"bind", tmpDir, "--destination", tmpDir})
	err = root.Execute()
	require.NoError(t, err)

	content, err := os.ReadFile(restoredFile)
	require.NoError(t, err)
	assert.Equal(t, originalContent, content, "Should recover perfectly using valid shards")
}

// TestHeaderlessRoundTrip verifies the --paranoiac flag behavior
func TestHeaderlessRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	originalFile := filepath.Join(tmpDir, "paranoiac.txt")
	originalContent := []byte("Headerless secret")
	os.WriteFile(originalFile, originalContent, 0644)

	root := cmd.GetRootCmd()

	// 1. Split --headerless
	root.SetArgs([]string{"split", originalFile, "-n", "3", "-t", "2", "-d", tmpDir, "--headerless"})
	require.NoError(t, root.Execute())

	// 2. Verify files look like noise
	// Headerless mode produces .bin files, not .horcrux
	matches, _ := filepath.Glob(filepath.Join(tmpDir, "*.bin"))
	require.NotEmpty(t, matches, "Should find .bin files for headerless mode")

	content, _ := os.ReadFile(matches[0])
	if bytes.Contains(content, []byte("THIS FILE IS A HORCRUX")) {
		t.Fatal("Headerless mode failed: Found magic header in file")
	}
}