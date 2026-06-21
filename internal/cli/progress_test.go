package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss/v2"
)

func TestProgressDisplayFallsBackToPlainOutput(t *testing.T) {
	var output bytes.Buffer
	progress := newProgressDisplay(&output, false)
	progress.Update("Building image", false)
	progress.Update("Image built", true)
	if got, want := output.String(), "-> Building image\nok Image built\n"; got != want {
		t.Fatalf("progress output = %q, want %q", got, want)
	}
}

func TestProgressBarsHaveStableWidth(t *testing.T) {
	for _, bar := range []string{animatedProgressBar(0), animatedProgressBar(18), completeProgressBar()} {
		if width := lipgloss.Width(bar); width != progressBarWidth {
			t.Fatalf("progress bar width = %d, want %d", width, progressBarWidth)
		}
	}
	if animatedProgressBar(0) == animatedProgressBar(18) {
		t.Fatal("animated progress frames should differ")
	}
	if !strings.Contains(completeProgressBar(), "█") {
		t.Fatal("complete progress bar is empty")
	}
}
