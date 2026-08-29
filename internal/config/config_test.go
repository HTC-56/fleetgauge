package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeYAML writes content into t.TempDir() and returns the path.
func writeYAML(t *testing.T, name string, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
	return path
}

// TestConfigRoundTrip asserts that a fully-populated config round-trips:
// all five top-level keys and a two-unit list load with the values written,
// and allow_restart is true only for the unit that declared it.
func TestConfigRoundTrip(t *testing.T) {
	yaml := `
listen: "192.0.2.1:9090"
bearer_token: "s3cret"
poll_interval: "10s"
journal_lines: 100
units:
  - name: "nginx.service"
    allow_restart: true
  - name: "worker@*.service"
    allow_restart: false
`
	cfg, err := Load(writeYAML(t, "full.yaml", yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Listen != "192.0.2.1:9090" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, "192.0.2.1:9090")
	}
	if cfg.BearerToken != "s3cret" {
		t.Errorf("BearerToken = %q, want %q", cfg.BearerToken, "s3cret")
	}
	if cfg.PollIntervalRaw != "10s" {
		t.Errorf("PollIntervalRaw = %q, want %q", cfg.PollIntervalRaw, "10s")
	}
	if cfg.JournalLines != 100 {
		t.Errorf("JournalLines = %d, want 100", cfg.JournalLines)
	}
	if len(cfg.Units) != 2 {
		t.Fatalf("len(Units) = %d, want 2", len(cfg.Units))
	}
	if cfg.Units[0].Name != "nginx.service" {
		t.Errorf("Units[0].Name = %q, want %q", cfg.Units[0].Name, "nginx.service")
	}
	if !cfg.Units[0].AllowRestart {
		t.Error("Units[0].AllowRestart = false, want true")
	}
	if cfg.Units[1].Name != "worker@*.service" {
		t.Errorf("Units[1].Name = %q, want %q", cfg.Units[1].Name, "worker@*.service")
	}
	if cfg.Units[1].AllowRestart {
		t.Error("Units[1].AllowRestart = true, want false")
	}
}

// TestConfigDefaults asserts that a minimal config (only a units list) gets the
// three defaults from §A7: listen=127.0.0.1:8080, poll_interval=5s,
// journal_lines=50.
func TestConfigDefaults(t *testing.T) {
	yaml := `
units:
  - name: "app.service"
`
	cfg, err := Load(writeYAML(t, "minimal.yaml", yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Listen != "127.0.0.1:8080" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, "127.0.0.1:8080")
	}
	if cfg.PollInterval != 5_000_000_000 { // 5s in nanoseconds
		t.Errorf("PollInterval = %v, want 5s", cfg.PollInterval)
	}
	if cfg.JournalLines != 50 {
		t.Errorf("JournalLines = %d, want 50", cfg.JournalLines)
	}
}

// TestConfigDurationParsing asserts that poll_interval: "2s" parses to a
// 2-second time.Duration.
func TestConfigDurationParsing(t *testing.T) {
	yaml := `
poll_interval: "2s"
units:
  - name: "test.service"
`
	cfg, err := Load(writeYAML(t, "duration.yaml", yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.PollInterval != 2_000_000_000 { // 2s in nanoseconds
		t.Errorf("PollInterval = %v, want 2s", cfg.PollInterval)
	}
}

// TestConfigEmptyUnits asserts that a config with an empty units list returns
// an error.
func TestConfigEmptyUnits(t *testing.T) {
	yaml := `
units: []
`
	_, err := Load(writeYAML(t, "empty.yaml", yaml))
	if err == nil {
		t.Fatal("Load: expected error for empty units list, got nil")
	}
}

// TestConfigZeroPollInterval asserts that poll_interval: "0s" returns a
// validation error.
func TestConfigZeroPollInterval(t *testing.T) {
	yaml := `
poll_interval: "0s"
units:
  - name: "test.service"
`
	_, err := Load(writeYAML(t, "zero.yaml", yaml))
	if err == nil {
		t.Fatal("Load: expected error for zero poll_interval, got nil")
	}
}

// TestConfigExampleLoad asserts that deploy/fleetgauge.example.yaml parses
// without error, yields at least four units, and has exactly one unit with
// AllowRestart == true.  This keeps the shipped example in sync with the
// loader.
func TestConfigExampleLoad(t *testing.T) {
	cfg, err := Load("../../deploy/fleetgauge.example.yaml")
	if err != nil {
		t.Fatalf("Load example config: %v", err)
	}

	if len(cfg.Units) < 4 {
		t.Errorf("len(Units) = %d, want >= 4", len(cfg.Units))
	}

	restartCount := 0
	for _, u := range cfg.Units {
		if u.AllowRestart {
			restartCount++
		}
	}
	if restartCount != 1 {
		t.Errorf("units with AllowRestart = true: %d, want exactly 1", restartCount)
	}
}

// TestConfigLoadErrors asserts that malformed YAML and a non-existent path
// each return an error rather than panicking.
func TestConfigLoadErrors(t *testing.T) {
	// Malformed YAML.
	badYaml := `
listen: "127.0.0.1:8080"
units:
  - name: "broken
    allow_restart: true
`
	_, err := Load(writeYAML(t, "malformed.yaml", badYaml))
	if err == nil {
		t.Fatal("Load: expected error for malformed YAML, got nil")
	}

	// Non-existent path.
	_, err = Load("/no/such/file/nonexistent.yaml")
	if err == nil {
		t.Fatal("Load: expected error for missing file, got nil")
	}
}
