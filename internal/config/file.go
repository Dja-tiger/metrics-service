package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

type fileOption struct {
	field    string
	flag     string
	env      []string
	duration bool
}

func configFileFlags() *string {
	var path string
	flag.StringVar(&path, "c", "", "JSON configuration file")
	flag.StringVar(&path, "config", "", "JSON configuration file (alias for -c)")
	return &path
}

// applyFile sets only options not explicitly supplied through flags or environment.
// Setting flags here also preserves whether file storage was explicitly configured.
func applyFile(path string, options []fileOption) error {
	if value, ok := os.LookupEnv("CONFIG"); ok {
		path = value
	}
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %q: %w", path, err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("decode config %q: %w", path, err)
	}
	if fields == nil {
		return fmt.Errorf("config %q must be a JSON object", path)
	}
	known := make(map[string]bool, len(options))
	for _, option := range options {
		known[option.field] = true
	}
	for field := range fields {
		if !known[field] {
			return fmt.Errorf("unknown config field %q", field)
		}
	}
	explicit := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	for _, option := range options {
		raw, ok := fields[option.field]
		if !ok || explicit[option.flag] {
			continue
		}
		overridden := false
		for _, name := range option.env {
			if _, ok := os.LookupEnv(name); ok {
				overridden = true
			}
		}
		if overridden {
			continue
		}
		value, err := fileFlagValue(raw, option)
		if err != nil {
			return fmt.Errorf("config field %q: %w", option.field, err)
		}
		if err := flag.Set(option.flag, value); err != nil {
			return fmt.Errorf("config field %q: %w", option.field, err)
		}
	}
	return nil
}

func fileFlagValue(raw json.RawMessage, option fileOption) (string, error) {
	if string(raw) == "null" {
		return "", fmt.Errorf("null is not a configuration value")
	}
	if option.duration {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("expected duration string: %w", err)
		}
		duration, err := time.ParseDuration(value)
		if err != nil {
			return "", fmt.Errorf("parse duration: %w", err)
		}
		if duration%time.Second != 0 {
			return "", fmt.Errorf("duration must be a whole number of seconds")
		}
		return strconv.FormatInt(int64(duration/time.Second), 10), nil
	}
	switch flag.Lookup(option.flag).Value.(flag.Getter).Get().(type) {
	case string:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", err
		}
		return value, nil
	case int:
		var value int
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", err
		}
		return strconv.Itoa(value), nil
	case bool:
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", err
		}
		return strconv.FormatBool(value), nil
	default:
		return "", fmt.Errorf("unsupported flag type")
	}
}
