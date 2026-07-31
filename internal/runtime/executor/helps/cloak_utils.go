package helps

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// userIDPattern matches Claude Code format: user_[64-hex]_account_[uuid]_session_[uuid]
var userIDPattern = regexp.MustCompile(`^user_[a-fA-F0-9]{64}_account_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}_session_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// generateFakeUserID generates a fake user ID in Claude Code format.
// Format: user_[64-hex-chars]_account_[UUID-v4]_session_[UUID-v4]
func generateFakeUserID() string {
	hexBytes := make([]byte, 32)
	_, _ = rand.Read(hexBytes)
	hexPart := hex.EncodeToString(hexBytes)
	accountUUID := uuid.New().String()
	sessionUUID := uuid.New().String()
	return "user_" + hexPart + "_account_" + accountUUID + "_session_" + sessionUUID
}

// isValidUserID checks if a user ID matches Claude Code format.
func isValidUserID(userID string) bool {
	return userIDPattern.MatchString(userID)
}

func GenerateFakeUserID() string {
	return generateFakeUserID()
}

func IsValidUserID(userID string) bool {
	return isValidUserID(userID)
}

// ShouldCloak determines if request should be cloaked based on config and client User-Agent.
// Returns true if cloaking should be applied.
func ShouldCloak(cloakMode string, userAgent string) bool {
	switch strings.ToLower(cloakMode) {
	case "always", "prefix":
		return true
	case "never":
		return false
	default: // "auto" or empty
		// If client is Claude Code, don't cloak
		return !strings.HasPrefix(userAgent, "claude-cli")
	}
}

// IsPrefixMode reports whether the credential asked for identity-prefix mode.
//
// The Anthropic subscription endpoint rejects a request whose FIRST system block
// is neither the Claude Code identity line nor the billing header, answering
// 429 {"type":"rate_limit_error","message":"Error"} — a generic refusal wearing a
// rate-limit label. Claude Code's own auto-mode Bash classifier sends a different
// first block ("You are a security monitor for autonomous AI coding agents."), so
// its calls are refused and auto mode loses Bash.
//
// Full cloaking makes such a request pass, but by REPLACING the system array
// outright and relocating the caller's prompt into the first user message — the
// classifier then receives Claude Code's own prompt instead of its instructions.
// Prefix mode only guarantees the required first block and leaves everything else
// in place, which measurement shows is sufficient:
//
//	[security]            -> 429
//	[identity]            -> 200
//	[identity, security]  -> 200   (prefix mode; caller's prompt intact)
func IsPrefixMode(cloakMode string) bool {
	return strings.EqualFold(strings.TrimSpace(cloakMode), "prefix")
}

// ClaudeCodeIdentityLine is the first system block the subscription endpoint expects.
const ClaudeCodeIdentityLine = "You are Claude Code, Anthropic's official CLI for Claude."

// isClaudeCodeClient checks if the User-Agent indicates a Claude Code client.
func isClaudeCodeClient(userAgent string) bool {
	return strings.HasPrefix(userAgent, "claude-cli")
}
