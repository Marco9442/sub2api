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

func TestConsumeMonitorSSE_AnthropicStartTextCounts(t *testing.T) {
	startOnly := strings.Join([]string{
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"hello"}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")
	_, text, completed, failed, reason := consumeMonitorSSE(strings.NewReader(startOnly), anthropicStreamProtocol, time.Now())
	require.Equal(t, "hello", text)
	require.True(t, completed)
	require.False(t, failed)
	require.Empty(t, reason)

	cleanEOF := strings.Join([]string{
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"hello"}}`,
		"",
	}, "\n")
	_, text, completed, failed, reason = consumeMonitorSSE(strings.NewReader(cleanEOF), anthropicStreamProtocol, time.Now())
	require.Equal(t, "hello", text)
	require.True(t, completed)
	require.False(t, failed)
	require.Empty(t, reason)
}

func TestConsumeMonitorSSE_AnthropicStartTextSetsTTFTBeforeDelta(t *testing.T) {
	// 起始事件必须以空行结束。bufio 会提前读，中间插入 EOF 会把后续帧拼进同一事件。
	head := strings.Join([]string{
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"hi"}}`,
		"",
		"",
	}, "\n")
	tail := strings.Join([]string{
		"event: content_block_delta",
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"there"}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")
	delay := 150 * time.Millisecond
	pr, pw := io.Pipe()
	t.Cleanup(func() {
		_ = pr.Close()
		_ = pw.Close()
	})
	go func() {
		defer pw.Close()
		if _, err := io.WriteString(pw, head); err != nil {
			return
		}
		time.Sleep(delay)
		_, _ = io.WriteString(pw, tail)
	}()
	measured, text, completed, failed, reason := consumeMonitorSSE(pr, anthropicStreamProtocol, time.Now())
	require.Equal(t, "hithere", text, reason)
	require.True(t, completed)
	require.False(t, failed)
	require.Less(t, measured, delay)
}

func TestConsumeMonitorSSE_AnthropicStartIgnoresNonTextAndEmpty(t *testing.T) {
	toolOnly := strings.Join([]string{
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"lookup","input":{}}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")
	_, text, completed, failed, _ := consumeMonitorSSE(strings.NewReader(toolOnly), anthropicStreamProtocol, time.Now())
	require.Empty(t, text)
	require.False(t, completed)
	require.False(t, failed)

	emptyThenDelta := strings.Join([]string{
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"pong"}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")
	_, text, completed, failed, _ = consumeMonitorSSE(strings.NewReader(emptyThenDelta), anthropicStreamProtocol, time.Now())
	require.Equal(t, "pong", text)
	require.True(t, completed)
	require.False(t, failed)
}

func TestConsumeMonitorSSE_GeminiThoughtIsNotAnswer(t *testing.T) {
	thoughtOnly := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"text":"planning the answer","thought":true}]},"finishReason":"MAX_TOKENS"}]}`,
		"",
	}, "\n")
	_, text, completed, failed, _ := consumeMonitorSSE(strings.NewReader(thoughtOnly), geminiStreamProtocol, time.Now())
	require.Empty(t, text)
	require.False(t, completed)
	require.False(t, failed)

	mixed := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"text":"planning","thought":true},{"text":"pong"}]}}]}`,
		"",
	}, "\n")
	_, text, completed, failed, _ = consumeMonitorSSE(strings.NewReader(mixed), geminiStreamProtocol, time.Now())
	require.Equal(t, "pong", text)
	require.True(t, completed)
	require.False(t, failed)
}

func TestConsumeMonitorSSE_GeminiThoughtDoesNotStartTTFT(t *testing.T) {
	head := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"text":"planning","thought":true}]}}]}`,
		"",
		"",
	}, "\n")
	tail := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"text":"pong"}]},"finishReason":"STOP"}]}`,
		"",
	}, "\n")
	delay := 150 * time.Millisecond
	pr, pw := io.Pipe()
	t.Cleanup(func() {
		_ = pr.Close()
		_ = pw.Close()
	})
	go func() {
		defer pw.Close()
		if _, err := io.WriteString(pw, head); err != nil {
			return
		}
		time.Sleep(delay)
		_, _ = io.WriteString(pw, tail)
	}()
	measured, text, completed, failed, reason := consumeMonitorSSE(pr, geminiStreamProtocol, time.Now())
	require.Equal(t, "pong", text, reason)
	require.True(t, completed)
	require.False(t, failed)
	require.GreaterOrEqual(t, measured, delay)
}

func TestConsumeMonitorSSE_AnthropicTruncatedAfterStartTextIsNotSuccess(t *testing.T) {
	raw := strings.Join([]string{
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"hello"}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"x"}}`,
	}, "\n")
	_, text, completed, failed, reason := consumeMonitorSSE(strings.NewReader(raw), anthropicStreamProtocol, time.Now())
	require.Equal(t, "hello", text)
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

func TestRunCustomCheck_ErrorDoesNotKeepAPIKey(t *testing.T) {
	orig := customMonitorHTTPClient
	customMonitorHTTPClient = &http.Client{}
	t.Cleanup(func() { customMonitorHTTPClient = orig })

	secret := "gateway-token-abcdef123456"
	patternKey := "sk-abcdefghijklmnopqrstuvwxyz"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"bad `+secret+` and `+patternKey+`","url":"https://example.com/v1?api_key=`+secret+`"}`)
	}))
	t.Cleanup(server.Close)

	res := runCustomCheckForModel(t.Context(), customProbeInput{
		MonitorID: 9,
		Provider:  MonitorProviderOpenAI,
		Endpoint:  server.URL,
		APIKey:    secret,
		Model:     "m",
		Config:    defaultChannelMonitorCustomConfig(),
	})
	require.NotContains(t, res.Message, secret)
	require.NotContains(t, res.Message, patternKey)
	require.Contains(t, res.Message, "REDACTED")

	repo := &historyCaptureRepo{}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	svc.persistCheckResults(t.Context(), ChannelMonitorModeCustom, &ChannelMonitor{ID: 9, CheckMode: MonitorCheckModeProbe}, []*CheckResult{res})
	require.Len(t, repo.rows, 1)
	require.NotContains(t, repo.rows[0].Message, secret)
	require.NotContains(t, repo.rows[0].Message, patternKey)
}

type historyCaptureRepo struct {
	ChannelMonitorRepository
	rows []*ChannelMonitorHistoryRow
}

func (r *historyCaptureRepo) InsertHistoryBatch(_ context.Context, rows []*ChannelMonitorHistoryRow) error {
	r.rows = append([]*ChannelMonitorHistoryRow(nil), rows...)
	return nil
}

func (r *historyCaptureRepo) MarkChecked(context.Context, int64, time.Time) error { return nil }

func TestRunCustomCheck_SplitKeyAtErrorCutoffIsNotStored(t *testing.T) {
	orig := customMonitorHTTPClient
	customMonitorHTTPClient = &http.Client{}
	t.Cleanup(func() { customMonitorHTTPClient = orig })

	secret := "gateway-token-abcdef123456"
	body := strings.Repeat("x", monitorErrorBodySnippetMaxBytes-len(secret)/2) + secret + "TAIL"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	res := runCustomCheckForModel(t.Context(), customProbeInput{
		MonitorID: 9,
		Provider:  MonitorProviderOpenAI,
		Endpoint:  server.URL,
		APIKey:    secret,
		Model:     "m",
		Config:    defaultChannelMonitorCustomConfig(),
	})
	require.NotContains(t, res.Message, secret)
	require.NotContains(t, res.Message, secret[:8])
	require.NotContains(t, res.Message, "TAIL")

	repo := &historyCaptureRepo{}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	svc.persistCheckResults(t.Context(), ChannelMonitorModeCustom, &ChannelMonitor{ID: 9, CheckMode: MonitorCheckModeProbe}, []*CheckResult{res})
	require.Len(t, repo.rows, 1)
	require.NotContains(t, repo.rows[0].Message, secret)
	require.NotContains(t, repo.rows[0].Message, secret[:8])
}

func TestRedactProbeErrorBody_DropsCutKey(t *testing.T) {
	secret := "gateway-token-abcdef123456"
	got := redactProbeErrorBody(secret, []byte(strings.Repeat("x", 20)+secret[:10]), true)
	require.Equal(t, "upstream request failed", got)
	require.NotContains(t, got, secret[:8])
}

func TestRunCustomCheck_LongErrorRedactsWholeKey(t *testing.T) {
	orig := customMonitorHTTPClient
	customMonitorHTTPClient = &http.Client{}
	t.Cleanup(func() { customMonitorHTTPClient = orig })

	secret := "gateway-token-abcdef123456"
	body := `{"error":"bad ` + secret + `"}` + strings.Repeat("y", monitorErrorBodySnippetMaxBytes)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	res := runCustomCheckForModel(t.Context(), customProbeInput{
		MonitorID: 9,
		Provider:  MonitorProviderOpenAI,
		Endpoint:  server.URL,
		APIKey:    secret,
		Model:     "m",
		Config:    defaultChannelMonitorCustomConfig(),
	})
	require.NotContains(t, res.Message, secret)
	require.Contains(t, res.Message, "REDACTED")
}

func TestCustomProbeMessage_LeavesShortTokenUntouched(t *testing.T) {
	got := customProbeMessage("error", "upstream stream error")
	require.Equal(t, "upstream stream error", got)
}

func TestRunCustomCheck_ShortAPIKeyEchoIsNotStored(t *testing.T) {
	orig := customMonitorHTTPClient
	customMonitorHTTPClient = &http.Client{}
	t.Cleanup(func() { customMonitorHTTPClient = orig })

	secret := "q7"
	marker := "echo-marker-zzz"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"bad `+secret+` `+marker+`"}`)
	}))
	t.Cleanup(server.Close)

	res := runCustomCheckForModel(t.Context(), customProbeInput{
		MonitorID: 9,
		Provider:  MonitorProviderOpenAI,
		Endpoint:  server.URL,
		APIKey:    secret,
		Model:     "m",
		Config:    defaultChannelMonitorCustomConfig(),
	})
	require.NotContains(t, res.Message, secret)
	require.NotContains(t, res.Message, marker)
	require.Contains(t, res.Message, "upstream HTTP 401")
	require.Contains(t, res.Message, "upstream request failed")

	repo := &historyCaptureRepo{}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	svc.persistCheckResults(t.Context(), ChannelMonitorModeCustom, &ChannelMonitor{ID: 9, CheckMode: MonitorCheckModeProbe}, []*CheckResult{res})
	require.Len(t, repo.rows, 1)
	require.NotContains(t, repo.rows[0].Message, secret)
	require.NotContains(t, repo.rows[0].Message, marker)
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

func TestApplyCustomStream_GeminiDropsLocalStreamField(t *testing.T) {
	_, _, err := applyCustomStream(MonitorProviderGemini, MonitorBodyOverrideModeReplace, []byte(`{"stream":false,"contents":[{"parts":[{"text":"ping"}]}]}`))
	require.ErrorIs(t, err, ErrChannelMonitorCustomStreamRequired)

	body, streaming, err := applyCustomStream(MonitorProviderGemini, MonitorBodyOverrideModeReplace, []byte(`{"stream":true,"contents":[{"parts":[{"text":"ping"}]}]}`))
	require.NoError(t, err)
	require.True(t, streaming)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	_, exists := payload["stream"]
	require.False(t, exists)
	require.Contains(t, string(body), "ping")

	body, streaming, err = applyCustomStream(MonitorProviderGemini, MonitorBodyOverrideModeOff, []byte(`{"contents":[{"parts":[{"text":"ping"}]}]}`))
	require.NoError(t, err)
	require.True(t, streaming)
	require.Equal(t, `{"contents":[{"parts":[{"text":"ping"}]}]}`, string(body))
}

func TestApplyCustomStream_ReplaceRejectsFalseAndKeepsOtherOverrides(t *testing.T) {
	_, _, err := applyCustomStream(MonitorProviderOpenAI, MonitorBodyOverrideModeReplace, []byte(`{"stream":false,"messages":[{"role":"user","content":"ping"}]}`))
	require.ErrorIs(t, err, ErrChannelMonitorCustomStreamRequired)

	body, streaming, err := applyCustomStream(MonitorProviderOpenAI, MonitorBodyOverrideModeMerge, []byte(`{"stream":false,"max_tokens":20}`))
	require.NoError(t, err)
	require.True(t, streaming)
	require.Contains(t, string(body), `"max_tokens":20`)
	require.Contains(t, string(body), `"stream":true`)

	_, streaming, err = applyCustomStream(MonitorProviderOpenAI, MonitorBodyOverrideModeOff, []byte(`{"stream":false,"messages":[]}`))
	require.NoError(t, err)
	require.True(t, streaming)
}

func TestRunCustomCheck_RejectsNonStreamingReplaceBeforeRequest(t *testing.T) {
	orig := customMonitorHTTPClient
	customMonitorHTTPClient = &http.Client{}
	t.Cleanup(func() { customMonitorHTTPClient = orig })

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	t.Cleanup(server.Close)

	res := runCustomCheckForModel(t.Context(), customProbeInput{
		Provider: MonitorProviderOpenAI,
		Endpoint: server.URL,
		APIKey:   "secret-key",
		Model:    "m",
		Opts: &CheckOptions{
			BodyOverrideMode: MonitorBodyOverrideModeReplace,
			BodyOverride: map[string]any{
				"stream":   false,
				"messages": []any{map[string]any{"role": "user", "content": "ping"}},
			},
		},
		Config: defaultChannelMonitorCustomConfig(),
	})
	require.False(t, called)
	require.Equal(t, MonitorStatusError, res.Status)
	require.Contains(t, res.Message, "streaming request")
	require.Nil(t, res.LatencyMs)
	require.Nil(t, res.TokensPerSec)
	require.NotContains(t, res.Message, "secret-key")
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
