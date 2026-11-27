package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTheme(t *testing.T) {
	def, err := resolveTheme("", theme{})
	if err != nil {
		t.Fatalf("resolveTheme default: %v", err)
	}
	if def != themes["default"] {
		t.Fatalf("resolveTheme(\"\") = %+v, want %+v", def, themes["default"])
	}

	ocean, err := resolveTheme("ocean", theme{Accent: "#ffcc00", Wrong: "196"})
	if err != nil {
		t.Fatalf("resolveTheme ocean: %v", err)
	}
	if ocean.Accent != "#ffcc00" || ocean.Wrong != "196" {
		t.Fatalf("overrides not applied: %+v", ocean)
	}
	if ocean.Correct != themes["ocean"].Correct || ocean.Pending != themes["ocean"].Pending {
		t.Fatalf("non-overridden colours changed: %+v", ocean)
	}

	if _, err := resolveTheme("nonexistent", theme{}); err == nil || !strings.Contains(err.Error(), "unknown theme") {
		t.Fatalf("resolveTheme(unknown) err = %v, want unknown theme error", err)
	}
}

func TestConfigSaveAndLoadSettings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Missing config.json gives default settings.
	conf, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig missing: %v", err)
	}
	if got := conf.toSettings(); got != defaultSettings() {
		t.Fatalf("default toSettings() = %+v, want %+v", got, defaultSettings())
	}

	// Pre-populate config.json with theme and colour overrides.
	cfgPath := filepath.Join(dir, "termtype", "config.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := `{
  "theme": "ocean",
  "colors": {
    "accent": "#ffcc00"
  }
}`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	want := settings{
		mode:    modeWords,
		seconds: 60,
		words:   50,
		punct:   true,
		nums:    true,
	}
	if err := saveSettings(want); err != nil {
		t.Fatalf("saveSettings: %v", err)
	}

	// Custom mode must not overwrite saved settings.
	if err := saveSettings(settings{mode: modeCustom, customText: "hello"}); err != nil {
		t.Fatalf("saveSettings(custom): %v", err)
	}

	reloaded, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig after save: %v", err)
	}
	if reloaded.Theme != "ocean" || reloaded.Colors.Accent != "#ffcc00" {
		t.Fatalf("theme/colors clobbered: %+v", reloaded)
	}
	if got := reloaded.toSettings(); got != want {
		t.Fatalf("reloaded.toSettings() = %+v, want %+v", got, want)
	}
}
