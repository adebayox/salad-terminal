package chat

import "testing"

func TestTerminalSafeRemovesTerminalControlSequences(t *testing.T) {
	input := "\x1b[31mred\x1b[0m\n\x1b]0;owned title\x07next\x00"
	if got, want := terminalSafe(input), "red\nnext"; got != want {
		t.Fatalf("terminalSafe() = %q, want %q", got, want)
	}
}

func TestTerminalSafeKeepsReadableWhitespace(t *testing.T) {
	if got, want := terminalSafe("a\tb\nc"), "a\tb\nc"; got != want {
		t.Fatalf("terminalSafe() = %q, want %q", got, want)
	}
}
