package claude

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

// vLLM-backed gateways emit "reasoning"; DeepSeek-style backends emit
// "reasoning_content". Both must reach Claude Code as thinking blocks.
func TestOpenAIReasoningNodeAcceptsBothSpellings(t *testing.T) {
	cases := []struct{ name, json, want string }{
		{"reasoning_content", `{"reasoning_content":"rc"}`, "rc"},
		{"reasoning", `{"reasoning":"r"}`, "r"},
		{"prefers reasoning_content", `{"reasoning_content":"rc","reasoning":"r"}`, "rc"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := openAIReasoningNode(gjson.Parse(c.json), "")
			if !got.Exists() {
				t.Fatalf("no reasoning node found in %s", c.json)
			}
			if got.String() != c.want {
				t.Fatalf("got %q, want %q", got.String(), c.want)
			}
		})
	}
}

func TestOpenAIReasoningNodeHonoursPrefix(t *testing.T) {
	got := openAIReasoningNode(gjson.Parse(`{"message":{"reasoning":"deep"}}`), "message.")
	if got.String() != "deep" {
		t.Fatalf("prefixed lookup got %q, want %q", got.String(), "deep")
	}
}

func TestOpenAIReasoningNodeAbsent(t *testing.T) {
	if openAIReasoningNode(gjson.Parse(`{"content":"hi"}`), "").Exists() {
		t.Fatal("reported a reasoning node where none exists")
	}
}

// End-to-end: a vLLM-shaped streaming delta must produce a thinking block.
func TestStreamingReasoningFieldBecomesThinkingBlock(t *testing.T) {
	chunk := `{"choices":[{"index":0,"delta":{"role":"assistant","reasoning":"step one"}}]}`
	got := openAIReasoningNode(gjson.Parse(chunk).Get("choices.0.delta"), "")
	if !got.Exists() || !strings.Contains(got.String(), "step one") {
		t.Fatalf("vLLM-shaped delta did not yield reasoning text, got %q", got.String())
	}
}
