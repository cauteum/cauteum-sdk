package gatewayclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestGetImportedProfile(t *testing.T) {
	c := New("http://gateway.invalid")
	c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/profiles/codex" || req.URL.Query().Get("scope") != "global" {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Etag": []string{`"abc123"`}},
			Body:       io.NopCloser(strings.NewReader(`{"profile":{"id":"codex","discovery":{"credentials":["api_key"]}},"source":"custom"}`)),
			Request:    req,
		}, nil
	})
	b, source, version, err := c.GetProfile(context.Background(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	if source != "custom" || version != "abc123" || !strings.Contains(string(b), `"credentials":["api_key"]`) {
		t.Fatalf("profile=%s source=%s version=%s", b, source, version)
	}
}

func TestWorkspaceProfileScopeQuery(t *testing.T) {
	c := New("http://gateway.invalid")
	c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("scope") != "workspace" || req.URL.Query().Get("workspace") != "team ml" {
			t.Fatalf("workspace scope missing from request: %s", req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"profiles":[]}`)), Request: req}, nil
	})
	if _, err := c.ListProfilesScoped(context.Background(), "workspace", "team ml"); err != nil {
		t.Fatal(err)
	}
}
