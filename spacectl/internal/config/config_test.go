package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromMissingFile(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "config.yaml")

	// Act
	cfg, err := LoadFrom(path)

	// Assert
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if *cfg != (Config{}) {
		t.Errorf("LoadFrom() = %+v, want empty config", cfg)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	want := &Config{
		Server:   "https://kubespaces.example.com",
		Issuer:   "https://keycloak.example.com/realms/kubespaces",
		ClientID: "kubespaces",
	}

	// Act
	if err := SaveTo(path, want); err != nil {
		t.Fatalf("SaveTo() error = %v", err)
	}
	got, err := LoadFrom(path)

	// Assert
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if *got != *want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config mode = %o, want 600", perm)
	}
}

func TestLoadFromInvalidYAML(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("{not yaml"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Act
	_, err := LoadFrom(path)

	// Assert
	if err == nil {
		t.Error("LoadFrom() error = nil, want parse error")
	}
}

func TestResolveServerPrecedence(t *testing.T) {
	// Arrange
	tests := []struct {
		name string
		flag string
		env  string
		cfg  string
		want string
	}{
		{"flag wins over env and config", "https://flag", "https://env", "https://cfg", "https://flag"},
		{"env wins over config", "", "https://env", "https://cfg", "https://env"},
		{"config as fallback", "", "", "https://cfg", "https://cfg"},
		{"nothing set", "", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv(EnvServer, tt.env)
			} else {
				t.Setenv(EnvServer, "")
			}
			cfg := &Config{Server: tt.cfg}

			// Act
			got := cfg.ResolveServer(tt.flag)

			// Assert
			if got != tt.want {
				t.Errorf("ResolveServer(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

func TestDirHonorsXDGConfigHome(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Act
	dir, err := Dir()

	// Assert
	if err != nil {
		t.Fatalf("Dir() error = %v", err)
	}
	if want := filepath.Join(tmp, "spacectl"); dir != want {
		t.Errorf("Dir() = %q, want %q", dir, want)
	}
}
