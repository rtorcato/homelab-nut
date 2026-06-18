package tui

import (
	_ "embed"
	"strings"
)

// mascotArt is the walnut mascot rendered as half-block + 24-bit ANSI by
// chafa. Re-render from assets/mascot.png with:
//
//	chafa --size 30x15 --symbols half --colors truecolor \
//	      --polite on --animate off internal/tui/assets/mascot.png \
//	      > internal/tui/assets/mascot.ans
//
//go:embed assets/mascot.ans
var mascotArtRaw string

// mascotArt strips the trailing newline so lipgloss vertical alignment
// stays tight when the mascot is joined beside the empty-state text.
var mascotArt = strings.TrimRight(mascotArtRaw, "\n")
