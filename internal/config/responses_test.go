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
	baseline, err := BundledResponses()
	if err != nil {
		t.Fatal("bundled defaults failed to load")
	}
	if file, err := LoadResponses(path); err != nil || len(file.Responses) != len(baseline.Responses) {
		t.Fatal("optional missing file did not preserve bundled defaults")
	}
	if _, err := LoadResponses(filepath.Join(dir, "explicit-missing.json")); err == nil {
		t.Fatal("missing explicit file accepted")
	}
	if err := os.WriteFile(path, []byte(`{"responses":[{"request_type":60000,"request_size":0,"response_type":60001,"parts":[{"hex":"0102"}]}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := LoadResponses(path)
	if err != nil {
		t.Fatal("local response file failed to load")
	}
	found := false
	for _, rule := range file.Responses {
		if rule.RequestType == 60000 && len(rule.Parts) == 1 && rule.Parts[0].Hex == "0102" {
			found = true
		}
	}
	if !found {
		t.Fatal("local response missing after loading defaults")
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

func TestResponseOverridesAndDuplicates(t *testing.T) {
	bundled := ResponseFile{Responses: []ResponseRule{{RequestType: 60000, ResponseType: 60001}, {RequestType: 60002, ResponseType: 60003}}}
	local := ResponseFile{Responses: []ResponseRule{{RequestType: 60000, ResponseType: 60004}, {RequestType: 60005, ResponseType: 60006}}}
	merged, err := MergeResponses(bundled, local)
	if err != nil || len(merged.Responses) != 3 {
		t.Fatalf("merge: %+v %v", merged, err)
	}
	if merged.Responses[0].ResponseType != 60004 || merged.Responses[1].ResponseType != 60003 || merged.Responses[2].ResponseType != 60006 {
		t.Fatal("local override or retained defaults incorrect")
	}
	if bundled.Responses[0].ResponseType != 60001 {
		t.Fatal("merge mutated input")
	}
	local.Responses = append(local.Responses, local.Responses[0])
	if _, err := MergeResponses(bundled, local); err == nil {
		t.Fatal("duplicate local definitions hidden")
	}
	if _, err := MergeResponses(local, bundled); err == nil {
		t.Fatal("duplicate bundled definitions hidden")
	}
}
