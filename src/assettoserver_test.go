package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A realistic (>extraCfgMinSize) stand-in for AssettoServer's generated config.
func sampleFullExtraCfg() string {
	return "!extra_cfg.yml\n" +
		strings.Repeat("# AssettoServer configuration line\n", 30) +
		"UseSteamAuth: false\n" +
		"EnablePlugins: []\n" +
		"IgnoreConfigurationErrors:\n" +
		"  MissingCarChecksums: false\n" +
		"  MissingTrackParams: false\n" +
		"  WrongServerDetails: false\n"
}

func writeCfg(t *testing.T, dir, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "cfg"), 0755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "cfg", "extra_cfg.yml")
	if content != "" {
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func TestEnsureExtraCfg(t *testing.T) {
	old := TempFolder
	defer func() { TempFolder = old }()

	const stub = "!extra_cfg.yml\nIgnoreConfigurationErrors:\n  MissingCarChecksums: true\n  MissingTrackParams: true\n"

	t.Run("default instance with full config is patched in place", func(t *testing.T) {
		root := t.TempDir()
		TempFolder = root
		p := writeCfg(t, root, sampleFullExtraCfg())

		ensureAssettoServerExtraCfg(root, true)

		got, _ := os.ReadFile(p)
		s := string(got)
		if !strings.Contains(s, "MissingCarChecksums: true") || !strings.Contains(s, "MissingTrackParams: true") {
			t.Fatalf("relax keys not set true:\n%s", s)
		}
		if !strings.Contains(s, "UseSteamAuth: false") || !strings.Contains(s, "EnablePlugins: []") {
			t.Fatalf("other keys lost:\n%s", s)
		}
	})

	t.Run("fresh instance stub is seeded from default then patched", func(t *testing.T) {
		root := t.TempDir()
		TempFolder = root
		writeCfg(t, root, sampleFullExtraCfg()) // default instance config

		inst2 := filepath.Join(root, "instance_2")
		p2 := writeCfg(t, inst2, stub) // invalid stub an old build wrote

		ensureAssettoServerExtraCfg(inst2, true)

		got, _ := os.ReadFile(p2)
		s := string(got)
		if len(got) < extraCfgMinSize || !strings.Contains(s, "UseSteamAuth: false") {
			t.Fatalf("stub not replaced by seeded full config:\n%s", s)
		}
		if !strings.Contains(s, "MissingCarChecksums: true") || !strings.Contains(s, "MissingTrackParams: true") {
			t.Fatalf("seeded config not patched:\n%s", s)
		}
	})

	t.Run("stub removed when no default to seed from", func(t *testing.T) {
		root := t.TempDir()
		TempFolder = root // default instance has no extra_cfg.yml

		inst3 := filepath.Join(root, "instance_3")
		p3 := writeCfg(t, inst3, stub)

		ensureAssettoServerExtraCfg(inst3, true)

		if _, err := os.Stat(p3); !os.IsNotExist(err) {
			t.Fatal("stub should be removed so AssettoServer regenerates a valid default")
		}
	})

	t.Run("relax off sets keys false", func(t *testing.T) {
		root := t.TempDir()
		TempFolder = root
		p := writeCfg(t, root, sampleFullExtraCfg())

		ensureAssettoServerExtraCfg(root, false)

		got, _ := os.ReadFile(p)
		if strings.Contains(string(got), "MissingCarChecksums: true") {
			t.Fatalf("expected false:\n%s", got)
		}
	})

	t.Run("legacy plugin interface is always enabled", func(t *testing.T) {
		root := t.TempDir()
		TempFolder = root
		p := writeCfg(t, root, sampleFullExtraCfg()) // no EnableLegacyPluginInterface line

		ensureAssettoServerExtraCfg(root, false)

		got, _ := os.ReadFile(p)
		s := string(got)
		if !strings.Contains(s, "EnableLegacyPluginInterface: true") {
			t.Fatalf("legacy plugin interface not enabled:\n%s", s)
		}
		// Existing keys preserved.
		if !strings.Contains(s, "UseSteamAuth: false") || !strings.Contains(s, "EnablePlugins: []") {
			t.Fatalf("other keys lost:\n%s", s)
		}
	})

	t.Run("existing legacy plugin interface false is flipped to true", func(t *testing.T) {
		root := t.TempDir()
		TempFolder = root
		p := writeCfg(t, root, sampleFullExtraCfg()+"EnableLegacyPluginInterface: false\n")

		ensureAssettoServerExtraCfg(root, true)

		got, _ := os.ReadFile(p)
		s := string(got)
		if strings.Contains(s, "EnableLegacyPluginInterface: false") {
			t.Fatalf("legacy interface still false:\n%s", s)
		}
		if !strings.Contains(s, "EnableLegacyPluginInterface: true") {
			t.Fatalf("legacy interface not true:\n%s", s)
		}
	})
}

func TestSetYamlTopLevelBool(t *testing.T) {
	t.Run("appends when absent", func(t *testing.T) {
		out := setYamlTopLevelBool("UseSteamAuth: false\n", "EnableLegacyPluginInterface", true)
		if !strings.Contains(out, "EnableLegacyPluginInterface: true") {
			t.Fatalf("not appended:\n%s", out)
		}
		if !strings.Contains(out, "UseSteamAuth: false") {
			t.Fatalf("existing content lost:\n%s", out)
		}
	})

	t.Run("patches in place when present", func(t *testing.T) {
		out := setYamlTopLevelBool("EnableLegacyPluginInterface: false\nUseSteamAuth: false\n", "EnableLegacyPluginInterface", true)
		if strings.Contains(out, "EnableLegacyPluginInterface: false") {
			t.Fatalf("old value not replaced:\n%s", out)
		}
		if strings.Count(out, "EnableLegacyPluginInterface:") != 1 {
			t.Fatalf("key duplicated:\n%s", out)
		}
	})

	t.Run("idempotent when already correct", func(t *testing.T) {
		in := "EnableLegacyPluginInterface: true\n"
		if out := setYamlTopLevelBool(in, "EnableLegacyPluginInterface", true); out != in {
			t.Fatalf("expected no change, got:\n%s", out)
		}
	})
}
