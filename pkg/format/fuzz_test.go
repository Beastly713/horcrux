package format_test

import (
	"bytes"
	"testing"

	"github.com/Beastly713/horcrux/pkg/format"
)

// FuzzNewReader feeds random byte streams into the parser.
// We don't care IF it fails (garbage in, garbage out), 
// we only care that it fails GRACEFULLY (returns error, doesn't panic).
func FuzzNewReader(f *testing.F) {
	// 1. Add some valid seed corpus to help the fuzzer start
	// This represents a minimal valid header structure
	validHeader := []byte(`# THIS FILE IS A HORCRUX...
-- HEADER --
{"originalFilename":"test.txt","timestamp":123,"index":1,"total":5,"threshold":3,"keyFragment":"YWJj"}
-- BODY --
somebinarycontent`)
	f.Add(validHeader)

	// 2. Add completely random seeds
	f.Add([]byte("random garbage"))
	f.Add([]byte("-- HEADER --"))
	f.Add([]byte("{}"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Pass the fuzzed data to the reader
		r := bytes.NewReader(data)
		_, err := format.NewReader(r)

		// We expect errors for garbage data. 
		// If NewReader panics, the fuzzer will catch it and report it as a failure.
		if err != nil {
			return
		}
	})
}