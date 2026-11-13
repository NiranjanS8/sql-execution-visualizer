package config

import (
	"reflect"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("QUERYLENS_ADDR", "")
	t.Setenv("QUERYLENS_ALLOWED_ORIGINS", "")

	cfg := Load()

	if cfg.Addr != DefaultAddr {
		t.Errorf("addr = %q, want %q", cfg.Addr, DefaultAddr)
	}
	if want := []string{DefaultAllowedOrigins}; !reflect.DeepEqual(cfg.AllowedOrigins, want) {
		t.Errorf("allowed origins = %v, want %v", cfg.AllowedOrigins, want)
	}
	if cfg.ShutdownGrace != DefaultShutdownGrace {
		t.Errorf("shutdown grace = %v, want %v", cfg.ShutdownGrace, DefaultShutdownGrace)
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("QUERYLENS_ADDR", " 127.0.0.1:9999 ")
	t.Setenv("QUERYLENS_ALLOWED_ORIGINS", " http://localhost:5173 , http://localhost:4173 ,, ")

	cfg := Load()

	if cfg.Addr != "127.0.0.1:9999" {
		t.Errorf("addr = %q, want trimmed value", cfg.Addr)
	}
	want := []string{"http://localhost:5173", "http://localhost:4173"}
	if !reflect.DeepEqual(cfg.AllowedOrigins, want) {
		t.Errorf("allowed origins = %v, want %v", cfg.AllowedOrigins, want)
	}
}

func TestSplitList(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "single", raw: "a", want: []string{"a"}},
		{name: "trims and drops empties", raw: " a , , b ", want: []string{"a", "b"}},
		{name: "empty input", raw: "", want: []string{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := splitList(test.raw); !reflect.DeepEqual(got, test.want) {
				t.Errorf("splitList(%q) = %v, want %v", test.raw, got, test.want)
			}
		})
	}
}
