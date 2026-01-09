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
	os.Remove(matches[3])

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

// Helper to handle headerless mode
func TestHeaderlessRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	originalFile := filepath.Join(tmpDir, "paranoiac.txt")
	originalContent := []byte("This is a secret message for headerless mode")
	os.WriteFile(originalFile, originalContent, 0644)

	root := cmd.GetRootCmd()

	// 1. Split with --headerless
	root.SetArgs([]string{"split", originalFile, "-n", "3", "-t", "2", "-d", tmpDir, "--headerless"})
	err := root.Execute()
	require.NoError(t, err)

	// 2. Verify files look like noise
	matches, _ := filepath.Glob(filepath.Join(tmpDir, "*.horcrux"))
	require.NotEmpty(t, matches)
	
	content, _ := os.ReadFile(matches[0])
	if bytes.Contains(content, []byte("THIS FILE IS A HORCRUX")) {
		t.Fatal("Headerless mode failed: Found magic header in file")
	}
}