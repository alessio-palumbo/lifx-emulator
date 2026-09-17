package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMembershipMigrationAndPersistence(t *testing.T) {
	old, err := Defaults()
	if err != nil {
		t.Fatal(err)
	}
	legacy := struct {
		Listen  string
		Devices []Definition
	}{old.Listen, old.Devices}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "devices.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	first, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if first.Location != second.Location || first.Group != second.Group {
		t.Fatal("metadata changed between launches")
	}
	if first.Location.ID.IsNil() || first.Group.ID.IsNil() || first.Location.UpdatedAt == 0 || first.Group.UpdatedAt == 0 {
		t.Fatal("missing metadata")
	}
	for index, d := range first.Devices {
		if d.Serial != old.Devices[index].Serial {
			t.Fatal("migration changed device identity")
		}
	}
	other, err := Load(filepath.Join(t.TempDir(), "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	if other.Location.ID == first.Location.ID || other.Group.ID == first.Group.ID {
		t.Fatal("independent configurations share IDs")
	}
	vs, err := first.Virtuals()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		if v.Device.LocationID != first.Location.ID || v.Device.GroupID != first.Group.ID || v.Device.Location != first.Location.Label || v.GroupUpdatedAt != first.Group.UpdatedAt {
			t.Fatal("virtual metadata differs from config")
		}
	}
}
func TestInvalidMembershipDoesNotReplaceConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", strings.Repeat("x", 33), strings.Repeat("界", 11)} {
		copy := f
		copy.Group.Label = invalid
		if err := Save(path, copy); err == nil {
			t.Fatal("invalid metadata saved")
		}
		raw, _ := os.ReadFile(path)
		if string(raw) != string(original) {
			t.Fatal("invalid save replaced config")
		}
	}
	f.Location.ID = [16]byte{}
	if err := Save(path, f); err == nil {
		t.Fatal("nil location accepted")
	}
}
