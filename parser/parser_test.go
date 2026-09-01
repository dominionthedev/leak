package parser

import (
	"testing"

	"github.com/dominionthedev/leak/event"
)

func TestBracketedPaste(t *testing.T) {
	p := New()

	// start + text + end in one feed
	p.Feed([]byte("\x1b[200~hello world\x1b[201~"))
	ev := p.Next()
	pe, ok := ev.(event.PasteEvent)
	if !ok {
		t.Fatalf("expected PasteEvent, got %T %#v", ev, ev)
	}
	if pe.Text != "hello world" {
		t.Fatalf("paste text = %q, want %q", pe.Text, "hello world")
	}
	if p.Next() != nil {
		t.Fatal("expected no more events")
	}
}

func TestBracketedPasteSplit(t *testing.T) {
	p := New()

	p.Feed([]byte("\x1b[200~hel"))
	if ev := p.Next(); ev != nil {
		t.Fatalf("expected nil mid-paste, got %#v", ev)
	}

	p.Feed([]byte("lo\x1b[201~"))
	ev := p.Next()
	pe, ok := ev.(event.PasteEvent)
	if !ok {
		t.Fatalf("expected PasteEvent, got %T %#v", ev, ev)
	}
	if pe.Text != "hello" {
		t.Fatalf("paste text = %q, want %q", pe.Text, "hello")
	}
}

func TestBracketedPasteMultiline(t *testing.T) {
	p := New()
	p.Feed([]byte("\x1b[200~line1\nline2\r\nline3\x1b[201~"))
	ev := p.Next()
	pe, ok := ev.(event.PasteEvent)
	if !ok {
		t.Fatalf("expected PasteEvent, got %T", ev)
	}
	if pe.Text != "line1\nline2\r\nline3" {
		t.Fatalf("got %q", pe.Text)
	}
}

func TestKeyAndPasteMixed(t *testing.T) {
	p := New()
	p.Feed([]byte("a\x1b[200~pasted\x1b[201~b"))

	ev := p.Next()
	if k, ok := ev.(event.KeyEvent); !ok || k.Rune != 'a' {
		t.Fatalf("want key 'a', got %#v", ev)
	}
	ev = p.Next()
	if pe, ok := ev.(event.PasteEvent); !ok || pe.Text != "pasted" {
		t.Fatalf("want paste 'pasted', got %#v", ev)
	}
	ev = p.Next()
	if k, ok := ev.(event.KeyEvent); !ok || k.Rune != 'b' {
		t.Fatalf("want key 'b', got %#v", ev)
	}
}

func TestArrowKeys(t *testing.T) {
	p := New()
	p.Feed([]byte("\x1b[A\x1b[B\x1b[C\x1b[D"))
	want := []event.Key{event.KeyUp, event.KeyDown, event.KeyRight, event.KeyLeft}
	for _, k := range want {
		ev := p.Next()
		ke, ok := ev.(event.KeyEvent)
		if !ok || ke.Key != k {
			t.Fatalf("want %v, got %#v", k, ev)
		}
	}
}

func TestSGRMouse(t *testing.T) {
	p := New()
	// left press at 5,10
	p.Feed([]byte("\x1b[<0;5;10M"))
	ev := p.Next()
	me, ok := ev.(event.MouseEvent)
	if !ok {
		t.Fatalf("expected MouseEvent, got %T", ev)
	}
	if me.X != 5 || me.Y != 10 || me.Button != event.ButtonLeft || me.Action != event.MousePress {
		t.Fatalf("unexpected mouse event: %#v", me)
	}

	// release
	p.Feed([]byte("\x1b[<0;5;10m"))
	ev = p.Next()
	me, ok = ev.(event.MouseEvent)
	if !ok || me.Action != event.MouseRelease {
		t.Fatalf("expected release, got %#v", ev)
	}
}

func TestCursorPositionReport(t *testing.T) {
	p := New()
	p.Feed([]byte("\x1b[12;34R"))
	ev := p.Next()
	cp, ok := ev.(event.CursorPositionEvent)
	if !ok || cp.Row != 12 || cp.Col != 34 {
		t.Fatalf("got %#v", ev)
	}
}

func TestDeviceAttributesPrimary(t *testing.T) {
	p := New()
	p.Feed([]byte("\x1b[?62;1;6c"))
	ev := p.Next()
	da, ok := ev.(event.DeviceAttributesEvent)
	if !ok {
		t.Fatalf("expected DeviceAttributesEvent, got %T %#v", ev, ev)
	}
	if da.Secondary {
		t.Fatal("expected Secondary=false for a primary DA reply")
	}
	want := []int{62, 1, 6}
	if len(da.Params) != len(want) {
		t.Fatalf("Params = %v, want %v", da.Params, want)
	}
	for i := range want {
		if da.Params[i] != want[i] {
			t.Fatalf("Params = %v, want %v", da.Params, want)
		}
	}
}

func TestDeviceAttributesSecondary(t *testing.T) {
	p := New()
	p.Feed([]byte("\x1b[>0;136;0c"))
	ev := p.Next()
	da, ok := ev.(event.DeviceAttributesEvent)
	if !ok {
		t.Fatalf("expected DeviceAttributesEvent, got %T %#v", ev, ev)
	}
	if !da.Secondary {
		t.Fatal("expected Secondary=true for a secondary DA reply")
	}
	want := []int{0, 136, 0}
	if len(da.Params) != len(want) {
		t.Fatalf("Params = %v, want %v", da.Params, want)
	}
	for i := range want {
		if da.Params[i] != want[i] {
			t.Fatalf("Params = %v, want %v", da.Params, want)
		}
	}
}

func TestFocus(t *testing.T) {
	p := New()
	p.Feed([]byte("\x1b[I\x1b[O"))
	ev := p.Next()
	if f, ok := ev.(event.FocusEvent); !ok || !f.Gained {
		t.Fatalf("want focus gained, got %#v", ev)
	}
	ev = p.Next()
	if f, ok := ev.(event.FocusEvent); !ok || f.Gained {
		t.Fatalf("want focus lost, got %#v", ev)
	}
}

func TestCtrlKeysAreDistinguishable(t *testing.T) {
	// Regression test: Ctrl-C and Ctrl-D used to both produce
	// KeyEvent{Rune: 0, Mod: ModCtrl} — identical events, no way to
	// tell them apart. Every Ctrl+letter now maps consistently to the
	// base rune with ModCtrl set.
	cases := []struct {
		input byte
		want  rune
	}{
		{0x01, 'a'},  // Ctrl-A
		{0x03, 'c'},  // Ctrl-C
		{0x04, 'd'},  // Ctrl-D
		{0x1a, 'z'},  // Ctrl-Z
		{0x1c, '\\'}, // Ctrl-\
		{0x1d, ']'},  // Ctrl-]
		{0x1e, '^'},  // Ctrl-^
		{0x1f, '_'},  // Ctrl-_
	}
	for _, c := range cases {
		p := New()
		p.Feed([]byte{c.input})
		ev := p.Next()
		ke, ok := ev.(event.KeyEvent)
		if !ok {
			t.Fatalf("input 0x%02x: expected KeyEvent, got %T %#v", c.input, ev, ev)
		}
		if ke.Mod&event.ModCtrl == 0 {
			t.Fatalf("input 0x%02x: expected ModCtrl set, got %#v", c.input, ke)
		}
		if ke.Rune != c.want {
			t.Fatalf("input 0x%02x: rune = %q, want %q", c.input, ke.Rune, c.want)
		}
	}

	// Ctrl-C and Ctrl-D specifically must not collapse to the same event.
	pc := New()
	pc.Feed([]byte{0x03})
	pd := New()
	pd.Feed([]byte{0x04})
	evC := pc.Next().(event.KeyEvent)
	evD := pd.Next().(event.KeyEvent)
	if evC == evD {
		t.Fatalf("Ctrl-C and Ctrl-D produced identical events: %#v", evC)
	}
}

func TestCtrlSpaceFromNUL(t *testing.T) {
	p := New()
	p.Feed([]byte{0x00})
	ev := p.Next()
	ke, ok := ev.(event.KeyEvent)
	if !ok || ke.Rune != ' ' || ke.Mod&event.ModCtrl == 0 {
		t.Fatalf("expected Ctrl-Space from NUL, got %#v", ev)
	}
}

func TestNamedControlKeysStillWork(t *testing.T) {
	cases := []struct {
		input byte
		want  event.Key
	}{
		{0x08, event.KeyBackspace},
		{0x7f, event.KeyBackspace},
		{0x09, event.KeyTab},
		{0x0a, event.KeyEnter},
		{0x0d, event.KeyEnter},
	}
	for _, c := range cases {
		p := New()
		p.Feed([]byte{c.input})
		ev := p.Next()
		ke, ok := ev.(event.KeyEvent)
		if !ok || ke.Key != c.want {
			t.Fatalf("input 0x%02x: got %#v, want Key=%v", c.input, ev, c.want)
		}
	}
}

func TestAtoiOverflowGuard(t *testing.T) {
	// A pathologically long digit run shouldn't wrap around to a
	// negative or nonsensical value via silent int overflow.
	huge := make([]byte, 40)
	for i := range huge {
		huge[i] = '9'
	}
	n := atoi(huge)
	if n < 0 {
		t.Fatalf("atoi overflowed to negative: %d", n)
	}
}
