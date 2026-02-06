// Package config provides persistent configuration storage for xbslink-ng.
package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Config holds the persistent configuration.
type Config struct {
	// LastXboxMAC is the MAC address of the last discovered Xbox.
	LastXboxMAC string `json:"last_xbox_mac,omitempty"`
}

// DefaultConfigDir returns the default configuration directory.
// Returns ~/.xbslink-ng on Unix-like systems, %USERPROFILE%\.xbslink-ng on Windows.
func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".xbslink-ng"), nil
}

// DefaultConfigPath returns the default configuration file path.
func DefaultConfigPath() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the configuration from the default config file.
// Returns an empty Config if the file doesn't exist.
func Load() (*Config, error) {
	path, err := DefaultConfigPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads the configuration from the specified file path.
// Returns an empty Config if the file doesn't exist.
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, return empty config
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// Save writes the configuration to the default config file.
func (c *Config) Save() error {
	path, err := DefaultConfigPath()
	if err != nil {
		return err
	}
	return c.SaveTo(path)
}

// SaveTo writes the configuration to the specified file path.
func (c *Config) SaveTo(path string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetXboxMAC returns the saved Xbox MAC address as a net.HardwareAddr.
// Returns nil if no MAC is saved or if the saved MAC is invalid.
func (c *Config) GetXboxMAC() net.HardwareAddr {
	if c.LastXboxMAC == "" {
		return nil
	}

	mac, err := net.ParseMAC(c.LastXboxMAC)
	if err != nil {
		return nil
	}

	return mac
}

// SetXboxMAC saves the Xbox MAC address.
func (c *Config) SetXboxMAC(mac net.HardwareAddr) {
	c.LastXboxMAC = mac.String()
}
