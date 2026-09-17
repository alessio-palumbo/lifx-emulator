package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lifx-emulator/internal/config"
)

func TestPrepareValidatesBeforeWriting(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "bundled", "compat.local.json")
	valid := `{"responses":[{"request_type":60000,"request_size":0,"response_type":60001,"parts":[{"query":"DeviceGetLabel"}]}]}`
	if err := prepare(destination, valid); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	file, err := config.DecodeResponses(original)
	if err != nil || len(file.Responses) != 1 {
		t.Fatalf("prepared resource: %+v %v", file, err)
	}
	for _, invalid := range []string{"", `{"responses":[]}`, `{"secret-marker":"test-sensitive-value"}`, `{"responses":[{"request_type":60000,"response_type":60001,"parts":[{"hex":"test-sensitive-value"}]}]}`} {
		err := prepare(destination, invalid)
		if err == nil {
			t.Fatal("invalid release configuration accepted")
		}
		if strings.Contains(err.Error(), "test-sensitive-value") || strings.Contains(err.Error(), "60000") {
			t.Fatal("error disclosed configuration contents")
		}
		remaining, err := os.ReadFile(destination)
		if err != nil {
			t.Fatal(err)
		}
		if string(remaining) != string(original) {
			t.Fatal("invalid input replaced previous resource")
		}
	}
}
