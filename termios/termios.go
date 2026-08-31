package termios

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// State holds the original terminal state so it can be restored later.
type State struct {
	state *term.State
	fd    int
}

// MakeRaw puts the terminal into raw mode.
// Returns the previous state so the caller can restore it.
func MakeRaw(fd int) (*State, error) {
	old, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("termios: make raw: %w", err)
	}
	return &State{state: old, fd: fd}, nil
}

// MakeCbreak puts the terminal into cbreak mode
// (character-at-a-time, no echo, signals still enabled).
func MakeCbreak(fd int) (*State, error) {
	old, err := term.GetState(fd)
	if err != nil {
		return nil, fmt.Errorf("termios: get state: %w", err)
	}

	// cbreak ≈ raw but keep ISIG
	raw, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("termios: make raw for cbreak: %w", err)
	}
	_ = raw // we already have old

	// Re-enable signals (ISIG)
	attr, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		term.Restore(fd, old)
		return nil, err
	}
	attr.Lflag |= unix.ISIG
	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, attr); err != nil {
		term.Restore(fd, old)
		return nil, err
	}

	return &State{state: old, fd: fd}, nil
}

// Restore puts the terminal back to the saved state.
func (s *State) Restore() error {
	if s == nil || s.state == nil {
		return nil
	}
	return term.Restore(s.fd, s.state)
}

// Size returns the current terminal size (cols, rows).
func Size(fd int) (cols, rows int, err error) {
	w, h, err := term.GetSize(fd)
	if err != nil {
		return 0, 0, fmt.Errorf("termios: get size: %w", err)
	}
	return w, h, nil
}

// IsTerminal reports whether the file descriptor is a terminal.
func IsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}

// OpenTTY opens /dev/tty for reading and writing.
// This is preferred over os.Stdin/os.Stdout because it still works
// when the process has been redirected.
func OpenTTY() (*os.File, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("termios: open /dev/tty: %w", err)
	}
	return f, nil
}

// (platform-specific ioctl constants live in termios_*.go)

