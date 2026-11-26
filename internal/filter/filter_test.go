package filter

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	fs, err := Parse("country:IN,source:GitHub,page:%2Fblog%2Ca%3Ab")
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 3 || fs[0].Dim.Column != "country" || fs[0].Value != "IN" || fs[1].Value != "GitHub" {
		t.Fatalf("parsed %+v", fs)
	}
	if fs[2].Dim.Column != "path" || fs[2].Value != "/blog,a:b" {
		t.Fatalf("encoded value: %+v", fs[2])
	}
	if got := fs.String(); got != "country:IN,source:GitHub,page:%2Fblog%2Ca:b" {
		t.Fatalf("round trip: %s", got)
	}
	if fs, err := Parse(""); err != nil || fs != nil {
		t.Fatal("empty means no filters")
	}
	if fs, err := Parse("source:"); err != nil || fs[0].Value != "" {
		t.Fatal("an empty value is a valid filter (for example, no referrer)")
	}
}

func TestParseRejects(t *testing.T) {
	for _, raw := range []string{
		"nope:1",                              // unknown dimension
		"visitor_id:1",                        // a real column, but not whitelisted
		"country",                             // no colon
		"country:IN,country:US",               // twice
		"source:%zz",                          // bad escape
		"country:IN;DROP TABLE events",        // stays one value... but see below
		"path:/",                              // the column name is not a dimension name
		"1=1--:x",                             // injection in the name
		"country = 'IN' OR 1=1 --:x",          // injection in the name
		strings.Repeat("source:a,", 11) + "x", // too many
	} {
		_, err := Parse(raw)
		if raw == "country:IN;DROP TABLE events" {
			// Allowed: it is a value, and values are bind parameters.
			if err != nil {
				t.Errorf("%q: values may contain anything: %v", raw, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("Parse(%q) should fail", raw)
		}
	}
}

func TestSQLUsesOnlyPlaceholdersForValues(t *testing.T) {
	fs, err := Parse("country:x' OR '1'='1,page:%2F,entry_page:%2Fa")
	if err != nil {
		t.Fatal(err)
	}
	a := &Args{}
	s := Scope{Site: a.Add("site"), From: a.Add("from"), To: a.Add("to")}
	ev := fs.Events(a, s)
	se := fs.Sessions(a, s)
	for _, sql := range []string{ev, se} {
		if strings.Contains(sql, "OR '1'") || strings.Contains(sql, "/a") {
			t.Fatalf("a value leaked into SQL: %s", sql)
		}
	}
	if !strings.Contains(ev, "country = $4") || !strings.Contains(ev, "path = $5") || !strings.Contains(ev, "entry_path = $6") {
		t.Fatalf("events SQL: %s", ev)
	}
	if !strings.Contains(se, "id IN (SELECT session_id FROM events") {
		t.Fatalf("sessions SQL should match visits containing the page: %s", se)
	}
	if len(a.Values()) != 9 || a.Values()[3] != "x' OR '1'='1" {
		t.Fatalf("args: %v", a.Values())
	}
}
