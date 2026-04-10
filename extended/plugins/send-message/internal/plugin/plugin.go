package plugin

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	stepExecutePath = "/api/v1/step.execute"
	authHeader      = "Authorization"
	bearerPrefix    = "Bearer "

	authTokenPath        = "/var/run/kargo/token"
	serviceAccountDir    = "/var/run/secrets/kubernetes.io/serviceaccount"
	serviceAccountToken  = "token"
	serviceAccountCA     = "ca.crt"
	defaultSystemNS      = "kargo-system-resources"
	defaultSlackAPIBase  = "https://slack.com/api"
	defaultKubernetesURL = "https://kubernetes.default.svc"
)

type Options struct {
	Logger                   *log.Logger
	ExpectedTokenPath        string
	SystemResourcesNamespace string
	SlackAPIBaseURL          string
	KubernetesClient         KubernetesClient
	SlackClient              SlackClient
}

type Server struct {
	logger                   *log.Logger
	expectedTokenPath        string
	systemResourcesNamespace string
	slackAPIBaseURL          string
	kubeClient               KubernetesClient
	slackClient              SlackClient
}

type KubernetesClient interface {
	GetMessageChannel(
		ctx context.Context,
		namespace string,
		name string,
	) (*ChannelResource, error)
	GetClusterMessageChannel(
		ctx context.Context,
		name string,
	) (*ChannelResource, error)
	GetSecret(
		ctx context.Context,
		namespace string,
		name string,
	) (map[string]string, error)
}

type SlackClient interface {
	PostMessage(
		ctx context.Context,
		apiBaseURL string,
		token string,
		payload map[string]any,
	) (*SlackPostMessageResponse, error)
}

type StepExecuteRequest struct {
	Context StepContext `json:"context"`
	Step    Step        `json:"step"`
}

type StepContext struct {
	Project string `json:"project,omitempty"`
}

type Step struct {
	Kind   string            `json:"kind"`
	Config SendMessageConfig `json:"config"`
}

type SendMessageConfig struct {
	Channel      ChannelRef   `json:"channel"`
	Message      string       `json:"message"`
	EncodingType string       `json:"encodingType,omitempty"`
	Slack        SlackOptions `json:"slack,omitempty"`
}

type ChannelRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type SlackOptions struct {
	ChannelID string `json:"channelID,omitempty"`
	ThreadTS  string `json:"threadTS,omitempty"`
}

type StepExecuteResponse struct {
	Status   string         `json:"status"`
	Message  string         `json:"message,omitempty"`
	Output   map[string]any `json:"output,omitempty"`
	Error    string         `json:"error,omitempty"`
	Terminal bool           `json:"terminal,omitempty"`
}

type ChannelResource struct {
	SecretName     string
	SlackChannelID string
	ResourceKind   string
	ResourceName   string
	ResourceNS     string
}

type SlackPostMessageResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	TS    string `json:"ts,omitempty"`
}

func NewServer(opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	expectedTokenPath := opts.ExpectedTokenPath
	if expectedTokenPath == "" {
		expectedTokenPath = authTokenPath
	}

	systemNS := opts.SystemResourcesNamespace
	if systemNS == "" {
		systemNS = strings.TrimSpace(os.Getenv("SYSTEM_RESOURCES_NAMESPACE"))
	}
	if systemNS == "" {
		systemNS = defaultSystemNS
	}

	slackBaseURL := opts.SlackAPIBaseURL
	if slackBaseURL == "" {
		slackBaseURL = strings.TrimSpace(os.Getenv("SLACK_API_BASE_URL"))
	}
	if slackBaseURL == "" {
		slackBaseURL = defaultSlackAPIBase
	}

	kubeClient := opts.KubernetesClient
	if kubeClient == nil {
		kubeClient = MustNewInClusterKubernetesClient()
	}
	slackClient := opts.SlackClient
	if slackClient == nil {
		slackClient = RealSlackClient{}
	}

	return &Server{
		logger:                   logger,
		expectedTokenPath:        expectedTokenPath,
		systemResourcesNamespace: systemNS,
		slackAPIBaseURL:          slackBaseURL,
		kubeClient:               kubeClient,
		slackClient:              slackClient,
	}
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.handle)
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != stepExecutePath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := s.authorize(r.Header.Get(authHeader)); err != nil {
		s.writeJSON(w, http.StatusForbidden, StepExecuteResponse{
			Status:   "Errored",
			Message:  err.Error(),
			Error:    err.Error(),
			Terminal: true,
		})
		return
	}

	var req StepExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, StepExecuteResponse{
			Status:   "Errored",
			Message:  "invalid request body",
			Error:    err.Error(),
			Terminal: true,
		})
		return
	}

	resp := s.execute(r.Context(), req)
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) execute(
	ctx context.Context,
	req StepExecuteRequest,
) StepExecuteResponse {
	if req.Step.Kind != "send-message" {
		return StepExecuteResponse{
			Status:   "Errored",
			Message:  fmt.Sprintf("unsupported step kind %q", req.Step.Kind),
			Error:    fmt.Sprintf("unsupported step kind %q", req.Step.Kind),
			Terminal: true,
		}
	}

	channel, secretNS, err := s.lookupChannel(ctx, req)
	if err != nil {
		return failedResponse(err)
	}

	secret, err := s.kubeClient.GetSecret(ctx, secretNS, channel.SecretName)
	if err != nil {
		return failedResponse(fmt.Errorf("error getting Slack Secret: %w", err))
	}
	token := secret["apiKey"]
	if token == "" {
		return failedResponse(errors.New("Slack Secret is missing key \"apiKey\""))
	}

	payload, outputThreadTS, err := buildSlackPayload(req.Step.Config, channel)
	if err != nil {
		return failedResponse(err)
	}

	slackResp, err := s.slackClient.PostMessage(
		ctx,
		s.slackAPIBaseURL,
		token,
		payload,
	)
	if err != nil {
		return failedResponse(fmt.Errorf("error posting Slack message: %w", err))
	}
	if !slackResp.OK {
		return failedResponse(fmt.Errorf("Slack API error: %s", slackResp.Error))
	}
	if outputThreadTS == "" {
		outputThreadTS = slackResp.TS
	}

	return StepExecuteResponse{
		Status: "Succeeded",
		Output: map[string]any{
			"slack": map[string]any{
				"threadTS": outputThreadTS,
			},
		},
	}
}

func (s *Server) lookupChannel(
	ctx context.Context,
	req StepExecuteRequest,
) (*ChannelResource, string, error) {
	ref := req.Step.Config.Channel
	switch ref.Kind {
	case "MessageChannel":
		if req.Context.Project == "" {
			return nil, "", errors.New("step context project is required for MessageChannel")
		}
		channel, err := s.kubeClient.GetMessageChannel(ctx, req.Context.Project, ref.Name)
		if err != nil {
			return nil, "", err
		}
		return channel, req.Context.Project, nil
	case "ClusterMessageChannel":
		channel, err := s.kubeClient.GetClusterMessageChannel(ctx, ref.Name)
		if err != nil {
			return nil, "", err
		}
		return channel, s.systemResourcesNamespace, nil
	default:
		return nil, "", fmt.Errorf("unsupported channel kind %q", ref.Kind)
	}
}

func buildSlackPayload(
	cfg SendMessageConfig,
	channel *ChannelResource,
) (map[string]any, string, error) {
	var payload map[string]any
	switch strings.TrimSpace(cfg.EncodingType) {
	case "":
		channelID := strings.TrimSpace(cfg.Slack.ChannelID)
		if channelID == "" {
			channelID = strings.TrimSpace(channel.SlackChannelID)
		}
		if channelID == "" {
			return nil, "", fmt.Errorf(
				"%s %q does not define spec.slack.channelID and config.slack.channelID is empty",
				channel.ResourceKind,
				channel.ResourceName,
			)
		}
		payload = map[string]any{
			"channel": channelID,
			"text":    cfg.Message,
		}
		threadTS := strings.TrimSpace(cfg.Slack.ThreadTS)
		if threadTS != "" {
			payload["thread_ts"] = threadTS
		}
		return payload, threadTS, nil
	case "json":
		if err := json.Unmarshal([]byte(cfg.Message), &payload); err != nil {
			return nil, "", fmt.Errorf("error decoding JSON Slack payload: %w", err)
		}
	case "yaml":
		if err := yaml.Unmarshal([]byte(cfg.Message), &payload); err != nil {
			return nil, "", fmt.Errorf("error decoding YAML Slack payload: %w", err)
		}
	case "xml":
		var err error
		payload, err = decodeXMLSlackPayload(cfg.Message)
		if err != nil {
			return nil, "", err
		}
	default:
		return nil, "", fmt.Errorf("unsupported encodingType %q", cfg.EncodingType)
	}

	if payload == nil {
		return nil, "", errors.New("Slack payload must decode to an object")
	}

	channelID := strings.TrimSpace(channel.SlackChannelID)
	if channelID == "" {
		return nil, "", fmt.Errorf(
			"%s %q does not define spec.slack.channelID",
			channel.ResourceKind,
			channel.ResourceName,
		)
	}
	if _, ok := payload["channel"]; !ok {
		payload["channel"] = channelID
	}
	threadTS := strings.TrimSpace(stringValue(payload["thread_ts"]))
	return payload, threadTS, nil
}

type xmlPayloadNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr       `xml:",any,attr"`
	Nodes   []xmlPayloadNode `xml:",any"`
	Text    string           `xml:",chardata"`
}

func decodeXMLSlackPayload(message string) (map[string]any, error) {
	var root xmlPayloadNode
	if err := xml.Unmarshal([]byte(message), &root); err != nil {
		return nil, fmt.Errorf("error decoding XML Slack payload: %w", err)
	}

	payload := xmlNodeToObject(root)
	if payload == nil {
		return nil, errors.New("Slack payload must decode to an object")
	}
	return payload, nil
}

func xmlNodeToObject(node xmlPayloadNode) map[string]any {
	payload := map[string]any{}
	for _, attr := range node.Attrs {
		payload[attr.Name.Local] = attr.Value
	}
	for _, child := range node.Nodes {
		appendXMLValue(payload, child.XMLName.Local, xmlNodeValue(child))
	}

	text := strings.TrimSpace(node.Text)
	if text != "" {
		if len(payload) == 0 {
			payload["text"] = text
		} else {
			payload["#text"] = text
		}
	}
	return payload
}

func xmlNodeValue(node xmlPayloadNode) any {
	if len(node.Attrs) == 0 && len(node.Nodes) == 0 {
		return strings.TrimSpace(node.Text)
	}
	return xmlNodeToObject(node)
}

func appendXMLValue(payload map[string]any, key string, value any) {
	if existing, ok := payload[key]; ok {
		switch typed := existing.(type) {
		case []any:
			payload[key] = append(typed, value)
		default:
			payload[key] = []any{typed, value}
		}
		return
	}
	payload[key] = value
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func failedResponse(err error) StepExecuteResponse {
	return StepExecuteResponse{
		Status:   "Failed",
		Message:  err.Error(),
		Error:    err.Error(),
		Terminal: true,
	}
}

func (s *Server) authorize(authHeaderValue string) error {
	expected, err := os.ReadFile(s.expectedTokenPath)
	if err != nil {
		return fmt.Errorf("error reading auth token: %w", err)
	}
	headerValue := strings.TrimSpace(authHeaderValue)
	if !strings.HasPrefix(headerValue, bearerPrefix) {
		return errors.New("missing bearer token")
	}
	received := strings.TrimSpace(strings.TrimPrefix(headerValue, bearerPrefix))
	if received != strings.TrimSpace(string(expected)) {
		return errors.New("invalid bearer token")
	}
	return nil
}

func (s *Server) writeJSON(w http.ResponseWriter, statusCode int, response StepExecuteResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Printf("error writing response: %v", err)
	}
}

type RealSlackClient struct{}

func (RealSlackClient) PostMessage(
	ctx context.Context,
	apiBaseURL string,
	token string,
	payload map[string]any,
) (*SlackPostMessageResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(apiBaseURL, "/")+"/chat.postMessage",
		strings.NewReader(string(body)),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", bearerPrefix+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SlackPostMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		if result.Error == "" {
			result.Error = resp.Status
		}
		result.OK = false
	}
	return &result, nil
}

type inClusterKubernetesClient struct {
	baseURL string
	client  *http.Client
	token   string
}

func MustNewInClusterKubernetesClient() KubernetesClient {
	client, err := NewInClusterKubernetesClient()
	if err != nil {
		panic(err)
	}
	return client
}

func NewInClusterKubernetesClient() (KubernetesClient, error) {
	tokenBytes, err := os.ReadFile(filepath.Join(serviceAccountDir, serviceAccountToken))
	if err != nil {
		return nil, fmt.Errorf("error reading service account token: %w", err)
	}
	caBytes, err := os.ReadFile(filepath.Join(serviceAccountDir, serviceAccountCA))
	if err != nil {
		return nil, fmt.Errorf("error reading service account ca: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, errors.New("error loading service account ca")
	}

	baseURL := defaultKubernetesURL
	if host := strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_HOST")); host != "" {
		port := strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS"))
		if port == "" {
			port = strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_PORT"))
		}
		if port == "" {
			port = "443"
		}
		baseURL = "https://" + host + ":" + port
	}

	return &inClusterKubernetesClient{
		baseURL: baseURL,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
					RootCAs:    pool,
				},
			},
		},
		token: strings.TrimSpace(string(tokenBytes)),
	}, nil
}

func (c *inClusterKubernetesClient) GetMessageChannel(
	ctx context.Context,
	namespace string,
	name string,
) (*ChannelResource, error) {
	var response struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec struct {
			SecretRef struct {
				Name string `json:"name"`
			} `json:"secretRef"`
			Slack struct {
				ChannelID string `json:"channelID"`
			} `json:"slack"`
		} `json:"spec"`
	}
	err := c.getJSON(
		ctx,
		fmt.Sprintf(
			"/apis/ee.kargo.akuity.io/v1alpha1/namespaces/%s/messagechannels/%s",
			namespace,
			name,
		),
		&response,
	)
	if err != nil {
		return nil, err
	}
	return &ChannelResource{
		SecretName:     response.Spec.SecretRef.Name,
		SlackChannelID: response.Spec.Slack.ChannelID,
		ResourceKind:   "MessageChannel",
		ResourceName:   response.Metadata.Name,
		ResourceNS:     response.Metadata.Namespace,
	}, nil
}

func (c *inClusterKubernetesClient) GetClusterMessageChannel(
	ctx context.Context,
	name string,
) (*ChannelResource, error) {
	var response struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			SecretRef struct {
				Name string `json:"name"`
			} `json:"secretRef"`
			Slack struct {
				ChannelID string `json:"channelID"`
			} `json:"slack"`
		} `json:"spec"`
	}
	err := c.getJSON(
		ctx,
		fmt.Sprintf(
			"/apis/ee.kargo.akuity.io/v1alpha1/clustermessagechannels/%s",
			name,
		),
		&response,
	)
	if err != nil {
		return nil, err
	}
	return &ChannelResource{
		SecretName:     response.Spec.SecretRef.Name,
		SlackChannelID: response.Spec.Slack.ChannelID,
		ResourceKind:   "ClusterMessageChannel",
		ResourceName:   response.Metadata.Name,
	}, nil
}

func (c *inClusterKubernetesClient) GetSecret(
	ctx context.Context,
	namespace string,
	name string,
) (map[string]string, error) {
	var response struct {
		Data map[string]string `json:"data"`
	}
	err := c.getJSON(
		ctx,
		fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", namespace, name),
		&response,
	)
	if err != nil {
		return nil, err
	}

	decoded := make(map[string]string, len(response.Data))
	for key, value := range response.Data {
		raw, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("error decoding secret key %q: %w", key, err)
		}
		decoded[key] = string(raw)
	}
	return decoded, nil
}

func (c *inClusterKubernetesClient) getJSON(
	ctx context.Context,
	path string,
	out any,
) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		strings.TrimRight(c.baseURL, "/")+path,
		nil,
	)
	if err != nil {
		return err
	}
	req.Header.Set(authHeader, bearerPrefix+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("resource not found at %s", path)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Kubernetes API error %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
