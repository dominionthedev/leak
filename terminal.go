package leak

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/dominionthedev/leak/ansi"
	"github.com/dominionthedev/leak/event"
	"github.com/dominionthedev/leak/parser"
	"github.com/dominionthedev/leak/termios"
)

// Terminal is the main handle for interacting with and controlling
// the terminal emulator the process is attached to.
type Terminal struct {
	f         *os.File // usually /dev/tty
	fd        int
	orig      *termios.State
	parser    *parser.Parser
	mu        sync.Mutex
	raw       bool
	altScreen bool
	mouse     bool
	paste     bool
	focus     bool

	// resize handling
	winchCh   chan os.Signal
	resizeCh  chan event.ResizeEvent
	stopWinch chan struct{}
	winchOnce sync.Once

	// input reading — a single background goroutine owns the only
	// blocking Read on f and feeds raw chunks through readCh, so
	// ReadEvent/TryReadEvent can select across input and resize instead
	// of sequentially blocking on Read. See readLoop for why.
	readCh    chan []byte
	readErrMu sync.Mutex
	readErr   error
}

// Open opens the controlling terminal (/dev/tty) and prepares it for
// interaction. The terminal is left in its original mode until MakeRaw
// or MakeCbreak is called.
func Open() (*Terminal, error) {
	f, err := termios.OpenTTY()
	if err != nil {
		// fallback to stdin if /dev/tty is unavailable
		f = os.Stdin
		if !termios.IsTerminal(int(f.Fd())) {
			return nil, fmt.Errorf("leak: neither /dev/tty nor stdin is a terminal")
		}
	}

	t := &Terminal{
		f:         f,
		fd:        int(f.Fd()),
		parser:    parser.New(),
		resizeCh:  make(chan event.ResizeEvent, 4),
		stopWinch: make(chan struct{}),
		readCh:    make(chan []byte, 4),
	}
	go t.readLoop()
	return t, nil
}

// Close restores the original terminal state, stops resize watching,
// and closes the file if we opened /dev/tty ourselves.
// TODO: update Terminal to have a State type of map for all actions that has been performed
// and modes that has been enabled(or something like that) so Close() can know what to do
func (t *Terminal) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopWinchLocked()

	var first error
	if t.orig != nil {
		if err := t.orig.Restore(); err != nil && first == nil {
			first = err
		}
		t.orig = nil
		t.raw = false
	}

	// best-effort leave modes we may have enabled
	t.writeLocked(ansi.MouseTracking(false))
	t.writeLocked(ansi.BracketedPaste(false))
	t.writeLocked(ansi.FocusReporting(false))
	t.writeLocked(ansi.AltScreen(false))
	t.writeLocked(ansi.CursorShow())
	t.writeLocked(ansi.Reset())

	if t.f != os.Stdin && t.f != os.Stdout {
		if err := t.f.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Fd returns the underlying file descriptor.
func (t *Terminal) Fd() int { return t.fd }

// --- 1. termios control ---

// MakeRaw switches the terminal to raw mode and starts watching for resizes.
func (t *Terminal) MakeRaw() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.raw {
		return nil
	}
	st, err := termios.MakeRaw(t.fd)
	if err != nil {
		return err
	}
	t.orig = st
	t.raw = true
	t.startWinchLocked()
	return nil
}

// MakeCbreak switches the terminal to cbreak mode and starts watching for resizes.
func (t *Terminal) MakeCbreak() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.raw {
		return nil
	}
	st, err := termios.MakeCbreak(t.fd)
	if err != nil {
		return err
	}
	t.orig = st
	t.raw = true
	t.startWinchLocked()
	return nil
}

// Restore puts the terminal back to the state it had when Open was called.
func (t *Terminal) Restore() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.orig == nil {
		return nil
	}
	err := t.orig.Restore()
	t.orig = nil
	t.raw = false
	return err
}

// Size returns the current size of the terminal (cols, rows).
func (t *Terminal) Size() (cols, rows int, err error) {
	return termios.Size(t.fd)
}

// --- resize (SIGWINCH) ---

func (t *Terminal) startWinchLocked() {
	t.winchOnce.Do(func() {
		t.winchCh = make(chan os.Signal, 1)
		signal.Notify(t.winchCh, syscall.SIGWINCH)
		go t.winchLoop()
	})
}

func (t *Terminal) stopWinchLocked() {
	select {
	case <-t.stopWinch:
		// already stopped
	default:
		close(t.stopWinch)
		if t.winchCh != nil {
			signal.Stop(t.winchCh)
		}
	}
}

func (t *Terminal) winchLoop() {
	for {
		select {
		case <-t.stopWinch:
			return
		case <-t.winchCh:
			cols, rows, err := termios.Size(t.fd)
			if err != nil {
				continue
			}
			ev := event.ResizeEvent{Cols: cols, Rows: rows}
			// non-blocking send; drop if full so we never stall the signal handler path
			select {
			case t.resizeCh <- ev:
			default:
			}
		}
	}
}

// --- 2. give instructions ---

// Write sends raw bytes to the terminal.
func (t *Terminal) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.f.Write(p)
}

func (t *Terminal) writeLocked(s string) (int, error) {
	return t.f.Write([]byte(s))
}

// WriteString is a convenience wrapper.
func (t *Terminal) WriteString(s string) (int, error) {
	return t.Write([]byte(s))
}

// Print formats and writes to the terminal.
func (t *Terminal) Print(a ...any) error {
	_, err := t.WriteString(fmt.Sprint(a...))
	return err
}

// Printf formats and writes to the terminal.
func (t *Terminal) Printf(format string, a ...any) error {
	_, err := t.WriteString(fmt.Sprintf(format, a...))
	return err
}

// CSI sends a CSI sequence.
func (t *Terminal) CSI(params ...any) error {
	_, err := t.WriteString(ansi.CSI(params...))
	return err
}

// OSC sends an OSC sequence.
func (t *Terminal) OSC(cmd int, args ...string) error {
	_, err := t.WriteString(ansi.OSC(cmd, args...))
	return err
}

// Clear erases the whole screen and moves the cursor home.
func (t *Terminal) Clear() error {
	_, err := t.WriteString(ansi.ClearScreen())
	return err
}

// MoveTo moves the cursor to 1-based (row, col).
func (t *Terminal) MoveTo(row, col int) error {
	_, err := t.WriteString(ansi.CursorPosition(row, col))
	return err
}

// SetTitle sets the terminal window title.
func (t *Terminal) SetTitle(title string) error {
	_, err := t.WriteString(ansi.SetTitle(title))
	return err
}

// HideCursor / ShowCursor
func (t *Terminal) HideCursor() error {
	_, err := t.WriteString(ansi.CursorHide())
	return err
}
func (t *Terminal) ShowCursor() error {
	_, err := t.WriteString(ansi.CursorShow())
	return err
}

// --- 3. understand the terminal ---

// readLoop owns the only blocking Read on f, for the Terminal's lifetime.
// It exists because a blocked Read is not reliably interrupted by
// SIGWINCH: Go's runtime routes a tty's Read through the integrated
// poller (epoll/kqueue), and a goroutine parked there only wakes when the
// fd becomes readable — a signal alone doesn't do that. Without this, a
// resize delivered while nobody is typing would sit in resizeCh unnoticed
// until the next keypress unblocked the read. Running the blocking read
// in its own goroutine and feeding chunks through readCh lets
// ReadEvent/TryReadEvent select across resize and input instead.
func (t *Terminal) readLoop() {
	buf := make([]byte, 256)
	for {
		n, err := t.f.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			t.readCh <- chunk
		}
		if err != nil {
			t.readErrMu.Lock()
			t.readErr = err
			t.readErrMu.Unlock()
			close(t.readCh)
			return
		}
	}
}

func (t *Terminal) readErrLocked() error {
	t.readErrMu.Lock()
	defer t.readErrMu.Unlock()
	if t.readErr != nil {
		return t.readErr
	}
	return io.EOF
}

// ReadEvent blocks until a complete event is available.
// It also surfaces ResizeEvent when the terminal is resized (SIGWINCH),
// even if nothing has been typed since the resize happened.
func (t *Terminal) ReadEvent() (event.Event, error) {
	for {
		if ev := t.parser.Next(); ev != nil {
			return ev, nil
		}
		select {
		case ev := <-t.resizeCh:
			return ev, nil
		case data, ok := <-t.readCh:
			if !ok {
				return nil, t.readErrLocked()
			}
			t.parser.Feed(data)
		}
	}
}

// TryReadEvent returns an event if one is already available,
// otherwise (nil, false, nil). Also checks pending resizes, and drains
// any input the background reader has already picked up without blocking.
func (t *Terminal) TryReadEvent() (event.Event, bool, error) {
	if ev := t.parser.Next(); ev != nil {
		return ev, true, nil
	}
	select {
	case ev := <-t.resizeCh:
		return ev, true, nil
	case data, ok := <-t.readCh:
		if !ok {
			return nil, false, t.readErrLocked()
		}
		t.parser.Feed(data)
		if ev := t.parser.Next(); ev != nil {
			return ev, true, nil
		}
		return nil, false, nil
	default:
		return nil, false, nil
	}
}

// QueryCursor asks the terminal for the current cursor position
// and waits for the reply.
func (t *Terminal) QueryCursor() (row, col int, err error) {
	if _, err = t.WriteString(ansi.QueryCursorPosition()); err != nil {
		return 0, 0, err
	}
	for {
		ev, err := t.ReadEvent()
		if err != nil {
			return 0, 0, err
		}
		if cp, ok := ev.(event.CursorPositionEvent); ok {
			return cp.Row, cp.Col, nil
		}
		// other events (including resize) are discarded while waiting
	}
}

// --- 4. mode management ---

// EnterAltScreen switches to the alternate screen buffer.
func (t *Terminal) EnterAltScreen() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.altScreen {
		return nil
	}
	if _, err := t.writeLocked(ansi.AltScreen(true)); err != nil {
		return err
	}
	t.altScreen = true
	return nil
}

// ExitAltScreen returns to the normal screen buffer.
func (t *Terminal) ExitAltScreen() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.altScreen {
		return nil
	}
	if _, err := t.writeLocked(ansi.AltScreen(false)); err != nil {
		return err
	}
	t.altScreen = false
	return nil
}

// EnableMouse turns on SGR mouse tracking.
func (t *Terminal) EnableMouse() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.mouse {
		return nil
	}
	if _, err := t.writeLocked(ansi.MouseTracking(true)); err != nil {
		return err
	}
	t.mouse = true
	return nil
}

// DisableMouse turns mouse tracking off.
func (t *Terminal) DisableMouse() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.mouse {
		return nil
	}
	if _, err := t.writeLocked(ansi.MouseTracking(false)); err != nil {
		return err
	}
	t.mouse = false
	return nil
}

// EnableBracketedPaste enables bracketed paste mode.
func (t *Terminal) EnableBracketedPaste() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.paste {
		return nil
	}
	if _, err := t.writeLocked(ansi.BracketedPaste(true)); err != nil {
		return err
	}
	t.paste = true
	return nil
}

// DisableBracketedPaste disables bracketed paste mode.
func (t *Terminal) DisableBracketedPaste() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.paste {
		return nil
	}
	if _, err := t.writeLocked(ansi.BracketedPaste(false)); err != nil {
		return err
	}
	t.paste = false
	return nil
}

// EnableFocusReporting enables focus in/out events.
func (t *Terminal) EnableFocusReporting() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.focus {
		return nil
	}
	if _, err := t.writeLocked(ansi.FocusReporting(true)); err != nil {
		return err
	}
	t.focus = true
	return nil
}

// DisableFocusReporting disables focus reporting.
func (t *Terminal) DisableFocusReporting() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.focus {
		return nil
	}
	if _, err := t.writeLocked(ansi.FocusReporting(false)); err != nil {
		return err
	}
	t.focus = false
	return nil
}
