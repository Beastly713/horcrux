package format

import (
	"errors"
	"fmt"
)

// Standard Markers used to delineate sections in the text-friendly format
const (
	// MagicHeader is the user-friendly introduction found at the top of the file
	MagicHeader = `# THIS FILE IS A HORCRUX.
# IT IS ONE OF %d HORCRUXES THAT EACH CONTAIN PART OF AN ORIGINAL FILE.
# THIS IS HORCRUX NUMBER %d.
# IN ORDER TO RESURRECT THIS ORIGINAL FILE YOU MUST FIND THE OTHER %d HORCRUX(ES)
# AND BIND THEM USING THE PROGRAM FOUND AT:
# https://github.com/Beastly713/horcrux
`
	// HeaderMarker indicates the start of the JSON metadata
	HeaderMarker = "-- HEADER --"

	// BodyMarker indicates the start of the encrypted/sharded binary content
	BodyMarker = "-- BODY --"
)

// Header contains all the metadata required to bind horcruxes together.
type Header struct {
	// OriginalFilename is the name of the file before splitting
	OriginalFilename string `json:"originalFilename"`

	// Timestamp is the unix timestamp when the split occurred.
	// Used to ensure we aren't mixing horcruxes from different sessions.
	Timestamp int64 `json:"timestamp"`

	// Index is the shard index (1-based)
	Index int `json:"index"`

	// Total is the total number of shards created
	Total int `json:"total"`

	// Threshold is the number of shards required to recover the file
	Threshold int `json:"threshold"`

	// KeyFragment is the Shamir secret share for this specific shard.
	// This reconstructs the AES-GCM key.
	KeyFragment []byte `json:"keyFragment"`
}

// Validate checks if the header contains sane values.
func (h *Header) Validate() error {
	if h.Index < 1 || h.Index > h.Total {
		return fmt.Errorf("invalid index %d for total %d", h.Index, h.Total)
	}
	if h.Threshold < 2 || h.Threshold > h.Total {
		return fmt.Errorf("invalid threshold %d for total %d", h.Threshold, h.Total)
	}
	if len(h.KeyFragment) == 0 {
		return errors.New("header is missing key fragment")
	}
	if h.OriginalFilename == "" {
		return errors.New("header is missing original filename")
	}
	return nil
}