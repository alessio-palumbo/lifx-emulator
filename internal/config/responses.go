package config

import (
	"bytes"
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

// LoadResponses overlays an optional local file on bundled defaults.
func LoadResponses(path string) (ResponseFile, error) {
	bundled, err := BundledResponses()
	if err != nil {
		return ResponseFile{}, err
	}
	var f ResponseFile
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) && path == ResponsePath() && os.Getenv("LIFX_EMULATOR_RESPONSES") == "" {
		return bundled, nil
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

	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil {
		return f, err
	}
	local, err := DecodeResponses(data)
	if err != nil {
		return f, err
	}
	return MergeResponses(bundled, local)
}

// DecodeResponses parses the same schema for local files and build-time input.
func DecodeResponses(data []byte) (ResponseFile, error) {
	var f ResponseFile
	if len(data) > 1<<20 {
		return f, fmt.Errorf("response configuration exceeds 1 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&f); err != nil {
		return f, fmt.Errorf("response configuration: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return f, fmt.Errorf("response configuration: expected exactly one JSON object")
	}
	return f, nil
}
