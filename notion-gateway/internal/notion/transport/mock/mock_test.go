package mock

import "testing"

func TestEchoInputReturnsCurrentUserRequest(t *testing.T) {
    prompt := "<recent_turns>\nUser: earlier decision\n</recent_turns>\n\n<current_user_request>\nDecision: use local-first continuity.\n</current_user_request>"
    got := echoInput(prompt)
    want := "Decision: use local-first continuity."
    if got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
}

func TestEchoInputPreservesPlainInput(t *testing.T) {
    if got := echoInput("hello"); got != "hello" {
        t.Fatalf("got %q", got)
    }
}
