package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"lifx-emulator/internal/config"
	"lifx-emulator/internal/lan"
)

func main() {
	path := flag.String("config", config.Path(), "device configuration JSON")
	address := flag.String("listen", "", "override UDP listen address")
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
	if err = s.Serve(ctx); err != nil {
		log.Fatal(err)
	}
}
