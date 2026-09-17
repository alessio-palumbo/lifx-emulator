package config

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
)

// The tracked empty resource keeps ordinary builds independent of secrets.
// Release CI supplies compat.local.json, which is ignored by Git.
//
//go:embed bundled/*.json
var bundledResponses embed.FS

func BundledResponses() (ResponseFile, error) {
	data, err := bundledResponses.ReadFile("bundled/compat.local.json")
	if errors.Is(err, fs.ErrNotExist) {
		return ResponseFile{}, nil
	}
	if err != nil {
		return ResponseFile{}, err
	}
	file, err := DecodeResponses(data)
	if err != nil {
		return ResponseFile{}, fmt.Errorf("invalid bundled response configuration")
	}
	return file, nil
}

// MergeResponses lets local definitions replace individual bundled rules.
// Duplicates within either layer remain errors instead of being hidden by merging.
func MergeResponses(bundled, local ResponseFile) (ResponseFile, error) {
	merged := ResponseFile{}
	indexes := map[uint16]int{}
	for _, layer := range []ResponseFile{bundled, local} {
		seen := map[uint16]bool{}
		for _, rule := range layer.Responses {
			if seen[rule.RequestType] {
				return ResponseFile{}, fmt.Errorf("duplicate response request type %d", rule.RequestType)
			}
			seen[rule.RequestType] = true
			if index, exists := indexes[rule.RequestType]; exists {
				merged.Responses[index] = rule
			} else {
				indexes[rule.RequestType] = len(merged.Responses)
				merged.Responses = append(merged.Responses, rule)
			}
		}
	}
	return merged, nil
}
