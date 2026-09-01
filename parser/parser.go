package parser

import (
	"bytes"
	"unicode/utf8"

	"github.com/dominionthedev/leak/event"
)

// Parser turns a stream of bytes coming from the terminal into Events.
// It handles keys, mouse (SGR), bracketed paste, focus, and DSR replies.
type Parser struct {
	buf   []byte
	paste bool   // inside bracketed paste (after CSI 200 ~)
	pbuf  []byte // accumulated paste text
}

func New() *Parser {
	return &Parser{}
}

// Feed accepts more bytes from the terminal.
// Call Next() afterwards to pull complete events.
func (p *Parser) Feed(data []byte) {
	p.buf = append(p.buf, data...)
}

// Next returns the next complete event, or nil if more data is needed.
func (p *Parser) Next() event.Event {
	for len(p.buf) > 0 {
		// While pasting, only look for the end marker; everything else is text.
		if p.paste {
			ev, n := p.consumePaste()
			if n > 0 {
				p.buf = p.buf[n:]
			}
			if ev != nil {
				return ev
			}
			if n == 0 {
				return nil // need more data
			}
			continue
		}

		// ESC sequence?
		if p.buf[0] == 0x1b {
			ev, n := p.parseESC()
			if n == 0 {
				return nil // incomplete
			}
			p.buf = p.buf[n:]
			if ev != nil {
				return ev
			}
			continue
		}

		// Control characters with dedicated named keys.
		switch p.buf[0] {
		case 0x08, 0x7f: // BS / DEL
			p.buf = p.buf[1:]
			return event.KeyEvent{Key: event.KeyBackspace}
		case 0x09:
			p.buf = p.buf[1:]
			return event.KeyEvent{Key: event.KeyTab}
		case 0x0a, 0x0d: // LF / CR
			p.buf = p.buf[1:]
			return event.KeyEvent{Key: event.KeyEnter}
		}

		// Every other C0 control byte (0x00-0x1f) is Ctrl+<letter> —
		// represented the same way as any other modified key, the base
		// rune with ModCtrl set, instead of a special zero-value case
		// per key. This used to special-case only Ctrl-C and Ctrl-D as
		// Rune:0, which made them indistinguishable from each other and
		// inconsistent with every other Ctrl+letter combo.
		if c := p.buf[0]; c <= 0x1f {
			p.buf = p.buf[1:]
			return event.KeyEvent{Key: event.KeyRune, Rune: ctrlRune(c), Mod: event.ModCtrl}
		}

		// UTF-8 rune
		r, size := utf8.DecodeRune(p.buf)
		if r == utf8.RuneError && size == 1 {
			if !utf8.FullRune(p.buf) {
				return nil
			}
			p.buf = p.buf[1:]
			continue
		}
		p.buf = p.buf[size:]
		return event.KeyEvent{Key: event.KeyRune, Rune: r}
	}
	return nil
}

// consumePaste scans for CSI 201 ~ while accumulating text.
// Returns (event, bytesConsumed). n==0 means incomplete.
func (p *Parser) consumePaste() (event.Event, int) {
	// Look for ESC [ 201 ~
	for i := 0; i < len(p.buf); i++ {
		if p.buf[i] != 0x1b {
			continue
		}
		// Possible end marker starting at i
		rest := p.buf[i:]
		if len(rest) < 6 {
			// Copy everything before the partial ESC into paste buffer,
			// leave the partial sequence in p.buf.
			if i > 0 {
				p.pbuf = append(p.pbuf, p.buf[:i]...)
			}
			return nil, i // consumed prefix; need more for the sequence
		}
		// ESC [ 2 0 1 ~
		if rest[1] == '[' && rest[2] == '2' && rest[3] == '0' && rest[4] == '1' && rest[5] == '~' {
			p.pbuf = append(p.pbuf, p.buf[:i]...)
			text := string(p.pbuf)
			p.pbuf = p.pbuf[:0]
			p.paste = false
			return event.PasteEvent{Text: text}, i + 6
		}
	}

	// No end marker in sight — consume all as paste text.
	p.pbuf = append(p.pbuf, p.buf...)
	return nil, len(p.buf)
}

func (p *Parser) parseESC() (event.Event, int) {
	if len(p.buf) < 2 {
		return nil, 0
	}

	switch p.buf[1] {
	case '[': // CSI
		return p.parseCSI()
	case 'O': // SS3 – often application cursor keys
		if len(p.buf) < 3 {
			return nil, 0
		}
		switch p.buf[2] {
		case 'A':
			return event.KeyEvent{Key: event.KeyUp}, 3
		case 'B':
			return event.KeyEvent{Key: event.KeyDown}, 3
		case 'C':
			return event.KeyEvent{Key: event.KeyRight}, 3
		case 'D':
			return event.KeyEvent{Key: event.KeyLeft}, 3
		case 'H':
			return event.KeyEvent{Key: event.KeyHome}, 3
		case 'F':
			return event.KeyEvent{Key: event.KeyEnd}, 3
		}
		return nil, 3 // consume unknown
	case ']': // OSC – ignore most replies
		return p.parseOSC()
	default:
		// plain ESC + key → Alt+key
		if len(p.buf) >= 2 {
			r, size := utf8.DecodeRune(p.buf[1:])
			if r != utf8.RuneError {
				return event.KeyEvent{Key: event.KeyRune, Rune: r, Mod: event.ModAlt}, 1 + size
			}
		}
		return event.KeyEvent{Key: event.KeyEscape}, 1
	}
}

func (p *Parser) parseCSI() (event.Event, int) {
	i := 2
	for i < len(p.buf) {
		b := p.buf[i]
		if b >= 0x40 && b <= 0x7e { // final byte
			seq := p.buf[2:i]
			final := b
			n := i + 1

			switch final {
			case 'A':
				return event.KeyEvent{Key: event.KeyUp, Mod: parseMod(seq)}, n
			case 'B':
				return event.KeyEvent{Key: event.KeyDown, Mod: parseMod(seq)}, n
			case 'C':
				return event.KeyEvent{Key: event.KeyRight, Mod: parseMod(seq)}, n
			case 'D':
				return event.KeyEvent{Key: event.KeyLeft, Mod: parseMod(seq)}, n
			case 'H':
				return event.KeyEvent{Key: event.KeyHome, Mod: parseMod(seq)}, n
			case 'F':
				return event.KeyEvent{Key: event.KeyEnd, Mod: parseMod(seq)}, n
			case '~':
				code := atoi(bytes.Split(seq, []byte{';'})[0])
				switch code {
				case 200: // bracketed paste start
					p.paste = true
					p.pbuf = p.pbuf[:0]
					return nil, n
				case 201: // paste end without start — ignore
					return nil, n
				default:
					return parseTildeKey(seq), n
				}
			case 'R': // cursor position report
				row, col := parseTwoNums(seq)
				return event.CursorPositionEvent{Row: row, Col: col}, n
			case 'c': // DA reply
				return parseDA(seq), n
			case 'M', 'm': // SGR mouse
				return parseSGRMouse(seq, final == 'm'), n
			case 'I': // focus gained
				return event.FocusEvent{Gained: true}, n
			case 'O': // focus lost
				return event.FocusEvent{Gained: false}, n
			}
			return nil, n // unknown CSI – consume
		}
		i++
	}
	return nil, 0 // incomplete
}

func (p *Parser) parseOSC() (event.Event, int) {
	for i := 2; i < len(p.buf); i++ {
		if p.buf[i] == 0x07 { // BEL
			return nil, i + 1
		}
		if p.buf[i] == 0x1b && i+1 < len(p.buf) && p.buf[i+1] == '\\' {
			return nil, i + 2
		}
	}
	return nil, 0
}

// --- helpers ---

func parseMod(seq []byte) event.Modifier {
	parts := bytes.Split(seq, []byte{';'})
	if len(parts) < 2 {
		return event.ModNone
	}
	mod := atoi(parts[len(parts)-1])
	var m event.Modifier
	if mod >= 2 {
		mod--
		if mod&1 != 0 {
			m |= event.ModShift
		}
		if mod&2 != 0 {
			m |= event.ModAlt
		}
		if mod&4 != 0 {
			m |= event.ModCtrl
		}
		if mod&8 != 0 {
			m |= event.ModMeta
		}
	}
	return m
}

func parseTildeKey(seq []byte) event.Event {
	parts := bytes.Split(seq, []byte{';'})
	code := atoi(parts[0])
	mod := event.ModNone
	if len(parts) > 1 {
		mod = parseMod(seq)
	}
	var k event.Key
	switch code {
	case 1, 7:
		k = event.KeyHome
	case 2:
		k = event.KeyInsert
	case 3:
		k = event.KeyDelete
	case 4, 8:
		k = event.KeyEnd
	case 5:
		k = event.KeyPageUp
	case 6:
		k = event.KeyPageDown
	case 11:
		k = event.KeyF1
	case 12:
		k = event.KeyF2
	case 13:
		k = event.KeyF3
	case 14:
		k = event.KeyF4
	case 15:
		k = event.KeyF5
	case 17:
		k = event.KeyF6
	case 18:
		k = event.KeyF7
	case 19:
		k = event.KeyF8
	case 20:
		k = event.KeyF9
	case 21:
		k = event.KeyF10
	case 23:
		k = event.KeyF11
	case 24:
		k = event.KeyF12
	default:
		return event.KeyEvent{Key: event.KeyEscape, Mod: mod}
	}
	return event.KeyEvent{Key: k, Mod: mod}
}

func parseSGRMouse(seq []byte, release bool) event.Event {
	if len(seq) == 0 || seq[0] != '<' {
		return nil
	}
	parts := bytes.Split(seq[1:], []byte{';'})
	if len(parts) < 3 {
		return nil
	}
	btn := atoi(parts[0])
	x := atoi(parts[1])
	y := atoi(parts[2])

	var button event.MouseButton
	var action event.MouseAction
	var mod event.Modifier

	if btn&4 != 0 {
		mod |= event.ModShift
	}
	if btn&8 != 0 {
		mod |= event.ModAlt
	}
	if btn&16 != 0 {
		mod |= event.ModCtrl
	}

	b := btn & 3
	switch {
	case btn&64 != 0:
		if b == 0 {
			button = event.ButtonWheelUp
		} else {
			button = event.ButtonWheelDown
		}
		action = event.MousePress
	default:
		switch b {
		case 0:
			button = event.ButtonLeft
		case 1:
			button = event.ButtonMiddle
		case 2:
			button = event.ButtonRight
		}
		if release {
			action = event.MouseRelease
		} else if btn&32 != 0 {
			action = event.MouseDrag
		} else {
			action = event.MousePress
		}
	}

	return event.MouseEvent{
		X: x, Y: y,
		Button: button,
		Action: action,
		Mod:    mod,
	}
}

func parseTwoNums(seq []byte) (int, int) {
	parts := bytes.Split(seq, []byte{';'})
	if len(parts) < 2 {
		return 1, 1
	}
	return atoi(parts[0]), atoi(parts[1])
}

// parseDA parses a DA reply body. Primary DA replies look like
// "?62;1;6" (leading '?'); secondary DA replies look like ">0;136;0"
// (leading '>'). Either marker is stripped before splitting the
// remaining ;-separated numbers into Params.
func parseDA(seq []byte) event.DeviceAttributesEvent {
	ev := event.DeviceAttributesEvent{Raw: string(seq)}
	body := seq
	if len(body) > 0 && (body[0] == '?' || body[0] == '>') {
		ev.Secondary = body[0] == '>'
		body = body[1:]
	}
	for _, part := range bytes.Split(body, []byte{';'}) {
		if len(part) == 0 {
			continue
		}
		ev.Params = append(ev.Params, atoi(part))
	}
	return ev
}

func atoi(b []byte) int {
	n := 0
	for _, c := range b {
		if c < '0' || c > '9' {
			break
		}
		// Guard against a pathologically long digit run (a malicious or
		// corrupted terminal reply) silently overflowing int via
		// wraparound. Any real terminal reply value is small; once we're
		// well past anything meaningful, stop accumulating.
		if n > 1_000_000 {
			continue
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ctrlRune maps a C0 control byte (0x00-0x1f) to the base rune it
// represents when combined with ModCtrl, mirroring the standard
// Ctrl+<key> = <key>&0x1F convention (and its reverse) that terminals use.
func ctrlRune(c byte) rune {
	switch {
	case c == 0x00:
		return ' ' // Ctrl-Space / Ctrl-@
	case c >= 0x01 && c <= 0x1a:
		return rune(c) + 'a' - 1 // 0x01→'a' ... 0x1a→'z'
	case c == 0x1c:
		return '\\'
	case c == 0x1d:
		return ']'
	case c == 0x1e:
		return '^'
	case c == 0x1f:
		return '_'
	}
	return rune(c)
}
