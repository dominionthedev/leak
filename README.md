# leak

**leak** is a Go library for interacting with and controlling the terminal.

## What it does

1. **termios control** — raw / cbreak, restore, size
2. **Instruction emission** — CSI, OSC and other sequences that tell the emulator what to do
3. **Understanding the terminal** — parse the replies and input the emulator sends back
4. **Mode management** — alternate screen, mouse tracking, bracketed paste, focus reporting, etc.

## Install

```bash
go get github.com/dominionthedev/leak@latest
```

## Quick example

```go
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
        panic(err)
    }
    defer t.Close()

    _ = t.MakeRaw()
    _ = t.EnterAltScreen()
    _ = t.HideCursor()
    _ = t.Clear()
    _ = t.SetTitle("hello from leak")

    _ = t.MoveTo(2, 2)
    _ = t.Print("Press q to quit")

    for {
        ev, err := t.ReadEvent()
        if err != nil {
            break
        }
        if k, ok := ev.(event.KeyEvent); ok {
            if k.Rune == 'q' {
                break
            }
            fmt.Fprintf(os.Stderr, "key: %v\n", k)
        }
    }
}
```

## Package layout

```
leak/
├── terminal.go     // main Terminal type
├── termios/        // raw mode, restore, size, /dev/tty
├── ansi/           // sequence builders (CSI, OSC, modes…)
├── parser/         // bytes → Event
└── event/          // Key, Mouse, Paste, Focus, CursorPosition…
```

## Status

Early foundation. The core loop (open → raw → write sequences → read
events → restore) works, resize events are delivered even on an idle
terminal, and the input/color/DA parsing path has real test coverage.

Landed: more sequences (insert/delete line, erase char, scroll region),
richer DA parsing (structured params, primary vs secondary), synchronized
output helpers, and color queries (OSC 10/11 replies now actually decode
instead of being silently discarded).

Not started: terminal profile-based functions (color-capability detection
— NoColor/16/256/TrueColor, env-var + terminal-query heuristics). Deferred
on purpose — it's a real design surface (detection strategy, fallback
behavior, API shape) that deserves its own pass rather than being bolted
on alongside everything else.
