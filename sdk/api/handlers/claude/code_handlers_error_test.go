package claude

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/interfaces"
	"github.com/tidwall/gjson"
)

func TestClaudeErrorExtractsOpenAIStyleUpstreamJSON(t *testing.T) {
	handler := &ClaudeCodeAPIHandler{}
	msg := &interfaces.ErrorMessage{
		StatusCode: http.StatusBadRequest,
		Error:      errors.New(`{"error":{"message":"Your input exceeds the context window of this model. Please adjust your input and try again.","type":"invalid_request_error","code":"context_too_large"}}`),
	}

	got := handler.toClaudeError(msg)

	if got.Type != "error" {
		t.Fatalf("type = %q, want error", got.Type)
	}
	if got.Error.Type != "invalid_request_error" {
		t.Fatalf("error.type = %q, want invalid_request_error", got.Error.Type)
	}
	if got.Error.Message != "Your input exceeds the context window of this model. Please adjust your input and try again." {
		t.Fatalf("error.message = %q", got.Error.Message)
	}
}

func TestClaudeErrorExtractsClaudeStyleUpstreamJSON(t *testing.T) {
	handler := &ClaudeCodeAPIHandler{}
	msg := &interfaces.ErrorMessage{
		StatusCode: http.StatusTooManyRequests,
		Error:      errors.New(`{"type":"error","error":{"type":"rate_limit_error","message":"This request would exceed your account's rate limit. Please try again later."},"request_id":"req_123"}`),
	}

	got := handler.toClaudeError(msg)

	if got.Error.Type != "rate_limit_error" {
		t.Fatalf("error.type = %q, want rate_limit_error", got.Error.Type)
	}
	if got.Error.Message != "This request would exceed your account's rate limit. Please try again later." {
		t.Fatalf("error.message = %q", got.Error.Message)
	}
}

func TestWriteClaudeErrorResponseUsesClaudeEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	handler := &ClaudeCodeAPIHandler{}
	msg := &interfaces.ErrorMessage{
		StatusCode: http.StatusBadRequest,
		Error:      errors.New(`{"error":{"message":"Your input exceeds the context window of this model. Please adjust your input and try again.","type":"invalid_request_error","code":"context_too_large"}}`),
	}

	handler.WriteErrorResponse(c, msg)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	body := recorder.Body.Bytes()
	if got := gjson.GetBytes(body, "type").String(); got != "error" {
		t.Fatalf("type = %q, want error; body=%s", got, body)
	}
	if got := gjson.GetBytes(body, "error.type").String(); got != "invalid_request_error" {
		t.Fatalf("error.type = %q, want invalid_request_error; body=%s", got, body)
	}
	if got := gjson.GetBytes(body, "error.message").String(); got != "Your input exceeds the context window of this model. Please adjust your input and try again." {
		t.Fatalf("error.message = %q; body=%s", got, body)
	}
}

func TestPendingClaudeStreamErrorUsesBufferedError(t *testing.T) {
	wantErr := &interfaces.ErrorMessage{
		StatusCode: http.StatusBadRequest,
		Error:      errors.New(`{"error":{"message":"Your input exceeds the context window of this model. Please adjust your input and try again.","type":"invalid_request_error","code":"context_too_large"}}`),
	}
	errs := make(chan *interfaces.ErrorMessage, 1)
	errs <- wantErr
	close(errs)

	gotErr, ok := pendingClaudeStreamError(errs)
	if !ok {
		t.Fatal("expected pending stream error")
	}
	if gotErr != wantErr {
		t.Fatalf("pending error = %p, want %p", gotErr, wantErr)
	}
}

// claudeCodeCtxLimitPattern is the pattern Claude Code uses to decide that an
// over-length prompt is recoverable, extracted from the shipped 2.1.220 client.
// Normalized messages must match it or the client will not compact and retry.
var claudeCodeCtxLimitPattern = regexp.MustCompile(`prompt is too long[^0-9]*(\d+)\s*tokens?\s*>\s*(\d+)`)

func TestClaudeErrorNormalizesVLLMContextLimit(t *testing.T) {
	handler := &ClaudeCodeAPIHandler{}
	msg := &interfaces.ErrorMessage{
		StatusCode: http.StatusBadRequest,
		Error: errors.New(`{"detail":"This model's maximum context length is 32768 tokens. ` +
			`However, your request has 66511 input tokens. Please reduce the length of the ` +
			`input messages. (parameter=input_tokens, value=66511)","status_code":400}`),
	}

	got := handler.toClaudeError(msg)

	m := claudeCodeCtxLimitPattern.FindStringSubmatch(got.Error.Message)
	if m == nil {
		t.Fatalf("message does not match the Claude Code context-limit pattern: %q", got.Error.Message)
	}
	if m[1] != "66511" {
		t.Fatalf("used tokens = %q, want 66511 (message=%q)", m[1], got.Error.Message)
	}
	if m[2] != "32768" {
		t.Fatalf("limit tokens = %q, want 32768 (message=%q)", m[2], got.Error.Message)
	}
	if !strings.Contains(got.Error.Message, "maximum context length is 32768") {
		t.Fatalf("original upstream text was dropped: %q", got.Error.Message)
	}
}

func TestClaudeErrorNormalizesOpenAIStyleRequestedTokens(t *testing.T) {
	handler := &ClaudeCodeAPIHandler{}
	msg := &interfaces.ErrorMessage{
		StatusCode: http.StatusBadRequest,
		Error: errors.New(`{"error":{"message":"This model's maximum context length is 8192 ` +
			`tokens. However, you requested 9000 tokens (8000 in the messages, 1000 in the ` +
			`completion).","type":"invalid_request_error","code":"context_length_exceeded"}}`),
	}

	got := handler.toClaudeError(msg)

	m := claudeCodeCtxLimitPattern.FindStringSubmatch(got.Error.Message)
	if m == nil {
		t.Fatalf("message does not match the Claude Code context-limit pattern: %q", got.Error.Message)
	}
	if m[1] != "9000" || m[2] != "8192" {
		t.Fatalf("used/limit = %q/%q, want 9000/8192 (message=%q)", m[1], m[2], got.Error.Message)
	}
}

func TestClaudeErrorExtractsFastAPIDetailField(t *testing.T) {
	handler := &ClaudeCodeAPIHandler{}
	msg := &interfaces.ErrorMessage{
		StatusCode: http.StatusBadRequest,
		Error:      errors.New(`{"detail":"Unsupported parameter: reasoning_effort","status_code":400}`),
	}

	got := handler.toClaudeError(msg)

	if got.Error.Message != "Unsupported parameter: reasoning_effort" {
		t.Fatalf("error.message = %q, want the detail field to be unwrapped", got.Error.Message)
	}
}

func TestClaudeErrorLeavesMaxTokensRejectionAlone(t *testing.T) {
	// Compacting the conversation cannot fix an oversized max_tokens, so this must not
	// be rewritten into a phrase that makes the client retry.
	handler := &ClaudeCodeAPIHandler{}
	msg := &interfaces.ErrorMessage{
		StatusCode: http.StatusBadRequest,
		Error: errors.New(`{"detail":"max_tokens=500000 cannot be greater than ` +
			`max_model_len=max_total_tokens=393216. Please request fewer output tokens.","status_code":400}`),
	}

	got := handler.toClaudeError(msg)

	if claudeCodeCtxLimitPattern.MatchString(got.Error.Message) {
		t.Fatalf("max_tokens rejection was rewritten as a prompt-length error: %q", got.Error.Message)
	}
	if !strings.HasPrefix(got.Error.Message, "max_tokens=500000") {
		t.Fatalf("error.message = %q, want the detail field unwrapped unchanged", got.Error.Message)
	}
}

func TestClaudeErrorLeavesAlreadyNormalizedMessageAlone(t *testing.T) {
	handler := &ClaudeCodeAPIHandler{}
	msg := &interfaces.ErrorMessage{
		StatusCode: http.StatusBadRequest,
		Error:      errors.New(`{"error":{"message":"prompt is too long: 250000 tokens > 200000 maximum","type":"invalid_request_error"}}`),
	}

	got := handler.toClaudeError(msg)

	if got.Error.Message != "prompt is too long: 250000 tokens > 200000 maximum" {
		t.Fatalf("error.message = %q, want it passed through unchanged", got.Error.Message)
	}
}

func TestNormalizeContextLimitMessageIgnoresNon4xx(t *testing.T) {
	in := "This model's maximum context length is 32768 tokens. However, your request has 66511 input tokens."
	if got := normalizeContextLimitMessage(http.StatusInternalServerError, in); got != in {
		t.Fatalf("500 response was rewritten: %q", got)
	}
	if got := normalizeContextLimitMessage(http.StatusRequestEntityTooLarge, in); !claudeCodeCtxLimitPattern.MatchString(got) {
		t.Fatalf("413 response was not normalized: %q", got)
	}
}
