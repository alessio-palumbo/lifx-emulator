package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ResponseFile describes opt-in local replies without embedding packet layouts.
// Query parts reuse the generated public protocol; hex parts replay local bytes.
type ResponseFile struct {
	Responses []ResponseRule `json:"responses"`
}
type ResponseRule struct {
	RequestType  uint16         `json:"request_type"`
	RequestSize  uint16         `json:"request_size"`
	ResponseType uint16         `json:"response_type"`
	RequestName  string         `json:"request_name,omitempty"`
	ResponseName string         `json:"response_name,omitempty"`
	Parts        []ResponsePart `json:"parts"`
}
type ResponsePart struct {
	Query string `json:"query,omitempty"`
	Hex   string `json:"hex,omitempty"`
}

func ResponsePath() string {
	if p := os.Getenv("LIFX_EMULATOR_RESPONSES"); p != "" {
		return p
	}
	return filepath.Join(filepath.Dir(Path()), "responses.local.json")
}

// Missing optional files leave normal public protocol behavior enabled.
func LoadResponses(path string) (ResponseFile, error) {
	var f ResponseFile
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) && path == ResponsePath() && os.Getenv("LIFX_EMULATOR_RESPONSES") == "" {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return f, err
	}
	if info.Size() > 1<<20 {
		return f, fmt.Errorf("local responses file exceeds 1 MiB")
	}
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&f); err != nil {
		return f, fmt.Errorf("local responses: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return f, fmt.Errorf("local responses: expected exactly one JSON object")
	}
	return f, nil
}
