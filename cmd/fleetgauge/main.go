// Command fleetgauge watches a declared fleet of systemd units and serves one
// page that answers "is everything up?".
//
// Phase A wires the toolchain and the backend seam only: there is no HTTP
// surface yet, by design (see SPEC.md, "Pre-registered rules"). The flags below
// are the committed surface; later phases give them behaviour.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var (
		configPath = flag.String("config", "fleetgauge.yaml", "path to the fleet config file")
		demo       = flag.Bool("demo", false, "serve a synthetic fleet; no systemd required")
		addr       = flag.String("addr", "", "listen address; overrides the config file")
	)
	flag.Parse()

	_ = configPath
	_ = demo
	_ = addr

	fmt.Fprintln(os.Stderr, "fleetgauge: no server yet -- Phase A builds the backend seam only")
}
