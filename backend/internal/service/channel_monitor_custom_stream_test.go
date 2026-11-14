//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConsumeMonitorSSE_Protocols(t *testing.T) {
	start := time.Now()
	cases := []struct {
		name     string
		protocol monitorStreamProtocol
		raw      string
		want     string
		done     bool
		failed   bool
	}{
		{
			name:     "openai chat",
			protocol: openAIChatStreamProtocol,
			raw:      "data: {\"choices\":[{\"delta\":{\"content\":\"pong\"}}]}\n\ndata: [DONE]\n\n",
			want:     "pong",
			done:     true,
		},
		{
			name:     "openai error after text",
			protocol: openAIChatStreamProtocol,
			raw:      "data: {\"choices\":[{\"delta\":{\"content\":\"pong\"}}]}\n\ndata: {\"error\":{\"message\":\"no\"}}\n\n",
			want:     "pong",
			failed:   true,
		},
		{
			name:     "responses",
			protocol: responsesStreamProtocol,
			raw:      "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"pong\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\"}\n\n",
			want:     "pong",
			done:     true,
		},
		{
			name:     "anthropic",
			protocol: anthropicStreamProtocol,
			raw:      "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"pong\"}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
			want:     "pong",
			done:     true,
		},
		{
			name:     "gemini eof",
			protocol: geminiStreamProtocol,
			raw:      "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"pong\"}]},\"finishReason\":\"STOP\"}]}\n\n",
			want:     "pong",
			done:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, text, completed, failed, _ := consumeMonitorSSE(strings.NewReader(tc.raw), tc.protocol, start)
			require.Equal(t, tc.want, text)
			require.Equal(t, tc.done, completed)
			require.Equal(t, tc.failed, failed)
		})
	}
}

func TestConsumeMonitorSSE_CleanEOFWithTextSucceeds(t *testing.T) {
	raw := "data: {\"choices\":[{\"delta\":{\"content\":\"pong\"}}]}\n\n"
	_, text, completed, failed, reason := consumeMonitorSSE(strings.NewReader(raw), openAIChatStreamProtocol, time.Now())
	require.Equal(t, "pong", text)
	require.True(t, completed)
	require.False(t, failed)
	require.Empty(t, reason)
}

func TestConsumeMonitorSSE_TruncatedFrameIsNotSuccess(t *testing.T) {
	raw := "data: {\"choices\":[{\"delta\":{\"content\":\"pong\"}}]}\n\ndata: [DONE]"
	_, text, completed, failed, reason := consumeMonitorSSE(strings.NewReader(raw), openAIChatStreamProtocol, time.Now())
	require.Equal(t, "pong", text)
	require.False(t, completed)
	require.True(t, failed)
	require.Contains(t, reason, "truncated")
}

func TestConsumeMonitorSSE_SizeLimitIsNotSuccess(t *testing.T) {
	raw := "data: " + strings.Repeat("a", monitorResponseMaxBytes) + "\n\n"
	_, _, completed, failed, reason := consumeMonitorSSE(strings.NewReader(raw), openAIChatStreamProtocol, time.Now())
	require.False(t, completed)
	require.True(t, failed)
	require.Contains(t, reason, "size limit")
}

func TestRunCustomCheck_StreamTTFTAndHeaders(t *testing.T) {
	orig := customMonitorHTTPClient
	customMonitorHTTPClient = &http.Client{}
	t.Cleanup(func() { customMonitorHTTPClient = orig })

	var gotHeader, gotPrompt string
	var streamed bool
	server := newMonitorSSEServer(t, func(r *http.Request, body map[string]any) {
		gotHeader = r.Header.Get("X-Test")
		streamed, _ = body["stream"].(bool)
		msgs, _ := body["messages"].([]any)
		if len(msgs) > 0 {
			msg, _ := msgs[0].(map[string]any)
			gotPrompt, _ = msg["content"].(string)
		}
	}, "data: {\"choices\":[{\"delta\":{\"content\":\"pong\"}}]}\n\ndata: [DONE]\n\n")

	res := runCustomCheckForModel(t.Context(), customProbeInput{
		MonitorID: 3,
		Provider:  MonitorProviderOpenAI,
		Endpoint:  server.URL,
		APIKey:    "k",
		Model:     "m",
		Opts:      &CheckOptions{ExtraHeaders: map[string]string{"X-Test": "yes"}},
		Config:    ChannelMonitorCustomConfig{Prompt: "ping", YellowSeconds: 2, RedSeconds: 8, TPSEnabled: true, TPSMin: 10, TPSMax: 40},
	})
	require.Equal(t, MonitorStatusOperational, res.Status)
	require.Equal(t, MonitorMetricKindTTFT, res.MetricKind)
	require.NotNil(t, res.LatencyMs)
	require.NotNil(t, res.TokensPerSec)
	require.Equal(t, "yes", gotHeader)
	require.True(t, streamed)
	require.Equal(t, "ping", gotPrompt)
}

func TestRunCustomCheck_DeadlineAborts(t *testing.T) {
	orig := customMonitorHTTPClient
	customMonitorHTTPClient = &http.Client{}
	t.Cleanup(func() { customMonitorHTTPClient = orig })

	server := newBlockingMonitorServer(t)
	ctx, cancel := contextWithTimeout(t, 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	res := runCustomCheckForModel(ctx, customProbeInput{
		Provider: MonitorProviderOpenAI,
		Endpoint: server.URL,
		APIKey:   "k",
		Model:    "m",
		Config:   defaultChannelMonitorCustomConfig(),
	})
	require.Less(t, time.Since(start), time.Second)
	require.Equal(t, MonitorStatusError, res.Status)
	require.Contains(t, res.Message, "deadline")
	require.NotNil(t, res.LatencyMs)
}

func TestApplyCustomStream_ReplaceHonorsFalse(t *testing.T) {
	body, streaming, err := applyCustomStream(MonitorProviderOpenAI, MonitorBodyOverrideModeReplace, []byte(`{"stream":false,"messages":[{"role":"user","content":"ping"}]}`))
	require.NoError(t, err)
	require.False(t, streaming)
	require.Contains(t, string(body), `"stream":false`)

	_, streaming, err = applyCustomStream(MonitorProviderOpenAI, MonitorBodyOverrideModeOff, []byte(`{"stream":false,"messages":[]}`))
	require.NoError(t, err)
	require.True(t, streaming)
}

func contextWithTimeout(t *testing.T, d time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), d)
}

func newMonitorSSEServer(t *testing.T, capture func(*http.Request, map[string]any), payload string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		if capture != nil {
			capture(r, body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, payload)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestSSEEventKindPrefersEventLine(t *testing.T) {
	require.Equal(t, "message_stop", sseEventKind("message_stop", `{"type":"content_block_delta"}`))
	require.Equal(t, "error", sseEventKind("", `{"type":"error"}`))
}

func newBlockingMonitorServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 读完请求体后，服务端才会监听客户端断开。否则 context 截止只结束客户端，handler 会一直占着连接。
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	return server
}
