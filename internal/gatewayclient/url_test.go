package gatewayclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestLogsSnapshotEscapesNamesAndFilters(t *testing.T) {
	c := New("http://gateway.example")
	c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.EscapedPath(); got != "/v1/sandboxes/box%2Fchild%3Fx/logs" {
			t.Fatalf("path=%q", got)
		}
		if got := req.URL.Query().Get("source"); got != "proxy&all=1" {
			t.Fatalf("source=%q", got)
		}
		if req.URL.Query().Has("all") {
			t.Fatal("filter injected an extra query parameter")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"lines":[]}`))}, nil
	})
	if _, err := c.GetLogsSnapshot(context.Background(), "box/child?x", "", "proxy&all=1", ""); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteSandboxEscapesName(t *testing.T) {
	c := New("http://gateway.example")
	c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.EscapedPath(); got != "/v1/sandboxes/box%2Fother%3Fall=1" {
			t.Fatalf("path=%q", got)
		}
		if req.URL.RawQuery != "" {
			t.Fatalf("name injected query: %s", req.URL)
		}
		return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if err := c.DeleteSandbox(context.Background(), "box/other?all=1"); err != nil {
		t.Fatal(err)
	}
}
