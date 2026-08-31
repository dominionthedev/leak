package main

import (
	"fmt"
	"os"

	"github.com/dominionthedev/leak"
	"github.com/dominionthedev/leak/event"
)

func main() {
	t, err := leak.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(1)
	}
	defer t.Close()

	if err := t.MakeRaw(); err != nil {
		fmt.Fprintf(os.Stderr, "raw: %v\n", err)
		os.Exit(1)
	}

	_ = t.EnterAltScreen()
	_ = t.HideCursor()
	_ = t.EnableMouse()
	_ = t.EnableBracketedPaste()
	_ = t.EnableFocusReporting()
	_ = t.Clear()
	_ = t.SetTitle("leak demo")

	_ = t.MoveTo(2, 4)
	_ = t.Print("leak – terminal interaction & control library\r\n")
	_ = t.MoveTo(4, 4)
	_ = t.Print("Type keys, move mouse, paste text, resize window. Press q to quit.\r\n")

	for {
		ev, err := t.ReadEvent()
		if err != nil {
			break
		}

		switch e := ev.(type) {
		case event.KeyEvent:
			if e.Key == event.KeyRune && (e.Rune == 'q' || e.Rune == 'Q') {
				return
			}
			if e.Mod&event.ModCtrl != 0 && e.Rune == 0 {
				return
			}
			_ = t.MoveTo(6, 4)
			_ = t.Printf("Key   : key=%v rune=%q mod=%v     \r\n", e.Key, e.Rune, e.Mod)

		case event.MouseEvent:
			_ = t.MoveTo(7, 4)
			_ = t.Printf("Mouse : (%d,%d) button=%v action=%v mod=%v     \r\n",
				e.X, e.Y, e.Button, e.Action, e.Mod)

		case event.PasteEvent:
			_ = t.MoveTo(8, 4)
			_ = t.Printf("Paste : %q     \r\n", e.Text)

		case event.FocusEvent:
			_ = t.MoveTo(9, 4)
			if e.Gained {
				_ = t.Print("Focus : gained     \r\n")
			} else {
				_ = t.Print("Focus : lost       \r\n")
			}

		case event.ResizeEvent:
			_ = t.MoveTo(10, 4)
			_ = t.Printf("Resize: %d×%d     \r\n", e.Cols, e.Rows)

		case event.CursorPositionEvent:
			_ = t.MoveTo(11, 4)
			_ = t.Printf("Cursor: row=%d col=%d     \r\n", e.Row, e.Col)
		}
	}
}
