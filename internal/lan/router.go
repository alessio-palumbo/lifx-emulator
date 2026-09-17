// Package lan owns routing and deterministic LAN response policy.
package lan

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"lifx-emulator/internal/emulator"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// Packet names come from generated library payload types, keeping the UI free
// of a duplicated protocol dictionary or a dependency on numeric type values.
var payloadNames = func() map[uint16]string {
	names := make(map[uint16]string, len(packets.Payloads))
	for id, newPayload := range packets.Payloads {
		names[id] = reflect.TypeOf(newPayload()).Elem().Name()
	}
	return names
}()

type Activity struct {
	Direction string
	Peer      string
	Source    uint32
	Sequence  uint8
	Replies   int
	Error     string
	At        time.Time
	Target    string
	Type      uint16
	TypeName  string
	Label     string
	Applied   bool
}
type Snapshot struct {
	Serial  string
	Label   string
	Product uint32
	Model   string
	Kind    string
	Enabled bool
	Power   uint16
	Surface device.Surface
	Colors  []device.Color
	Active  bool
}
type Router struct {
	mu       sync.Mutex
	Devices  []*emulator.VirtualDevice
	Clock    func() time.Time
	Port     uint32
	Revision uint64
	Recent   []Activity
	// Applied is a bounded invalidation stream: accepted Sets publish immediately,
	// consumers fetch coalesced snapshots. Packet handling never waits for the UI.
	Applied chan struct{}
}

func New(devices []*emulator.VirtualDevice, clock func() time.Time) *Router {
	if clock == nil {
		clock = time.Now
	}
	return &Router{Devices: devices, Clock: clock, Port: 56700, Applied: make(chan struct{}, 1)}
}
func (r *Router) changed() {
	r.Revision++
	select {
	case r.Applied <- struct{}{}:
	default:
	}
}
func (r *Router) Replace(devices []*emulator.VirtualDevice) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Devices = devices
	r.changed()
}
func (r *Router) SetPort(port uint32) { r.mu.Lock(); defer r.mu.Unlock(); r.Port = port }
func (r *Router) Snapshots() ([]Snapshot, uint64, bool, []Activity) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.Clock()
	out := []Snapshot{}
	active := false
	for _, v := range r.Devices {
		f := v.Frame(now)
		a := v.Enabled && v.State.Active()
		active = active || a
		out = append(out, Snapshot{v.Device.Serial.String(), v.Device.Label, v.Device.ProductID, v.Device.RegistryName, v.Device.LightType.String(), v.Enabled, v.State.Power, device.SurfaceFromDevice(v.Device), f.Colors, a})
	}
	return out, r.Revision, active, append([]Activity(nil), r.Recent...)
}
func (r *Router) Handle(m *protocol.Message) []*protocol.Message {
	return r.HandleFrom(m, "")
}

func (r *Router) HandleFrom(m *protocol.Message, peer string) []*protocol.Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.Clock()
	var responses []*protocol.Message
	matched := false
	for _, v := range r.Devices {
		if !v.Enabled || (m.Target() != protocol.TargetBroadcast && m.Target() != [8]byte(v.Device.Serial)) {
			continue
		}
		matched = true
		v.State.Evaluate(now)
		applied := v.Apply(m.Payload, now)
		if applied {
			r.changed()
		}
		payloads := r.respond(v, m.Payload, applied)
		for _, p := range payloads {
			out := protocol.NewMessage(p)
			out.SetTarget([8]byte(v.Device.Serial))
			out.SetSource(m.Source())
			out.SetSequence(m.Sequence())
			responses = append(responses, out)
		}
		r.activity(Activity{Direction: "RX", Peer: peer, Source: m.Source(), Sequence: m.Sequence(), Replies: len(payloads), At: now, Target: v.Device.Serial.String(), Type: m.Type(), TypeName: payloadNames[m.Type()], Label: v.Device.Label, Applied: applied})
	}
	if !matched {
		r.activity(Activity{Direction: "RX", Peer: peer, Source: m.Source(), Sequence: m.Sequence(), At: now, Target: device.Serial(m.Target()).String(), Type: m.Type(), TypeName: payloadNames[m.Type()], Error: "No enabled device matches target"})
	}
	return responses
}

// activity is called while r.mu is held; the UI reads it at its render cadence.
func (r *Router) activity(a Activity) {
	r.Recent = append(r.Recent, a)
	if len(r.Recent) > 80 {
		r.Recent = r.Recent[len(r.Recent)-80:]
	}
}

// RecordSend records a completed write, not a claim that the peer received it.
func (r *Router) RecordSend(m *protocol.Message, peer string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := Activity{Direction: "TX", Peer: peer, Source: m.Source(), Sequence: m.Sequence(), At: r.Clock(), Target: device.Serial(m.Target()).String(), Type: m.Type(), TypeName: payloadNames[m.Type()]}
	for _, v := range r.Devices {
		if v.Device.Serial == device.Serial(m.Target()) {
			a.Label = v.Device.Label
			break
		}
	}
	if err != nil {
		a.Error = err.Error()
	}
	r.activity(a)
}

func (r *Router) Update(serial, replacement, labelText string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	parsed, err := device.SerialFromHex(replacement)
	if err != nil || parsed.IsNil() {
		return fmt.Errorf("invalid serial")
	}
	if len(labelText) > 32 {
		return fmt.Errorf("label exceeds 32 bytes")
	}
	for _, v := range r.Devices {
		if v.Device.Serial.String() != serial && v.Device.Serial == parsed {
			return fmt.Errorf("duplicate serial")
		}
	}
	for _, v := range r.Devices {
		if v.Device.Serial.String() == serial {
			if v.Enabled && v.Device.Serial != parsed {
				return fmt.Errorf("disable device before editing serial")
			}
			v.Device.Serial = parsed
			v.Device.Label = labelText
			v.Enabled = enabled
			r.changed()
			return nil
		}
	}
	return fmt.Errorf("device not found")
}
func (r *Router) Add(v *emulator.VirtualDevice) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range r.Devices {
		if d.Device.Serial == v.Device.Serial {
			return fmt.Errorf("duplicate serial")
		}
	}
	r.Devices = append(r.Devices, v)
	r.changed()
	return nil
}
func (r *Router) Remove(serial string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, v := range r.Devices {
		if v.Device.Serial.String() == serial {
			r.Devices = append(r.Devices[:i], r.Devices[i+1:]...)
			r.changed()
			return
		}
	}
}
