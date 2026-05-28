package transport

import (
	"errors"
	"fmt"
	"io"
)

const MaxResponseBytes = 1024 * 1024

var ErrResponseTooLarge = errors.New("response too large")

func ReadLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes < 0 {
		return nil, fmt.Errorf("max bytes must be non-negative")
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: limit %d bytes", ErrResponseTooLarge, maxBytes)
	}
	return data, nil
}
