package leak

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/dominionthedev/leak/ansi"
	"github.com/dominionthedev/leak/event"
	"github.com/dominionthedev/leak/parser"
)

// newTestTerminal builds a Terminal around an already-open file without
// going through Open(), so tests can use an os.Pipe() instead of needing a
// real controlling tty.
func newTestTerminal(f *os.File) *Terminal {
	t := &Terminal{
		f:         f,
		fd:        int(f.Fd()),
		parser:    parser.New(),
		resizeCh:  make(chan event.ResizeEvent, 4),
		stopWinch: make(chan struct{}),
		readCh:    make(chan []byte, 4),
	}
	go t.readLoop()
	return t
}

type readResult struct {
	ev  event.Event
	err error
}

// TestReadEventDeliversResizeWithoutInput is a regression test for the bug
// where ReadEvent relied on SIGWINCH interrupting a blocked f.Read call.
// That doesn't happen under Go's runtime-integrated poller, so a resize
// delivered while nobody is typing used to sit in resizeCh unnoticed until
// the next keypress. Here nothing is ever written to the pipe — if this
// regresses, the test times out instead of failing cleanly.
func TestReadEventDeliversResizeWithoutInput(t *testing.T) {
	r, w, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("os.Pipe: %v", perr)
	}
	defer w.Close()
	defer r.Close()
	term := newTestTerminal(r)

	resultCh := make(chan readResult, 1)
	go func() {
		ev, err := term.ReadEvent()
		resultCh <- readResult{ev, err}
	}()

	// Give the reader goroutine time to actually be parked in Read
	// before the resize arrives, so this genuinely exercises the
	// "blocked read, then resize" ordering.
	time.Sleep(20 * time.Millisecond)
	term.resizeCh <- event.ResizeEvent{Cols: 80, Rows: 24}

	select {
	case res := <-resultCh:
		if res.err != nil {
			t.Fatalf("ReadEvent error: %v", res.err)
		}
		re, ok := res.ev.(event.ResizeEvent)
		if !ok || re.Cols != 80 || re.Rows != 24 {
			t.Fatalf("got %#v", res.ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReadEvent did not return the resize event promptly — regression of the blocked-read resize bug")
	}
}

func TestReadEventDeliversKeyInput(t *testing.T) {
	r, w, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("os.Pipe: %v", perr)
	}
	defer w.Close()
	defer r.Close()
	term := newTestTerminal(r)

	go func() {
		w.Write([]byte("a"))
	}()

	ev, err := term.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent error: %v", err)
	}
	ke, ok := ev.(event.KeyEvent)
	if !ok || ke.Rune != 'a' {
		t.Fatalf("got %#v", ev)
	}
}

func TestReadEventReturnsErrorAfterFileCloses(t *testing.T) {
	r, w, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("os.Pipe: %v", perr)
	}
	defer w.Close()
	term := newTestTerminal(r)
	r.Close()

	resultCh := make(chan readResult, 1)
	go func() {
		ev, err := term.ReadEvent()
		resultCh <- readResult{ev, err}
	}()

	select {
	case res := <-resultCh:
		if res.err == nil {
			t.Fatal("expected an error after the underlying file closed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReadEvent never returned after the file closed")
	}
}

func TestSyncUpdateWritesBeginAndEnd(t *testing.T) {
	r, w, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("os.Pipe: %v", perr)
	}
	defer r.Close()
	term := &Terminal{f: w, fd: int(w.Fd()), parser: parser.New()}

	readAll := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 256)
		n, _ := r.Read(buf)
		readAll <- buf[:n]
	}()

	if err := term.SyncUpdate(func() error {
		_, err := term.WriteString("payload")
		return err
	}); err != nil {
		t.Fatalf("SyncUpdate: %v", err)
	}
	w.Close()

	got := <-readAll
	want := ansi.SynchronizedOutput(true) + "payload" + ansi.SynchronizedOutput(false)
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSyncUpdateEndsEvenOnError(t *testing.T) {
	r, w, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("os.Pipe: %v", perr)
	}
	defer r.Close()
	term := &Terminal{f: w, fd: int(w.Fd()), parser: parser.New()}

	readAll := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 256)
		n, _ := r.Read(buf)
		readAll <- buf[:n]
	}()

	wantErr := errors.New("frame failed")
	err := term.SyncUpdate(func() error {
		return wantErr
	})
	w.Close()

	if err != wantErr {
		t.Fatalf("SyncUpdate error = %v, want %v", err, wantErr)
	}
	got := <-readAll
	want := ansi.SynchronizedOutput(true) + ansi.SynchronizedOutput(false)
	if string(got) != want {
		t.Fatalf("got %q, want %q — EndSyncUpdate should still have run", got, want)
	}
}

func TestTryReadEventDrainsAsyncInput(t *testing.T) {
	r, w, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("os.Pipe: %v", perr)
	}
	defer w.Close()
	defer r.Close()
	term := newTestTerminal(r)

	if ev, ok, err := term.TryReadEvent(); ev != nil || ok || err != nil {
		t.Fatalf("expected no event yet, got ev=%#v ok=%v err=%v", ev, ok, err)
	}

	w.Write([]byte("q"))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ev, ok, err := term.TryReadEvent()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			ke, isKey := ev.(event.KeyEvent)
			if !isKey || ke.Rune != 'q' {
				t.Fatalf("got %#v", ev)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("TryReadEvent never observed the written byte")
}
