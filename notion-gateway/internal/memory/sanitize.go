package memory

import (
    "regexp"
    "strings"
)

var secretPatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)(?:token_v2\s*[:=]?\s*[A-Za-z0-9._~+/-]{8,}|sk-[A-Za-z0-9_-]{16,}|gh[pousr]_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16})`),
    regexp.MustCompile(`(?i)(?:authorization|cookie|set-cookie|password|api[_-]?key|secret|private[_-]?key)\s*[:=]\s*[^\s,;]+`),
    regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/-]{12,}`),
}

// Sanitize removes credentials before text is persisted, indexed, embedded, or logged.
func Sanitize(input string) string {
    out := input
    for _, pattern := range secretPatterns {
        out = pattern.ReplaceAllStringFunc(out, func(match string) string {
            lower := strings.ToLower(match)
            switch {
            case strings.Contains(lower, "cookie"):
                return "[REDACTED:cookie]"
            case strings.Contains(lower, "password"):
                return "[REDACTED:password]"
            case strings.Contains(lower, "key"), strings.Contains(lower, "token"), strings.Contains(lower, "bearer"):
                return "[REDACTED:credential]"
            default:
                return "[REDACTED:secret]"
            }
        })
    }
    return out
}

func containsSecret(input string) bool {
    return HasSensitiveContent(input)
}

// HasSensitiveContent reports whether persistence would redact credential-like data.
func HasSensitiveContent(input string) bool {
    return Sanitize(input) != input
}
