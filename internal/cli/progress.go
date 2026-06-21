package cli

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
)

const progressBarWidth = 32

type progressDisplay struct {
	writer  io.Writer
	enabled bool

	mu       sync.Mutex
	active   bool
	stop     chan struct{}
	stopped  chan struct{}
	rendered bool
	started  time.Time
}

func newProgressDisplay(writer io.Writer, enabled bool) *progressDisplay {
	return &progressDisplay{writer: writer, enabled: enabled}
}

func (p *progressDisplay) Update(message string, done bool) {
	if !p.enabled {
		prefix := "-> "
		if done {
			prefix = "ok "
		}
		fmt.Fprintln(p.writer, prefix+message)
		return
	}
	if done {
		p.finish(message)
		return
	}
	p.start(message)
}

func (p *progressDisplay) start(message string) {
	p.abort()
	fmt.Fprintln(p.writer, "→ "+message)
	p.mu.Lock()
	p.active = true
	p.stop = make(chan struct{})
	p.stopped = make(chan struct{})
	p.rendered = false
	p.started = time.Now()
	stop := p.stop
	stopped := p.stopped
	p.mu.Unlock()
	go p.animate(stop, stopped)
}

func (p *progressDisplay) animate(stop <-chan struct{}, stopped chan<- struct{}) {
	defer close(stopped)
	delay := time.NewTimer(150 * time.Millisecond)
	defer delay.Stop()
	select {
	case <-stop:
		return
	case <-delay.C:
	}
	ticker := time.NewTicker(90 * time.Millisecond)
	defer ticker.Stop()
	frame := 0
	for {
		p.renderActive(frame)
		frame++
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
	}
}

func (p *progressDisplay) renderActive(frame int) {
	p.mu.Lock()
	p.rendered = true
	elapsed := time.Since(p.started).Round(time.Second)
	p.mu.Unlock()
	fmt.Fprintf(p.writer, "\r\x1b[2K   %s  %s", animatedProgressBar(frame), elapsed)
}

func (p *progressDisplay) finish(message string) {
	rendered := p.stopAnimation()
	if rendered {
		fmt.Fprintf(p.writer, "\r\x1b[2K   %s  100%%\n", completeProgressBar())
	}
	fmt.Fprintln(p.writer, "ok "+message)
}

func (p *progressDisplay) abort() {
	if p.stopAnimation() {
		fmt.Fprint(p.writer, "\r\x1b[2K")
	}
}

func (p *progressDisplay) Close() {
	p.abort()
}

func (p *progressDisplay) stopAnimation() bool {
	p.mu.Lock()
	if !p.active {
		p.mu.Unlock()
		return false
	}
	stop := p.stop
	stopped := p.stopped
	p.active = false
	close(stop)
	p.mu.Unlock()
	<-stopped
	p.mu.Lock()
	rendered := p.rendered
	p.mu.Unlock()
	return rendered
}

func animatedProgressBar(frame int) string {
	colors := []string{"#a855f7", "#8b5cf6", "#6366f1", "#3b82f6", "#06b6d4", "#14b8a6", "#10b981"}
	segmentWidth := 9
	position := frame%(progressBarWidth+segmentWidth) - segmentWidth
	var bar strings.Builder
	for index := 0; index < progressBarWidth; index++ {
		offset := index - position
		if offset >= 0 && offset < segmentWidth {
			color := colors[offset*len(colors)/segmentWidth]
			bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("█"))
		} else {
			bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#475569")).Render("░"))
		}
	}
	return bar.String()
}

func completeProgressBar() string {
	colors := []string{"#a855f7", "#6366f1", "#3b82f6", "#06b6d4", "#14b8a6", "#10b981"}
	var bar strings.Builder
	for index := 0; index < progressBarWidth; index++ {
		color := colors[index*len(colors)/progressBarWidth]
		bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("█"))
	}
	return bar.String()
}
