// Package cauteum is the Go SDK for the cauteum gateway control plane.
//
// RPC methods authenticate with a bearer token (OIDC access token or the
// local-dev token from `<gateway data dir>/auth_token`). New picks up
// CAUTEUM_GATEWAY_TOKEN; NewWithToken sets it explicitly. Resource facades
// use OpenShell or Cauteum RPC; health and auth bootstrap remain HTTP while
// the remaining legacy resource calls are migrated.
//
// Exec runs over the pinned OpenShell ExecSandbox gRPC method.
// Interactive sessions and IDE access use SSH sessions: CreateSSHSession plus
// `cauteum ssh-proxy` / `cauteum sandbox connect`.
package cauteum

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	openshell "github.com/NVIDIA/OpenShell/sdk/go/openshell/v1"
	gc "github.com/cauteum/cauteum-sdk/internal/gatewayclient"
	"google.golang.org/grpc"
)

// EnvToken is the default bearer for New.
const EnvToken = "CAUTEUM_GATEWAY_TOKEN"

// ErrConnectUnsupported means interactive sessions stay on the CLI.
var ErrConnectUnsupported = errors.New("cauteum-sdk: interactive connect is not supported; use: cauteum sandbox connect <name>")

// ErrSandboxNotReady is returned when the sandbox supervisor relay is not connected.
var ErrSandboxNotReady = gc.ErrSandboxNotReady

// Client wraps generated gateway RPC clients and remaining legacy HTTP methods.
type Client struct {
	*gc.Client
	// Workspace selects the default workspace for resource-oriented methods.
	Workspace string
	rpcMu     sync.Mutex
	rpcConn   *grpc.ClientConn
	upstream  *openshell.Client
}

func (c *Client) workspace() string {
	if c.Workspace != "" {
		return c.Workspace
	}
	return defaultWorkspace
}

// New builds a client for a gateway base URL (e.g. http://127.0.0.1:7443),
// authenticated with $CAUTEUM_GATEWAY_TOKEN when set.
func New(baseURL string) *Client {
	return NewWithToken(baseURL, os.Getenv(EnvToken))
}

// NewWithToken returns a client with bearer auth.
func NewWithToken(base, token string) *Client {
	c := gc.NewWithToken(base, strings.TrimSpace(token))
	c.HTTP.Timeout = gc.RelayTimeout
	return &Client{Client: c}
}

// Close releases the native gRPC connection opened for RPC-backed methods.
func (c *Client) Close() error {
	c.rpcMu.Lock()
	defer c.rpcMu.Unlock()
	if c.upstream != nil {
		err := c.upstream.Close()
		c.upstream = nil
		if err != nil {
			return err
		}
	}
	if c.rpcConn == nil {
		return nil
	}
	err := c.rpcConn.Close()
	c.rpcConn = nil
	return err
}

// Stable type aliases (gateway HTTP payloads).
type (
	Labels                = gc.Labels
	Credentials           = gc.Credentials
	ProviderConfig        = gc.ProviderConfig
	RefreshMaterial       = gc.RefreshMaterial
	CredentialBindings    = gc.CredentialBindings
	CredentialExpiry      = gc.CredentialExpiry
	Sandbox               = gc.Sandbox
	ExecResult            = gc.ExecResult
	LogLine               = gc.LogLine
	Proposal              = gc.Proposal
	ProviderRecord        = gc.ProviderRecord
	ProviderRefreshConfig = gc.ProviderRefreshConfig
	PolicyRevisionMeta    = gc.PolicyRevisionMeta
	InferenceRoute        = gc.InferenceRoute
	ServiceRecord         = gc.ServiceRecord
	SSHSession            = gc.SSHSession
	SSHSessionInfo        = gc.SSHSessionInfo
)

// Create creates a runtime sandbox through the authorized Control RPC.
func (c *Client) Create(ctx context.Context, sb Sandbox) error {
	_, err := c.CreateControlSandbox(ctx, sb, nil)
	return err
}

// List returns caller-visible sandboxes through the allowlisted control API.
func (c *Client) List(ctx context.Context) ([]Sandbox, error) {
	return c.ListControlSandboxes(ctx, "", true)
}

// Get returns one redacted sandbox summary by name in the default workspace.
func (c *Client) Get(ctx context.Context, name string) (Sandbox, error) {
	return c.GetControlSandbox(ctx, "default", name)
}

// Delete removes a runtime sandbox using its current resource version.
func (c *Client) Delete(ctx context.Context, name string) error {
	return c.DeleteControlSandbox(ctx, "default", name)
}

// Start starts a runtime sandbox in the default workspace.
func (c *Client) Start(ctx context.Context, name string) (Sandbox, error) {
	return c.StartControlSandbox(ctx, "default", name)
}

// Stop stops a runtime sandbox in the default workspace.
func (c *Client) Stop(ctx context.Context, name string) (Sandbox, error) {
	return c.StopControlSandbox(ctx, "default", name)
}

// Exec runs argv in the sandbox using the pinned OpenShell RPC client.
func (c *Client) Exec(ctx context.Context, name string, argv ...string) (ExecResult, error) {
	if name == "" || len(argv) == 0 {
		return ExecResult{}, fmt.Errorf("usage: Exec(name, argv...)")
	}
	client, err := c.openShellClient()
	if err != nil {
		return ExecResult{}, err
	}
	result, err := client.Exec().Run(ctx, "default", name, argv)
	if err != nil {
		return ExecResult{}, fmt.Errorf("execute in sandbox %q: %w", name, err)
	}
	return ExecResult{ExitCode: result.ExitCode, Output: string(result.Stdout) + string(result.Stderr)}, nil
}

// Connect is intentionally unsupported in the SDK.
func (c *Client) Connect(_ context.Context, _ string) error {
	return ErrConnectUnsupported
}
