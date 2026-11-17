package config

import (
	"reflect"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("QUERYLENS_ADDR", "")
	t.Setenv("QUERYLENS_ALLOWED_ORIGINS", "")
	t.Setenv("QUERYLENS_QUERY_MAX_SQL_LENGTH", "")
	t.Setenv("QUERYLENS_QUERY_TIMEOUT", "")
	t.Setenv("QUERYLENS_QUERY_MAX_ROWS", "")
	t.Setenv("QUERYLENS_QUERY_SAMPLE_LIMIT", "")

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
	if cfg.QueryMaxSQLLength != DefaultQueryMaxSQLLength {
		t.Errorf("query max SQL length = %d, want %d", cfg.QueryMaxSQLLength, DefaultQueryMaxSQLLength)
	}
	if cfg.QueryTimeout != DefaultQueryTimeout {
		t.Errorf("query timeout = %v, want %v", cfg.QueryTimeout, DefaultQueryTimeout)
	}
	if cfg.QueryMaxRows != DefaultQueryMaxRows {
		t.Errorf("query max rows = %d, want %d", cfg.QueryMaxRows, DefaultQueryMaxRows)
	}
	if cfg.QuerySampleLimit != DefaultQuerySampleLimit {
		t.Errorf("query sample limit = %d, want %d", cfg.QuerySampleLimit, DefaultQuerySampleLimit)
	}
}

func TestLoadQueryLimitsFromEnvironment(t *testing.T) {
	t.Setenv("QUERYLENS_QUERY_MAX_SQL_LENGTH", "2048")
	t.Setenv("QUERYLENS_QUERY_TIMEOUT", "750ms")
	t.Setenv("QUERYLENS_QUERY_MAX_ROWS", "25")
	t.Setenv("QUERYLENS_QUERY_SAMPLE_LIMIT", "2")

	cfg := Load()

	if cfg.QueryMaxSQLLength != 2048 {
		t.Errorf("query max SQL length = %d, want 2048", cfg.QueryMaxSQLLength)
	}
	if cfg.QueryTimeout != 750*time.Millisecond {
		t.Errorf("query timeout = %v, want 750ms", cfg.QueryTimeout)
	}
	if cfg.QueryMaxRows != 25 {
		t.Errorf("query max rows = %d, want 25", cfg.QueryMaxRows)
	}
	if cfg.QuerySampleLimit != 2 {
		t.Errorf("query sample limit = %d, want 2", cfg.QuerySampleLimit)
	}
}

func TestLoadFallsBackForInvalidQueryLimits(t *testing.T) {
	t.Setenv("QUERYLENS_QUERY_MAX_SQL_LENGTH", "not-a-number")
	t.Setenv("QUERYLENS_QUERY_TIMEOUT", "not-a-duration")
	t.Setenv("QUERYLENS_QUERY_MAX_ROWS", "")
	t.Setenv("QUERYLENS_QUERY_SAMPLE_LIMIT", "-1")

	cfg := Load()

	if cfg.QueryMaxSQLLength != DefaultQueryMaxSQLLength {
		t.Errorf("query max SQL length = %d, want the default", cfg.QueryMaxSQLLength)
	}
	if cfg.QueryTimeout != DefaultQueryTimeout {
		t.Errorf("query timeout = %v, want the default", cfg.QueryTimeout)
	}
	if cfg.QueryMaxRows != DefaultQueryMaxRows {
		t.Errorf("query max rows = %d, want the default", cfg.QueryMaxRows)
	}
	if cfg.QuerySampleLimit != -1 {
		t.Errorf("query sample limit = %d, want -1 (normalized by the query runner)", cfg.QuerySampleLimit)
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
