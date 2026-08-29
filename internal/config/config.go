// Package config loads and validates the YAML fleet configuration.
//
// The config file declares which units to watch, how often to poll them, and
// how to bind the HTTP listener. See deploy/fleetgauge.example.yaml for a
// fully commented reference that ships with the repo.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Unit is one service (or unit pattern) the poller watches.
type Unit struct {
	Name         string `yaml:"name"`
	AllowRestart bool   `yaml:"allow_restart"`
}

// Config is the parsed fleet configuration.
type Config struct {
	Listen          string `yaml:"listen"`
	BearerToken     string `yaml:"bearer_token"`
	PollIntervalRaw string `yaml:"poll_interval"`
	PollInterval    time.Duration
	JournalLines    int    `yaml:"journal_lines"`
	Units           []Unit `yaml:"units"`
}

// Load reads the YAML file at path, unmarshals it, applies defaults, then
// validates. Every returned error names the file and the failing key.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: unmarshal: %w", path, err)
	}

	// Apply defaults when a key is absent.
	if c.Listen == "" {
		c.Listen = "127.0.0.1:8080"
	}
	if c.PollIntervalRaw == "" {
		c.PollInterval = 5 * time.Second
	} else {
		d, err := time.ParseDuration(c.PollIntervalRaw)
		if err != nil {
			return nil, fmt.Errorf("%s: poll_interval %q: %w", path, c.PollIntervalRaw, err)
		}
		c.PollInterval = d
	}
	if c.JournalLines == 0 {
		c.JournalLines = 50
	}

	// Validate.
	if len(c.Units) == 0 {
		return nil, fmt.Errorf("%s: units list must not be empty", path)
	}
	if c.PollInterval <= 0 {
		return nil, fmt.Errorf("%s: poll_interval must be positive, got %s", path, c.PollIntervalRaw)
	}
	if c.JournalLines < 0 {
		return nil, fmt.Errorf("%s: journal_lines must not be negative, got %d", path, c.JournalLines)
	}

	return &c, nil
}
