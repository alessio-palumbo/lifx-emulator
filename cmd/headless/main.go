package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lifx-emulator/internal/config"
	"lifx-emulator/internal/lan"
)

func main() {
	path := flag.String("config", config.Path(), "device configuration JSON")
	address := flag.String("listen", "", "override UDP listen address")
	traffic := flag.Bool("traffic", false, "log socket counters when traffic changes")
	flag.Parse()
	f, err := config.Load(*path)
	if err != nil {
		log.Fatal(err)
	}
	if *address != "" {
		f.Listen = *address
	}
	vs, err := f.Virtuals()
	if err != nil {
		log.Fatal(err)
	}
	r := lan.New(vs, nil)
	s, err := lan.Listen(f.Listen, r)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	defer s.Close()
	log.Printf("LIFX UDP %s · %d virtual lights", s.Address(), len(vs))
	if *traffic {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			previous := lan.TransportStats{}
			lastActivity := time.Time{}
			log.Printf("RX 0 · decoded 0 · TX 0 · dropped 0 · send errors 0")
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_, _, _, recent := r.Snapshots()
					for _, activity := range recent {
						if activity.At.After(lastActivity) {
							log.Printf("%s %s · %s (%s) · peer %s · source %d · seq %d · replies %d · error %s", activity.Direction, activity.TypeName, activity.Label, activity.Target, activity.Peer, activity.Source, activity.Sequence, activity.Replies, activity.Error)
						}
					}
					if len(recent) > 0 {
						lastActivity = recent[len(recent)-1].At
					}
					stats := s.Stats()
					if stats != previous {
						log.Printf("RX %d · decoded %d · TX %d · dropped %d · send errors %d · last sender %s · last error %s", stats.Received, stats.Decoded, stats.Replies, stats.Filtered+stats.Invalid, stats.SendErrors, stats.LastPeer, stats.LastError)
						previous = stats
					}
				}
			}
		}()
	}
	if err = s.Serve(ctx); err != nil {
		log.Fatal(err)
	}
}
