package runtime

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

type downloadProgress struct {
	total   int
	current int
	width   int
	lastLen int
	enabled bool
	color   bool
	mu      sync.Mutex
}

func newDownloadProgress(total int) *downloadProgress {
	if total <= 0 || isQuietOutput() {
		return &downloadProgress{}
	}

	return &downloadProgress{
		total:   total,
		width:   28,
		enabled: true,
		color:   supportsColor(),
	}
}

func (p *downloadProgress) step(label string) {
	if p == nil || !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.current++
	if p.current > p.total {
		p.current = p.total
	}
	p.render(strings.TrimSpace(label))
}

func (p *downloadProgress) render(label string) {
	if p.total <= 0 {
		return
	}

	percent := int(float64(p.current)*100/float64(p.total) + 0.5)
	filled := p.width * p.current / p.total

	var bar string
	if p.color {
		bar = "\x1b[32m" + strings.Repeat("=", filled) + "\x1b[0m" + strings.Repeat("-", p.width-filled)
	} else {
		bar = strings.Repeat("=", filled) + strings.Repeat("-", p.width-filled)
	}

	line := fmt.Sprintf("\rDownloading gems [%s] %3d%% (%d/%d)", bar, percent, p.current, p.total)
	if label != "" {
		line += " " + label
	}

	if len(line) < p.lastLen {
		line += strings.Repeat(" ", p.lastLen-len(line))
	}
	p.lastLen = len(line)

	_, _ = fmt.Fprint(os.Stdout, line)
	if p.current >= p.total {
		_, _ = fmt.Fprint(os.Stdout, "\n")
	}
}

func isTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func supportsColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if term := strings.ToLower(os.Getenv("TERM")); term == "" || term == "dumb" {
		return false
	}
	return true
}

func isQuietOutput() bool {
	if os.Getenv("CI") != "" {
		return true
	}
	return !isTerminal()
}
