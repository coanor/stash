package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigWithDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	dbPath := filepath.Join(dir, "stash.sqlite")

	err := os.WriteFile(configPath, []byte(`database_path = "`+dbPath+`"`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.DatabasePath != dbPath {
		t.Fatalf("DatabasePath = %q, want %q", cfg.DatabasePath, dbPath)
	}
	if cfg.GraphicsMode != GraphicsAuto {
		t.Fatalf("GraphicsMode = %q, want %q", cfg.GraphicsMode, GraphicsAuto)
	}
	if cfg.CacheDir == "" {
		t.Fatal("CacheDir should have a default")
	}
	if cfg.LogFile == "" {
		t.Fatal("LogFile should have a default")
	}
	if !strings.HasSuffix(cfg.LogFile, filepath.Join("stash-cli", "stash-cli.log")) {
		t.Fatalf("LogFile = %q, want stash-cli/stash-cli.log suffix", cfg.LogFile)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.LogStdout {
		t.Fatal("LogStdout should default to false")
	}
	if cfg.Blobs.Storage != "database" {
		t.Fatalf("Blobs.Storage = %q, want database", cfg.Blobs.Storage)
	}
	if cfg.PlayerPath == "" {
		t.Fatal("PlayerPath should have a default")
	}
	if len(cfg.PlayerArgs) == 0 {
		t.Fatal("PlayerArgs should have a default")
	}
}

func TestDefaultPathUsesUserConfigDir(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	got := DefaultPath()
	want := filepath.Join(configDir, "stash-cli", "config.toml")
	if got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestLoadConfigCanSetPlayerCommand(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(configPath, []byte(`
database_path = "/tmp/stash.sqlite"
player_path = "/usr/local/bin/mpv"
player_args = ["--force-window=yes", "--keep-open=no"]
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.PlayerPath != "/usr/local/bin/mpv" {
		t.Fatalf("PlayerPath = %q, want configured path", cfg.PlayerPath)
	}
	if got := strings.Join(cfg.PlayerArgs, ","); got != "--force-window=yes,--keep-open=no" {
		t.Fatalf("PlayerArgs = %q, want configured args", got)
	}
}

func TestLoadConfigDefaultsMPVArgs(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(configPath, []byte(`
database_path = "/tmp/stash.sqlite"
player_path = "/usr/local/bin/mpv"
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if got := strings.Join(cfg.PlayerArgs, ","); got != "--force-window=yes" {
		t.Fatalf("PlayerArgs = %q, want mpv default args", got)
	}
}

func TestLoadConfigAcceptsLegacyFFplayCommand(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(configPath, []byte(`
database_path = "/tmp/stash.sqlite"
ffplay_path = "/usr/local/bin/ffplay"
ffplay_args = ["-autoexit", "-fs"]
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.PlayerPath != "/usr/local/bin/ffplay" {
		t.Fatalf("PlayerPath = %q, want legacy ffplay path", cfg.PlayerPath)
	}
	if got := strings.Join(cfg.PlayerArgs, ","); got != "-autoexit,-fs" {
		t.Fatalf("PlayerArgs = %q, want legacy ffplay args", got)
	}
}

func TestLoadConfigCanUseFilesystemBlobs(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	blobsPath := filepath.Join(dir, "blobs")
	err := os.WriteFile(configPath, []byte(`
database_path = "/tmp/stash.sqlite"

[blobs]
storage = "filesystem"
path = "`+blobsPath+`"
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Blobs.Storage != "filesystem" {
		t.Fatalf("Blobs.Storage = %q, want filesystem", cfg.Blobs.Storage)
	}
	if cfg.Blobs.Path != blobsPath {
		t.Fatalf("Blobs.Path = %q, want %q", cfg.Blobs.Path, blobsPath)
	}
}

func TestLoadConfigValidatesFilesystemBlobsPath(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(configPath, []byte(`
database_path = "/tmp/stash.sqlite"

[blobs]
storage = "filesystem"
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "blobs.path") {
		t.Fatalf("error = %v, want blobs.path validation", err)
	}
}

func TestLoadConfigValidatesStartupScanDirs(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(configPath, []byte(`
database_path = "/tmp/stash.sqlite"
scan_on_startup = true
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "media_dirs") {
		t.Fatalf("error = %v, want media_dirs validation", err)
	}
}

func TestLoadConfigValidatesGraphicsMode(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(configPath, []byte(`
database_path = "/tmp/stash.sqlite"
graphics_mode = "sixel"
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "graphics_mode") {
		t.Fatalf("error = %v, want graphics_mode validation", err)
	}
}

func TestExampleConfigIsLoadable(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte(Example()), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ScanOnStartup {
		t.Fatal("example should enable startup scan")
	}
	if len(cfg.MediaDirs) == 0 {
		t.Fatal("example should include media directories")
	}
}
