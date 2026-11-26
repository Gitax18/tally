// Package minify turns tracker/t.js into the script the server embeds. It is used by the
// generator (go generate ./tracker) and by the test that checks the committed output is current;
// the server binary never imports it, so esbuild is not linked into it.
package minify

import (
	"errors"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// Minify returns the minified ES2017 script.
func Minify(src []byte) ([]byte, error) {
	res := api.Transform(string(src), api.TransformOptions{
		Loader:            api.LoaderJS,
		Target:            api.ES2017,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		LegalComments:     api.LegalCommentsNone,
		Charset:           api.CharsetUTF8,
	})
	if len(res.Errors) > 0 {
		var msgs []string
		for _, m := range res.Errors {
			msgs = append(msgs, m.Text)
		}
		return nil, errors.New("esbuild: " + strings.Join(msgs, "; "))
	}
	return res.Code, nil
}
