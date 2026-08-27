package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Load resolves four layers in a fixed order: defaults, preset, config file,
// environment. Nothing tested that order, and getting it wrong fails quietly -
// a user sets a value, the run ignores it, and the result is attributed to a
// configuration that was never in effect.

// isolatedLoad points every search path at a temporary directory so a developer's
// real ~/.config/logprobe and the repository's own .env cannot reach the test.
func isolatedLoad(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("EVALWITNESS_ENV_FILE", filepath.Join(dir, "absent.env"))
	t.Setenv("EVALWITNESS_CONFIG_FILE", filepath.Join(dir, "absent.toml"))
	// Change into the temp dir so the relative ".env" and "logprobe.toml" probes
	// cannot pick up the repository's files.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	return dir
}

func TestLoadWithNoConfigurationYieldsTheDefaultRoute(t *testing.T) {
	isolatedLoad(t)
	t.Setenv("BAI_API_KEY", "k")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	def := Default()
	if cfg.Provider != def.Provider || cfg.Model != def.Model || cfg.BaseURL != def.BaseURL {
		t.Fatalf("bare Load gave %s/%s/%s, want the default route", cfg.Provider, cfg.Model, cfg.BaseURL)
	}
}

func TestLoadForDiagnosticsReportsConfigurationWithoutAPIKey(t *testing.T) {
	isolatedLoad(t)
	t.Setenv("OPENCODE_GO_CN_API_KEY", "")
	t.Setenv("OPENCODE_GO_API_KEY", "")
	t.Setenv("OPENCODE_API_KEY", "")
	t.Setenv("EVALWITNESS_API_KEY", "")
	t.Setenv("LOGPROBE_API_KEY", "")

	cfg, err := LoadForDiagnostics()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "" {
		t.Fatal("diagnostic load manufactured an API key")
	}
	if cfg.Provider != "bai" || cfg.Model != "deepseek-v4-flash" {
		t.Fatalf("diagnostic route = %s/%s", cfg.Provider, cfg.Model)
	}
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "API key required") {
		t.Fatalf("production load without a key error = %v", err)
	}
}

func TestLoadAllowsExactReplayWithoutAPIKey(t *testing.T) {
	dir := isolatedLoad(t)
	for _, key := range []string{
		"OPENCODE_GO_CN_API_KEY", "OPENCODE_GO_API_KEY", "OPENCODE_API_KEY",
		"EVALWITNESS_API_KEY", "LOGPROBE_API_KEY",
	} {
		t.Setenv(key, "")
	}
	replayPath := filepath.Join(dir, "exact-replay.jsonl")
	t.Setenv("EVALWITNESS_REPLAY_FROM", replayPath)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "" || cfg.ReplayFrom != replayPath {
		t.Fatalf("exact replay load = key %q fixture %q", cfg.APIKey, cfg.ReplayFrom)
	}
}

func TestLoadRejectsAnUnknownPresetAndNamesTheAlternatives(t *testing.T) {
	isolatedLoad(t)
	t.Setenv("EVALWITNESS_PRESET", "not-a-preset")

	_, err := Load()
	if err == nil {
		t.Fatal("an unknown preset loaded without error")
	}
	// The message has to list what is available, because a typo is the common
	// case and the reader cannot guess the registry.
	for _, name := range PresetNames() {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("error %q does not mention available preset %s", err, name)
		}
	}
}

func TestEnvironmentOverridesTheConfigFile(t *testing.T) {
	dir := isolatedLoad(t)
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
model = "from-file"
max_workers = 3
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVALWITNESS_CONFIG_FILE", path)
	t.Setenv("EVALWITNESS_MODEL", "from-env")
	t.Setenv("BAI_API_KEY", "k")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "from-env" {
		t.Fatalf("model = %q, want the environment to win over the file", cfg.Model)
	}
	// A file value with no environment counterpart still has to apply, otherwise
	// the file would be pointless.
	if cfg.MaxWorkers != 3 {
		t.Fatalf("max workers = %d, want 3 from the file", cfg.MaxWorkers)
	}
}

func TestProviderEnvironmentKeyReplacesPresetCredentialAfterRouteOverride(t *testing.T) {
	isolatedLoad(t)
	t.Setenv("EVALWITNESS_PRESET", "bai-deepseek-v4-flash")
	t.Setenv("BAI_API_KEY", "bai-key")
	t.Setenv("EVALWITNESS_PROVIDER", "fireworks")
	t.Setenv("EVALWITNESS_BASE_URL", "https://api.fireworks.ai/inference/v1")
	t.Setenv("EVALWITNESS_MODEL", "deepseek-v4-flash-0731")
	t.Setenv("FIREWORKS_API_KEY", "fireworks-key")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "fireworks" || cfg.APIKey != "fireworks-key" {
		t.Fatalf("route credential = provider %q key %q, want fireworks/fireworks-key", cfg.Provider, cfg.APIKey)
	}
}

func TestConfigFileOverridesThePreset(t *testing.T) {
	dir := isolatedLoad(t)
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("model = \"file-model\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVALWITNESS_CONFIG_FILE", path)
	t.Setenv("EVALWITNESS_PRESET", "deepseek-v4-pro")
	t.Setenv("DEEPSEEK_API_KEY", "k")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "file-model" {
		t.Fatalf("model = %q, want the file to win over the preset", cfg.Model)
	}
	// The preset's other fields must survive: a file that sets one key must not
	// reset the rest of the route.
	if cfg.Provider != "deepseek" {
		t.Fatalf("provider = %q, want the preset's provider preserved", cfg.Provider)
	}
}

func TestEnvFileDoesNotOverrideAlreadySetEnvironment(t *testing.T) {
	// A .env is a fallback for unset values. Letting it overwrite a variable the
	// caller exported would make an explicit command-line environment silently
	// ineffective.
	dir := isolatedLoad(t)
	envPath := filepath.Join(dir, "test.env")
	if err := os.WriteFile(envPath, []byte("EVALWITNESS_MODEL=from-env-file\nEVALWITNESS_LOG_LEVEL=debug\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVALWITNESS_ENV_FILE", envPath)
	t.Setenv("EVALWITNESS_MODEL", "already-exported")
	t.Setenv("BAI_API_KEY", "k")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "already-exported" {
		t.Fatalf("model = %q; the env file overwrote an exported variable", cfg.Model)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("log level = %q, want the env file to fill an unset variable", cfg.LogLevel)
	}
	// Setenv from the file leaks into the process, so undo it for later tests.
	t.Cleanup(func() { _ = os.Unsetenv("EVALWITNESS_LOG_LEVEL") })
}

func TestEnvFileParsingHandlesCommentsQuotesAndJunk(t *testing.T) {
	dir := isolatedLoad(t)
	envPath := filepath.Join(dir, "messy.env")
	content := strings.Join([]string{
		"# a comment",
		"",
		"   ",
		"EVALWITNESS_MODEL = \"quoted-model\"  ",
		"EVALWITNESS_LOG_LEVEL='warn'",
		"this line has no equals sign",
		"EVALWITNESS_MAX_WORKERS=4",
	}, "\n")
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVALWITNESS_ENV_FILE", envPath)
	t.Setenv("BAI_API_KEY", "k")
	t.Cleanup(func() {
		for _, k := range []string{"EVALWITNESS_MODEL", "EVALWITNESS_LOG_LEVEL", "EVALWITNESS_MAX_WORKERS"} {
			_ = os.Unsetenv(k)
		}
	})

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "quoted-model" {
		t.Fatalf("model = %q, want quotes and surrounding spaces stripped", cfg.Model)
	}
	if cfg.LogLevel != "warn" {
		t.Fatalf("log level = %q, want single quotes stripped", cfg.LogLevel)
	}
	if cfg.MaxWorkers != 4 {
		t.Fatalf("max workers = %d, want 4; a junk line aborted parsing", cfg.MaxWorkers)
	}
}

func TestMalformedConfigFileFailsLoudlyAndNamesThePath(t *testing.T) {
	// A silently ignored config file is worse than a rejected one: the run
	// proceeds under settings the user believes they changed.
	dir := isolatedLoad(t)
	path := filepath.Join(dir, "broken.toml")
	if err := os.WriteFile(path, []byte("model = = = \"broken\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVALWITNESS_CONFIG_FILE", path)

	_, err := Load()
	if err == nil {
		t.Fatal("a malformed config file loaded without error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error %q does not name the offending file", err)
	}
}

func TestJSONConfigFileIsAcceptedToo(t *testing.T) {
	dir := isolatedLoad(t)
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"model":"json-model","max_retries":9}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVALWITNESS_CONFIG_FILE", path)
	t.Setenv("BAI_API_KEY", "k")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "json-model" || cfg.MaxRetries != 9 {
		t.Fatalf("model = %q retries = %d, want the JSON file applied", cfg.Model, cfg.MaxRetries)
	}
}

func TestAbsentConfigAndEnvFilesAreNotErrors(t *testing.T) {
	// Both are optional. Pointing at a missing one must not fail the run.
	dir := isolatedLoad(t)
	t.Setenv("EVALWITNESS_CONFIG_FILE", filepath.Join(dir, "does-not-exist.toml"))
	t.Setenv("EVALWITNESS_ENV_FILE", filepath.Join(dir, "does-not-exist.env"))
	t.Setenv("BAI_API_KEY", "k")

	if _, err := Load(); err != nil {
		t.Fatalf("missing optional files produced %v", err)
	}
}

func TestLoadValidatesAndRejectsAnImpossibleConfiguration(t *testing.T) {
	isolatedLoad(t)
	t.Setenv("BAI_API_KEY", "k")
	t.Setenv("EVALWITNESS_BASE_URL", "not-a-url")

	if _, err := Load(); err == nil {
		t.Fatal("a base URL with no scheme was accepted")
	}
}

func TestSearchPathsPreferTheExplicitOverride(t *testing.T) {
	// EVALWITNESS_ENV_FILE and EVALWITNESS_CONFIG_FILE must come first, so a caller can
	// pin configuration for one run without a developer's home directory
	// interfering. run-claimcheck.sh depends on exactly this.
	t.Setenv("EVALWITNESS_ENV_FILE", "/tmp/explicit.env")
	resolver := newEnvironmentResolver(os.LookupEnv)
	if got := envFileSearchPaths(resolver); len(got) == 0 || got[0] != "/tmp/explicit.env" {
		t.Fatalf("env search order = %v, want the override first", got)
	}
	t.Setenv("EVALWITNESS_CONFIG_FILE", "/tmp/explicit.toml")
	if got := configFileSearchPaths(resolver); len(got) == 0 || got[0].Path != "/tmp/explicit.toml" {
		t.Fatalf("config search order = %v, want the override first", got)
	}
}

func TestCanonicalEnvironmentWinsOverLegacyAndLegacyFallsBack(t *testing.T) {
	t.Run("canonical wins", func(t *testing.T) {
		isolatedLoad(t)
		t.Setenv("EVALWITNESS_MODEL", "canonical-model")
		t.Setenv("LOGPROBE_MODEL", "legacy-model")
		t.Setenv("BAI_API_KEY", "k")

		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Model != "canonical-model" {
			t.Fatalf("model = %q, want canonical-model", cfg.Model)
		}
	})

	t.Run("legacy fallback", func(t *testing.T) {
		isolatedLoad(t)
		t.Setenv("LOGPROBE_MODEL", "legacy-model")
		t.Setenv("BAI_API_KEY", "k")

		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Model != "legacy-model" {
			t.Fatalf("model = %q, want legacy-model", cfg.Model)
		}
	})
}

func TestLegacyCacheDirIsReadOnlyImportNotPrimaryWriter(t *testing.T) {
	dir := isolatedLoad(t)
	legacy := filepath.Join(dir, "legacy-cache")
	t.Setenv("LOGPROBE_CACHE_DIR", legacy)
	t.Setenv("BAI_API_KEY", "k")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LegacyCacheDir != legacy {
		t.Fatalf("legacy cache dir = %q, want %q", cfg.LegacyCacheDir, legacy)
	}
	if cfg.CacheDir == legacy || !strings.HasSuffix(cfg.CacheDir, filepath.Join(".cache", "evalwitness")) {
		t.Fatalf("primary cache dir = %q; legacy path became writable", cfg.CacheDir)
	}
}

func TestCanonicalCacheDirSuppressesLegacyCacheSetting(t *testing.T) {
	dir := isolatedLoad(t)
	canonical := filepath.Join(dir, "canonical-cache")
	t.Setenv("EVALWITNESS_CACHE_DIR", canonical)
	t.Setenv("LOGPROBE_CACHE_DIR", filepath.Join(dir, "legacy-cache"))
	t.Setenv("BAI_API_KEY", "k")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CacheDir != canonical || cfg.LegacyCacheDir != "" {
		t.Fatalf("cache dirs = primary %q legacy %q", cfg.CacheDir, cfg.LegacyCacheDir)
	}
}

func TestCanonicalConfigFilesPrecedeLegacyFallbacks(t *testing.T) {
	dir := isolatedLoad(t)
	canonical := filepath.Join(dir, "evalwitness.toml")
	if err := os.WriteFile(canonical, []byte("model = \"canonical-file\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacyHomeDir := filepath.Join(dir, ".config", "logprobe")
	if err := os.MkdirAll(legacyHomeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyHomeDir, "config.toml"), []byte("model = \"legacy-file\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BAI_API_KEY", "k")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "canonical-file" {
		t.Fatalf("model = %q, canonical project config did not beat legacy home config", cfg.Model)
	}
}

func TestLegacyExplicitConfigCannotOverrideCanonicalProjectConfig(t *testing.T) {
	dir := isolatedLoad(t)
	legacyPath := filepath.Join(dir, "legacy.toml")
	if err := os.WriteFile(legacyPath, []byte("model = \"legacy-explicit\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evalwitness.toml"), []byte("model = \"canonical-project\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOGPROBE_CONFIG_FILE", legacyPath)
	t.Setenv("BAI_API_KEY", "k")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "canonical-project" {
		t.Fatalf("model = %q, want canonical project config", cfg.Model)
	}
}

func TestLegacyConfigCacheDirBecomesReadOnlyImport(t *testing.T) {
	dir := isolatedLoad(t)
	if err := os.Unsetenv("EVALWITNESS_CONFIG_FILE"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("LOGPROBE_CONFIG_FILE") })
	legacyPath := filepath.Join(dir, "legacy.toml")
	legacyCache := filepath.Join(dir, "legacy-cache")
	content := "model = \"legacy-file\"\ncache_dir = \"" + legacyCache + "\"\n"
	if err := os.WriteFile(legacyPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOGPROBE_CONFIG_FILE", legacyPath)
	t.Setenv("BAI_API_KEY", "k")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "legacy-file" || cfg.LegacyCacheDir != legacyCache {
		t.Fatalf("legacy config = model %q cache %q", cfg.Model, cfg.LegacyCacheDir)
	}
	if cfg.CacheDir == legacyCache {
		t.Fatal("legacy config cache directory became the primary writer")
	}
}
