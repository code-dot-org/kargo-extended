package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandlerRejectsMissingBearerToken(t *testing.T) {
	srv := newTestServer(t, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		stepExecutePath,
		bytes.NewReader(mustJSON(t, minimalRequest())),
	)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	var resp StepExecuteResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "Errored", resp.Status)
}

func TestExecuteUsesNamespacedMessageChannel(t *testing.T) {
	kube := &fakeKubernetesClient{
		messageChannels: map[string]*ChannelResource{
			"demo/send": {
				SecretName:     "slack-token",
				SlackChannelID: "C123",
				ResourceKind:   "MessageChannel",
				ResourceName:   "send",
				ResourceNS:     "demo",
			},
		},
		secrets: map[string]map[string]string{
			"demo/slack-token": {"apiKey": "xoxb-demo"},
		},
	}
	slack := &fakeSlackClient{
		response: &SlackPostMessageResponse{
			OK: true,
			TS: "1712345678.000100",
		},
	}
	srv := newTestServer(t, kube, slack)

	req := minimalRequest()
	req.Context.Project = "demo"
	req.Step.Config.Channel = ChannelRef{
		Kind: "MessageChannel",
		Name: "send",
	}
	resp := executeRequest(t, srv, req)

	require.Equal(t, "Succeeded", resp.Status)
	require.Equal(t, "1712345678.000100", resp.Output["slack"].(map[string]any)["threadTS"])
	require.Equal(
		t,
		map[string]any{
			"channel": "C123",
			"text":    "hello from plugin",
		},
		slack.lastPayload,
	)
}

func TestExecuteUsesClusterMessageChannel(t *testing.T) {
	kube := &fakeKubernetesClient{
		clusterMessageChannels: map[string]*ChannelResource{
			"send": {
				SecretName:     "slack-token",
				SlackChannelID: "C777",
				ResourceKind:   "ClusterMessageChannel",
				ResourceName:   "send",
			},
		},
		secrets: map[string]map[string]string{
			"kargo-system-resources/slack-token": {"apiKey": "xoxb-demo"},
		},
	}
	slack := &fakeSlackClient{
		response: &SlackPostMessageResponse{
			OK: true,
			TS: "1712345678.000200",
		},
	}
	srv := newTestServer(t, kube, slack)

	req := minimalRequest()
	req.Context.Project = "demo"
	req.Step.Config.Channel = ChannelRef{
		Kind: "ClusterMessageChannel",
		Name: "send",
	}
	resp := executeRequest(t, srv, req)

	require.Equal(t, "Succeeded", resp.Status)
	require.Equal(
		t,
		map[string]any{
			"channel": "C777",
			"text":    "hello from plugin",
		},
		slack.lastPayload,
	)
}

func TestExecuteHonorsSlackOverrides(t *testing.T) {
	kube := &fakeKubernetesClient{
		messageChannels: map[string]*ChannelResource{
			"demo/send": {
				SecretName:     "slack-token",
				SlackChannelID: "C123",
				ResourceKind:   "MessageChannel",
				ResourceName:   "send",
				ResourceNS:     "demo",
			},
		},
		secrets: map[string]map[string]string{
			"demo/slack-token": {"apiKey": "xoxb-demo"},
		},
	}
	slack := &fakeSlackClient{
		response: &SlackPostMessageResponse{
			OK: true,
			TS: "1712345678.000300",
		},
	}
	srv := newTestServer(t, kube, slack)

	req := minimalRequest()
	req.Context.Project = "demo"
	req.Step.Config.Slack.ChannelID = "C999"
	req.Step.Config.Slack.ThreadTS = "1700000000.000001"
	resp := executeRequest(t, srv, req)

	require.Equal(t, "Succeeded", resp.Status)
	require.Equal(
		t,
		map[string]any{
			"channel":   "C999",
			"text":      "hello from plugin",
			"thread_ts": "1700000000.000001",
		},
		slack.lastPayload,
	)
	require.Equal(
		t,
		"1700000000.000001",
		resp.Output["slack"].(map[string]any)["threadTS"],
	)
}

func TestExecuteSupportsJSONEncoding(t *testing.T) {
	kube := &fakeKubernetesClient{
		messageChannels: map[string]*ChannelResource{
			"demo/send": {
				SecretName:     "slack-token",
				SlackChannelID: "C123",
				ResourceKind:   "MessageChannel",
				ResourceName:   "send",
				ResourceNS:     "demo",
			},
		},
		secrets: map[string]map[string]string{
			"demo/slack-token": {"apiKey": "xoxb-demo"},
		},
	}
	slack := &fakeSlackClient{
		response: &SlackPostMessageResponse{
			OK: true,
			TS: "1712345678.000400",
		},
	}
	srv := newTestServer(t, kube, slack)

	req := minimalRequest()
	req.Context.Project = "demo"
	req.Step.Config.EncodingType = "json"
	req.Step.Config.Message = `{"text":"rich","blocks":[{"type":"section"}]}`
	resp := executeRequest(t, srv, req)

	require.Equal(t, "Succeeded", resp.Status)
	require.Equal(
		t,
		map[string]any{
			"channel": "C123",
			"text":    "rich",
			"blocks": []any{
				map[string]any{"type": "section"},
			},
		},
		slack.lastPayload,
	)
}

func TestExecuteEncodedPayloadIgnoresSlackConfigOverrides(t *testing.T) {
	kube := &fakeKubernetesClient{
		messageChannels: map[string]*ChannelResource{
			"demo/send": {
				SecretName:     "slack-token",
				SlackChannelID: "C123",
				ResourceKind:   "MessageChannel",
				ResourceName:   "send",
				ResourceNS:     "demo",
			},
		},
		secrets: map[string]map[string]string{
			"demo/slack-token": {"apiKey": "xoxb-demo"},
		},
	}
	slack := &fakeSlackClient{
		response: &SlackPostMessageResponse{
			OK: true,
			TS: "1712345678.000450",
		},
	}
	srv := newTestServer(t, kube, slack)

	req := minimalRequest()
	req.Context.Project = "demo"
	req.Step.Config.EncodingType = "json"
	req.Step.Config.Message = `{"text":"rich","thread_ts":"1700000000.000002"}`
	req.Step.Config.Slack.ChannelID = "C999"
	req.Step.Config.Slack.ThreadTS = "1700000000.000001"
	resp := executeRequest(t, srv, req)

	require.Equal(t, "Succeeded", resp.Status)
	require.Equal(
		t,
		map[string]any{
			"channel":   "C123",
			"text":      "rich",
			"thread_ts": "1700000000.000002",
		},
		slack.lastPayload,
	)
	require.Equal(
		t,
		"1700000000.000002",
		resp.Output["slack"].(map[string]any)["threadTS"],
	)
}

func TestExecuteFailsWhenSlackErrors(t *testing.T) {
	kube := &fakeKubernetesClient{
		messageChannels: map[string]*ChannelResource{
			"demo/send": {
				SecretName:     "slack-token",
				SlackChannelID: "C123",
				ResourceKind:   "MessageChannel",
				ResourceName:   "send",
				ResourceNS:     "demo",
			},
		},
		secrets: map[string]map[string]string{
			"demo/slack-token": {"apiKey": "xoxb-demo"},
		},
	}
	slack := &fakeSlackClient{
		response: &SlackPostMessageResponse{
			OK:    false,
			Error: "channel_not_found",
		},
	}
	srv := newTestServer(t, kube, slack)

	req := minimalRequest()
	req.Context.Project = "demo"
	resp := executeRequest(t, srv, req)

	require.Equal(t, "Failed", resp.Status)
	require.Contains(t, resp.Error, "channel_not_found")
}

func TestExecuteFailsWhenSecretMissing(t *testing.T) {
	kube := &fakeKubernetesClient{
		messageChannels: map[string]*ChannelResource{
			"demo/send": {
				SecretName:     "slack-token",
				SlackChannelID: "C123",
				ResourceKind:   "MessageChannel",
				ResourceName:   "send",
				ResourceNS:     "demo",
			},
		},
		secrets: map[string]map[string]string{},
	}
	srv := newTestServer(t, kube, &fakeSlackClient{})

	req := minimalRequest()
	req.Context.Project = "demo"
	resp := executeRequest(t, srv, req)

	require.Equal(t, "Failed", resp.Status)
	require.Contains(t, resp.Error, "error getting Slack Secret")
}

func TestExecuteSupportsXMLEncoding(t *testing.T) {
	kube := &fakeKubernetesClient{
		messageChannels: map[string]*ChannelResource{
			"demo/send": {
				SecretName:     "slack-token",
				SlackChannelID: "C123",
				ResourceKind:   "MessageChannel",
				ResourceName:   "send",
				ResourceNS:     "demo",
			},
		},
		secrets: map[string]map[string]string{
			"demo/slack-token": {"apiKey": "xoxb-demo"},
		},
	}
	slack := &fakeSlackClient{
		response: &SlackPostMessageResponse{
			OK: true,
			TS: "1712345678.000500",
		},
	}
	srv := newTestServer(t, kube, slack)

	req := minimalRequest()
	req.Context.Project = "demo"
	req.Step.Config.EncodingType = "xml"
	req.Step.Config.Message = `
<message>
  <text>rich</text>
  <thread_ts>1700000000.000003</thread_ts>
</message>`
	resp := executeRequest(t, srv, req)

	require.Equal(t, "Succeeded", resp.Status)
	require.Equal(
		t,
		map[string]any{
			"channel":   "C123",
			"text":      "rich",
			"thread_ts": "1700000000.000003",
		},
		slack.lastPayload,
	)
	require.Equal(
		t,
		"1700000000.000003",
		resp.Output["slack"].(map[string]any)["threadTS"],
	)
}

type fakeKubernetesClient struct {
	messageChannels        map[string]*ChannelResource
	clusterMessageChannels map[string]*ChannelResource
	secrets                map[string]map[string]string
}

func (f *fakeKubernetesClient) GetMessageChannel(
	_ context.Context,
	namespace string,
	name string,
) (*ChannelResource, error) {
	if channel, ok := f.messageChannels[namespace+"/"+name]; ok {
		return channel, nil
	}
	return nil, errors.New("message channel not found")
}

func (f *fakeKubernetesClient) GetClusterMessageChannel(
	_ context.Context,
	name string,
) (*ChannelResource, error) {
	if channel, ok := f.clusterMessageChannels[name]; ok {
		return channel, nil
	}
	return nil, errors.New("cluster message channel not found")
}

func (f *fakeKubernetesClient) GetSecret(
	_ context.Context,
	namespace string,
	name string,
) (map[string]string, error) {
	if secret, ok := f.secrets[namespace+"/"+name]; ok {
		return secret, nil
	}
	return nil, errors.New("secret not found")
}

type fakeSlackClient struct {
	lastPayload map[string]any
	response    *SlackPostMessageResponse
	err         error
}

func (f *fakeSlackClient) PostMessage(
	_ context.Context,
	_ string,
	_ string,
	payload map[string]any,
) (*SlackPostMessageResponse, error) {
	f.lastPayload = payload
	if f.err != nil {
		return nil, f.err
	}
	if f.response == nil {
		return &SlackPostMessageResponse{OK: true, TS: "0"}, nil
	}
	return f.response, nil
}

func newTestServer(
	t *testing.T,
	kube KubernetesClient,
	slack SlackClient,
) *Server {
	t.Helper()

	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("expected-token"), 0o600))

	if kube == nil {
		kube = &fakeKubernetesClient{}
	}
	if slack == nil {
		slack = &fakeSlackClient{}
	}

	return NewServer(Options{
		Logger:                   log.New(io.Discard, "", 0),
		ExpectedTokenPath:        tokenPath,
		SystemResourcesNamespace: "kargo-system-resources",
		KubernetesClient:         kube,
		SlackClient:              slack,
	})
}

func executeRequest(
	t *testing.T,
	srv *Server,
	req StepExecuteRequest,
) StepExecuteResponse {
	t.Helper()

	httpReq := httptest.NewRequest(
		http.MethodPost,
		stepExecutePath,
		bytes.NewReader(mustJSON(t, req)),
	)
	httpReq.Header.Set(authHeader, bearerPrefix+"expected-token")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httpReq)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp StepExecuteResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp
}

func minimalRequest() StepExecuteRequest {
	return StepExecuteRequest{
		Step: Step{
			Kind: "send-message",
			Config: SendMessageConfig{
				Channel: ChannelRef{
					Kind: "MessageChannel",
					Name: "send",
				},
				Message: "hello from plugin",
			},
		},
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}
