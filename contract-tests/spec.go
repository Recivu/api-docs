// Package contracttests verifies that the API deployed on staging answers as
// openapi.yaml promises (TECH-894).
//
// The spec and the code live in different repos and nothing else ties them:
// the contract can drift without any test failing, and it has. This suite is
// the check that was missing. It is not a functional test of the platform: it
// exercises each documented operation once, with a sandbox (test) API key, and
// validates status code, headers and body against the schema declared for
// that operation. A failure means one of the two sides is wrong and a human
// has to decide which.
package contracttests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

const (
	envBaseURL = "RECIVU_API_BASE_URL"
	envAPIKey  = "RECIVU_API_KEY"

	defaultBaseURL = "https://staging-api.recivu.it"
	specPath       = "../openapi.yaml"
)

type spec struct {
	doc    *openapi3.T
	router routers.Router
	// base is the server URL the router matches against, e.g.
	// https://staging-api.recivu.it/v1 — the spec only lists production.
	base string
}

var (
	loadOnce sync.Once
	loaded   *spec
	loadErr  error
)

// loadSpec parses and validates openapi.yaml once per test binary and points
// its single server at the environment under test.
func loadSpec(t *testing.T) *spec {
	t.Helper()
	loadOnce.Do(func() {
		// application/pdf is what GET /invoices_download/{id} returns; the
		// default decoders only know JSON-ish and form types.
		openapi3filter.RegisterBodyDecoder("application/pdf", openapi3filter.FileBodyDecoder)

		loader := openapi3.NewLoader()
		abs, err := filepath.Abs(specPath)
		if err != nil {
			loadErr = err
			return
		}
		doc, err := loader.LoadFromFile(abs)
		if err != nil {
			loadErr = fmt.Errorf("load %s: %w", specPath, err)
			return
		}
		if err := doc.Validate(context.Background()); err != nil {
			loadErr = fmt.Errorf("openapi.yaml is not a valid OpenAPI document: %w", err)
			return
		}

		base := strings.TrimRight(baseURL(), "/") + "/v1"
		doc.Servers = openapi3.Servers{{URL: base}}
		router, err := gorillamux.NewRouter(doc)
		if err != nil {
			loadErr = fmt.Errorf("build router: %w", err)
			return
		}
		loaded = &spec{doc: doc, router: router, base: base}
	})
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	return loaded
}

func baseURL() string {
	if v := os.Getenv(envBaseURL); v != "" {
		return v
	}
	return defaultBaseURL
}

// apiKey returns the sandbox key, or skips the test when it is not set so a
// bare `go test ./...` stays green (and honest: it says "skipped", not "ok").
func apiKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv(envAPIKey)
	if key == "" {
		t.Skipf("%s not set: the contract test needs a *test* API key for %s", envAPIKey, baseURL())
	}
	return key
}
