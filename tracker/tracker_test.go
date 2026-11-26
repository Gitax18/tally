package tracker

import (
	"bytes"
	"compress/gzip"
	"os"
	"strings"
	"testing"

	"github.com/Gitax18/tally/tracker/minify"
)

// maxGzipBytes is the tracker's size budget, from the plan.
const maxGzipBytes = 1024

func TestMinifiedIsCurrent(t *testing.T) {
	src, err := os.ReadFile("t.js")
	if err != nil {
		t.Fatal(err)
	}
	want, err := minify.Minify(src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, minified) {
		t.Fatal("t.min.js is out of date: run `go generate ./tracker` and commit the result")
	}
}

func TestScriptUnder1KBGzipped(t *testing.T) {
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = zw.Write(Script("abcdefghij", false))
	_ = zw.Close()
	t.Logf("tracker: %d bytes minified, %d bytes gzipped", len(minified), buf.Len())
	if buf.Len() > maxGzipBytes {
		t.Fatalf("tracker is %d bytes gzipped; the budget is %d", buf.Len(), maxGzipBytes)
	}
}

func TestScriptWritesSiteAndLocalFlag(t *testing.T) {
	s := string(Script("k3j9x2abcd", true))
	if strings.Contains(s, "__TALLY_") {
		t.Fatal("a placeholder is left in the script")
	}
	if !strings.Contains(s, `"k3j9x2abcd","1")`) {
		t.Fatalf("site ID and local flag are not the IIFE's arguments: ...%s", s[len(s)-40:])
	}
	if !strings.HasSuffix(strings.TrimSpace(string(Script("k3j9x2abcd", false))), `"k3j9x2abcd","0");`) {
		t.Fatal("local flag 0 not written")
	}
}
