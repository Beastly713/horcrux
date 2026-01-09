package format

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestRoundTrip_Standard(t *testing.T) {
	// 1. Setup Input Data
	originalHeader := &Header{
		OriginalFilename: "secret_plans.txt",
		Timestamp:        1620000000,
		Index:            1,
		Total:            5,
		Threshold:        3,
		KeyFragment:      []byte("super-secret-key-fragment"),
	}
	originalBody := []byte("This is the encrypted content of the file.")

	// 2. Write to a buffer (Simulating a file on disk)
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	err := writer.Write(originalHeader, originalBody, false) // headerless=false
	if err != nil {
		t.Fatalf("Failed to write horcrux: %v", err)
	}

	// 3. Read back from the buffer
	reader, err := NewReader(&buf)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	// 4. Verify Header Integrity
	if !reflect.DeepEqual(reader.Header, originalHeader) {
		t.Errorf("Headers do not match.\nGot: %+v\nWant: %+v", reader.Header, originalHeader)
	}

	// 5. Verify Body Integrity
	readBody, err := io.ReadAll(reader.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	if !bytes.Equal(readBody, originalBody) {
		t.Errorf("Body content does not match.\nGot: %s\nWant: %s", readBody, originalBody)
	}
}

func TestParanoiacMode(t *testing.T) {
	header := &Header{
		OriginalFilename: "hidden.txt",
		Index:            1,
		Total:            3,
		Threshold:        2,
		KeyFragment:      []byte("key"),
	}
	body := []byte("raw-binary-data")

	var buf bytes.Buffer
	writer := NewWriter(&buf)

	// Write with headerless = true
	err := writer.Write(header, body, true)
	if err != nil {
		t.Fatalf("Failed to write headerless horcrux: %v", err)
	}

	output := buf.String()

	// 1. Ensure NO metadata markers exist
	if strings.Contains(output, "THIS FILE IS A HORCRUX") {
		t.Error("Paranoiac mode failed: Magic Header found in output")
	}
	if strings.Contains(output, "-- HEADER --") {
		t.Error("Paranoiac mode failed: Header Marker found in output")
	}

	// 2. Ensure Reader correctly FAILS (It should not recognize this file)
	_, err = NewReader(&buf)
	if err == nil {
		t.Error("Reader should have failed to parse a headerless file, but it succeeded")
	}
}

func TestCorruptFile(t *testing.T) {
	// A file that looks right but has broken JSON
	corruptData := `# THIS FILE IS A HORCRUX...
-- HEADER --
{ "broken_json": "missing_bracket"
-- BODY --
payload`

	buf := bytes.NewBufferString(corruptData)
	_, err := NewReader(buf)

	if err == nil {
		t.Error("Reader should have failed on corrupt JSON, but succeeded")
	}
}