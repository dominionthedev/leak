package ansi

import (
	"fmt"
	"strconv"
	"strings"
)

const ESC = "\x1b"

// CSI builds a Control Sequence Introducer sequence: ESC [ params... final
// Example: CSI(1, 1, 'H') → "\x1b[1;1H"
func CSI(params ...any) string {
	if len(params) == 0 {
		return ESC + "["
	}

	var b strings.Builder
	b.WriteString(ESC)
	b.WriteByte('[')

	for i, p := range params {
		if i == len(params)-1 {
			// last argument is the final byte(s)
			switch v := p.(type) {
			case byte:
				b.WriteByte(v)
			case rune:
				b.WriteRune(v)
			case string:
				b.WriteString(v)
			default:
				b.WriteString(fmt.Sprint(v))
			}
			break
		}

		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(fmt.Sprint(p))
	}
	return b.String()
}

// OSC builds an Operating System Command: ESC ] cmd ; args BEL/ST
func OSC(cmd int, args ...string) string {
	var b strings.Builder
	b.WriteString(ESC)
	b.WriteByte(']')
	b.WriteString(strconv.Itoa(cmd))
	for _, a := range args {
		b.WriteByte(';')
		b.WriteString(a)
	}
	b.WriteByte('\x07') // BEL terminator (widely supported)
	return b.String()
}

// --- Cursor ---

func CursorUp(n int) string        { return CSI(n, 'A') }
func CursorDown(n int) string      { return CSI(n, 'B') }
func CursorForward(n int) string   { return CSI(n, 'C') }
func CursorBack(n int) string      { return CSI(n, 'D') }
func CursorNextLine(n int) string  { return CSI(n, 'E') }
func CursorPrevLine(n int) string  { return CSI(n, 'F') }
func CursorColumn(col int) string  { return CSI(col, 'G') }
func CursorPosition(row, col int) string {
	return CSI(row, col, 'H')
}
func CursorSave() string    { return ESC + "7" }
func CursorRestore() string { return ESC + "8" }
func CursorHide() string    { return CSI('?', 25, 'l') }
func CursorShow() string    { return CSI('?', 25, 'h') }
func CursorStyle(style int) string { // 0=block, 1=block blink, 2=block, 3=underline blink, 4=underline, 5=bar blink, 6=bar
	return CSI(style, " q")
}

// --- Erase ---

func EraseDisplay(n int) string { // 0=below, 1=above, 2=all, 3=all+scrollback
	return CSI(n, 'J')
}
func EraseLine(n int) string { // 0=to end, 1=to start, 2=all
	return CSI(n, 'K')
}
func ClearScreen() string {
	return EraseDisplay(2) + CursorPosition(1, 1)
}

// --- Scroll ---

func ScrollUp(n int) string   { return CSI(n, 'S') }
func ScrollDown(n int) string { return CSI(n, 'T') }

// --- Modes (private DEC / xterm) ---

func AltScreen(enable bool) string {
	if enable {
		return CSI('?', 1049, 'h')
	}
	return CSI('?', 1049, 'l')
}

func MouseTracking(enable bool) string {
	// 1000 = normal tracking, 1002 = button-event, 1006 = SGR
	if enable {
		return CSI('?', 1000, 'h') + CSI('?', 1002, 'h') + CSI('?', 1006, 'h')
	}
	return CSI('?', 1000, 'l') + CSI('?', 1002, 'l') + CSI('?', 1006, 'l')
}

func BracketedPaste(enable bool) string {
	if enable {
		return CSI('?', 2004, 'h')
	}
	return CSI('?', 2004, 'l')
}

func FocusReporting(enable bool) string {
	if enable {
		return CSI('?', 1004, 'h')
	}
	return CSI('?', 1004, 'l')
}

func SynchronizedOutput(enable bool) string {
	// 2026 = synchronized output (kitty / modern terminals)
	if enable {
		return CSI('?', 2026, 'h')
	}
	return CSI('?', 2026, 'l')
}

// --- Title / Icon ---

func SetTitle(title string) string {
	return OSC(0, title)
}

func SetIconName(name string) string {
	return OSC(1, name)
}

// --- Colors / Attributes ---

func Reset() string          { return CSI(0, 'm') }
func Bold(on bool) string    { return toggle(1, 22, on) }
func Dim(on bool) string     { return toggle(2, 22, on) }
func Italic(on bool) string  { return toggle(3, 23, on) }
func Underline(on bool) string { return toggle(4, 24, on) }
func Blink(on bool) string   { return toggle(5, 25, on) }
func Reverse(on bool) string { return toggle(7, 27, on) }
func Hidden(on bool) string  { return toggle(8, 28, on) }
func Strike(on bool) string  { return toggle(9, 29, on) }

func FgColor(c int) string { // 0-7 or 8-15 bright
	if c < 8 {
		return CSI(30+c, 'm')
	}
	return CSI(90+(c-8), 'm')
}

func BgColor(c int) string {
	if c < 8 {
		return CSI(40+c, 'm')
	}
	return CSI(100+(c-8), 'm')
}

func Fg256(n int) string { return CSI(38, 5, n, 'm') }
func Bg256(n int) string { return CSI(48, 5, n, 'm') }

func FgTrueColor(r, g, b int) string {
	return CSI(38, 2, r, g, b, 'm')
}
func BgTrueColor(r, g, b int) string {
	return CSI(48, 2, r, g, b, 'm')
}

// --- Device queries (the terminal will reply) ---

func QueryCursorPosition() string { return CSI(6, 'n') } // DSR → ESC [ row ; col R
func QueryDeviceAttributes() string { return CSI('c') }  // DA
func QuerySecondaryDA() string     { return CSI('>', 'c') }
func QueryFgColor() string         { return OSC(10, "?") }
func QueryBgColor() string         { return OSC(11, "?") }

// --- helpers ---

func toggle(onCode, offCode int, on bool) string {
	if on {
		return CSI(onCode, 'm')
	}
	return CSI(offCode, 'm')
}
