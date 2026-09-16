package emulator

import (
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

var epoch = time.Unix(1000, 0)

func bulb() *VirtualDevice { d := device.Device{}; d.SetProductInfo(27); return New(d, true) }
func closeTo(t *testing.T, a, b uint16) {
	t.Helper()
	if int(a)-int(b) > 1 || int(b)-int(a) > 1 {
		t.Fatalf("got %d want %d", a, b)
	}
}
func TestImmediateLossless(t *testing.T) {
	s := bulb().State
	c := Color{Hue: 12345, Saturation: 23456, Brightness: 34567, Kelvin: 4567}
	s.Whole(c, 0, epoch)
	if s.First() != c {
		t.Fatal(s.First())
	}
	f, _ := effects.AdaptPhysicalColorStateToFrame(s.Colors, device.SurfaceFromDevice(bulb().Device), 0)
	if f.Colors[0].ToDeviceColor() != c {
		t.Fatal("lost precision")
	}
}
func TestTransitionMidpointCompletionAndInterruption(t *testing.T) {
	s := bulb().State
	s.Whole(Color{}, 0, epoch)
	s.Whole(Color{Brightness: 60000, Kelvin: 4000}, 1000, epoch)
	s.Evaluate(epoch.Add(500 * time.Millisecond))
	closeTo(t, s.First().Brightness, 30000)
	closeTo(t, s.First().Kelvin, 2000)
	s.Whole(Color{Brightness: 10000, Kelvin: 4000}, 1000, epoch.Add(500*time.Millisecond))
	s.Evaluate(epoch.Add(time.Second))
	closeTo(t, s.First().Brightness, 20000)
	s.Evaluate(epoch.Add(1500 * time.Millisecond))
	if s.First().Brightness != 10000 || s.Active() {
		t.Fatal(s.First())
	}
}
func TestHueWraparound(t *testing.T) {
	for _, pair := range [][2]uint16{{65000, 1000}, {1000, 65000}} {
		c := interpolate(Color{Hue: pair[0]}, Color{Hue: pair[1]}, .5)
		if c.Hue > 2000 && c.Hue < 64000 {
			t.Fatal(c)
		}
	}
}
func TestPowerEnvelope(t *testing.T) {
	s := bulb().State
	c := s.First()
	s.SetPower(0, 1000, epoch)
	s.Evaluate(epoch.Add(500 * time.Millisecond))
	closeTo(t, s.Power, 32768)
	if s.First() != c {
		t.Fatal("power destroyed color")
	}
	s.SetPower(65535, 1000, epoch.Add(500*time.Millisecond))
	s.Evaluate(epoch.Add(time.Second))
	closeTo(t, s.Power, 49152)
	s.Evaluate(epoch.Add(1500 * time.Millisecond))
	if s.Power != 65535 || s.Active() {
		t.Fatal(s.Power)
	}
}
func TestBufferedZonesAndOffsets(t *testing.T) {
	d := device.Device{}
	d.SetProductInfo(32)
	d.MultizoneProperties.Zones = make([]Color, 6)
	s := New(d, true).State
	original := s.Colors.Zones[2]
	a := Color{Hue: 123}
	b := Color{Hue: 456}
	s.Zones(2, []Color{a, a}, 0, 0, epoch)
	if s.Colors.Zones[2] != original {
		t.Fatal("NO_APPLY mutated state")
	}
	s.Zones(4, []Color{b}, 1, 0, epoch)
	if s.Colors.Zones[2] != a || s.Colors.Zones[3] != a || s.Colors.Zones[4] != b {
		t.Fatal(s.Colors.Zones)
	}
	s.Zones(0, []Color{b}, 0, 0, epoch)
	s.Zones(0, []Color{a}, 2, 0, epoch)
	if s.Colors.Zones[0] != b {
		t.Fatal("APPLY_ONLY wrote packet color")
	}
	s.Whole(a, 0, epoch)
	for _, c := range s.Colors.Zones {
		if c != a {
			t.Fatal("whole-device set missed zone")
		}
	}
}
func TestPerZoneTransitions(t *testing.T) {
	d := device.Device{LightType: device.LightTypeMultiZone, MultizoneProperties: device.MultizoneProperties{Zones: make([]Color, 2)}}
	s := New(d, true).State
	s.Whole(Color{}, 0, epoch)
	s.SetColors(map[int]Color{0: {Brightness: 60000}}, 1000, epoch)
	s.SetColors(map[int]Color{1: {Brightness: 40000}}, 1000, epoch.Add(500*time.Millisecond))
	s.Evaluate(epoch.Add(time.Second))
	if s.Colors.Zones[0].Brightness != 60000 || s.Colors.Zones[1].Brightness != 20000 {
		t.Fatal(s.Colors.Zones)
	}
}
func TestMatrixPartialChainsOrientation(t *testing.T) {
	d := device.Device{}
	d.SetProductInfo(55)
	d.MatrixProperties = device.MatrixProperties{Width: 8, Height: 8, NZones: 64, ChainLength: 2, ChainZones: [][]Color{make([]Color, 64), make([]Color, 64)}, ChainOrientations: []device.Orientation{device.OrientationLeft, device.OrientationUpsideDown}}
	v := New(d, true)
	before := v.State.Colors.MatrixChains[0][0]
	p := &packets.TileSet64{TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{X: 7, Y: 7, Width: 2}}
	p.Colors[0] = Color{Hue: 1234, Brightness: 50000}
	p.Colors[1] = Color{Hue: 999}
	if !v.Matrix(p, epoch) {
		t.Fatal("rejected")
	}
	for _, c := range v.State.Colors.MatrixChains {
		if c[0] != before || c[63] != p.Colors[0] {
			t.Fatal(c)
		}
	}
	f := v.Frame(epoch)
	for c := range 2 {
		expected := device.PhysicalMatrixColorsToLogical(8, 8, d.MatrixProperties.ChainOrientations[c], v.State.Colors.MatrixChains[c])
		for i, e := range expected {
			x := i%8 + c*8
			y := i / 8
			if f.Colors[y*f.Width+x].ToDeviceColor() != e {
				t.Fatalf("chain %d pixel %d", c, i)
			}
		}
	}
	v.State.Whole(Color{Hue: 42}, 0, epoch)
	for _, c := range v.State.flat() {
		if c.Hue != 42 {
			t.Fatal(c)
		}
	}
}
func TestHiddenCellsAndCapsuleSurface(t *testing.T) {
	for _, pid := range []uint32{57, 176, 201, 219} {
		d := device.Device{}
		d.SetProductInfo(pid)
		w, h := 8, 8
		if pid == 57 {
			w, h = 5, 6
		}
		if pid == 201 {
			w, h = 8, 16
		}
		if pid == 219 {
			w, h = 7, 5
		}
		d.MatrixProperties = device.MatrixProperties{Width: w, Height: h, NZones: w * h, ChainLength: 1, ChainZones: [][]Color{make([]Color, w*h)}}
		v := New(d, true)
		v.State.Whole(Color{Brightness: 65535, Kelvin: 3500}, 0, epoch)
		f := v.Frame(epoch)
		surface := device.SurfaceFromDevice(d)
		for _, chain := range surface.Matrix.Chains {
			for y, row := range chain.Rows {
				for _, x := range row.HiddenCols {
					if f.Colors[(chain.Bounds.Y+y)*f.Width+chain.Bounds.X+row.Offset+x].Brightness != 0 {
						t.Fatal("hidden cell emitted")
					}
				}
			}
		}
	}
}
func TestWaveformsAndCompletion(t *testing.T) {
	values := []uint16{15000, 30000, 22961, 30000, 60000}
	for kind := uint8(0); kind < 5; kind++ {
		for _, transient := range []bool{false, true} {
			s := bulb().State
			s.Whole(Color{}, 0, epoch)
			if !s.Wave(Color{Brightness: 60000}, 1000, 2, 0, kind, transient, [4]bool{true, true, true, true}, epoch) {
				t.Fatal("rejected")
			}
			s.Evaluate(epoch.Add(250 * time.Millisecond))
			closeTo(t, s.First().Brightness, values[kind])
			s.Evaluate(epoch.Add(1250 * time.Millisecond))
			closeTo(t, s.First().Brightness, values[kind])
			s.Evaluate(epoch.Add(2 * time.Second))
			expected := uint16(60000)
			if transient || kind == 1 || kind == 3 {
				expected = 0
			}
			if s.First().Brightness != expected || s.Active() {
				t.Fatalf("kind %d transient %v got %+v", kind, transient, s.First())
			}
		}
	}
}
func TestWaveformMasksSkewAndInterruption(t *testing.T) {
	s := bulb().State
	base := Color{Hue: 12345, Saturation: 34567, Brightness: 10000, Kelvin: 4567}
	s.Whole(base, 0, epoch)
	s.Wave(Color{Hue: 1, Saturation: 2, Brightness: 60000, Kelvin: 3}, 1000, 1, 16383, 4, true, [4]bool{false, false, true, false}, epoch)
	c := s.First()
	if c.Hue != base.Hue || c.Saturation != base.Saturation || c.Kelvin != base.Kelvin || c.Brightness != 60000 {
		t.Fatal(c)
	}
	s.Evaluate(epoch.Add(300 * time.Millisecond))
	if s.First() != base {
		t.Fatal("pulse skew")
	}
	s.Whole(base, 0, epoch.Add(time.Second))
	s.Wave(Color{Brightness: 60000}, 1000, 1, 0, 0, false, [4]bool{false, false, true, false}, epoch.Add(time.Second))
	s.Whole(Color{Brightness: 20000}, 1000, epoch.Add(1500*time.Millisecond))
	s.Evaluate(epoch.Add(2 * time.Second))
	closeTo(t, s.First().Brightness, 27500)
}
func TestOptionalFieldsIndividually(t *testing.T) {
	for field := range 4 {
		s := bulb().State
		base := Color{Hue: 100, Saturation: 200, Brightness: 300, Kelvin: 400}
		target := Color{Hue: 1000, Saturation: 2000, Brightness: 3000, Kelvin: 4000}
		s.Whole(base, 0, epoch)
		mask := [4]bool{}
		mask[field] = true
		s.Wave(target, 1000, 1, 0, 0, false, mask, epoch)
		s.Evaluate(epoch.Add(time.Second))
		a := s.First()
		expected := base
		switch field {
		case 0:
			expected.Hue = target.Hue
		case 1:
			expected.Saturation = target.Saturation
		case 2:
			expected.Brightness = target.Brightness
		case 3:
			expected.Kelvin = target.Kelvin
		}
		if a != expected {
			t.Fatal(a, expected)
		}
	}
}

func TestWaveformAllZonesAndMatrixChains(t *testing.T) {
	for _, kind := range []device.LightType{device.LightTypeSingleZone, device.LightTypeMultiZone, device.LightTypeMatrix} {
		d := device.Device{LightType: kind}
		if kind == device.LightTypeMultiZone {
			d.MultizoneProperties.Zones = make([]Color, 6)
		}
		if kind == device.LightTypeMatrix {
			d.MatrixProperties = device.MatrixProperties{Width: 8, Height: 8, NZones: 64, ChainLength: 2, ChainZones: [][]Color{make([]Color, 64), make([]Color, 64)}}
		}
		v := New(d, true)
		base := Color{Hue: 12345, Saturation: 23456, Brightness: 10000, Kelvin: 4567}
		v.State.Whole(base, 0, epoch)
		v.State.Wave(Color{Brightness: 50000}, 1000, 1, 0, 0, true, [4]bool{false, false, true, false}, epoch)
		v.State.Evaluate(epoch.Add(500 * time.Millisecond))
		for _, c := range v.State.flat() {
			if c.Brightness != 30000 || c.Hue != base.Hue || c.Saturation != base.Saturation || c.Kelvin != base.Kelvin {
				t.Fatal(kind, c)
			}
		}
		v.State.Evaluate(epoch.Add(time.Second))
		for _, c := range v.State.flat() {
			if c != base {
				t.Fatal("transient restore", kind, c)
			}
		}
	}
}
