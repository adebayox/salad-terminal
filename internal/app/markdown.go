package app

import (
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
)

var (
	mdMu        sync.Mutex
	mdRenderer  *glamour.TermRenderer
	mdRenderW   int
)

func renderMarkdown(body string, width int) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	if width < 20 {
		width = 20
	}
	r := markdownRenderer(width)
	if r == nil {
		return body
	}
	out, err := r.Render(body)
	if err != nil {
		return body
	}
	return strings.TrimRight(out, "\n")
}

func markdownRenderer(width int) *glamour.TermRenderer {
	mdMu.Lock()
	defer mdMu.Unlock()
	if mdRenderer != nil && mdRenderW == width {
		return mdRenderer
	}
	// Light style matches Salad's cream canvas better than dark CLI defaults.
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(styles.LightStyleConfig),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return mdRenderer
	}
	mdRenderer = r
	mdRenderW = width
	return mdRenderer
}
