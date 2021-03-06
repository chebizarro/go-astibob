package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/asticode/go-astilog"
	"github.com/pkg/errors"
)

// Flags
var (
	ability   = flag.String("a", "", "the path to the directory containing the ability")
	abilities = flag.String("as", "", "the path to the directory containing abilities")
)

type Data struct {
	Package   string
	Static    map[string][]byte
	Templates map[string][]byte
}

const s = `// Generated by cmd/operatable
// DO NOT EDIT
package {{ .Package }}

import (
	{{ if gt (len .Static) 0 }}"net/http"

	{{ end }}"github.com/asticode/go-astibob"
)

func newBaseOperatable() (o *astibob.BaseOperatable) {
	// Create operatable
	o = astibob.NewBaseOperatable()
{{ if gt (len .Static) 0 }}
	// Add static
	{{ range $k, $v := .Static }}o.AddRoute("{{ $k }}", http.MethodGet, astibob.ContentHandle("{{ $k }}", []byte{ {{ range $_, $b := $v }}{{ printf "%#x," $b }}{{ end }} }))
	{{ end }}
{{ end }}{{ if gt (len .Templates) 0 }}	// Add templates
	{{ range $k, $v := .Templates }}o.AddTemplate("{{ $k }}", []byte{ {{ range $_, $b := $v }}{{ printf "%#x," $b }}{{ end }} })
	{{ end }}{{ end }}return
}
`

func main() {
	// Parse flags
	flag.Parse()
	astilog.FlagInit()

	// Read dir
	var fs []os.FileInfo
	var baseDir string
	var err error
	if *abilities != "" {
		baseDir = *abilities
		if fs, err = ioutil.ReadDir(*abilities); err != nil {
			astilog.Fatal(errors.Wrapf(err, "main: reading dir %s failed", *abilities))
		}
	} else if *ability != "" {
		baseDir = filepath.Dir(*ability)
		f, err := os.Stat(*ability)
		if err != nil {
			astilog.Fatal(errors.Wrapf(err, "main: stating %s failed", *ability))
		}
		fs = append(fs, f)
	} else {
		astilog.Fatal("main: no input path provided")
	}

	// Parse template
	r := template.New("root")
	t, err := r.Parse(s)
	if err != nil {
		astilog.Fatal(errors.Wrap(err, "main: parsing template failed"))
	}

	// Loop through files
	for _, f := range fs {
		// Not a dir
		if !f.IsDir() {
			continue
		}

		// Create data
		d := Data{
			Package:   f.Name(),
			Static:    make(map[string][]byte),
			Templates: make(map[string][]byte),
		}

		// Stat resources folder
		rp := filepath.Join(baseDir, f.Name(), "resources")
		if _, err = os.Stat(rp); err != nil && !os.IsNotExist(err) {
			astilog.Fatal(errors.Wrapf(err, "main: stating %s failed", rp))
		} else if os.IsNotExist(err) {
			continue
		}

		// Process statics
		if err = processStatics(filepath.Join(baseDir, f.Name()), &d); err != nil {
			astilog.Fatal(errors.Wrap(err, "main: processing statics failed"))
		}

		// Process templates
		if err = processTemplates(filepath.Join(baseDir, f.Name()), &d); err != nil {
			astilog.Fatal(errors.Wrap(err, "main: processing templates failed"))
		}

		// Create destination
		dp := filepath.Join(baseDir, f.Name(), "operatable.go")
		f, err := os.Create(dp)
		if err != nil {
			astilog.Fatal(errors.Wrapf(err, "main: creating %s failed", dp))
		}
		defer f.Close()

		// Execute template
		if err = t.Execute(f, d); err != nil {
			astilog.Fatal(errors.Wrapf(err, "main: executing template for %s failed", rp))
		}
	}
}

func processStatics(basePath string, d *Data) (err error) {
	// Stat static folder
	sp := filepath.Join(basePath, "resources", "static")
	if _, err = os.Stat(sp); err != nil && !os.IsNotExist(err) {
		err = errors.Wrapf(err, "main: stating %s failed", sp)
		return
	}

	// Loop through static
	if err == nil {
		if err = filepath.Walk(sp, func(path string, info os.FileInfo, e error) (err error) {
			// Check input error
			if e != nil {
				err = errors.Wrapf(e, "main: walking templates has an input error for path %s", path)
				return
			}

			// Only process files
			if info.IsDir() {
				return
			}

			// Read file
			var b []byte
			if b, err = ioutil.ReadFile(path); err != nil {
				err = errors.Wrapf(err, "main: reading %s failed", path)
				return
			}

			// Add to data
			d.Static["/static"+filepath.ToSlash(strings.TrimPrefix(path, sp))] = b
			return
		}); err != nil {
			err = errors.Wrapf(err, "main: looping through static in %s failed", sp)
			return
		}
	}
	return
}

func processTemplates(basePath string, d *Data) (err error) {
	// Stat templates folder
	tp := filepath.Join(basePath, "resources", "templates")
	if _, err = os.Stat(tp); err != nil && !os.IsNotExist(err) {
		err = errors.Wrapf(err, "main: stating %s failed", tp)
		return
	}

	// Loop through templates
	if err == nil {
		if err = filepath.Walk(tp, func(path string, info os.FileInfo, e error) (err error) {
			// Check input error
			if e != nil {
				err = errors.Wrapf(e, "main: walking templates has an input error for path %s", path)
				return
			}

			// Only process files
			if info.IsDir() {
				return
			}

			// Check extension
			if filepath.Ext(path) != ".html" {
				return
			}

			// Read file
			var b []byte
			if b, err = ioutil.ReadFile(path); err != nil {
				err = errors.Wrapf(err, "main: reading %s failed", path)
				return
			}

			// Add to data
			d.Templates[filepath.ToSlash(strings.TrimSuffix(strings.TrimPrefix(path, tp), ".html"))] = b
			return
		}); err != nil {
			err = errors.Wrapf(err, "main: looping through templates in %s failed", tp)
			return
		}
	}
	return
}
