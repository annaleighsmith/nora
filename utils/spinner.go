package utils

import (
	"os"
	"sync"
	"time"
)

// activeSpinner holds the mutex that coordinates the spinner with Dimf output.
// When Dimf needs to print, it locks this mutex, clears the spinner line, prints,
// then unlocks so the spinner can redraw on the next tick.
var activeSpinner struct {
	mu     sync.Mutex
	active bool
}

func StartSpinner(msg string) func() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	var once sync.Once
	done := make(chan struct{})

	activeSpinner.mu.Lock()
	activeSpinner.active = true
	activeSpinner.mu.Unlock()

	go func() {
		i := 0
		for {
			select {
			case <-done:
				activeSpinner.mu.Lock()
				ClearLine(os.Stderr)
				activeSpinner.active = false
				activeSpinner.mu.Unlock()
				return
			default:
				activeSpinner.mu.Lock()
				ClearLine(os.Stderr)
				Dim.Fprintf(os.Stderr, "%s %s", frames[i%len(frames)], msg)
				activeSpinner.mu.Unlock()
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()

	return func() {
		once.Do(func() { close(done) })
		time.Sleep(100 * time.Millisecond)
	}
}

// SpinnerAwarePrint clears the spinner line, prints the message, then lets
// the spinner redraw on its next tick. Safe to call with no active spinner.
func SpinnerAwarePrint(fn func()) {
	activeSpinner.mu.Lock()
	defer activeSpinner.mu.Unlock()
	if activeSpinner.active {
		ClearLine(os.Stderr)
	}
	fn()
}
