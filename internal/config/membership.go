package config

import (
	"crypto/rand"
	"fmt"
	"math"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"lifx-emulator/internal/emulator"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

type Location struct {
	ID        device.LocationID
	Label     string
	UpdatedAt uint64
}
type Group struct {
	ID        device.GroupID
	Label     string
	UpdatedAt uint64
}

func newUUID() ([16]byte, error) {
	var id [16]byte
	_, err := rand.Read(id[:])
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return id, err
}

// Missing fields in older configurations are filled once and persisted by Load.
func (f *File) initializeMembership() (bool, error) {
	changed := false
	now := uint64(time.Now().UnixNano())
	if f.Location.ID.IsNil() {
		id, err := newUUID()
		if err != nil {
			return false, err
		}
		f.Location.ID = device.LocationID(id)
		changed = true
	}
	if f.Group.ID.IsNil() {
		id, err := newUUID()
		if err != nil {
			return false, err
		}
		f.Group.ID = device.GroupID(id)
		changed = true
	}
	if f.Location.Label == "" {
		f.Location.Label = "Virtual LAN"
		changed = true
	}
	if f.Group.Label == "" {
		hostname, _ := os.Hostname()
		name := strings.TrimSpace(hostname)
		if name == "" {
			name = "Emulator"
		}
		for len(name) > 32 {
			name = name[:len(name)-1]
		}
		for !utf8.ValidString(name) {
			name = name[:len(name)-1]
		}
		f.Group.Label = name
		changed = true
	}
	if f.Location.UpdatedAt == 0 {
		f.Location.UpdatedAt = now
		changed = true
	}
	if f.Group.UpdatedAt == 0 {
		f.Group.UpdatedAt = now
		changed = true
	}
	return changed, nil
}
func (f File) ValidateMembership() error {
	if f.Location.ID.IsNil() || f.Group.ID.IsNil() {
		return fmt.Errorf("location and group IDs must be nonzero UUIDs")
	}
	for _, entry := range []struct {
		name, label string
		updated     uint64
	}{{"location", f.Location.Label, f.Location.UpdatedAt}, {"group", f.Group.Label, f.Group.UpdatedAt}} {
		if strings.TrimSpace(entry.label) == "" || len(entry.label) > 32 || !utf8.ValidString(entry.label) || strings.ContainsRune(entry.label, '\x00') {
			return fmt.Errorf("%s label must contain 1–32 UTF-8 bytes", entry.name)
		}
		if entry.updated == 0 || entry.updated >= math.MaxInt64 {
			return fmt.Errorf("invalid %s update timestamp", entry.name)
		}
	}
	return nil
}
func (f File) ApplyMembership(v *emulator.VirtualDevice) {
	v.Device.LocationID = f.Location.ID
	v.Device.Location = f.Location.Label
	v.Device.GroupID = f.Group.ID
	v.Device.Group = f.Group.Label
	v.LocationUpdatedAt = f.Location.UpdatedAt
	v.GroupUpdatedAt = f.Group.UpdatedAt
}
func (f File) MembershipPackets() (packets.DeviceStateLocation, packets.DeviceStateGroup) {
	location := packets.DeviceStateLocation{Location: [16]byte(f.Location.ID), UpdatedAt: f.Location.UpdatedAt}
	group := packets.DeviceStateGroup{Group: [16]byte(f.Group.ID), UpdatedAt: f.Group.UpdatedAt}
	copy(location.Label[:], f.Location.Label)
	copy(group.Label[:], f.Group.Label)
	return location, group
}
