package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	stepplugincommon "github.com/akuity/kargo/extended/pkg/stepplugin/common"
)

// Adapted from Argo Workflows:
// /Users/seth/src/argo-workflows/workflow/util/plugin/plugin.go
// at commit 03ebaaca08c692015338fc88e2fbdff75840cc34.

type Client struct {
	address string
	token   string
	client  http.Client
	backoff wait.Backoff

	mu      sync.Mutex
	invalid map[string]bool
}

func New(
	address string,
	token string,
	timeout time.Duration,
	backoff wait.Backoff,
) (*Client, error) {
	validatedAddress, err := validateAddress(address)
	if err != nil {
		return nil, err
	}

	return &Client{
		address: validatedAddress,
		token:   token,
		client: http.Client{
			Timeout: timeout,
		},
		backoff: backoff,
		invalid: map[string]bool{},
	}, nil
}

func validateAddress(address string) (string, error) {
	parsed, err := url.Parse(address)
	if err != nil {
		return "", fmt.Errorf("invalid plugin address %q: %w", address, err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("plugin address %q must use http or https", address)
	}

	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("plugin address %q has empty host", address)
	}
	if host != "localhost" && net.ParseIP(host) == nil {
		return "", fmt.Errorf(
			"plugin address %q must use localhost or an IP address",
			address,
		)
	}

	return strings.TrimRight(parsed.String(), "/"), nil
}

func (c *Client) Call(ctx context.Context, method string, args any, reply any) error {
	if c.isInvalid(method) {
		return nil
	}

	body, err := json.Marshal(args)
	if err != nil {
		return err
	}

	return retry.OnError(
		c.backoff,
		func(err error) bool {
			var tempErr interface{ Temporary() bool }
			if errors.As(err, &tempErr) && tempErr.Temporary() {
				return true
			}
			return strings.Contains(err.Error(), "connection refused")
		},
		func() error {
			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodPost,
				c.address+stepplugincommon.APIPathPrefix+method,
				bytes.NewBuffer(body),
			)
			if err != nil {
				return err
			}
			req.Header.Add("Content-Type", "application/json")
			if c.token != "" {
				req.Header.Add("Authorization", "Bearer "+c.token)
			}

			//nolint:gosec // target addresses are validated by New() and restricted to localhost or explicit IPs
			resp, err := c.client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusOK:
				return json.NewDecoder(resp.Body).Decode(reply)
			case http.StatusNotFound:
				c.markInvalid(method)
				_, err := io.Copy(io.Discard, resp.Body)
				return err
			case http.StatusServiceUnavailable:
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}
				return transientError{err: fmt.Errorf("%s", string(data))}
			default:
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}
				return fmt.Errorf("%s: %s", resp.Status, string(data))
			}
		},
	)
}

func (c *Client) isInvalid(method string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.invalid[method]
}

func (c *Client) markInvalid(method string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.invalid[method] = true
}

type transientError struct {
	err error
}

func (e transientError) Error() string {
	return e.err.Error()
}

func (e transientError) Temporary() bool {
	return true
}
