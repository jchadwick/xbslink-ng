package config

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_SaveAndLoad(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Test saving config
	cfg := &Config{
		LastXboxMAC: "00:50:F2:1A:2B:3C",
	}

	if err := cfg.SaveTo(configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Test loading config
	loaded, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loaded.LastXboxMAC != cfg.LastXboxMAC {
		t.Errorf("Expected LastXboxMAC %q, got %q", cfg.LastXboxMAC, loaded.LastXboxMAC)
	}
}

func TestConfig_LoadNonExistent(t *testing.T) {
	// Test loading from non-existent file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.json")

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("Expected no error when loading non-existent file, got: %v", err)
	}

	if cfg.LastXboxMAC != "" {
		t.Errorf("Expected empty config, got LastXboxMAC=%q", cfg.LastXboxMAC)
	}
}

func TestConfig_GetXboxMAC(t *testing.T) {
	tests := []struct {
		name        string
		macStr      string
		expectNil   bool
		expectValue string
	}{
		{
			name:        "valid MAC",
			macStr:      "00:50:F2:1A:2B:3C",
			expectNil:   false,
			expectValue: "00:50:f2:1a:2b:3c",
		},
		{
			name:      "empty MAC",
			macStr:    "",
			expectNil: true,
		},
		{
			name:      "invalid MAC",
			macStr:    "invalid",
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{LastXboxMAC: tt.macStr}
			mac := cfg.GetXboxMAC()

			if tt.expectNil {
				if mac != nil {
					t.Errorf("Expected nil MAC, got %v", mac)
				}
			} else {
				if mac == nil {
					t.Fatal("Expected non-nil MAC, got nil")
				}
				if mac.String() != tt.expectValue {
					t.Errorf("Expected MAC %q, got %q", tt.expectValue, mac.String())
				}
			}
		})
	}
}

func TestConfig_SetXboxMAC(t *testing.T) {
	cfg := &Config{}
	mac, _ := net.ParseMAC("00:50:F2:1A:2B:3C")

	cfg.SetXboxMAC(mac)

	if cfg.LastXboxMAC != "00:50:f2:1a:2b:3c" {
		t.Errorf("Expected LastXboxMAC %q, got %q", "00:50:f2:1a:2b:3c", cfg.LastXboxMAC)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("Failed to get default config path: %v", err)
	}

	if path == "" {
		t.Error("Expected non-empty config path")
	}

	// Verify it ends with .xbslink-ng/config.json
	if filepath.Base(path) != "config.json" {
		t.Errorf("Expected config filename to be config.json, got %q", filepath.Base(path))
	}

	dir := filepath.Dir(path)
	if filepath.Base(dir) != ".xbslink-ng" {
		t.Errorf("Expected config directory to be .xbslink-ng, got %q", filepath.Base(dir))
	}
}
