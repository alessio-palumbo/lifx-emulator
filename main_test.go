package main

import "testing"

func TestAppTitleIncludesNormalizedVersion(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })
	for _, test := range []struct {
		version string
		want    string
	}{{"0.1.0", "LIFX Emulator v0.1.0"}, {"v1.2.3-beta.1", "LIFX Emulator v1.2.3-beta.1"}} {
		version = test.version
		if got := appTitle(); got != test.want {
			t.Errorf("appTitle() = %q, want %q", got, test.want)
		}
	}
}
