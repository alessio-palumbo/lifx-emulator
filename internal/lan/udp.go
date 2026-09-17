package lan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"golang.org/x/net/ipv4"
)

// TransportStats counts datagrams before routing, including rejected packets.
type TransportStats struct {
	Received   uint64
	Decoded    uint64
	Filtered   uint64
	Invalid    uint64
	Replies    uint64
	SendErrors uint64
	LastPeer   string
	LastError  string
}

type Server struct {
	statsMu        sync.Mutex
	stats          TransportStats
	conn           *net.UDPConn
	packet         *ipv4.PacketConn
	selected       net.IP
	interfaceIndex int
	Router         *Router
	done           chan struct{}
	once           sync.Once
}

func Listen(address string, r *Router) (*Server, error) {
	a, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		return nil, err
	}
	selected := a.IP
	index := 0
	if selected != nil && !selected.IsUnspecified() {
		interfaces, err := net.Interfaces()
		if err != nil {
			return nil, err
		}
		for _, iface := range interfaces {
			addresses, _ := iface.Addrs()
			for _, address := range addresses {
				ip, _, _ := net.ParseCIDR(address.String())
				if ip.Equal(selected) {
					index = iface.Index
				}
			}
		}
		if index == 0 {
			return nil, fmt.Errorf("listen IP is not a local interface")
		}
	}
	// Wildcard binding receives subnet broadcasts even when an interface is selected.
	bind := &net.UDPAddr{IP: net.IPv4zero, Port: a.Port}
	c, err := net.ListenUDP("udp4", bind)
	if err != nil {
		return nil, err
	}
	r.SetPort(uint32(c.LocalAddr().(*net.UDPAddr).Port))
	packet := ipv4.NewPacketConn(c)
	if index != 0 {
		if err := packet.SetControlMessage(ipv4.FlagInterface|ipv4.FlagDst, true); err != nil {
			c.Close()
			return nil, fmt.Errorf("interface selection requires IPv4 packet metadata: %w", err)
		}
	}
	return &Server{conn: c, packet: packet, selected: selected, interfaceIndex: index, Router: r, done: make(chan struct{})}, nil
}
func (s *Server) Address() string {
	a := *s.conn.LocalAddr().(*net.UDPAddr)
	if s.selected != nil {
		a.IP = s.selected
	}
	return a.String()
}
func (s *Server) Stats() TransportStats {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	return s.stats
}
func (s *Server) record(update func(*TransportStats)) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	update(&s.stats)
}
func (s *Server) Close() error { var err error; s.once.Do(func() { err = s.conn.Close() }); return err }
func (s *Server) Serve(ctx context.Context) error {
	go func() {
		select {
		case <-ctx.Done():
			_ = s.Close()
		case <-s.done:
		}
	}()
	defer close(s.done)
	buf := make([]byte, 4096)
	for {
		n, metadata, peer, err := s.packet.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		s.record(func(v *TransportStats) { v.Received++; v.LastPeer = peer.String() })
		if s.interfaceIndex != 0 && (metadata == nil || metadata.IfIndex != s.interfaceIndex) {
			s.record(func(v *TransportStats) { v.Filtered++ })
			continue
		}
		// Validate framing before handing library decoding a datagram.
		if n < 36 || int(buf[0])|int(buf[1])<<8 != n || (uint16(buf[2])|uint16(buf[3])<<8)&0xfff != 1024 {
			s.record(func(v *TransportStats) { v.Invalid++; v.LastError = "Invalid LIFX framing" })
			continue
		}
		m := &protocol.Message{}
		if err = m.UnmarshalBinary(buf[:n]); err != nil {
			s.record(func(v *TransportStats) { v.Invalid++; v.LastError = err.Error() })
			continue
		}
		s.record(func(v *TransportStats) { v.Decoded++ })
		for _, out := range s.Router.HandleFrom(m, peer.String()) {
			b, err := out.MarshalBinary()
			if err == nil {
				var reply *ipv4.ControlMessage
				if s.interfaceIndex != 0 {
					reply = &ipv4.ControlMessage{Src: s.selected, IfIndex: s.interfaceIndex}
				}
				_, err = s.packet.WriteTo(b, reply, peer)
			}
			s.Router.RecordSend(out, peer.String(), err)
			s.record(func(v *TransportStats) {
				if err != nil {
					v.SendErrors++
					v.LastError = err.Error()
				} else {
					v.Replies++
				}
			})
		}
	}
}
