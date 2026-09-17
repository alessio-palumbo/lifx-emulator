package lan

import (
	"context"
	"encoding/hex"
	"errors"
	"net"
	"testing"
	"time"

	"lifx-emulator/internal/config"

	"github.com/alessio-palumbo/lifxlan-go/pkg/client"
	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/alessio-palumbo/lifxregistry-go/gen/registry"
)

func testRouter(t *testing.T) *Router {
	t.Helper()
	f, err := config.Defaults()
	if err != nil {
		t.Fatal(err)
	}
	v, err := f.Virtuals()
	if err != nil {
		t.Fatal(err)
	}
	return New(v, nil)
}
func message(p packets.Payload, target [8]byte) *protocol.Message {
	m := protocol.NewMessage(p)
	m.SetTarget(target)
	m.SetSource(123456)
	m.SetSequence(42)
	return m
}
func TestGetDuringTransitionAndTargetRouting(t *testing.T) {
	r := testRouter(t)
	now := time.Unix(100, 0)
	r.Clock = func() time.Time { return now }
	target := [8]byte(r.Devices[0].Device.Serial)
	r.Handle(message(&packets.LightSetColor{Color: packets.LightHsbk{}}, target))
	r.Handle(message(&packets.LightSetColor{Color: packets.LightHsbk{Brightness: 60000}, Duration: 1000}, target))
	now = now.Add(500 * time.Millisecond)
	out := r.Handle(message(&packets.LightGet{}, target))
	if len(out) != 1 || out[0].Payload.(*packets.LightState).Color.Brightness != 30000 {
		t.Fatal(out)
	}
	if r.Devices[1].State.First().Brightness != 32768 {
		t.Fatal("target isolation failed")
	}
	if out[0].Source() != 123456 || out[0].Sequence() != 42 || out[0].Target() != target {
		t.Fatal("metadata")
	}
	select {
	case <-r.Applied:
	default:
		t.Fatal("no applied event")
	}
	unknown := [8]byte{1, 2, 3, 4, 5, 6}
	if len(r.Handle(message(&packets.LightGet{}, unknown))) != 0 {
		t.Fatal("unknown target")
	}
	r.Devices[0].Enabled = false
	if len(r.Handle(message(&packets.LightGet{}, target))) != 0 {
		t.Fatal("disabled target")
	}
}
func TestUDPDiscoveryIdentityAndRoundTrips(t *testing.T) {
	r := testRouter(t)
	s, err := Listen("0.0.0.0:0", r)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()
	defer func() {
		cancel()
		s.Close()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	addr, _ := net.ResolveUDPAddr("udp4", s.Address())
	addr.IP = net.IPv4(127, 0, 0, 1)
	c, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	exchange := func(p packets.Payload, target [8]byte, n int) []*protocol.Message {
		t.Helper()
		b, err := message(p, target).MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = c.Write(b); err != nil {
			t.Fatal(err)
		}
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		out := []*protocol.Message{}
		for range n {
			buf := make([]byte, 4096)
			size, err := c.Read(buf)
			if err != nil {
				t.Fatal(err)
			}
			m := &protocol.Message{}
			if err = m.UnmarshalBinary(buf[:size]); err != nil {
				t.Fatal(err)
			}
			if m.Source() != 123456 || m.Sequence() != 42 {
				t.Fatal("response metadata")
			}
			out = append(out, m)
		}
		return out
	}
	discovered := exchange(&packets.DeviceGetService{}, protocol.TargetBroadcast, 3)
	seen := map[[8]byte]bool{}
	for _, m := range discovered {
		seen[m.Target()] = true
		if m.Payload.(*packets.DeviceStateService).Port != r.Port {
			t.Fatal("wrong service port")
		}
	}
	if len(seen) != 3 {
		t.Fatal("discovery did not report unique devices")
	}
	color := packets.LightHsbk{Hue: 12345, Saturation: 23456, Brightness: 34567, Kelvin: 4567}
	for _, v := range r.Devices {
		target := [8]byte(v.Device.Serial)
		m := exchange(&packets.DeviceGetVersion{}, target, 1)[0]
		pid := m.Payload.(*packets.DeviceStateVersion).Product
		if pid != v.Device.ProductID || !registry.ProductsByPID[int(pid)].Features.Color {
			t.Fatal("product/capabilities")
		}
		exchange(&packets.LightSetColor{Color: color}, target, 1)
		q := exchange(&packets.LightGet{}, target, 1)[0].Payload.(*packets.LightState)
		if q.Color != color {
			t.Fatal(q.Color)
		}
	}
	strip := [8]byte(r.Devices[1].Device.Serial)
	c2 := packets.LightHsbk{Hue: 111, Brightness: 222, Kelvin: 3333}
	exchange(&packets.MultiZoneSetColorZones{StartIndex: 2, EndIndex: 4, Color: c2, Apply: 0}, strip, 1)
	z := exchange(&packets.MultiZoneGetColorZones{StartIndex: 2, EndIndex: 2}, strip, 1)[0].Payload.(*packets.MultiZoneStateZone)
	if z.Color != color {
		t.Fatal("NO_APPLY")
	}
	exchange(&packets.MultiZoneSetColorZones{Apply: 2}, strip, 1)
	z = exchange(&packets.MultiZoneGetColorZones{StartIndex: 2, EndIndex: 2}, strip, 1)[0].Payload.(*packets.MultiZoneStateZone)
	if z.Color != c2 {
		t.Fatal("APPLY_ONLY")
	}
	p := &packets.MultiZoneExtendedSetColorZones{Index: 8, ColorsCount: 2, Apply: 1}
	p.Colors[0] = c2
	p.Colors[1] = color
	exchange(p, strip, 1)
	extended := exchange(&packets.MultiZoneExtendedGetColorZones{}, strip, 1)[0].Payload.(*packets.MultiZoneExtendedStateMultiZone)
	if extended.Count != 16 || extended.Colors[8] != c2 || extended.Colors[9] != color {
		t.Fatal("extended offset")
	}
	matrix := [8]byte(r.Devices[2].Device.Serial)
	chain := exchange(&packets.TileGetDeviceChain{}, matrix, 1)[0].Payload.(*packets.TileStateDeviceChain)
	if chain.TileDevicesCount != 5 || chain.TileDevices[0].Width != 8 {
		t.Fatal("chain metadata")
	}
	tile := &packets.TileSet64{TileIndex: 1, Length: 2, Rect: packets.TileBufferRect{X: 7, Y: 7, Width: 1}}
	tile.Colors[0] = c2
	exchange(tile, matrix, 2)
	states := exchange(&packets.TileGet64{TileIndex: 1, Length: 2, Rect: tile.Rect}, matrix, 2)
	for _, m := range states {
		if m.Payload.(*packets.TileState64).Colors[0] != c2 {
			t.Fatal("matrix partial update")
		}
	}
	// Match a client painting each Tile independently and verify every emitter.
	expected := make([]packets.LightHsbk, 5)
	for i := range 5 {
		expected[i] = packets.LightHsbk{Hue: uint16(10000 * i), Saturation: 60000, Brightness: 40000, Kelvin: 3500}
		p := &packets.TileSet64{TileIndex: uint8(i), Length: 1, Rect: packets.TileBufferRect{Width: 8}}
		for j := range p.Colors {
			p.Colors[j] = expected[i]
		}
		exchange(p, matrix, 1)
	}
	allTiles := exchange(&packets.TileGet64{Length: 5, Rect: packets.TileBufferRect{Width: 8}}, matrix, 5)
	for _, m := range allTiles {
		state := m.Payload.(*packets.TileState64)
		for _, c := range state.Colors {
			if c != expected[state.TileIndex] {
				t.Fatalf("Tile %d returned %+v, expected %+v", state.TileIndex, c, expected[state.TileIndex])
			}
		}
	}
	single := exchange(&packets.LightGet{}, [8]byte(r.Devices[0].Device.Serial), 1)[0].Payload.(*packets.LightState)
	if single.Color != color {
		t.Fatal("other target mutated")
	}
}
func TestBroadcastSetAndExtendedChunks(t *testing.T) {
	r := testRouter(t)
	color := packets.LightHsbk{Hue: 42}
	out := r.Handle(message(&packets.LightSetColor{Color: color}, protocol.TargetBroadcast))
	if len(out) != 3 {
		t.Fatal(out)
	}
	for _, v := range r.Devices {
		if v.State.First() != color {
			t.Fatal("broadcast missed device")
		}
	}
	r.Devices[1].State.Colors.Zones = make([]packets.LightHsbk, 200)
	out = r.Handle(message(&packets.MultiZoneExtendedGetColorZones{}, [8]byte(r.Devices[1].Device.Serial)))
	if len(out) != 3 || out[2].Payload.(*packets.MultiZoneExtendedStateMultiZone).Index != 164 {
		t.Fatal("chunking")
	}
}
func TestMalformedDatagramsAndInvalidSets(t *testing.T) {
	r := testRouter(t)
	target := [8]byte(r.Devices[2].Device.Serial)
	revision := r.Revision
	r.Handle(message(&packets.TileSet64{Length: 1}, target))
	if r.Revision != revision {
		t.Fatal("invalid matrix set accepted")
	}
	r.Handle(message(&packets.LightSetWaveform{Period: 0, Cycles: 1}, target))
	if r.Revision != revision {
		t.Fatal("invalid waveform accepted")
	}
}

// Exercise the actual consumer's discovery/classification session, not just packet decoding.
func TestLifxlanControllerClassifiesAllDevices(t *testing.T) {
	r := testRouter(t)
	s, err := Listen("127.0.0.1:0", r)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()
	defer func() { cancel(); s.Close(); <-done }()
	addr, _ := net.ResolveUDPAddr("udp4", s.Address())
	ctrl, err := controller.New(controller.WithClientConfig(&client.Config{BroadcastAddr: addr}), controller.WithPreflightHandshakeTimeout(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()
	events := ctrl.SubscribeDevices(ctx)
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatalf("classification timeout: %+v", ctrl.GetDevices())
		case <-events:
			ds := ctrl.GetDevices()
			if len(ds) != 3 {
				continue
			}
			ready := true
			for _, d := range ds {
				if !d.RegistryKnown || d.Label == "" || d.FirmwareVersion == "" || d.Location == "" || d.Group == "" {
					ready = false
				}
				switch d.LightType {
				case device.LightTypeMultiZone:
					if len(d.MultizoneProperties.Zones) != 16 {
						ready = false
					}
				case device.LightTypeMatrix:
					if d.MatrixProperties.ChainLength != 5 || d.MatrixProperties.Width != 8 {
						ready = false
					}
				}
			}
			if ready {
				return
			}
		}
	}
}

func TestActivityUsesGeneratedPayloadNames(t *testing.T) {
	r := testRouter(t)
	target := [8]byte(r.Devices[1].Device.Serial)
	r.Handle(message(&packets.MultiZoneExtendedGetColorZones{}, target))
	_, _, _, recent := r.Snapshots()
	a := recent[len(recent)-1]
	if a.Label != r.Devices[1].Device.Label || a.TypeName != "MultiZoneExtendedGetColorZones" || a.Type != uint16(packets.PayloadTypeMultiZoneExtendedGetColorZones) {
		t.Fatalf("unexpected activity: %+v", a)
	}
	r.Handle(message(&packets.LightSetColor{}, target))
	_, _, _, recent = r.Snapshots()
	a = recent[len(recent)-1]
	if a.TypeName != "LightSetColor" || !a.Applied {
		t.Fatalf("unexpected Set activity: %+v", a)
	}
}

// The official app sets reserved LIFXV2 bytes and a reserved flag. Neither
// changes the public GetService request or the UDP service value.
func TestCapturedAppDiscoveryAndTransportCounters(t *testing.T) {
	s, err := Listen("127.0.0.1:0", testRouter(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()
	defer func() {
		cancel()
		s.Close()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	addr, _ := net.ResolveUDPAddr("udp4", s.Address())
	c, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	request, err := hex.DecodeString("240000341100000000000000000000004c49465856320400000000000000000002000000")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write([]byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(request); err != nil {
		t.Fatal(err)
	}
	c.SetReadDeadline(time.Now().Add(time.Second))
	targets := map[[8]byte]bool{}
	for i := 0; i < 3; i++ {
		b := make([]byte, 4096)
		n, err := c.Read(b)
		if err != nil {
			t.Fatal(err)
		}
		m := &protocol.Message{}
		if err := m.UnmarshalBinary(b[:n]); err != nil {
			t.Fatal(err)
		}
		service, ok := m.Payload.(*packets.DeviceStateService)
		if !ok || service.Service != 1 || m.Source() != 17 {
			t.Fatalf("unexpected response: %#v", m)
		}
		targets[m.Target()] = true
	}
	if len(targets) != 3 {
		t.Fatal("discovery targets not unique")
	}
	// A response may reach the reader just before the sender updates its counter.
	deadline := time.Now().Add(time.Second)
	for s.Stats().Replies != 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	stats := s.Stats()
	if stats.Received != 2 || stats.Decoded != 1 || stats.Invalid != 1 || stats.Replies != 3 || stats.SendErrors != 0 || stats.LastPeer == "" {
		t.Fatalf("stats: %+v", stats)
	}
}

func TestUDPPublicAppQueriesReturnCorrelatedStates(t *testing.T) {
	r := testRouter(t)
	s, err := Listen("127.0.0.1:0", r)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()
	defer func() {
		cancel()
		s.Close()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	addr, _ := net.ResolveUDPAddr("udp4", s.Address())
	c, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	type queryCase struct {
		name           string
		index          int
		request, state packets.Payload
	}
	cases := []queryCase{}
	for index := range r.Devices {
		cases = append(cases,
			queryCase{"group", index, &packets.DeviceGetGroup{}, &packets.DeviceStateGroup{}},
			queryCase{"location", index, &packets.DeviceGetLocation{}, &packets.DeviceStateLocation{}},
		)
	}
	cases = append(cases,
		queryCase{"strip effect", 1, &packets.MultiZoneGetEffect{}, &packets.MultiZoneStateEffect{}},
		queryCase{"matrix effect", 2, &packets.TileGetEffect{}, &packets.TileStateEffect{}},
	)
	for index, tc := range cases {
		t.Run(tc.name+"/"+r.Devices[tc.index].Device.Serial.String(), func(t *testing.T) {
			target := [8]byte(r.Devices[tc.index].Device.Serial)
			m := message(tc.request, target)
			m.SetSource(17)
			m.SetSequence(uint8(index))
			b, err := m.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}
			copy(b[16:22], "LIFXV2")
			b[22] |= 4
			if _, err := c.Write(b); err != nil {
				t.Fatal(err)
			}
			c.SetReadDeadline(time.Now().Add(time.Second))
			reply := make([]byte, 4096)
			n, peer, err := c.ReadFromUDP(reply)
			if err != nil {
				t.Fatal(err)
			}
			state := &protocol.Message{}
			if err := state.UnmarshalBinary(reply[:n]); err != nil {
				t.Fatal(err)
			}
			if peer.Port != addr.Port || !peer.IP.Equal(addr.IP) || state.Type() != tc.state.PayloadType() || state.Target() != target || state.Source() != 17 || state.Sequence() != uint8(index) {
				t.Fatalf("reply correlation: peer=%s state=%s", peer, state)
			}
			switch p := state.Payload.(type) {
			case *packets.DeviceStateGroup:
				if p.Group == [16]byte{} || p.Label == [32]byte{} {
					t.Fatal("empty group metadata")
				}
			case *packets.DeviceStateLocation:
				if p.Location == [16]byte{} || p.Label == [32]byte{} {
					t.Fatal("empty location metadata")
				}
			case *packets.MultiZoneStateEffect:
				if p.Settings.Type != 0 {
					t.Fatal("effect is not OFF")
				}
			case *packets.TileStateEffect:
				if p.Settings.Type != 0 {
					t.Fatal("effect is not OFF")
				}
			}
		})
	}
	deadline := time.Now().Add(time.Second)
	for s.Stats().Replies != uint64(len(cases)) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	_, _, _, recent := r.Snapshots()
	if len(recent) != 2*len(cases) {
		t.Fatalf("missing RX/TX entries: %+v", recent)
	}
	for i := 0; i < len(recent); i += 2 {
		rx, tx := recent[i], recent[i+1]
		if rx.Direction != "RX" || rx.Replies != 1 || tx.Direction != "TX" || tx.Error != "" || rx.Peer != c.LocalAddr().String() || rx.Peer != tx.Peer || rx.Source != tx.Source || rx.Sequence != tx.Sequence || rx.Target != tx.Target {
			t.Fatalf("exchange mismatch: RX=%+v TX=%+v", rx, tx)
		}
	}
}

func TestResponseDiagnosticsForNoReplyAndSendFailure(t *testing.T) {
	r := testRouter(t)
	m := message(&packets.TileGetEffect{}, [8]byte(r.Devices[0].Device.Serial))
	if got := r.HandleFrom(m, "192.0.2.1:1234"); len(got) != 0 {
		t.Fatal("bulb answered matrix effect query")
	}
	rx := r.Recent[len(r.Recent)-1]
	if rx.Direction != "RX" || rx.Replies != 0 || rx.Peer != "192.0.2.1:1234" {
		t.Fatalf("no-reply diagnostic: %+v", rx)
	}
	unknown := message(&packets.DeviceGetGroup{}, [8]byte{2, 3, 4, 5, 6, 7})
	r.HandleFrom(unknown, "192.0.2.1:1234")
	if r.Recent[len(r.Recent)-1].Error == "" {
		t.Fatal("unknown target not reported")
	}
	reply := message(&packets.DeviceStateGroup{}, [8]byte(r.Devices[0].Device.Serial))
	r.RecordSend(reply, "192.0.2.1:1234", errors.New("write denied"))
	tx := r.Recent[len(r.Recent)-1]
	if tx.Direction != "TX" || tx.Error != "write denied" || tx.Source != reply.Source() || tx.Sequence != reply.Sequence() || tx.Label != r.Devices[0].Device.Label {
		t.Fatalf("send-failure diagnostic: %+v", tx)
	}
}
