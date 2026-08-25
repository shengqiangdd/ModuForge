package saferead

import (
	"fmt"
	"io"
)

// MaxReadSize is the maximum number of bytes allowed from a single ReadAll call.
// This prevents OOM when reading untrusted HTTP responses.
const MaxReadSize = 10 << 20 // 10 MB

// SafeReadAll reads from r up to MaxReadSize bytes.
// Returns an error if the reader contains more than MaxReadSize bytes.
func SafeReadAll(r io.Reader) ([]byte, error) {
	limited := io.LimitReader(r, MaxReadSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > MaxReadSize {
		return nil, fmt.Errorf("response body exceeds %d bytes limit", MaxReadSize)
	}
	return data, nil
}
