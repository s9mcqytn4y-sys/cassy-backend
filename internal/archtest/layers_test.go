// Package archtest guards the backend layering (S5-Q6): handler -> service -> repository -> domain,
// with internal/pkg as leaf utilities. It reads the import graph from `go list`, so no extra tooling.
// Test files are not checked: integration tests may reach any layer to set up data.
package archtest

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
)

type pkg struct {
	ImportPath string
	Imports    []string
}

// rules: a package under the prefix must not import anything under the listed prefixes.
var rules = []struct {
	pkg       string
	forbidden []string
}{
	{"internal/domain", []string{"internal/"}},
	{"internal/repository", []string{"internal/service", "internal/handler", "internal/realtime", "github.com/gin-gonic/gin"}},
	{"internal/service", []string{"internal/handler", "github.com/gin-gonic/gin"}},
	{"internal/handler", []string{"internal/repository"}},
	{"internal/pkg", []string{"internal/service", "internal/handler", "internal/repository"}},
}

func TestLayering(t *testing.T) {
	out, err := exec.Command("go", "list", "-json", "../...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	module := strings.TrimSpace(mustOutput(t, "go", "list", "-m")) + "/"
	dec := json.NewDecoder(bytes.NewReader(out))
	checked := 0
	for {
		var p pkg
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("decode go list: %v", err)
		}
		rel := strings.TrimPrefix(p.ImportPath, module)
		for _, r := range rules {
			if rel != r.pkg && !strings.HasPrefix(rel, r.pkg+"/") {
				continue
			}
			checked++
			for _, imp := range p.Imports {
				dep := strings.TrimPrefix(imp, module)
				for _, f := range r.forbidden {
					if dep == f || strings.HasPrefix(dep, strings.TrimSuffix(f, "/")+"/") {
						t.Errorf("%s imports %s (forbidden for %s)", rel, dep, r.pkg)
					}
				}
			}
		}
	}
	if checked < 5 {
		t.Fatalf("only %d packages matched the rules; the module layout changed", checked)
	}
}

func mustOutput(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return string(out)
}
