package graph

import (
	"encoding/json"
	"io"

	"github.com/johnkil/outlook-agent/internal/transport"
)

func decodeLimitedJSON(reader io.Reader, output any) error {
	rawBody, err := transport.ReadLimited(reader, transport.MaxResponseBytes)
	if err != nil {
		return err
	}
	return json.Unmarshal(rawBody, output)
}
