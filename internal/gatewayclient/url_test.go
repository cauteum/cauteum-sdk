package gatewayclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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

func TestGetSandboxEscapesName(t *testing.T) {
	c := New("http://gateway.example")
	c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.EscapedPath(); got != "/v1/sandboxes/box%3Fother" {
			t.Fatalf("path=%q", got)
		}
		if req.URL.RawQuery != "" {
			t.Fatalf("name injected query: %s", req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"name":"box?other"}`))}, nil
	})
	if got, err := c.GetSandbox(context.Background(), "box?other"); err != nil || got.Name != "box?other" {
		t.Fatalf("GetSandbox()=(%+v, %v)", got, err)
	}
}

func TestSettingsAndProviderRefreshEscapePathSegments(t *testing.T) {
	c := New("http://gateway.example")
	want := []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/v1/settings/feature%3Fflag"},
		{http.MethodGet, "/v1/settings/feature%3Fflag"},
		{http.MethodPut, "/v1/providers/team%3Fblue/refresh/access%3Ftoken"},
		{http.MethodPost, "/v1/providers/team%3Fblue/refresh/access%3Ftoken/rotate"},
		{http.MethodDelete, "/v1/providers/team%3Fblue/refresh/access%3Ftoken"},
	}
	index := 0
	c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Helper()
		if index >= len(want) {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL)
		}
		if req.Method != want[index].method || req.URL.EscapedPath() != want[index].path {
			t.Fatalf("request[%d]=%s %s, want %s %s", index, req.Method, req.URL.EscapedPath(), want[index].method, want[index].path)
		}
		if req.URL.RawQuery != "" {
			t.Fatalf("path parameter escaped into query: %s", req.URL)
		}
		index++
		body := ""
		status := http.StatusNoContent
		if req.Method == http.MethodGet {
			body = `{"value":"enabled"}`
			status = http.StatusOK
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})

	ctx := context.Background()
	if err := c.PutSetting(ctx, "feature?flag", "enabled"); err != nil {
		t.Fatal(err)
	}
	if got, err := c.GetSetting(ctx, "feature?flag"); err != nil || got != "enabled" {
		t.Fatalf("GetSetting()=(%q, %v)", got, err)
	}
	if err := c.ConfigureProviderRefresh(ctx, "team?blue", "access?token", "env", nil, 0); err != nil {
		t.Fatal(err)
	}
	if err := c.RotateProviderRefresh(ctx, "team?blue", "access?token"); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteProviderRefresh(ctx, "team?blue", "access?token"); err != nil {
		t.Fatal(err)
	}
	if index != len(want) {
		t.Fatalf("requests=%d, want %d", index, len(want))
	}
}
