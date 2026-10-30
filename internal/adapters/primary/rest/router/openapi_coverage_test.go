package router_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

// docsAPIDir resolves docs/api relative to this source file:
// <repo>/core/internal/adapters/primary/rest/router -> <repo>/docs/api.
func docsAPIDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "docs", "api")
}

type response struct {
	Ref     string         `yaml:"$ref"`
	Content map[string]any `yaml:"content"`
}

type operation struct {
	Method, Path string
	OperationID  string                 `yaml:"operationId"`
	Security     *[]map[string][]string `yaml:"security"`
	RequestBody  *struct {
		Content map[string]any `yaml:"content"`
	} `yaml:"requestBody"`
	Responses map[string]response `yaml:"responses"`
}

type spec struct {
	Security   []map[string][]string           `yaml:"security"`
	Paths      map[string]map[string]yaml.Node `yaml:"paths"`
	Components struct {
		Responses map[string]response `yaml:"responses"`
	} `yaml:"components"`
	ops []operation
}

var httpMethods = map[string]bool{"get": true, "put": true, "post": true, "patch": true, "delete": true, "head": true, "options": true}

func loadSpec(t *testing.T) *spec {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(docsAPIDir(), "openapi.yaml"))
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}
	var s spec
	if err := yaml.Unmarshal(raw, &s); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}
	for path, item := range s.Paths {
		for method, node := range item {
			if !httpMethods[method] {
				continue // path-level parameters, summary, ...
			}
			var op operation
			if err := node.Decode(&op); err != nil {
				t.Fatalf("%s %s: %v", method, path, err)
			}
			op.Method, op.Path = strings.ToUpper(method), path
			s.ops = append(s.ops, op)
		}
	}
	sort.Slice(s.ops, func(i, j int) bool {
		if s.ops[i].Path != s.ops[j].Path {
			return s.ops[i].Path < s.ops[j].Path
		}
		return s.ops[i].Method < s.ops[j].Method
	})
	if len(s.ops) == 0 {
		t.Fatal("openapi.yaml has no operations")
	}
	return &s
}

// secured reports whether the operation requires a bearer token: its own security list, or the
// document default when it has none. `security: []` marks a public operation.
func (s *spec) secured(op operation) bool {
	sec := s.Security
	if op.Security != nil {
		sec = *op.Security
	}
	for _, req := range sec {
		if len(req) == 0 {
			return false // an empty requirement object makes authentication optional
		}
	}
	return len(sec) > 0
}

// returnsBody reports whether a 2xx response of the operation carries content.
func (s *spec) returnsBody(op operation) bool {
	for code, r := range op.Responses {
		if !strings.HasPrefix(code, "2") {
			continue
		}
		if r.Ref != "" {
			r = s.Components.Responses[strings.TrimPrefix(r.Ref, "#/components/responses/")]
		}
		if len(r.Content) > 0 {
			return true
		}
	}
	return false
}

var pathParam = regexp.MustCompile(`\{[^}]+\}`)

func (op operation) url() string { return pathParam.ReplaceAllString(op.Path, "1") }

func (op operation) String() string {
	return fmt.Sprintf("%s %s (%s)", op.Method, op.Path, op.OperationID)
}

// call sends the operation with an empty JSON object as body when it declares one.
func call(env *testapp.Env, op operation, opts ...testapp.Opt) *testapp.Resp {
	var body any
	if op.RequestBody != nil {
		body = map[string]any{}
	}
	return env.Do(op.Method, op.url(), body, opts...)
}

// unrouted reports whether the mux, not a handler, answered: no pattern matched the path (404 with
// the mux's plain-text body; every handler 404 is a JSON error) or none matched the method (405).
func unrouted(r *testapp.Resp) bool {
	if r.Status == http.StatusMethodNotAllowed {
		return true
	}
	if r.Status != http.StatusNotFound {
		return false
	}
	var e struct{ Code string }
	return json.Unmarshal(r.Body, &e) != nil || e.Code == ""
}

// TestEveryOpenAPIOperationHasAHandler calls every operation (an operator's token for secured ones,
// which passes every role check) and fails with the list of operations the router does not serve.
// It also guards conditional registration in router.New: a nil handler group drops its routes.
func TestEveryOpenAPIOperationHasAHandler(t *testing.T) {
	s := loadSpec(t)
	env := testapp.New(t)
	op := env.SignInOperator(testapp.OperatorPhone)

	var missing, crashed []string
	for _, o := range s.ops {
		var opts []testapp.Opt
		if s.secured(o) {
			opts = append(opts, testapp.Bearer(op.AccessToken))
		}
		r := call(env, o, opts...)
		switch {
		case unrouted(r):
			missing = append(missing, fmt.Sprintf("%s -> %d", o, r.Status))
		case r.Status >= 500:
			crashed = append(crashed, fmt.Sprintf("%s -> %d %s", o, r.Status, r.Body))
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d of %d operations have no handler:\n  %s", len(missing), len(s.ops), strings.Join(missing, "\n  "))
	}
	if len(crashed) > 0 {
		t.Errorf("operations answering 5xx to a minimal request:\n  %s", strings.Join(crashed, "\n  "))
	}
}

// TestEverySecuredOperationRejectsAnonymous401 calls every secured operation without a token.
func TestEverySecuredOperationRejectsAnonymous401(t *testing.T) {
	s := loadSpec(t)
	env := testapp.New(t)

	var bad []string
	n := 0
	for _, o := range s.ops {
		if !s.secured(o) {
			continue
		}
		n++
		r := call(env, o)
		var e struct{ Code string }
		if r.Status != http.StatusUnauthorized || json.Unmarshal(r.Body, &e) != nil || e.Code != "UNAUTHORIZED" {
			bad = append(bad, fmt.Sprintf("%s -> %d %s", o, r.Status, strings.TrimSpace(string(r.Body))))
		}
	}
	if n == 0 {
		t.Fatal("no secured operations found; is the document-level security parsed?")
	}
	if len(bad) > 0 {
		t.Errorf("secured operations not answering 401 UNAUTHORIZED to an anonymous call:\n  %s", strings.Join(bad, "\n  "))
	}
}

// TestEveryBodyOperationHasFixture requires docs/api/fixtures/<operationId>.json for every operation
// whose success response has a body, so contract.AssertShape can check it.
func TestEveryBodyOperationHasFixture(t *testing.T) {
	s := loadSpec(t)
	var missing []string
	for _, o := range s.ops {
		if o.OperationID == "" {
			missing = append(missing, o.String()+": no operationId")
			continue
		}
		if !s.returnsBody(o) {
			continue
		}
		if _, err := os.Stat(filepath.Join(docsAPIDir(), "fixtures", o.OperationID+".json")); err != nil {
			missing = append(missing, o.String())
		}
	}
	if len(missing) > 0 {
		t.Errorf("operations returning a body without a fixture:\n  %s", strings.Join(missing, "\n  "))
	}
}
