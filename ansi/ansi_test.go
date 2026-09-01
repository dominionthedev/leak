package ansi

import "testing"

func TestCSIPrivateModes(t *testing.T) {
	// These are the sequences that were silently broken: '?' rendered as
	// its decimal codepoint ("63") instead of the literal character, and
	// even after fixing that, a naive fix would insert a wrong ';'
	// between the marker and the following parameter.
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"AltScreen(true)", AltScreen(true), "\x1b[?1049h"},
		{"AltScreen(false)", AltScreen(false), "\x1b[?1049l"},
		{"CursorHide", CursorHide(), "\x1b[?25l"},
		{"CursorShow", CursorShow(), "\x1b[?25h"},
		{"BracketedPaste(true)", BracketedPaste(true), "\x1b[?2004h"},
		{"FocusReporting(true)", FocusReporting(true), "\x1b[?1004h"},
		{"SynchronizedOutput(true)", SynchronizedOutput(true), "\x1b[?2026h"},
		{"MouseTracking(true)", MouseTracking(true), "\x1b[?1000h\x1b[?1002h\x1b[?1006h"},
		{"QuerySecondaryDA", QuerySecondaryDA(), "\x1b[>c"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestCSIPlainParamsUnaffected(t *testing.T) {
	// The marker-prefix handling must not change any sequence that
	// doesn't start with a byte/rune marker.
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"CursorUp(5)", CursorUp(5), "\x1b[5A"},
		{"CursorPosition(3,7)", CursorPosition(3, 7), "\x1b[3;7H"},
		{"CursorStyle(2)", CursorStyle(2), "\x1b[2 q"},
		{"QueryDeviceAttributes", QueryDeviceAttributes(), "\x1b[c"},
		{"QueryCursorPosition", QueryCursorPosition(), "\x1b[6n"},
		{"FgTrueColor(1,2,3)", FgTrueColor(1, 2, 3), "\x1b[38;2;1;2;3m"},
		{"Fg256(200)", Fg256(200), "\x1b[38;5;200m"},
		{"Reset", Reset(), "\x1b[0m"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestOSCSanitizesControlBytes(t *testing.T) {
	// A title containing a raw ESC/BEL must not be able to terminate the
	// OSC sequence early and inject further escape sequences. Stripping
	// just the control bytes is what matters here — the surrounding
	// printable text is inert either way, it's not an escape sequence
	// without the ESC/BEL bytes that introduce one.
	malicious := "hi\x1b]0;evil\x07 title"
	got := SetTitle(malicious)
	want := ESC + "]0;hi]0;evil title\x07"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// No matter what, only one ESC byte (the sequence's own opener) and
	// one terminating BEL should ever appear in the output.
	if n := countByte(got, 0x1b); n != 1 {
		t.Fatalf("expected exactly 1 ESC byte in output, got %d: %q", n, got)
	}
	if n := countByte(got, 0x07); n != 1 {
		t.Fatalf("expected exactly 1 BEL byte in output, got %d: %q", n, got)
	}
}

func countByte(s string, b byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			n++
		}
	}
	return n
}

func TestOSCPassesThroughOrdinaryText(t *testing.T) {
	got := SetTitle("my-session")
	want := ESC + "]0;my-session\x07"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
