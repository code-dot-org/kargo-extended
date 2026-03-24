package plugin

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"

	stepplugincommon "github.com/akuity/kargo/extended/pkg/stepplugin/common"
)

func TestClientCallSendsBearerTokenAndDecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(
			t,
			stepplugincommon.APIPathPrefix+stepplugincommon.MethodStepExecute,
			r.URL.Path,
		)
		require.Equal(t, "Bearer secret-token", r.Header.Get("Authorization"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "world", body["hello"])

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"status":  "Succeeded",
			"message": "ok",
		}))
	}))
	defer server.Close()

	client, err := New(
		server.URL,
		"secret-token",
		time.Second,
		wait.Backoff{Steps: 1},
	)
	require.NoError(t, err)

	var reply struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	err = client.Call(
		t.Context(),
		stepplugincommon.MethodStepExecute,
		map[string]string{"hello": "world"},
		&reply,
	)
	require.NoError(t, err)
	require.Equal(t, "Succeeded", reply.Status)
	require.Equal(t, "ok", reply.Message)
}

func TestClientCall404CachesUnsupportedMethod(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := New(server.URL, "", time.Second, wait.Backoff{Steps: 1})
	require.NoError(t, err)

	var reply map[string]any
	err = client.Call(t.Context(), stepplugincommon.MethodStepExecute, map[string]string{}, &reply)
	require.NoError(t, err)

	err = client.Call(t.Context(), stepplugincommon.MethodStepExecute, map[string]string{}, &reply)
	require.NoError(t, err)
	require.EqualValues(t, 1, calls.Load())
}

func TestClientCall503RetriesThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := calls.Add(1)
		if call < 3 {
			http.Error(w, "retry later", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"status": "Succeeded",
		}))
	}))
	defer server.Close()

	client, err := New(
		server.URL,
		"",
		time.Second,
		wait.Backoff{Duration: time.Millisecond, Steps: 3},
	)
	require.NoError(t, err)

	var reply struct {
		Status string `json:"status"`
	}
	err = client.Call(t.Context(), stepplugincommon.MethodStepExecute, map[string]string{}, &reply)
	require.NoError(t, err)
	require.Equal(t, "Succeeded", reply.Status)
	require.EqualValues(t, 3, calls.Load())
}

func TestClientCall403Fails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "denied", http.StatusForbidden)
	}))
	defer server.Close()

	client, err := New(server.URL, "", time.Second, wait.Backoff{Steps: 1})
	require.NoError(t, err)

	var reply map[string]any
	err = client.Call(t.Context(), stepplugincommon.MethodStepExecute, map[string]string{}, &reply)
	require.Error(t, err)
	require.ErrorContains(t, err, "403 Forbidden")
	require.ErrorContains(t, err, "denied")
}

func TestClientCallTimeoutFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"Succeeded"}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "", 5*time.Millisecond, wait.Backoff{Steps: 1})
	require.NoError(t, err)

	var reply map[string]any
	err = client.Call(t.Context(), stepplugincommon.MethodStepExecute, map[string]string{}, &reply)
	require.Error(t, err)
}

func TestClientCallConnectionRefusedFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	client, err := New(
		"http://"+address,
		"",
		20*time.Millisecond,
		wait.Backoff{Duration: time.Millisecond, Steps: 1},
	)
	require.NoError(t, err)

	var reply map[string]any
	err = client.Call(context.Background(), stepplugincommon.MethodStepExecute, map[string]string{}, &reply)
	require.Error(t, err)
	require.ErrorContains(t, err, "connection refused")
}
