// Command gen writes tracker/t.min.js from tracker/t.js. Run it with `go generate ./tracker`
// (or `make tracker`) after editing t.js, and commit both files.
package main

import (
	"fmt"
	"os"

	"github.com/Gitax18/tally/tracker/minify"
)

func main() {
	src, err := os.ReadFile("t.js")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen: run from the tracker directory (go generate ./tracker):", err)
		os.Exit(1)
	}
	out, err := minify.Minify(src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile("t.min.js", out, 0o644); err != nil { //nolint:gosec // a public script, not a secret
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	fmt.Printf("gen: t.min.js is %d bytes\n", len(out))
}
