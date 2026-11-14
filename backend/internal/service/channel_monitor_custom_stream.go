package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/tidwall/gjson"
)

// customMonitorHTTPClient 不做 Client.Timeout 限制。
// Custom 的硬截止是红色阈值对应的 context deadline，按真实经过时间计算。
var customMonitorHTTPClient = newCustomMonitorHTTPClient()

func newCustomMonitorHTTPClient() *http.Client {
	tr := &http.Transport{
		DialContext:         safeDialContext,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        16,
		IdleConnTimeout:     monitorIdleConnTimeout,
		TLSHandshakeTimeout: monitorTLSHandshakeTimeout,
	}
	return &http.Client{Transport: servertiming.WrapRoundTripper(tr)}
}

type monitorStreamProtocol struct {
	text func(event, data string) string
	done func(event, data string) bool
	fail func(event, data string) bool
}

type customProbeInput struct {
	MonitorID int64
	Provider  string
	Endpoint  string
	APIKey    string
	Model     string
	Opts      *CheckOptions
	Config    ChannelMonitorCustomConfig
	Override  *ChannelMonitorCustomOverride
}

func runCustomCheckForModel(ctx context.Context, in customProbeInput) *CheckResult {
	res := &CheckResult{
		Model:      in.Model,
		Status:     MonitorStatusError,
		MetricKind: MonitorMetricKindTTFT,
		CheckedAt:  time.Now(),
	}
	prompt := strings.TrimSpace(in.Config.Prompt)
	if prompt == "" {
		prompt = monitorCustomPromptDefault
	}
	ttft := resolveTTFTPolicy(in.Config, in.Override)
	tps := resolveTPSPolicy(in.Config, in.Override)
	if err := validateTTFTPolicy(ttft); err != nil {
		res.Message = truncateMessage(err.Error())
		return res
	}
	if err := validateTPSPolicy(tps); err != nil {
		res.Message = truncateMessage(err.Error())
		return res
	}

	adapter, apiMode, ok := providerAdapterFor(in.Provider, checkAPIMode(in.Opts))
	if !ok {
		res.Message = truncateMessage("unsupported provider")
		return res
	}
	body, err := buildRequestBody(adapter, in.Provider, apiMode, in.Model, prompt, in.Opts)
	if err != nil {
		res.Message = truncateMessage(sanitizeErrorMessage(err.Error()))
		return res
	}
	body, streaming, err := applyCustomStream(in.Provider, bodyOverrideMode(in.Opts), body)
	if err != nil {
		res.Message = truncateMessage(sanitizeErrorMessage(err.Error()))
		return res
	}
	headers := mergeHeaders(adapter.buildHeaders(in.APIKey), in.Opts)
	path := monitorRequestPath(in.Provider, in.Endpoint, adapter, in.Model)
	if streaming && in.Provider == MonitorProviderGemini {
		path = strings.Replace(path, ":generateContent", ":streamGenerateContent", 1)
		path += "?alt=sse"
	}
	full := joinURL(in.Endpoint, path)

	start := time.Now()
	var measured time.Duration
	var text string
	var completed bool
	var statusCode int
	var cause string
	if streaming {
		measured, text, completed, statusCode, cause = readCustomStream(ctx, full, body, headers, streamProtocolFor(in.Provider, apiMode), start)
	} else {
		measured, text, completed, statusCode, cause = readCustomFull(ctx, full, body, headers, in.Provider, apiMode, start)
	}
	elapsedMs := int(time.Since(start) / time.Millisecond)
	processed, procErr := applyProcessedTTFT(int(measured/time.Millisecond), ttft)
	if procErr != nil {
		res.Message = truncateMessage(procErr.Error())
		res.LatencyMs = &elapsedMs
		return res
	}
	if measured > 0 {
		res.LatencyMs = &processed
	} else {
		res.LatencyMs = &elapsedMs
	}
	if tps.Enabled && strings.TrimSpace(text) != "" {
		value := deriveTokensPerSec(in.MonitorID, in.Model, processed, tps.Min, tps.Max)
		res.TokensPerSec = &value
	}

	deadlineHit := errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(cause, "deadline")
	if deadlineHit {
		res.Status = MonitorStatusError
		res.Message = customDiagnostic(res.Status, int(measured/time.Millisecond), processed, "detection deadline exceeded")
		return res
	}
	if cause != "" && !completed {
		res.Status = MonitorStatusError
		if statusCode >= 400 {
			res.Message = customDiagnostic(res.Status, int(measured/time.Millisecond), processed, fmt.Sprintf("upstream HTTP %d: %s", statusCode, cause))
			return res
		}
		res.Message = customDiagnostic(res.Status, int(measured/time.Millisecond), processed, cause)
		return res
	}
	if !completed || strings.TrimSpace(text) == "" {
		res.Status = MonitorStatusError
		res.Message = customDiagnostic(res.Status, int(measured/time.Millisecond), processed, "custom probe returned no text")
		return res
	}
	res.Status = healthFromProcessedTTFT(processed, in.Config.YellowSeconds, in.Config.RedSeconds, true)
	if res.Status != MonitorStatusOperational {
		res.Message = customDiagnostic(res.Status, int(measured/time.Millisecond), processed, "")
	}
	return res
}

func customDiagnostic(status string, measured, processed int, cause string) string {
	if cause == "" {
		cause = fmt.Sprintf("ttft %dms", processed)
	}
	if measured > 0 && measured != processed {
		cause = fmt.Sprintf("%s (measured %dms, processed %dms)", cause, measured, processed)
	}
	if status == "" {
		return truncateMessage(cause)
	}
	return truncateMessage(cause)
}

func applyCustomStream(provider string, mode string, body []byte) ([]byte, bool, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, fmt.Errorf("decode probe body: %w", err)
	}
	if provider == MonitorProviderGemini {
		streaming := true
		if mode == MonitorBodyOverrideModeReplace {
			if explicit, ok := payload["stream"].(bool); ok && !explicit {
				streaming = false
			}
		}
		return body, streaming, nil
	}
	streaming := true
	if mode == MonitorBodyOverrideModeReplace {
		if explicit, ok := payload["stream"].(bool); ok && !explicit {
			streaming = false
		}
	}
	if streaming {
		payload["stream"] = true
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}
	return out, streaming, nil
}

func streamProtocolFor(provider, apiMode string) monitorStreamProtocol {
	if provider == MonitorProviderOpenAI && apiMode == MonitorAPIModeResponses {
		return responsesStreamProtocol
	}
	switch provider {
	case MonitorProviderAnthropic:
		return anthropicStreamProtocol
	case MonitorProviderGemini:
		return geminiStreamProtocol
	default:
		return openAIChatStreamProtocol
	}
}

var openAIChatStreamProtocol = monitorStreamProtocol{
	text: func(_, data string) string {
		if data == "[DONE]" {
			return ""
		}
		return gjson.Get(data, "choices.0.delta.content").String()
	},
	done: func(_, data string) bool { return data == "[DONE]" },
	fail: func(_, data string) bool {
		return data != "[DONE]" && gjson.Get(data, "error").Exists()
	},
}

var responsesStreamProtocol = monitorStreamProtocol{
	text: func(_, data string) string {
		if gjson.Get(data, "type").String() != "response.output_text.delta" {
			return ""
		}
		return gjson.Get(data, "delta").String()
	},
	done: func(_, data string) bool {
		return gjson.Get(data, "type").String() == "response.completed"
	},
	fail: func(_, data string) bool {
		switch gjson.Get(data, "type").String() {
		case "response.failed", "error":
			return true
		default:
			return gjson.Get(data, "error").Exists() && gjson.Get(data, "type").String() != "response.output_text.delta"
		}
	},
}

func sseEventKind(event, data string) string {
	if event != "" {
		return event
	}
	return gjson.Get(data, "type").String()
}

var anthropicStreamProtocol = monitorStreamProtocol{
	text: func(event, data string) string {
		if sseEventKind(event, data) != "content_block_delta" {
			return ""
		}
		return gjson.Get(data, "delta.text").String()
	},
	done: func(event, data string) bool {
		return sseEventKind(event, data) == "message_stop"
	},
	fail: func(event, data string) bool {
		return sseEventKind(event, data) == "error"
	},
}

var geminiStreamProtocol = monitorStreamProtocol{
	text: func(_, data string) string {
		return gjson.Get(data, "candidates.0.content.parts.0.text").String()
	},
	done: func(_, data string) bool {
		return gjson.Get(data, "candidates.0.finishReason").String() != ""
	},
	fail: func(_, data string) bool { return gjson.Get(data, "error").Exists() },
}

func readCustomStream(ctx context.Context, fullURL string, body []byte, headers map[string]string, protocol monitorStreamProtocol, start time.Time) (time.Duration, string, bool, int, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(body))
	if err != nil {
		return 0, "", false, 0, err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := customMonitorHTTPClient.Do(req)
	if err != nil {
		return 0, "", false, 0, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, monitorErrorBodySnippetMaxBytes))
		return 0, "", false, resp.StatusCode, strings.TrimSpace(string(snippet))
	}
	measured, text, completed, failed, cause := consumeMonitorSSE(resp.Body, protocol, start)
	if failed {
		if cause == "" {
			cause = "upstream stream error"
		}
		return measured, text, false, resp.StatusCode, cause
	}
	return measured, text, completed, resp.StatusCode, cause
}

func readCustomFull(ctx context.Context, fullURL string, body []byte, headers map[string]string, provider, apiMode string, start time.Time) (time.Duration, string, bool, int, string) {
	raw, status, err := postJSON(ctx, customMonitorHTTPClient, fullURL, body, headers, "application/json")
	elapsed := time.Since(start)
	if err != nil {
		return elapsed, "", false, status, err.Error()
	}
	if status < 200 || status >= 300 {
		return elapsed, "", false, status, truncateForErrorBody(string(raw))
	}
	text := extractProviderProbeText(provider, apiMode, raw)
	if strings.TrimSpace(text) == "" {
		return elapsed, "", false, status, "replace-mode: upstream returned 2xx with empty text"
	}
	return elapsed, text, true, status, ""
}

func extractProviderProbeText(provider, apiMode string, body []byte) string {
	adapter, mode, ok := providerAdapterFor(provider, apiMode)
	if !ok {
		return ""
	}
	if provider == MonitorProviderOpenAI && mode == MonitorAPIModeResponses {
		return extractOpenAIResponsesText(body)
	}
	return extractMonitorResponseText(adapter, body)
}

func consumeMonitorSSE(r io.Reader, protocol monitorStreamProtocol, start time.Time) (time.Duration, string, bool, bool, string) {
	reader := bufio.NewReader(io.LimitReader(r, monitorResponseMaxBytes))
	var b strings.Builder
	var measured time.Duration
	sawText := false
	for {
		event, data, err := readSSEEvent(reader)
		if data != "" || event != "" {
			if protocol.fail != nil && protocol.fail(event, data) {
				return measured, b.String(), false, true, "upstream stream error"
			}
			if piece := protocol.text(event, data); strings.TrimSpace(piece) != "" {
				if !sawText {
					sawText = true
					measured = time.Since(start)
				}
				b.WriteString(piece)
			}
			if protocol.done != nil && protocol.done(event, data) {
				return measured, b.String(), sawText, false, ""
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return measured, b.String(), sawText, false, ""
			}
			return measured, b.String(), false, false, err.Error()
		}
	}
}

func readSSEEvent(r *bufio.Reader) (string, string, error) {
	var eventName string
	var data []string
	for {
		line, err := r.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				if eventName != "" || len(data) > 0 {
					return eventName, strings.Join(data, "\n"), err
				}
				if err != nil {
					return "", "", err
				}
				continue
			}
			if strings.HasPrefix(line, ":") {
				if err != nil {
					return "", "", err
				}
				continue
			}
			if strings.HasPrefix(line, "event:") {
				eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if err != nil {
			if eventName != "" || len(data) > 0 {
				return eventName, strings.Join(data, "\n"), nil
			}
			return "", "", err
		}
	}
}
