package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func valid() map[string]string {
	return map[string]string{
		"DATABASE_URL":        "postgres://u:secretpw@localhost:5432/tally",
		"API_SECRET":          strings.Repeat("a", 40),
		"ADMIN_PASSWORD_HASH": "scrypt$32768$8$1$c2FsdA$aGFzaA",
	}
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(valid()))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8090" || c.RetentionMonths != 24 || c.DBLimitBytes != 1024<<20 || c.TrustProxy {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestTrustProxyDefaultsOnHeroku(t *testing.T) {
	m := valid()
	m["DYNO"] = "web.1"
	c, err := Load(env(m))
	if err != nil {
		t.Fatal(err)
	}
	if !c.TrustProxy {
		t.Fatal("TrustProxy should default to true when DYNO is set")
	}
}

func TestLoadReportsEveryProblemWithoutValues(t *testing.T) {
	m := map[string]string{
		"DATABASE_URL":     "mysql://u:topsecret@h/db",
		"API_SECRET":       "short",
		"RETENTION_MONTHS": "0",
		"PORT":             "x",
	}
	_, err := Load(env(m))
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	for _, want := range []string{"DATABASE_URL", "API_SECRET", "ADMIN_PASSWORD_HASH", "RETENTION_MONTHS", "PORT"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not name %s: %s", want, msg)
		}
	}
	if strings.Contains(msg, "topsecret") {
		t.Errorf("error leaks a value: %s", msg)
	}
}
