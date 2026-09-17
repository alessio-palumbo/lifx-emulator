package lan

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"lifx-emulator/internal/config"
)

// Synthetic types and arbitrary fixture bytes keep real local definitions out
// of the repository, while exercising exactly the same UDP path.
func localRules() config.ResponseFile {
	return config.ResponseFile{Responses: []config.ResponseRule{
		{RequestType: 60000, ResponseType: 60001, RequestName: "LocalQuery", ResponseName: "LocalState", Parts: []config.ResponsePart{{Query: "DeviceGetLabel"}, {Hex: "feedcafe"}}},
		{RequestType: 60002, RequestSize: 3, ResponseType: 60003, Parts: []config.ResponsePart{{Hex: "01020304"}}},
	}}
}

func TestUDPLocalResponsesAndCorrelation(t *testing.T) {
	r := testRouter(t)
	if err := r.SetResponses(localRules()); err != nil {
		t.Fatal(err)
	}
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
	send := func(kind uint16, target [8]byte, data []byte) {
		t.Helper()
		m := message(&opaquePayload{kind: kind, data: data}, target)
		m.SetSource(987654321)
		m.SetSequence(253)
		wire, err := m.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Write(wire); err != nil {
			t.Fatal(err)
		}
	}
	read := func(kind uint16) (*protocol.Message, []byte) {
		t.Helper()
		c.SetReadDeadline(time.Now().Add(time.Second))
		wire := make([]byte, maxDatagramSize)
		n, peer, err := c.ReadFromUDP(wire)
		if err != nil {
			t.Fatal(err)
		}
		m := &protocol.Message{}
		// Library framing is decoded even when no public payload factory exists.
		if err := m.UnmarshalBinary(wire[:n]); err == nil {
			t.Fatal("synthetic type unexpectedly became public")
		}
		if m.Type() != kind || m.Source() != 987654321 || m.Sequence() != 253 || peer.Port != addr.Port || !peer.IP.Equal(addr.IP) {
			t.Fatalf("wrong local reply metadata: %s %s", peer, m)
		}
		return m, wire[36:n]
	}
	send(60000, protocol.TargetBroadcast, nil)
	seen := map[[8]byte]bool{}
	for range r.Devices {
		m, data := read(60001)
		seen[m.Target()] = true
		if len(data) != 36 || !bytes.Equal(data[32:], []byte{0xfe, 0xed, 0xca, 0xfe}) {
			t.Fatalf("wrong composed payload: %x", data)
		}
		expected := ""
		for _, v := range r.Devices {
			if [8]byte(v.Device.Serial) == m.Target() {
				expected = v.Device.Label
			}
		}
		if expected == "" || string(bytes.TrimRight(data[:32], "\x00")) != expected {
			t.Fatal("public query part does not reflect addressed device")
		}
	}
	if len(seen) != len(r.Devices) || r.Revision != 0 {
		t.Fatal("target isolation or unexpected state mutation")
	}
	target := [8]byte(r.Devices[1].Device.Serial)
	r.Handle(message(&packets.DeviceSetLabel{Label: label("Changed local label")}, target))
	send(60000, target, nil)
	m, data := read(60001)
	if m.Target() != target || string(bytes.TrimRight(data[:32], "\x00")) != "Changed local label" {
		t.Fatal("composed metadata is stale")
	}
	send(60002, target, []byte{1, 2, 3})
	m, data = read(60003)
	if m.Target() != target || !bytes.Equal(data, []byte{1, 2, 3, 4}) {
		t.Fatal("opaque replay is incorrect")
	}
	for _, kind := range []uint16{60002, 60004} {
		send(kind, target, nil)
		c.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		if _, err := c.Read(make([]byte, maxDatagramSize)); err == nil {
			t.Fatal("unexpected reply to unsupported or wrong-size request")
		}
	}
	stats := s.Stats()
	if stats.Invalid != 0 || stats.Decoded != 5 {
		t.Fatalf("unknown valid payloads classified as malformed: %+v", stats)
	}
	_, _, _, recent := r.Snapshots()
	last := recent[len(recent)-1]
	if last.Direction != "RX" || last.TypeName != "Type60004" || last.Error == "" || last.Replies != 0 {
		t.Fatalf("missing unsupported diagnostic: %+v", last)
	}
	found := false
	for _, a := range recent {
		if a.Direction == "TX" && a.Type == 60001 && a.TypeName == "LocalState" {
			found = true
		}
	}
	if !found {
		t.Fatal("local State name missing from TX activity")
	}
}

func TestLocalResponseValidation(t *testing.T) {
	publicKind := (&packets.DeviceGetLabel{}).PayloadType()
	cases := []struct {
		name  string
		rules []config.ResponseRule
	}{
		{"public collision", []config.ResponseRule{{RequestType: publicKind, ResponseType: 60001}}},
		{"duplicate", []config.ResponseRule{{RequestType: 60000, ResponseType: 60001}, {RequestType: 60000, ResponseType: 60002}}},
		{"zero request", []config.ResponseRule{{ResponseType: 60001}}},
		{"zero response", []config.ResponseRule{{RequestType: 60000}}},
		{"oversize request", []config.ResponseRule{{RequestType: 60000, ResponseType: 60001, RequestSize: maxLocalPayloadSize + 1}}},
		{"invalid hex", []config.ResponseRule{{RequestType: 60000, ResponseType: 60001, Parts: []config.ResponsePart{{Hex: "not hex"}}}}},
		{"both sources", []config.ResponseRule{{RequestType: 60000, ResponseType: 60001, Parts: []config.ResponsePart{{Hex: "aa", Query: "DeviceGetLabel"}}}}},
		{"empty source", []config.ResponseRule{{RequestType: 60000, ResponseType: 60001, Parts: []config.ResponsePart{{}}}}},
		{"unknown query", []config.ResponseRule{{RequestType: 60000, ResponseType: 60001, Parts: []config.ResponsePart{{Query: "MissingGet"}}}}},
		{"State query", []config.ResponseRule{{RequestType: 60000, ResponseType: 60001, Parts: []config.ResponsePart{{Query: "DeviceStateLabel"}}}}},
		{"query requires payload", []config.ResponseRule{{RequestType: 60000, ResponseType: 60001, Parts: []config.ResponsePart{{Query: "TileGet64"}}}}},
		{"oversize response", []config.ResponseRule{{RequestType: 60000, ResponseType: 60001, Parts: []config.ResponsePart{{Hex: strings.Repeat("aa", maxLocalPayloadSize+1)}}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := testRouter(t)
			if err := r.SetResponses(localRules()); err != nil {
				t.Fatal(err)
			}
			if err := r.SetResponses(config.ResponseFile{Responses: tc.rules}); err == nil {
				t.Fatal("invalid rule accepted")
			}
			if len(r.localResponses) != 2 {
				t.Fatal("invalid configuration replaced working rules")
			}
		})
	}
}

func TestLocalCompositionErrorsAndDisabledTargets(t *testing.T) {
	r := testRouter(t)
	file := config.ResponseFile{Responses: []config.ResponseRule{{RequestType: 60000, ResponseType: 60001, Parts: []config.ResponsePart{{Query: "TileGetDeviceChain"}}}}}
	if err := r.SetResponses(file); err != nil {
		t.Fatal(err)
	}
	target := [8]byte(r.Devices[0].Device.Serial)
	m := message(&opaquePayload{kind: 60000}, target)
	if out := r.Handle(m); len(out) != 0 {
		t.Fatal("matrix-only public query composed for bulb")
	}
	if r.Recent[len(r.Recent)-1].Error == "" {
		t.Fatal("composition failure hidden")
	}
	r.Devices[2].Enabled = false
	m.SetTarget([8]byte(r.Devices[2].Device.Serial))
	if out := r.Handle(m); len(out) != 0 {
		t.Fatal("disabled device answered local query")
	}
	// A composed response can exceed the limit even if every part is valid alone.
	parts := make([]config.ResponsePart, 130)
	for i := range parts {
		parts[i].Query = "DeviceGetLabel"
	}
	file.Responses[0].Parts = parts
	if err := r.SetResponses(file); err != nil {
		t.Fatal(err)
	}
	m.SetTarget(target)
	if out := r.Handle(m); len(out) != 0 {
		t.Fatal("oversize composition was transmitted")
	}
	if !strings.Contains(r.Recent[len(r.Recent)-1].Error, fmt.Sprint(maxLocalPayloadSize)) {
		t.Fatal("size error missing")
	}
}
