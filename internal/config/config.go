// Package config persists identity and topology, not transient light state.
package config

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"lifx-emulator/internal/emulator"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/alessio-palumbo/lifxregistry-go/gen/registry"
)

type Definition struct {
	Serial       string
	Label        string
	Product      uint32
	Enabled      bool
	Zones        int
	Width        int
	Height       int
	Chains       int
	Orientations []device.Orientation
}
type File struct {
	Listen   string
	Devices  []Definition
	Location Location
	Group    Group
}

func Path() string {
	if p := os.Getenv("LIFX_EMULATOR_CONFIG"); p != "" {
		return p
	}
	d, err := os.UserConfigDir()
	if err != nil {
		d = "."
	}
	return filepath.Join(d, "lifx-emulator", "devices.json")
}
func Serial() (string, error) {
	var a [6]byte
	if _, err := rand.Read(a[:]); err != nil {
		return "", err
	}
	a[0] = 0x02
	return fmt.Sprintf("%x", a), nil
}

// UniqueSerial retries local collisions before a definition is persisted.
func UniqueSerial(existing []Definition) (string, error) {
	seen := map[string]bool{}
	for _, d := range existing {
		serial, err := device.SerialFromHex(d.Serial)
		if err == nil {
			seen[serial.String()] = true
		}
	}
	for {
		serial, err := Serial()
		if err != nil {
			return "", err
		}
		if !seen[serial] {
			return serial, nil
		}
	}
}
func (d Definition) Virtual() (*emulator.VirtualDevice, error) {
	serial, err := device.SerialFromHex(d.Serial)
	if err != nil || serial.IsNil() {
		return nil, fmt.Errorf("invalid serial %q", d.Serial)
	}
	p, ok := registry.ProductsByPID[int(d.Product)]
	if !ok || p.Features.Relays || p.Features.Buttons {
		return nil, fmt.Errorf("unsupported light product %d", d.Product)
	}
	if len(d.Label) > 32 {
		return nil, fmt.Errorf("label exceeds 32 bytes")
	}
	v := device.Device{Serial: serial, Label: d.Label}
	v.SetProductInfo(d.Product)
	switch v.LightType {
	case device.LightTypeMultiZone:
		if d.Zones < 1 || d.Zones > 255 {
			return nil, fmt.Errorf("zones must be 1–255")
		}
		v.MultizoneProperties.Zones = make([]packets.LightHsbk, d.Zones)
	case device.LightTypeMatrix:
		if d.Width < 1 || d.Width > 255 || d.Height < 1 || d.Height > 255 || d.Width*d.Height > 4096 || d.Chains < 1 || d.Chains > 16 || (!p.Features.Chain && d.Chains != 1) {
			return nil, fmt.Errorf("invalid matrix topology")
		}
		props := &v.MatrixProperties
		props.Width = d.Width
		props.Height = d.Height
		props.NZones = d.Width * d.Height
		props.ChainLength = d.Chains
		props.StatePackets = (props.NZones + 63) / 64
		props.ChainZones = make([][]packets.LightHsbk, d.Chains)
		props.ChainOrientations = make([]device.Orientation, d.Chains)
		for i := range d.Chains {
			props.ChainZones[i] = make([]packets.LightHsbk, props.NZones)
			if i < len(d.Orientations) {
				if d.Orientations[i] > device.OrientationRight {
					return nil, fmt.Errorf("invalid orientation")
				}
				props.ChainOrientations[i] = d.Orientations[i]
			}
		}
	}
	return emulator.New(v, d.Enabled), nil
}
func Defaults() (File, error) {
	f := File{Listen: "0.0.0.0:56700"}
	if _, err := f.initializeMembership(); err != nil {
		return f, err
	}
	for _, d := range []Definition{{Label: "Virtual bulb", Product: 27, Enabled: true}, {Label: "Virtual strip", Product: 32, Enabled: true, Zones: 16}, {Label: "Virtual Tiles", Product: 55, Enabled: true, Width: 8, Height: 8, Chains: 5}} {
		var err error
		d.Serial, err = UniqueSerial(f.Devices)
		if err != nil {
			return f, err
		}
		f.Devices = append(f.Devices, d)
	}
	return f, nil
}
func Load(path string) (File, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		f, err := Defaults()
		if err == nil {
			err = Save(path, f)
		}
		return f, err
	}
	if err != nil {
		return File{}, err
	}
	var f File
	err = json.Unmarshal(b, &f)
	if err != nil {
		return f, err
	}
	migrated, err := f.initializeMembership()
	if err != nil {
		return f, err
	}
	vs, validationErr := f.Virtuals()
	if validationErr != nil {
		return f, validationErr
	}
	for i, v := range vs {
		f.Devices[i].Serial = v.Device.Serial.String()
	}
	if migrated {
		if err := Save(path, f); err != nil {
			return f, err
		}
	}
	return f, nil
}
func (f File) Virtuals() ([]*emulator.VirtualDevice, error) {
	if err := f.ValidateMembership(); err != nil {
		return nil, err
	}
	out := []*emulator.VirtualDevice{}
	seen := map[device.Serial]bool{}
	for _, d := range f.Devices {
		v, err := d.Virtual()
		if err != nil {
			return nil, err
		}
		if seen[v.Device.Serial] {
			return nil, fmt.Errorf("duplicate serial")
		}
		seen[v.Device.Serial] = true
		f.ApplyMembership(v)
		out = append(out, v)
	}
	return out, nil
}
func Save(path string, f File) error {
	if _, err := f.Virtuals(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".devices-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(b); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}
