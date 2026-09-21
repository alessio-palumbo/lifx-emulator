package config

import (
	"path/filepath"
	"testing"
)

func TestPersistenceAndUniqueSerials(t *testing.T) {
	p := filepath.Join(t.TempDir(), "devices.json")
	a, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, d := range a.Devices {
		if seen[d.Serial] || d.Serial[:2] != "02" {
			t.Fatal(d.Serial)
		}
		seen[d.Serial] = true
	}
	a.Devices[0].Label = "Edited bulb"
	a.Devices[1].Enabled = false
	if err = Save(p, a); err != nil {
		t.Fatal(err)
	}
	b, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if b.Devices[0].Serial != a.Devices[0].Serial || b.Devices[0].Label != "Edited bulb" || b.Devices[1].Enabled {
		t.Fatal(b)
	}
	a.Devices[1].Serial = a.Devices[0].Serial
	if _, err = a.Virtuals(); err == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestTopologyValidation(t *testing.T) {
	for _, d := range []Definition{{Serial: "bad", Product: 27}, {Serial: "020000000001", Product: 55, Width: 8, Height: 8, Chains: 17}, {Serial: "020000000001", Product: 32, Zones: 0}, {Serial: "020000000001", Product: 99999}} {
		if _, err := d.Virtual(); err == nil {
			t.Fatal(d)
		}
	}
}

func TestHybridLightAcceptedAndSwitchRejected(t *testing.T) {
	luna := Definition{Serial: "020000000001", Label: "Luna", Product: 219, Enabled: true, Width: 8, Height: 8, Chains: 1}
	if _, err := luna.Virtual(); err != nil {
		t.Fatalf("hybrid matrix light rejected: %v", err)
	}
	switchProduct := Definition{Serial: "020000000002", Label: "Switch", Product: 70, Enabled: true}
	if _, err := switchProduct.Virtual(); err == nil {
		t.Fatal("relay product accepted")
	}
}

func TestSerialNormalization(t *testing.T) {
	f, err := Defaults()
	if err != nil {
		t.Fatal(err)
	}
	f.Devices[0].Serial = "02ABCDEF0001"
	p := filepath.Join(t.TempDir(), "devices.json")
	if err = Save(p, f); err != nil {
		t.Fatal(err)
	}
	f, err = Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if f.Devices[0].Serial != "02abcdef0001" {
		t.Fatal("serial not normalized")
	}
	s, err := UniqueSerial(f.Devices)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Devices {
		if s == d.Serial {
			t.Fatal("collision")
		}
	}
}
