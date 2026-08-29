// Command fleetgauge watches a declared fleet of systemd units and serves one
// page that answers "is everything up?".
//
// Flags:
//
//	-config path to the fleet config file (default "fleetgauge.yaml")
//	-demo serve a synthetic fleet; no systemd required
//	-addr listen address; overrides the config file
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"fleetgauge/internal/backend"
	"fleetgauge/internal/backend/fake"
	"fleetgauge/internal/backend/systemd"
	"fleetgauge/internal/config"
	"fleetgauge/internal/poller"
	"fleetgauge/internal/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var (
		configPath = flag.String("config", "fleetgauge.yaml", "path to the fleet config file")
		demo       = flag.Bool("demo", false, "serve a synthetic fleet; no systemd required")
		addr       = flag.String("addr", "", "listen address; overrides the config file")
	)
	flag.Parse()

	if *demo {
		return runDemo(*addr)
	}
	return runReal(*configPath, *addr)
}

// runDemo builds a synthetic fleet, polls it, and serves the HTTP surface.
func runDemo(addr string) error {
	be := fake.New()
	p := poller.New(be, []string{"*.service"}, time.Second, 120)
	return serve(addr, be, p, server.Options{})
}

// runReal loads the config, builds a systemd backend, and serves the HTTP
// surface.
func runReal(configPath, addr string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	be := systemd.New()

	patterns := make([]string, len(cfg.Units))
	for i, u := range cfg.Units {
		patterns[i] = u.Name
	}

	allowRestart := make(map[string]bool, len(cfg.Units))
	for _, u := range cfg.Units {
		allowRestart[u.Name] = u.AllowRestart
	}

	opts := server.Options{
		AllowRestart:      allowRestart,
		JournalLines:      cfg.JournalLines,
		BroadcastInterval: server.DefaultBroadcastInterval,
		Heartbeat:         server.DefaultHeartbeat,
	}

	p := poller.New(be, patterns, cfg.PollInterval, 60)
	if addr == "" {
		addr = cfg.Listen
	}
	return serve(addr, be, p, opts)
}

// serve starts the poller, the broadcast loop, and an HTTP server on the
// given address. It blocks until an interrupt arrives, then shuts down
// gracefully.
func serve(addr string, be backend.Backend, p *poller.Poller, opts server.Options) error {
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	opts.Store = p.Store()
	opts.Backend = be
	opts.Logger = log
	srv := server.New(opts)
	defer srv.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go p.Run(ctx)
	go srv.Broadcast(ctx)

	hs := &http.Server{
		Addr:    addr,
		Handler: srv.Handler(),
	}

	log.Info("listening", "addr", addr)

	go func() {
		if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen", "err", err)
		}
	}()

	<-ctx.Done()

	if err := hs.Shutdown(context.Background()); err != nil {
		log.Error("shutdown", "err", err)
		return err
	}

	return nil
}
