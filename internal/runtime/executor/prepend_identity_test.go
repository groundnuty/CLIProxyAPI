package executor

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

const identity = "You are Claude Code, Anthropic's official CLI for Claude."

func firstSystemText(payload []byte) string {
	return gjson.GetBytes(payload, "system.0.text").String()
}

// The classifier's prompt must survive, not be replaced or relocated.
func TestPrependIdentityPreservesCallerPrompt(t *testing.T) {
	in := []byte(`{"model":"claude-opus-5","system":[{"type":"text","text":"You are a security monitor for autonomous AI coding agents."}]}`)
	out := prependIdentityBlock(in)

	if got := firstSystemText(out); got != identity {
		t.Fatalf("first system block = %q, want the identity line", got)
	}
	blocks := gjson.GetBytes(out, "system").Array()
	if len(blocks) != 2 {
		t.Fatalf("got %d system blocks, want 2", len(blocks))
	}
	if !strings.Contains(blocks[1].Get("text").String(), "security monitor") {
		t.Fatalf("caller's prompt was lost: %q", blocks[1].Get("text").String())
	}
}

func TestPrependIdentityIsIdempotent(t *testing.T) {
	in := []byte(`{"system":[{"type":"text","text":"` + identity + `"},{"type":"text","text":"more"}]}`)
	out := prependIdentityBlock(in)
	if n := len(gjson.GetBytes(out, "system").Array()); n != 2 {
		t.Fatalf("identity already first, but block count changed to %d", n)
	}
}

// Full cloaking already put its billing header first; do not double-prefix.
func TestPrependIdentityRespectsBillingHeader(t *testing.T) {
	in := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=1;"},{"type":"text","text":"` + identity + `"}]}`)
	out := prependIdentityBlock(in)
	if n := len(gjson.GetBytes(out, "system").Array()); n != 2 {
		t.Fatalf("billing header present, but block count changed to %d", n)
	}
}

func TestPrependIdentityNormalizesStringSystem(t *testing.T) {
	in := []byte(`{"system":"be terse"}`)
	out := prependIdentityBlock(in)
	blocks := gjson.GetBytes(out, "system").Array()
	if len(blocks) != 2 || firstSystemText(out) != identity {
		t.Fatalf("string system not normalized: %s", gjson.GetBytes(out, "system").Raw)
	}
	if blocks[1].Get("text").String() != "be terse" {
		t.Fatalf("original string prompt lost")
	}
}

func TestPrependIdentityHandlesMissingSystem(t *testing.T) {
	out := prependIdentityBlock([]byte(`{"model":"claude-opus-5"}`))
	if firstSystemText(out) != identity {
		t.Fatalf("missing system not populated")
	}
}
