// Package dialector unit tests — design §17.1.1 coverage matrix.
//
// Three map-case test functions, each pinned to one design §17.1.1 bucket:
//
//   - TestDialector_DefaultPort               default port branching
//   - TestDialector_BuildDSN_BaselineFields   baseline DSN field layout +
//                                             sslmode default per variant
//   - TestDialector_BuildDSN_AdditionalParamsAndEscape
//                                             user override priority + libpq
//                                             single-quote / backslash escape
//
// Hard constraints (todo.md Task-Dev-003 §C / Task-level §约束):
//   - All cases use a Go map keyed by human-readable scenario name (no
//     for-range slice + index assertion).
//   - No sql.Open, no time.Sleep, no network — pure string assertions.
//   - sslmode presence / absence is asserted strictly; other field order is
//     not, to match libpq's order-independent parser.
package dialector

import (
	"strings"
	"testing"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"

	"github.com/actiontech/sqle-gaussdb-plugin/pkg/consts"
)

// -----------------------------------------------------------------------------
// TestDialector_DefaultPort
// -----------------------------------------------------------------------------

func TestDialector_DefaultPort(t *testing.T) {
	type tc struct {
		variant  string
		wantPort int
	}
	cases := map[string]tc{
		"gaussdb variant returns commercial default 8000": {
			variant:  consts.PluginNameGaussDB,
			wantPort: consts.DefaultPortGaussDB, // 8000
		},
		"opengauss variant returns opengauss-6.0 default 5432": {
			variant:  consts.PluginNameOpenGauss,
			wantPort: consts.DefaultPortOpenGauss, // 5432
		},
		"empty variant returns 0 (sentinel for unknown variant)": {
			variant:  "",
			wantPort: 0,
		},
		"unknown variant returns 0 (defensive default)": {
			variant:  "Postgres",
			wantPort: 0,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			d := &Dialector{Variant: c.variant}
			got := d.DefaultPort()
			if got != c.wantPort {
				t.Fatalf("variant=%q DefaultPort: got %d, want %d", c.variant, got, c.wantPort)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// TestDialector_BuildDSN_BaselineFields
// -----------------------------------------------------------------------------

func TestDialector_BuildDSN_BaselineFields(t *testing.T) {
	type tc struct {
		variant     string
		dsn         *driverV2.DSN
		mustContain []string // every substring must appear in the rendered DSN
		mustReject  []string // every substring must NOT appear
	}
	cases := map[string]tc{
		"gaussdb: 5 baseline fields + connect_timeout + sslmode=disable": {
			variant: consts.PluginNameGaussDB,
			dsn: &driverV2.DSN{
				Host:         "10.0.0.1",
				Port:         "8000",
				User:         "root",
				Password:     "secret",
				DatabaseName: "gaussdb",
			},
			mustContain: []string{
				"host='10.0.0.1'",
				"port='8000'",
				"user='root'",
				"password='secret'",
				"dbname='gaussdb'",
				"connect_timeout=5",
				"sslmode=disable",
			},
		},
		"opengauss: 5 baseline fields + connect_timeout, sslmode NOT declared": {
			variant: consts.PluginNameOpenGauss,
			dsn: &driverV2.DSN{
				Host:         "127.0.0.1",
				Port:         "5432",
				User:         "gaussdb",
				Password:     "openGauss@123",
				DatabaseName: "postgres",
			},
			mustContain: []string{
				"host='127.0.0.1'",
				"port='5432'",
				"user='gaussdb'",
				"password='openGauss@123'",
				"dbname='postgres'",
				"connect_timeout=5",
			},
			mustReject: []string{
				"sslmode", // openGauss leaves sslmode untouched per design §4.3
			},
		},
		"gaussdb: empty DatabaseName falls back to libpq 'postgres'": {
			variant: consts.PluginNameGaussDB,
			dsn: &driverV2.DSN{
				Host:         "db.internal",
				Port:         "8000",
				User:         "gaussdba",
				Password:     "pwd",
				DatabaseName: "", // <- triggers fallback
			},
			mustContain: []string{
				"dbname='postgres'",
				"sslmode=disable",
			},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			d := &Dialector{Variant: c.variant}
			got := d.BuildDSN(c.dsn)

			for _, want := range c.mustContain {
				if !strings.Contains(got, want) {
					t.Errorf("variant=%q DSN must contain %q\n  full DSN: %s", c.variant, want, got)
				}
			}
			for _, reject := range c.mustReject {
				if strings.Contains(got, reject) {
					t.Errorf("variant=%q DSN must NOT contain %q\n  full DSN: %s", c.variant, reject, got)
				}
			}
		})
	}
}

// -----------------------------------------------------------------------------
// TestDialector_BuildDSN_AdditionalParamsAndEscape
// -----------------------------------------------------------------------------

func TestDialector_BuildDSN_AdditionalParamsAndEscape(t *testing.T) {
	type tc struct {
		variant     string
		dsn         *driverV2.DSN
		mustContain []string
		// dupeKeyLast asserts that within the rendered DSN string the *last*
		// occurrence of a libpq key is the one expected — i.e. user-supplied
		// AdditionalParams override the default value. The map key is the
		// libpq key, the value is the expected key=value substring that must
		// be the last hit for that key.
		dupeKeyLast map[string]string
	}
	cases := map[string]tc{
		"gaussdb: user AdditionalParam sslmode=verify-full overrides default disable": {
			variant: consts.PluginNameGaussDB,
			dsn: &driverV2.DSN{
				Host:         "10.0.0.2",
				Port:         "8000",
				User:         "root",
				Password:     "p",
				DatabaseName: "gaussdb",
				AdditionalParams: params.Params{
					&params.Param{Key: "sslmode", Value: "verify-full"},
				},
			},
			mustContain: []string{
				"sslmode=disable",        // default emitted first
				"sslmode='verify-full'",  // user override emitted after
			},
			dupeKeyLast: map[string]string{
				"sslmode": "sslmode='verify-full'",
			},
		},
		"opengauss: user AdditionalParam sslmode=require lands in DSN (no default to override)": {
			variant: consts.PluginNameOpenGauss,
			dsn: &driverV2.DSN{
				Host:         "127.0.0.1",
				Port:         "5432",
				User:         "gaussdb",
				Password:     "p",
				DatabaseName: "postgres",
				AdditionalParams: params.Params{
					&params.Param{Key: "sslmode", Value: "require"},
					&params.Param{Key: "application_name", Value: "sqle-plugin"},
				},
			},
			mustContain: []string{
				"sslmode='require'",
				"application_name='sqle-plugin'",
			},
		},
		"password with single quote must be escaped via backslash": {
			variant: consts.PluginNameGaussDB,
			dsn: &driverV2.DSN{
				Host:         "h",
				Port:         "8000",
				User:         "u",
				Password:     "it's",
				DatabaseName: "d",
			},
			mustContain: []string{
				// libpq escape: ' → \', wrapped in single quotes →
				// password='it\'s'
				`password='it\'s'`,
			},
		},
		"password with backslash must be escaped to double backslash": {
			variant: consts.PluginNameOpenGauss,
			dsn: &driverV2.DSN{
				Host:         "h",
				Port:         "5432",
				User:         "u",
				Password:     `back\slash`,
				DatabaseName: "d",
			},
			mustContain: []string{
				`password='back\\slash'`,
			},
		},
		"empty AdditionalParam key is ignored (defensive)": {
			variant: consts.PluginNameGaussDB,
			dsn: &driverV2.DSN{
				Host:         "h",
				Port:         "8000",
				User:         "u",
				Password:     "p",
				DatabaseName: "d",
				AdditionalParams: params.Params{
					&params.Param{Key: "", Value: "noise"},
					&params.Param{Key: "application_name", Value: "ok"},
				},
			},
			mustContain: []string{
				"application_name='ok'",
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			d := &Dialector{Variant: c.variant}
			got := d.BuildDSN(c.dsn)

			for _, want := range c.mustContain {
				if !strings.Contains(got, want) {
					t.Errorf("variant=%q DSN must contain %q\n  full DSN: %s", c.variant, want, got)
				}
			}
			// Ensure user-supplied key occurs after the default so libpq's
			// last-wins parsing resolves to the user value. We check this
			// purely by index in the rendered string.
			for key, lastSub := range c.dupeKeyLast {
				lastIdx := strings.LastIndex(got, lastSub)
				if lastIdx < 0 {
					t.Fatalf("variant=%q expected last %s key form %q missing\n  full DSN: %s", c.variant, key, lastSub, got)
				}
				// Anything matching key= after lastIdx + len(lastSub) would
				// mean the override is not last; ensure there is no
				// subsequent "<key>=" occurrence.
				tail := got[lastIdx+len(lastSub):]
				if strings.Contains(tail, key+"=") || strings.Contains(tail, " "+key+"=") {
					t.Errorf("variant=%q key %q is not the last occurrence in DSN\n  full DSN: %s", c.variant, key, got)
				}
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Compile-time sanity: the three required helper methods exist and have the
// expected signatures. This is cheaper than a typed assertion in production
// code and surfaces accidental signature drift during refactors.
// -----------------------------------------------------------------------------

var (
	_ func() string                  = (&Dialector{}).DriverName
	_ func() int                     = (&Dialector{}).DefaultPort
	_ func(dsn *driverV2.DSN) string = (&Dialector{}).BuildDSN
)

// TestDialector_DriverName locks the database/sql driver name registered by
// the openGauss official Go driver. The blank import at the top of
// dialector.go is the only place that calls sql.Register("opengauss", ...),
// so any drift here is an immediate red flag.
func TestDialector_DriverName(t *testing.T) {
	d := &Dialector{Variant: consts.PluginNameGaussDB}
	if got := d.DriverName(); got != "opengauss" {
		t.Fatalf(`DriverName: got %q, want "opengauss"`, got)
	}
	d2 := &Dialector{Variant: consts.PluginNameOpenGauss}
	if got := d2.DriverName(); got != "opengauss" {
		t.Fatalf(`DriverName (openGauss variant): got %q, want "opengauss"`, got)
	}
}
