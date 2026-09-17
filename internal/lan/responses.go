package lan

import (
	"lifx-emulator/internal/emulator"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func label(s string) (a [32]byte) { copy(a[:], s); return }
func light(v *emulator.VirtualDevice) *packets.LightState {
	return &packets.LightState{Color: v.State.First(), Power: v.State.Power, Label: label(v.Device.Label)}
}

// respond is intentionally independent of ack/res flags and firmware timing.
func (r *Router) respond(v *emulator.VirtualDevice, p packets.Payload, applied bool) []packets.Payload {
	one := func(p packets.Payload) []packets.Payload { return []packets.Payload{p} }
	switch p := p.(type) {
	case *packets.DeviceGetService:
		return one(&packets.DeviceStateService{Service: 1, Port: r.Port})
	case *packets.DeviceGetLocation:
		return one(&packets.DeviceStateLocation{Location: [16]byte{2, 1}, Label: label("Virtual LAN"), UpdatedAt: 1})
	case *packets.DeviceGetGroup:
		return one(&packets.DeviceStateGroup{Group: [16]byte{2, 2}, Label: label("Emulator"), UpdatedAt: 1})
	case *packets.DeviceGetWifiInfo:
		return one(&packets.DeviceStateWifiInfo{Signal: -50})
	case *packets.DeviceGetVersion:
		return one(&packets.DeviceStateVersion{Vendor: 1, Product: v.Device.ProductID})
	case *packets.DeviceGetHostFirmware:
		return one(&packets.DeviceStateHostFirmware{VersionMajor: 3, VersionMinor: 90})
	case *packets.DeviceGetWifiFirmware:
		return one(&packets.DeviceStateWifiFirmware{VersionMajor: 3, VersionMinor: 90})
	case *packets.DeviceGetLabel, *packets.DeviceSetLabel:
		return one(&packets.DeviceStateLabel{Label: label(v.Device.Label)})
	case *packets.DeviceGetPower, *packets.DeviceSetPower:
		return one(&packets.DeviceStatePower{Level: v.State.Power})
	case *packets.LightGetPower, *packets.LightSetPower:
		return one(&packets.LightStatePower{Level: v.State.Power})
	case *packets.LightGet, *packets.LightSetColor, *packets.LightSetWaveform, *packets.LightSetWaveformOptional:
		if !applied {
			switch p.(type) {
			case *packets.LightGet:
			default:
				return nil
			}
		}
		return one(light(v))
	case *packets.MultiZoneGetEffect:
		if v.Device.LightType != device.LightTypeMultiZone {
			return nil
		}
		return one(&packets.MultiZoneStateEffect{Settings: packets.MultiZoneEffectSettings{Type: enums.MultiZoneEffectTypeMULTIZONEEFFECTTYPEOFF}})
	case *packets.TileGetEffect:
		if v.Device.LightType != device.LightTypeMatrix {
			return nil
		}
		return one(&packets.TileStateEffect{Settings: packets.TileEffectSettings{Type: enums.TileEffectTypeTILEEFFECTTYPEOFF}})
	case *packets.MultiZoneExtendedGetColorZones, *packets.MultiZoneExtendedSetColorZones:
		if v.Device.LightType != device.LightTypeMultiZone {
			return nil
		}
		var out []packets.Payload
		a := v.State.Colors.Zones
		for i := 0; i < len(a); i += 82 {
			q := &packets.MultiZoneExtendedStateMultiZone{Count: uint16(len(a)), Index: uint16(i), ColorsCount: uint8(min(82, len(a)-i))}
			copy(q.Colors[:], a[i:])
			out = append(out, q)
		}
		return out
	case *packets.MultiZoneGetColorZones:
		return legacy(v, int(p.StartIndex), int(p.EndIndex))
	case *packets.MultiZoneSetColorZones:
		if !applied {
			return nil
		}
		return legacy(v, int(p.StartIndex), int(p.EndIndex))
	case *packets.TileGetDeviceChain:
		if v.Device.LightType != device.LightTypeMatrix {
			return nil
		}
		props := v.Device.MatrixProperties
		q := &packets.TileStateDeviceChain{TileDevicesCount: uint8(props.ChainLength)}
		for i := 0; i < props.ChainLength; i++ {
			q.TileDevices[i] = packets.TileStateDevice{Width: uint8(props.Width), Height: uint8(props.Height), UserX: float32(i), AccelMeas: acceleration(v.Device.ProductID, props.ChainOrientations[i]), DeviceVersion: packets.DeviceStateVersion{Vendor: 1, Product: v.Device.ProductID}, Firmware: packets.DeviceStateHostFirmware{VersionMajor: 3, VersionMinor: 90}}
		}
		return one(q)
	case *packets.TileGet64:
		return matrixStates(v, int(p.TileIndex), int(p.Length), p.Rect)
	case *packets.TileSet64:
		if !applied {
			return nil
		}
		return matrixStates(v, int(p.TileIndex), int(p.Length), p.Rect)
	default:
		return nil
	}
}
func legacy(v *emulator.VirtualDevice, start, end int) []packets.Payload {
	a := v.State.Colors.Zones
	if v.Device.LightType != device.LightTypeMultiZone || start >= len(a) || end < start {
		return nil
	}
	end = min(end, len(a)-1)
	if start == end {
		return []packets.Payload{&packets.MultiZoneStateZone{Count: uint8(len(a)), Index: uint8(start), Color: a[start]}}
	}
	var out []packets.Payload
	for i := start; i <= end; i += 8 {
		p := &packets.MultiZoneStateMultiZone{Count: uint8(len(a)), Index: uint8(i)}
		copy(p.Colors[:], a[i:min(i+8, end+1)])
		out = append(out, p)
	}
	return out
}
func matrixStates(v *emulator.VirtualDevice, start, length int, rect packets.TileBufferRect) []packets.Payload {
	if v.Device.LightType != device.LightTypeMatrix || rect.FbIndex != 0 || rect.Width == 0 || length == 0 || start+length > len(v.State.Colors.MatrixChains) {
		return nil
	}
	var out []packets.Payload
	w := v.Device.MatrixProperties.Width
	h := v.Device.MatrixProperties.Height
	for c := start; c < start+length; c++ {
		p := &packets.TileState64{TileIndex: uint8(c), Rect: rect}
		for i := range p.Colors {
			x := int(rect.X) + i%int(rect.Width)
			y := int(rect.Y) + i/int(rect.Width)
			if x < w && y < h {
				p.Colors[i] = v.State.Colors.MatrixChains[c][y*w+x]
			}
		}
		out = append(out, p)
	}
	return out
}

// Derive accelerometer metadata using the library's product-specific orientation rules.
func acceleration(pid uint32, o device.Orientation) packets.TileAccelMeas {
	for _, a := range []packets.TileAccelMeas{{X: 1000}, {X: -1000}, {Y: 1000}, {Y: -1000}, {Z: 1000}, {Z: -1000}, {}} {
		if device.NearestOrientation(pid, a.X, a.Y, a.Z) == o {
			return a
		}
	}
	return packets.TileAccelMeas{}
}
