package geo

import (
	"net/netip"
	"testing"
)

func TestZeroValueAnswersUnknown(t *testing.T) {
	var g *DB
	if g.Country(netip.MustParseAddr("8.8.8.8")) != "" || g.Ready() {
		t.Fatal("a nil DB must answer unknown")
	}
	if (&DB{}).Country(netip.MustParseAddr("8.8.8.8")) != "" {
		t.Fatal("an empty DB must answer unknown")
	}
}
