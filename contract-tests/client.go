package contracttests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
)

// client sends one request per documented operation and validates the reply
// against the spec before handing it to the test.
type client struct {
	spec *spec
	key  string
	http *http.Client
}

func newClient(t *testing.T) *client {
	t.Helper()
	return &client{
		spec: loadSpec(t),
		key:  apiKey(t),
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// response is what a call returns once it has been validated.
type response struct {
	Status int
	Header http.Header
	Body   []byte
}

// JSON decodes the body into v; the body has already been validated against
// the schema, so this only fails on a test bug.
func (r response) JSON(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("decode body: %v\n%s", err, r.Body)
	}
}

type callOpt func(*callOpts)

type callOpts struct {
	noAuth bool
	// rawBody skips request validation: used for the negative cases where
	// the request is deliberately not what the spec asks for.
	rawBody []byte
	header  http.Header
}

func withoutAuth() callOpt           { return func(o *callOpts) { o.noAuth = true } }
func withRawBody(b []byte) callOpt   { return func(o *callOpts) { o.rawBody = b } }
func withHeader(k, v string) callOpt { return func(o *callOpts) { o.header.Set(k, v) } }

// call sends method+path (path relative to /v1, query included) with body
// JSON-encoded, and asserts:
//  1. the operation exists in the spec (route found);
//  2. the request itself conforms to the spec, so a red test is never the
//     test's own fault (skipped for rawBody);
//  3. the status code is documented for the operation;
//  4. headers and body match the documented response.
//
// It also logs top-level JSON keys the spec does not document: drift in the
// other direction, reported but not failed.
func (c *client) call(t *testing.T, method, path string, body any, opts ...callOpt) response {
	t.Helper()
	o := callOpts{header: http.Header{}}
	for _, opt := range opts {
		opt(&o)
	}

	var reqBody []byte
	switch {
	case o.rawBody != nil:
		reqBody = o.rawBody
	case body != nil:
		var err error
		if reqBody, err = json.Marshal(body); err != nil {
			t.Fatalf("marshal request: %v", err)
		}
	}

	// Two requests from the same bytes: one for kin-openapi, which consumes
	// and may rewrite the body while applying schema defaults, and a pristine
	// one for the wire. Sending the validated one got a Cloudflare 400 from a
	// Content-Length that no longer matched.
	build := func() *http.Request {
		req, err := http.NewRequestWithContext(context.Background(), method, c.spec.base+path, bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		if reqBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if !o.noAuth {
			req.Header.Set("x-api-key", c.key)
		}
		for k, vs := range o.header {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
		return req
	}
	req := build()

	route, pathParams, err := c.spec.router.FindRoute(req)
	if err != nil {
		t.Fatalf("%s %s is not in openapi.yaml: %v", method, path, err)
	}
	rvi := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			AuthenticationFunc:  openapi3filter.NoopAuthenticationFunc,
			ExcludeRequestBody:  o.rawBody != nil,
			SkipSettingDefaults: true,
		},
	}
	if !o.noAuth {
		if err := openapi3filter.ValidateRequest(context.Background(), rvi); err != nil {
			t.Fatalf("the test's own request for %s %s violates the spec (fix the test or the spec):\n%v", method, path, err)
		}
	}

	resp, err := c.http.Do(build())
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s %s: read body: %v", method, path, err)
	}

	err = openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: rvi,
		Status:                 resp.StatusCode,
		Header:                 resp.Header,
		Body:                   io.NopCloser(bytes.NewReader(respBody)),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	})
	if err != nil {
		t.Fatalf("%s %s → %d does not match openapi.yaml (operationId %s):\n%v\nbody: %s",
			method, path, resp.StatusCode, route.Operation.OperationID, err, truncate(respBody, 800))
	}

	logUndocumentedKeys(t, method, path, route.Operation, resp, respBody)
	return response{Status: resp.StatusCode, Header: resp.Header, Body: respBody}
}

// logUndocumentedKeys reports response keys the schema does not list. kin-openapi
// accepts extra properties unless additionalProperties is false, which is
// right for partners (additive is not breaking) but hides the drift where
// the code returns more than the spec says.
func logUndocumentedKeys(t *testing.T, method, path string, op *openapi3.Operation, resp *http.Response, body []byte) {
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		return
	}
	ref := op.Responses.Status(resp.StatusCode)
	if ref == nil || ref.Value == nil {
		return
	}
	mt := ref.Value.Content.Get("application/json")
	if mt == nil || mt.Schema == nil || mt.Schema.Value == nil {
		return
	}
	documented := map[string]bool{}
	collectProperties(mt.Schema.Value, documented, 0)
	if len(documented) == 0 {
		return
	}
	var got map[string]json.RawMessage
	if json.Unmarshal(body, &got) != nil {
		return
	}
	var extra []string
	for k := range got {
		if !documented[k] {
			extra = append(extra, k)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		t.Logf("NOTE %s %s → %d returns keys not in the spec: %s", method, path, resp.StatusCode, strings.Join(extra, ", "))
	}
}

func collectProperties(s *openapi3.Schema, into map[string]bool, depth int) {
	if s == nil || depth > 4 {
		return
	}
	for k := range s.Properties {
		into[k] = true
	}
	for _, group := range [][]*openapi3.SchemaRef{s.AllOf, s.OneOf, s.AnyOf} {
		for _, ref := range group {
			if ref != nil {
				collectProperties(ref.Value, into, depth+1)
			}
		}
	}
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + fmt.Sprintf("… (%d bytes)", len(b))
}
