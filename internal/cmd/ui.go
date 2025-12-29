package cmd

import (
	"io"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
)

// UI provides user interface helpers.
type UI struct {
	out       io.Writer
	spinner   *spinner.Spinner
	json      bool
	noColor   bool
	nonInteractive bool
}

// NewUI creates a new UI helper.
func NewUI(out io.Writer, jsonMode bool) *UI {
	// Detect if running in a non-interactive environment
	nonInteractive := !isTerminal() || os.Getenv("CI") != "" || os.Getenv("NO_COLOR") != ""

	// Respect --no-color flag or NO_COLOR env
	noColor := cfg.NoColor || os.Getenv("NO_COLOR") != "" || nonInteractive

	if noColor {
		color.NoColor = true
	}

	return &UI{
		out:            out,
		json:           jsonMode,
		noColor:        noColor,
		nonInteractive: nonInteractive,
	}
}

// StartSpinner starts a spinner with a message.
func (u *UI) StartSpinner(msg string) {
	if u.json || u.nonInteractive {
		return
	}
	u.spinner = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	u.spinner.Suffix = " " + msg
	u.spinner.Writer = u.out
	u.spinner.Start()
}

// StopSpinner stops the spinner with success or failure.
func (u *UI) StopSpinner(success bool) {
	if u.spinner == nil {
		return
	}
	u.spinner.Stop()
	if success {
		color.New(color.FgGreen).Fprint(u.out, "✓")
	} else {
		color.New(color.FgRed).Fprint(u.out, "✗")
	}
	u.spinner = nil
}

// StopSpinnerMsg stops spinner and prints a message.
func (u *UI) StopSpinnerMsg(success bool, msg string) {
	if u.spinner == nil {
		return
	}
	u.spinner.Stop()
	if success {
		color.New(color.FgGreen).Fprintf(u.out, "✓ %s\n", msg)
	} else {
		color.New(color.FgRed).Fprintf(u.out, "✗ %s\n", msg)
	}
	u.spinner = nil
}

// Success prints a success message.
func (u *UI) Success(msg string) {
	if u.json {
		return
	}
	color.New(color.FgGreen).Fprintf(u.out, "✓ %s\n", msg)
}

// Error prints an error message.
func (u *UI) Error(msg string) {
	if u.json {
		return
	}
	color.New(color.FgRed).Fprintf(u.out, "✗ %s\n", msg)
}

// Info prints an info message.
func (u *UI) Info(msg string) {
	if u.json {
		return
	}
	color.New(color.FgCyan).Fprintf(u.out, "ℹ %s\n", msg)
}

// Warning prints a warning message.
func (u *UI) Warning(msg string) {
	if u.json {
		return
	}
	color.New(color.FgYellow).Fprintf(u.out, "⚠ %s\n", msg)
}

// Header prints a header/title.
func (u *UI) Header(msg string) {
	if u.json {
		return
	}
	color.New(color.FgWhite, color.Bold).Fprintf(u.out, "\n%s\n", msg)
}

// Step prints a step indicator.
func (u *UI) Step(current, total int, msg string) {
	if u.json {
		return
	}
	color.New(color.FgCyan).Fprintf(u.out, "[%d/%d] ", current, total)
	color.New(color.FgWhite).Fprintf(u.out, "%s", msg)
}

func isTerminal() bool {
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		return true
	}
	return false
}
