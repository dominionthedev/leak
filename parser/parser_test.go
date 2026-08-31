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
