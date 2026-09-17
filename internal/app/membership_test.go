package app

import (
	"path/filepath"
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"lifx-emulator/internal/config"
	"lifx-emulator/internal/lan"
)

func TestUpdateMembershipPreservesStateAndPublishesPublicMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	f, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	vs, err := f.Virtuals()
	if err != nil {
		t.Fatal(err)
	}
	a := &App{file: f, path: path, router: lan.New(vs, nil)}
	state := vs[0].State
	location := f.Location
	group := f.Group
	if err := a.UpdateMembership("Test lab", location.ID.String(), "Alice", group.ID.String()); err != nil {
		t.Fatal(err)
	}
	changed, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Location.ID != location.ID || changed.Group.ID != group.ID || changed.Location.UpdatedAt <= location.UpdatedAt || changed.Group.UpdatedAt <= group.UpdatedAt {
		t.Fatal("rename did not preserve IDs or advance timestamps")
	}
	if vs[0].State != state {
		t.Fatal("metadata reset light state")
	}
	for _, v := range vs {
		for _, payload := range []packets.Payload{&packets.DeviceGetLocation{}, &packets.DeviceGetGroup{}} {
			m := protocol.NewMessage(payload)
			m.SetTarget([8]byte(v.Device.Serial))
			replies := a.router.Handle(m)
			if len(replies) != 1 {
				t.Fatal("missing query response")
			}
			switch p := replies[0].Payload.(type) {
			case *packets.DeviceStateLocation:
				if p.Location != [16]byte(location.ID) || device.ParseLabel(p.Label) != "Test lab" || p.UpdatedAt != changed.Location.UpdatedAt {
					t.Fatal("wrong location")
				}
			case *packets.DeviceStateGroup:
				if p.Group != [16]byte(group.ID) || device.ParseLabel(p.Label) != "Alice" || p.UpdatedAt != changed.Group.UpdatedAt {
					t.Fatal("wrong group")
				}
			}
		}
	}
	if err := a.UpdateMembership("Test lab", location.ID.String(), "Alice", group.ID.String()); err != nil {
		t.Fatal(err)
	}
	if a.file.Location != changed.Location || a.file.Group != changed.Group {
		t.Fatal("no-op changed timestamps")
	}
	if err := a.Add(config.Definition{Label: "New bulb", Product: 27, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	added := a.router.Devices[len(a.router.Devices)-1]
	if added.Device.Group != "Alice" || added.Device.GroupID != group.ID {
		t.Fatal("new device missed shared metadata")
	}
	before := a.file
	for _, id := range []string{"invalid", "00000000-0000-0000-0000-000000000000"} {
		if err := a.UpdateMembership("bad", id, "bad", group.ID.String()); err == nil {
			t.Fatal("invalid identity accepted")
		}
	}
	if a.file.Location != before.Location || a.file.Group != before.Group {
		t.Fatal("invalid update changed live metadata")
	}
}
