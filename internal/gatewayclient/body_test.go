package gatewayclient

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

type bodyFailure struct{ err error }

func (b bodyFailure) Read([]byte) (int, error) { return 0, b.err }
func (bodyFailure) Close() error               { return nil }

type responseTransport struct{ body bodyFailure }

func (t responseTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: t.body, Header: make(http.Header)}, nil
}
func TestGetGlobalPolicyReportsBodyFailure(t *testing.T) {
	expected := errors.New("truncated response")
	c := New("http://gateway.example")
	c.HTTP.Transport = responseTransport{bodyFailure{expected}}
	_, err := c.GetGlobalPolicy(context.Background())
	if !errors.Is(err, expected) {
		t.Fatalf("error=%v; want body failure", err)
	}
}
