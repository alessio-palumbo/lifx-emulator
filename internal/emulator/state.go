// Package emulator evaluates virtual light state without transport or UI dependencies.
package emulator

import (
	"math"
	"slices"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

type Color = packets.LightHsbk

// Physical storage preserves all 16 bits, unlike device.NewColor's rounded conversion.
type LightState struct {
	Colors  effects.PhysicalColorState
	pending map[int]Color
	moves   map[int]transition
	wave    *waveform
	Power   uint16
	power   *powerTransition
}
type transition struct {
	from, to   Color
	start, end time.Time
}
type powerTransition struct {
	from, to   uint16
	start, end time.Time
}
type waveform struct {
	from, to  []Color
	start     time.Time
	period    time.Duration
	cycles    float64
	kind      uint8
	skew      float64
	transient bool
}
type VirtualDevice struct {
	Device            device.Device
	State             *LightState
	Enabled           bool
	LocationUpdatedAt uint64
	GroupUpdatedAt    uint64
}

func New(d device.Device, enabled bool) *VirtualDevice {
	s := &LightState{Colors: effects.NewPhysicalColorState(device.SurfaceFromDevice(d)), Power: 65535, pending: map[int]Color{}, moves: map[int]transition{}}
	v := &VirtualDevice{Device: d, State: s, Enabled: enabled}
	for i := range s.flat() {
		s.put(i, Color{Brightness: 32768, Kelvin: 3500})
	}
	return v
}
func (s *LightState) flat() []Color {
	if len(s.Colors.MatrixChains) == 0 {
		return slices.Clone(s.Colors.Zones)
	}
	var a []Color
	for _, c := range s.Colors.MatrixChains {
		a = append(a, c...)
	}
	return a
}
func (s *LightState) put(i int, c Color) {
	if len(s.Colors.MatrixChains) == 0 {
		s.Colors.Zones[i] = c
		return
	}
	for _, a := range s.Colors.MatrixChains {
		if i < len(a) {
			a[i] = c
			return
		}
		i -= len(a)
	}
}
func fraction(now, start, end time.Time) float64 {
	if !end.After(start) {
		return 1
	}
	return max(0, min(1, float64(now.Sub(start))/float64(end.Sub(start))))
}
func lerp(a, b uint16, t float64) uint16 {
	return uint16(math.Round(float64(a) + (float64(b)-float64(a))*t))
}
func interpolate(a, b Color, t float64) Color {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	delta := float64(b.Hue) - float64(a.Hue)
	if delta > 32768 {
		delta -= 65536
	}
	if delta < -32768 {
		delta += 65536
	}
	h := math.Mod(math.Round(float64(a.Hue)+delta*t)+65536, 65536)
	return Color{Hue: uint16(h), Saturation: lerp(a.Saturation, b.Saturation, t), Brightness: lerp(a.Brightness, b.Brightness, t), Kelvin: lerp(a.Kelvin, b.Kelvin, t)}
}
func (s *LightState) Evaluate(now time.Time) {
	for i, m := range s.moves {
		t := fraction(now, m.start, m.end)
		s.put(i, interpolate(m.from, m.to, t))
		if t >= 1 {
			delete(s.moves, i)
		}
	}
	if p := s.power; p != nil {
		t := fraction(now, p.start, p.end)
		s.Power = lerp(p.from, p.to, t)
		if t >= 1 {
			s.power = nil
		}
	}
	if w := s.wave; w != nil {
		phase := max(0, float64(now.Sub(w.start))/float64(w.period))
		done := phase >= w.cycles
		t := waveValue(w.kind, math.Mod(phase, 1), w.skew)
		if done {
			t = 1
			if w.transient || w.kind == 1 || w.kind == 3 {
				t = 0
			}
			s.wave = nil
		}
		for i := range w.from {
			s.put(i, interpolate(w.from[i], w.to[i], t))
		}
	}
}
func waveValue(kind uint8, p, skew float64) float64 {
	switch kind {
	case 0:
		return p
	case 1:
		return (1 - math.Cos(2*math.Pi*p)) / 2
	case 2:
		return math.Sin(math.Pi * p / 2)
	case 3:
		return 1 - math.Abs(2*p-1)
	case 4:
		if p < 1-skew {
			return 1
		}
		return 0
	}
	return 0
}
func (s *LightState) Active() bool { return len(s.moves) > 0 || s.power != nil || s.wave != nil }
func (s *LightState) SetColors(updates map[int]Color, duration uint32, now time.Time) {
	s.Evaluate(now)
	s.wave = nil
	current := s.flat()
	for i, c := range updates {
		if i < 0 || i >= len(current) {
			continue
		}
		delete(s.moves, i)
		if duration == 0 {
			s.put(i, c)
		} else {
			s.moves[i] = transition{current[i], c, now, now.Add(time.Duration(duration) * time.Millisecond)}
		}
	}
}
func (s *LightState) Whole(c Color, duration uint32, now time.Time) {
	u := map[int]Color{}
	for i := range s.flat() {
		u[i] = c
	}
	s.SetColors(u, duration, now)
}
func (s *LightState) SetPower(level uint16, duration uint32, now time.Time) {
	s.Evaluate(now)
	s.power = nil
	if duration == 0 {
		s.Power = level
	} else {
		s.power = &powerTransition{s.Power, level, now, now.Add(time.Duration(duration) * time.Millisecond)}
	}
}
func (s *LightState) Zones(start int, colors []Color, apply uint8, duration uint32, now time.Time) {
	if apply != 2 {
		for i, c := range colors {
			if start+i >= 0 && start+i < len(s.Colors.Zones) {
				s.pending[start+i] = c
			}
		}
	}
	if apply == 1 || apply == 2 {
		s.SetColors(s.pending, duration, now)
		clear(s.pending)
	}
}
func (v *VirtualDevice) Matrix(p *packets.TileSet64, now time.Time) bool {
	if v.Device.LightType != device.LightTypeMatrix || p.Rect.FbIndex != 0 || p.Rect.Width == 0 || p.Length == 0 || int(p.TileIndex)+int(p.Length) > len(v.State.Colors.MatrixChains) {
		return false
	}
	// Use the inverse surface API to resolve physical partial updates; sentinel marks affected slots even if color is unchanged.
	mask := effects.NewPhysicalColorState(device.SurfaceFromDevice(v.Device))
	mark := make([]Color, 64)
	for i := range mark {
		mark[i] = Color{Hue: 1}
	}
	for c := int(p.TileIndex); c < int(p.TileIndex)+int(p.Length); c++ {
		if err := mask.MergeMatrixColors(device.SurfaceFromDevice(v.Device), c, int(p.Rect.X), int(p.Rect.Y), int(p.Rect.Width), mark); err != nil {
			return false
		}
	}
	target := effects.NewPhysicalColorState(device.SurfaceFromDevice(v.Device))
	for c := int(p.TileIndex); c < int(p.TileIndex)+int(p.Length); c++ {
		_ = target.MergeMatrixColors(device.SurfaceFromDevice(v.Device), c, int(p.Rect.X), int(p.Rect.Y), int(p.Rect.Width), p.Colors[:])
	}
	updates := map[int]Color{}
	offset := 0
	for c, a := range mask.MatrixChains {
		for i, m := range a {
			if m.Hue == 1 {
				updates[offset+i] = target.MatrixChains[c][i]
			}
		}
		offset += len(a)
	}
	v.State.SetColors(updates, p.Duration, now)
	return true
}
func (s *LightState) Wave(c Color, period uint32, cycles float32, skew int16, kind uint8, transient bool, mask [4]bool, now time.Time) bool {
	if period == 0 || cycles <= 0 || math.IsNaN(float64(cycles)) || math.IsInf(float64(cycles), 0) || kind > 4 {
		return false
	}
	s.Evaluate(now)
	from := s.flat()
	to := slices.Clone(from)
	for i := range to {
		if mask[0] {
			to[i].Hue = c.Hue
		}
		if mask[1] {
			to[i].Saturation = c.Saturation
		}
		if mask[2] {
			to[i].Brightness = c.Brightness
		}
		if mask[3] {
			to[i].Kelvin = c.Kelvin
		}
	}
	clear(s.moves)
	s.wave = &waveform{from, to, now, time.Duration(period) * time.Millisecond, float64(cycles), kind, (float64(skew) + 32768) / 65535, transient}
	s.Evaluate(now)
	return true
}
func (v *VirtualDevice) Frame(now time.Time) effects.Frame {
	v.State.Evaluate(now)
	f, _ := effects.AdaptPhysicalColorStateToFrame(v.State.Colors, device.SurfaceFromDevice(v.Device), 0)
	return f
}
func (s *LightState) First() Color { return s.flat()[0] }
