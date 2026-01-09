package format

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Reader is a wrapper around the file stream that separates the
// metadata header from the binary body.
type Reader struct {
	Header *Header
	Body   io.Reader
}

// NewReader attempts to parse a horcrux stream.
// It consumes the text header (if present) and returns a Reader
// with the populated Header and a Body reader positioned at the start of the ciphertext.
func NewReader(r io.Reader) (*Reader, error) {
	// We use a bufio.Reader so we can read line-by-line without consuming
	// the binary body that follows.
	bufReader := bufio.NewReader(r)

	// 1. Scan for the Header Marker
	// We read line by line. If we don't find the header marker within a reasonable
	// amount of lines, we assume this is not a valid formatted horcrux.
	foundHeader := false
	for i := 0; i < 50; i++ { // limit scan to 50 lines to prevent infinite loops on garbage files
		line, err := bufReader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read stream while looking for header: %w", err)
		}
		if strings.TrimSpace(line) == HeaderMarker {
			foundHeader = true
			break
		}
	}

	if !foundHeader {
		return nil, fmt.Errorf("invalid format: could not find %q marker", HeaderMarker)
	}

	// 2. Read the JSON content until the Body Marker
	var jsonBuilder bytes.Buffer
	foundBody := false
	for {
		line, err := bufReader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read stream while reading header json: %w", err)
		}

		cleanLine := strings.TrimSpace(line)
		if cleanLine == BodyMarker {
			foundBody = true
			break
		}

		jsonBuilder.WriteString(line)
	}

	if !foundBody {
		return nil, fmt.Errorf("invalid format: could not find %q marker", BodyMarker)
	}

	// 3. Unmarshal the Header
	header := &Header{}
	if err := json.Unmarshal(jsonBuilder.Bytes(), header); err != nil {
		return nil, fmt.Errorf("failed to parse header json: %w", err)
	}

	// 4. Validate the parsed header
	if err := header.Validate(); err != nil {
		return nil, fmt.Errorf("header validation failed: %w", err)
	}

	return &Reader{
		Header: header,
		// The bufReader has buffered some of the body, but subsequent Read() calls
		// will drain that buffer before reading more from the underlying source.
		Body: bufReader,
	}, nil
}