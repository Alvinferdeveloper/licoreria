package server

import (
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	api "licoreria/api"
)

var (
	specPath   = regexp.MustCompile(`^  (/[^:]*):\s*$`)
	specMethod = regexp.MustCompile(`^    (get|post|put|patch|delete):\s*$`)
)

func specOperations() []string {
	var ops []string
	path := ""
	for _, line := range strings.Split(strings.ReplaceAll(string(api.OpenAPISpec), "\r", ""), "\n") {
		if m := specPath.FindStringSubmatch(line); m != nil {
			path = m[1]
		} else if m := specMethod.FindStringSubmatch(line); m != nil {
			ops = append(ops, strings.ToUpper(m[1])+" "+path)
		}
	}
	return ops
}

func TestOpenAPIReferencesResolve(t *testing.T) {
	section := regexp.MustCompile(`^  ([a-zA-Z]+):`)
	entry := regexp.MustCompile(`^    ([A-Za-z]+):`)
	ref := regexp.MustCompile(`\$ref: '#/components/([a-zA-Z]+)/([A-Za-z]+)'`)

	text := strings.ReplaceAll(string(api.OpenAPISpec), "\r", "")
	_, components, found := strings.Cut(text, "\ncomponents:\n")
	if !found {
		t.Fatal("spec has no components")
	}
	defined := map[string]bool{}
	current := ""
	for _, line := range strings.Split(components, "\n") {
		if m := section.FindStringSubmatch(line); m != nil {
			current = m[1]
		} else if m := entry.FindStringSubmatch(line); m != nil {
			defined[current+"/"+m[1]] = true
		}
	}
	for _, m := range ref.FindAllStringSubmatch(text, -1) {
		if !defined[m[1]+"/"+m[2]] {
			t.Errorf("unresolved reference: %s/%s", m[1], m[2])
		}
	}
}

// The spec is written by hand, so this keeps it from drifting away from the router.
func TestOpenAPIMatchesRouter(t *testing.T) {
	env := newTestEnv(t)
	router, ok := env.server.Config.Handler.(chi.Routes)
	if !ok {
		t.Fatal("router does not expose its routes")
	}
	var routes []string
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		path := strings.TrimPrefix(strings.TrimSuffix(route, "/"), "/api/v1")
		if path == "/docs" || path == "/openapi.yaml" {
			return nil
		}
		routes = append(routes, method+" "+path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/openapi.yaml", "/docs"} {
		if res := env.newClient().do(http.MethodGet, path, nil); res.status != http.StatusOK {
			t.Errorf("GET %s: %d", path, res.status)
		}
	}
	spec := specOperations()
	for _, r := range routes {
		if !slices.Contains(spec, r) {
			t.Errorf("route missing from openapi.yaml: %s", r)
		}
	}
	for _, s := range spec {
		if !slices.Contains(routes, s) {
			t.Errorf("openapi.yaml documents a route that does not exist: %s", s)
		}
	}
}
