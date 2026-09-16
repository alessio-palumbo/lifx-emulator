package emulator

import (
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// Apply translates supported light-control messages into transport-independent state operations.
func (v *VirtualDevice) Apply(p packets.Payload, now time.Time) bool {
	s := v.State
	switch p := p.(type) {
	case *packets.LightSetColor:
		s.Whole(p.Color, p.Duration, now)
	case *packets.LightSetPower:
		s.SetPower(p.Level, p.Duration, now)
	case *packets.DeviceSetPower:
		s.SetPower(p.Level, 0, now)
	case *packets.DeviceSetLabel:
		v.Device.Label = string(p.Label[:])
		for len(v.Device.Label) > 0 && v.Device.Label[len(v.Device.Label)-1] == 0 {
			v.Device.Label = v.Device.Label[:len(v.Device.Label)-1]
		}
	case *packets.MultiZoneSetColorZones:
		if v.Device.LightType != device.LightTypeMultiZone || p.Apply > 2 || (p.Apply != 2 && p.EndIndex < p.StartIndex) {
			return false
		}
		colors := make([]Color, max(0, int(p.EndIndex)-int(p.StartIndex)+1))
		for i := range colors {
			colors[i] = p.Color
		}
		s.Zones(int(p.StartIndex), colors, uint8(p.Apply), p.Duration, now)
	case *packets.MultiZoneExtendedSetColorZones:
		if v.Device.LightType != device.LightTypeMultiZone || p.Apply > 2 || p.ColorsCount > 82 {
			return false
		}
		s.Zones(int(p.Index), p.Colors[:p.ColorsCount], uint8(p.Apply), p.Duration, now)
	case *packets.TileSet64:
		return v.Matrix(p, now)
	case *packets.LightSetWaveform:
		return s.Wave(p.Color, p.Period, p.Cycles, p.SkewRatio, uint8(p.Waveform), p.Transient, [4]bool{true, true, true, true}, now)
	case *packets.LightSetWaveformOptional:
		return s.Wave(p.Color, p.Period, p.Cycles, p.SkewRatio, uint8(p.Waveform), p.Transient, [4]bool{p.SetHue, p.SetSaturation, p.SetBrightness, p.SetKelvin}, now)
	default:
		return false
	}
	return true
}
