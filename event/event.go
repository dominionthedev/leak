package event

// Event is the interface implemented by all terminal events.
type Event interface {
	event()
}

// Key represents a special key or a printable character.
type Key int

const (
	KeyNone Key = iota
	KeyRune    // printable character is in KeyEvent.Rune
	KeyUp
	KeyDown
	KeyRight
	KeyLeft
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyInsert
	KeyDelete
	KeyBackspace
	KeyTab
	KeyEnter
	KeyEscape
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
)

// Modifier is a bit-field of modifier keys.
type Modifier int

const (
	ModNone  Modifier = 0
	ModShift Modifier = 1 << iota
	ModAlt
	ModCtrl
	ModMeta // Super / Win / Cmd
)

// KeyEvent is produced for keyboard input.
type KeyEvent struct {
	Key Key
	Rune rune
	Mod Modifier
}

func (KeyEvent) event() {}

// MouseButton identifies which button.
type MouseButton int

const (
	ButtonNone MouseButton = iota
	ButtonLeft
	ButtonMiddle
	ButtonRight
	ButtonWheelUp
	ButtonWheelDown
)

// MouseAction is the kind of mouse action.
type MouseAction int

const (
	MousePress MouseAction = iota
	MouseRelease
	MouseDrag
	MouseMotion
)

// MouseEvent is produced when mouse tracking is enabled.
type MouseEvent struct {
	X, Y   int
	Button MouseButton
	Action MouseAction
	Mod    Modifier
}

func (MouseEvent) event() {}

// PasteEvent is produced under bracketed paste mode.
type PasteEvent struct {
	Text string
}

func (PasteEvent) event() {}

// ResizeEvent is produced when the terminal size changes
// (you normally detect this via SIGWINCH yourself and call Size()).
type ResizeEvent struct {
	Cols, Rows int
}

func (ResizeEvent) event() {}

// FocusEvent is produced when focus reporting is enabled.
type FocusEvent struct {
	Gained bool
}

func (FocusEvent) event() {}

// CursorPositionEvent is the reply to a cursor-position query (DSR).
type CursorPositionEvent struct {
	Row, Col int
}

func (CursorPositionEvent) event() {}

// DeviceAttributesEvent is the reply to a DA query.
type DeviceAttributesEvent struct {
	Raw string // keep the raw reply for now; can be parsed later
}

func (DeviceAttributesEvent) event() {}
