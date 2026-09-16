// Package app provides Wails bindings and coalesced render snapshots.
package app

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"lifx-emulator/internal/config"
	"lifx-emulator/internal/lan"

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
			if revision != previous || active || wasActive || packet != lastPacket {
				a.mu.Lock()
				v := View{Devices: devices, Recent: recent, Listening: a.address, Error: a.failure}
				a.mu.Unlock()
				runtime.EventsEmit(a.ctx, "frame", v)
				previous = revision
				lastPacket = packet
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
	return View{d, r, a.address, a.failure, interfaces}
}
func (a *App) Products() []Product {
	out := []Product{}
	for id, p := range registry.ProductsByPID {
		if !p.Features.Relays && !p.Features.Buttons {
			out = append(out, Product{id, p.Name, p.Features.Multizone, p.Features.Matrix, p.Features.Chain})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
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
