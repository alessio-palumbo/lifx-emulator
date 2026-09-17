package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalResponseLoading(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LIFX_EMULATOR_CONFIG", filepath.Join(dir, "devices.json"))
	t.Setenv("LIFX_EMULATOR_RESPONSES", "")
	path := ResponsePath()
	if file, err := LoadResponses(path); err != nil || len(file.Responses) != 0 {
		t.Fatalf("optional missing file: %+v %v", file, err)
	}
	if _, err := LoadResponses(filepath.Join(dir, "explicit-missing.json")); err == nil {
		t.Fatal("missing explicit file accepted")
	}
	if err := os.WriteFile(path, []byte(`{"responses":[{"request_type":60000,"request_size":0,"response_type":60001,"parts":[{"hex":"0102"}]}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := LoadResponses(path)
	if err != nil || len(file.Responses) != 1 || file.Responses[0].Parts[0].Hex != "0102" {
		t.Fatalf("load: %+v %v", file, err)
	}
	for _, invalid := range []string{`{"responses":[],"typo":true}`, `{"responses":[]} {}`, `{"responses":[{"request_type":65536}]}`, `not json`} {
		if err := os.WriteFile(path, []byte(invalid), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadResponses(path); err == nil {
			t.Fatalf("invalid config accepted: %s", invalid)
		}
	}
	t.Setenv("LIFX_EMULATOR_RESPONSES", filepath.Join(dir, "missing-override.json"))
	if _, err := LoadResponses(ResponsePath()); err == nil {
		t.Fatal("missing environment override accepted")
	}
}
