// Package app provides Wails bindings and coalesced render snapshots.
package app

import (
	"context"
	"fmt"
	"net"
	goruntime "runtime"
	"sort"
	"sync"
	"time"

	"lifx-emulator/internal/config"
	"lifx-emulator/internal/lan"

	"github.com/alessio-palumbo/lifxlan-go/pkg/client"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxregistry-go/gen/registry"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Product struct {
	ID        int
	Name      string
	Multizone bool
	Matrix    bool
	Chain     bool
}
type View struct {
	Platform   string
	Location   config.Location
	Group      config.Group
	Transport  lan.TransportStats
	Devices    []lan.Snapshot
	Recent     []lan.Activity
	Listening  string
	Error      string
	Interfaces []string
}
type App struct {
	run       context.Context
	loadError error
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	router    *lan.Router
	server    *lan.Server
	file      config.File
	path      string
	failure   string
	address   string
}

func New() *App { return &App{path: config.Path()} }
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	run, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.run = run
	a.done = make(chan struct{})
	f, err := config.Load(a.path)
	a.file = f
	a.loadError = err
	if err != nil {
		a.failure = err.Error()
	}
	vs, err := f.Virtuals()
	if err != nil {
		a.failure = err.Error()
	}
	a.router = lan.New(vs, nil)
	responses, responseErr := config.LoadResponses(config.ResponsePath())
	if responseErr == nil {
		responseErr = a.router.SetResponses(responses)
	}
	if responseErr != nil {
		a.failure = responseErr.Error()
	}
	if a.failure == "" {
		s, err := lan.Listen(f.Listen, a.router)
		if err != nil {
			a.failure = err.Error()
		} else {
			a.server = s
			a.address = s.Address()
			go func() {
				if err := s.Serve(run); err != nil {
					a.mu.Lock()
					a.failure = err.Error()
					a.mu.Unlock()
				}
			}()
		}
	}
	go a.frames(run)
}
func (a *App) Shutdown(context.Context) {
	if a.cancel != nil {
		a.cancel()
		<-a.done
	}
}
func (a *App) frames(ctx context.Context) {
	defer close(a.done)
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	fast := false
	var previous uint64
	wasActive := false
	lastPacket := time.Time{}
	var lastTransport lan.TransportStats
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.router.Applied:
			// Schedule a frame once; repeated packets cannot push its deadline back.
			if !fast {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(time.Second / 60)
				fast = true
			}
		case <-timer.C:
			devices, revision, active, recent := a.router.Snapshots()
			packet := time.Time{}
			if len(recent) > 0 {
				packet = recent[len(recent)-1].At
			}
			a.mu.Lock()
			transport := a.transportStats()
			a.mu.Unlock()
			if revision != previous || active || wasActive || packet != lastPacket || transport != lastTransport {
				a.mu.Lock()
				v := View{Platform: goruntime.GOOS, Location: a.file.Location, Group: a.file.Group, Transport: transport, Devices: devices, Recent: recent, Listening: a.address, Error: a.failure}
				a.mu.Unlock()
				runtime.EventsEmit(a.ctx, "frame", v)
				previous = revision
				lastPacket = packet
				lastTransport = transport
			}
			wasActive = active
			fast = active
			delay := 250 * time.Millisecond
			if active {
				delay = time.Second / 60
			}
			timer.Reset(delay)
		}
	}
}

// Caller holds a.mu.
func (a *App) transportStats() lan.TransportStats {
	if a.server != nil {
		return a.server.Stats()
	}
	return lan.TransportStats{}
}
func (a *App) Snapshot() View {
	a.mu.Lock()
	defer a.mu.Unlock()
	d, _, _, r := a.router.Snapshots()
	interfaces := []string{"0.0.0.0"}
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		addrs, _ := i.Addrs()
		for _, addr := range addrs {
			ip, _, _ := net.ParseCIDR(addr.String())
			if ip.To4() != nil {
				interfaces = append(interfaces, ip.String())
			}
		}
	}
	return View{Platform: goruntime.GOOS, Location: a.file.Location, Group: a.file.Group, Transport: a.transportStats(), Devices: d, Recent: r, Listening: a.address, Error: a.failure, Interfaces: interfaces}
}
func (a *App) Products() []Product {
	out := []Product{}
	for id, p := range registry.ProductsByPID {
		classified := device.Device{}
		classified.SetProductInfo(uint32(id))
		if classified.Type == device.DeviceTypeLight || classified.Type == device.DeviceTypeHybrid {
			out = append(out, Product{id, p.Name, p.Features.Multizone, p.Features.Matrix, p.Features.Chain})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].ID < out[j].ID
		}
		return out[i].Name < out[j].Name
	})
	return out
}
func (a *App) Add(d config.Definition) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.loadError != nil {
		return a.loadError
	}
	if a.failure != "" && a.router == nil {
		return fmt.Errorf("not initialized")
	}
	var err error
	d.Serial, err = config.UniqueSerial(a.file.Devices)
	if err != nil {
		return err
	}
	v, err := d.Virtual()
	if err != nil {
		return err
	}
	a.file.ApplyMembership(v)
	f := a.file
	f.Devices = append(append([]config.Definition(nil), f.Devices...), d)
	if err = config.Save(a.path, f); err != nil {
		return err
	}
	if err = a.router.Add(v); err != nil {
		return err
	}
	a.file = f
	return nil
}
func (a *App) Remove(serial string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.loadError != nil {
		return a.loadError
	}
	f := a.file
	f.Devices = nil
	for _, d := range a.file.Devices {
		if d.Serial != serial {
			f.Devices = append(f.Devices, d)
		}
	}
	if err := config.Save(a.path, f); err != nil {
		return err
	}
	a.router.Remove(serial)
	a.file = f
	return nil
}
func (a *App) Update(serial, replacement, label string, enabled bool) error {
	parsed, parseErr := device.SerialFromHex(replacement)
	if parseErr != nil {
		return parseErr
	}
	replacement = parsed.String()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.loadError != nil {
		return a.loadError
	}
	f := a.file
	f.Devices = append([]config.Definition(nil), f.Devices...)
	found := false
	for i := range f.Devices {
		if f.Devices[i].Serial == serial {
			if f.Devices[i].Enabled && replacement != serial {
				return fmt.Errorf("disable device before editing serial")
			}
			f.Devices[i].Serial = replacement
			f.Devices[i].Label = label
			f.Devices[i].Enabled = enabled
			found = true
		}
	}
	if !found {
		return fmt.Errorf("device not found")
	}
	if err := config.Save(a.path, f); err != nil {
		return err
	}
	if err := a.router.Update(serial, replacement, label, enabled); err != nil {
		return err
	}
	a.file = f
	return nil
}
func (a *App) ListenOn(ip string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.loadError != nil {
		return a.loadError
	}
	if net.ParseIP(ip).To4() == nil {
		return fmt.Errorf("invalid IPv4 address")
	}
	address := net.JoinHostPort(ip, "56700")
	if a.server != nil && a.address == address {
		return nil
	}
	// Rebinding must release the shared UDP port before selecting a different local interface.
	if a.server != nil {
		_ = a.server.Close()
		a.server = nil
	}
	s, err := lan.Listen(address, a.router)
	if err != nil {
		a.failure = err.Error()
		a.address = ""
		return err
	}
	a.server = s
	a.address = s.Address()
	a.failure = ""
	a.file.Listen = address
	go func() {
		if err := s.Serve(a.run); err != nil {
			a.mu.Lock()
			a.failure = err.Error()
			a.mu.Unlock()
		}
	}()
	return config.Save(a.path, a.file)
}

// RequestLANAccess performs an outgoing socket connection to the same limited
// broadcast address used by the official app. macOS can use this operation to
// show its local-network permission prompt. Connecting sends no datagram;
// success is not a general permission-status check.
func (a *App) RequestLANAccess() error {
	if goruntime.GOOS != "darwin" {
		return nil
	}
	a.mu.Lock()
	listen := a.file.Listen
	a.mu.Unlock()
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	selected := net.ParseIP(host)
	interfaces, err := client.BroadcastInterfaces()
	if err != nil {
		return err
	}
	for _, iface := range interfaces {
		if selected != nil && !selected.IsUnspecified() && !selected.Equal(iface.IP) {
			continue
		}
		conn, err := net.DialUDP("udp4", &net.UDPAddr{IP: iface.IP}, &net.UDPAddr{IP: net.IPv4bcast, Port: 56700})
		if err != nil {
			return fmt.Errorf("LAN access request failed: %w. Allow lifx-emulator in System Settings → Privacy & Security → Local Network, then retry", err)
		}
		return conn.Close()
	}
	return fmt.Errorf("no broadcast-capable IPv4 interface is available for the selected listen address")
}

// UpdateMembership changes the shared location/group identity of this installation.
func (a *App) UpdateMembership(locationLabel, locationID, groupLabel, groupID string) error {
	location, err := device.ParseLocationID(locationID)
	if err != nil {
		return err
	}
	group, err := device.ParseGroupID(groupID)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.loadError != nil {
		return a.loadError
	}
	if a.router == nil {
		return fmt.Errorf("not initialized")
	}
	f := a.file
	now := uint64(time.Now().UnixNano())
	if f.Location.ID != location || f.Location.Label != locationLabel {
		f.Location = config.Location{ID: location, Label: locationLabel, UpdatedAt: max(now, f.Location.UpdatedAt+1)}
	}
	if f.Group.ID != group || f.Group.Label != groupLabel {
		f.Group = config.Group{ID: group, Label: groupLabel, UpdatedAt: max(now, f.Group.UpdatedAt+1)}
	}
	if err := config.Save(a.path, f); err != nil {
		return err
	}
	l, g := f.MembershipPackets()
	a.router.SetMembership(l, g)
	a.file = f
	return nil
}
