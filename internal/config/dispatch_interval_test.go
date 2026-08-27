package config

import "testing"

func TestDispatchIntervalEnvironmentIsValidated(t *testing.T) {
	settings := map[string]string{string(EnvMinDispatchInterval): "90"}
	resolver := newEnvironmentResolver(func(key string) (string, bool) {
		value, ok := settings[key]
		return value, ok
	})
	configuration := Default()
	configuration.applyEnv(resolver)
	if configuration.MinDispatchIntervalSec != 90 {
		t.Fatalf("dispatch interval = %d, want 90", configuration.MinDispatchIntervalSec)
	}
	configuration.MinDispatchIntervalSec = -1
	if err := configuration.validateWithoutAPIKey(); err == nil {
		t.Fatal("negative dispatch interval was accepted")
	}
}
