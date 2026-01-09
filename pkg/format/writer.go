package format

import (
	"encoding/json"
	"fmt"
	"io"
)

// Writer handles the writing of a single horcrux file.
type Writer struct {
	w io.Writer
}

// NewWriter creates a new Writer around an io.Writer (usually an os.File).
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// Write serializes the header and content to the underlying writer.
// If headerless is true, it skips the metadata entirely (Paranoiac Mode).
func (hw *Writer) Write(header *Header, content []byte, headerless bool) error {
	if !headerless {
		// 1. Validate the header before writing anything
		if err := header.Validate(); err != nil {
			return fmt.Errorf("invalid header: %w", err)
		}

		// 2. Format and write the "Magic Header" text.
		// We calculate (Threshold - 1) to tell the user exactly how many *more*
		// files they need to find (assuming they found this one).
		magicText := fmt.Sprintf(MagicHeader, header.Total, header.Index, header.Threshold-1)
		if _, err := fmt.Fprint(hw.w, magicText); err != nil {
			return fmt.Errorf("failed to write magic header: %w", err)
		}

		// 3. Write the Header Marker
		if _, err := fmt.Fprintln(hw.w, HeaderMarker); err != nil {
			return fmt.Errorf("failed to write header marker: %w", err)
		}

		// 4. Marshal and write the Header JSON
		headerBytes, err := json.Marshal(header)
		if err != nil {
			return fmt.Errorf("failed to marshal header: %w", err)
		}
		if _, err := hw.w.Write(headerBytes); err != nil {
			return fmt.Errorf("failed to write json header: %w", err)
		}

		// Add a newline for readability before the body marker
		if _, err := fmt.Fprintln(hw.w); err != nil {
			return err
		}

		// 5. Write the Body Marker
		if _, err := fmt.Fprintln(hw.w, BodyMarker); err != nil {
			return fmt.Errorf("failed to write body marker: %w", err)
		}
	}

	// 6. Write the Content (The encrypted/sharded payload)
	if _, err := hw.w.Write(content); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	return nil
}