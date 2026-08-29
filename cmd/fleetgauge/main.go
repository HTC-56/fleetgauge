// Command fleetgauge watches a declared fleet of systemd units and serves one
// page that answers "is everything up?".
//
// Phase A wires the toolchain and the backend seam only: there is no HTTP
// surface yet, by design (see SPEC.md, "Pre-registered rules"). The flags below
// are the committed surface; later phases give them behaviour.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"fleetgauge/internal/backend"
	"fleetgauge/internal/backend/fake"
	"fleetgauge/internal/backend/systemd"
	"fleetgauge/internal/config"
	"fleetgauge/internal/poller"
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

	_ = addr

	if *demo {
		return runDemo()
	}
	return runReal(*configPath)
}

// runDemo polls the synthetic fleet five times and prints an overview table.
func runDemo() error {
	be := fake.New()
	p := poller.New(be, []string{"*.service"}, time.Second, 60)

	for i := 0; i < 5; i++ {
		if i > 0 {
			be.Tick()
		}
		if _, err := p.PollOnce(context.Background()); err != nil {
			return fmt.Errorf("demo poll %d: %w", i+1, err)
		}
	}

	printOverview(p.Store(), time.Now())
	return nil
}

// runReal loads the config, builds a systemd backend, polls once, and prints
// the overview table.
func runReal(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	be := systemd.New()

	// Extract unit names (patterns) from config.
	patterns := make([]string, len(cfg.Units))
	for i, u := range cfg.Units {
		patterns[i] = u.Name
	}

	p := poller.New(be, patterns, cfg.PollInterval, 60)
	if _, err := p.PollOnce(context.Background()); err != nil {
		return fmt.Errorf("poll: %w", err)
	}

	printOverview(p.Store(), time.Now())
	return nil
}

// printOverview writes a tabwriter table of all unit views to stdout.
func printOverview(s *poller.Store, now time.Time) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATE\tRESTARTS\tMEMORY\tUPTIME\tTRANSITIONS")

	for _, v := range s.Overview(now) {
		mem := formatMemory(v.MemoryBytes)
		uptime := formatUptime(v.Uptime)
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%d\n",
			v.Name, v.SubState, v.NRestarts, mem, uptime, v.Transitions)
	}
	w.Flush()
}

// formatMemory renders MemoryBytes as a human-readable string. MemoryUnknown
// (accounting off or unavailable) becomes "-".
func formatMemory(b int64) string {
	if b == backend.MemoryUnknown {
		return "-"
	}
	return fmt.Sprintf("%d", b)
}

// formatUptime renders a duration as a compact human-readable string.
func formatUptime(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	return d.Round(time.Second).String()
}
