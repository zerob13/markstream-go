package markstream

import (
	"github.com/egoist/mygo/ui"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type revealState struct {
	cursor int

	source        string
	shown         int
	last, changed time.Time
	credit        float64
	active        bool
}

func (r *revealState) step(c *ui.Context, source string, receiving bool, o *Reveal) (string, bool, bool) {
	options := Reveal{Unit: ByCharacter, CatchUp: 120 * time.Millisecond, Fade: 180 * time.Millisecond}
	if o != nil {
		options = *o
	}
	if options.CatchUp <= 0 {
		options.CatchUp = 120 * time.Millisecond
	}
	now := c.Now()
	replacement := !strings.HasPrefix(source, r.source)
	if replacement {
		r.shown = 0
		r.cursor = 0
		r.credit = 0
		r.active = receiving
		r.last = now
		r.changed = now
	}
	if source != r.source {
		if receiving || r.active {
			r.changed = now
		} else {
			r.changed = time.Time{}
		}
		if receiving {
			r.active = true
		}
	}
	if !r.active && !receiving || options.Unit == AtOnce {
		r.shown = len(source)
		r.cursor = len(source)
	} else {
		elapsed := now.Sub(r.last)
		if elapsed < 0 {
			elapsed = 0
		}
		backlog := utf8.RuneCountInString(source[min(r.cursor, len(source)):])
		r.credit += float64(elapsed) / float64(options.CatchUp) * float64(max(backlog, 1))
		count := int(math.Floor(r.credit))
		r.credit -= float64(count)
		for count > 0 && r.cursor < len(source) {
			_, n := utf8.DecodeRuneInString(source[r.cursor:])
			r.cursor += n
			count--
		}
		if !receiving && backlog > 0 && elapsed > 0 && r.cursor < len(source) && now.Sub(r.changed) > options.CatchUp*2 {
			r.cursor = len(source)
		}
		r.shown = r.cursor
		if options.Unit != ByCharacter {
			r.shown = unitBoundary(source, r.shown, options.Unit, !receiving)
		}
	}
	r.last = now
	r.source = source
	if r.shown > len(source) {
		r.shown = len(source)
	}
	pending := r.shown < len(source)
	fading := options.Fade > 0 && !r.changed.IsZero() && now.Sub(r.changed) < options.Fade
	if (pending && (r.cursor < len(source) || !receiving)) || fading {
		c.AnimationFrame()
	}
	if !pending && !receiving {
		r.active = false
	}
	return source[:r.shown], pending, fading
}
func unitBoundary(source string, shown int, unit RevealUnit, final bool) int {
	if shown >= len(source) && final {
		return len(source)
	}
	if unit == ByWord {
		if shown == 0 || shown >= len(source) {
			return shown
		}
		last, _ := utf8.DecodeLastRuneInString(source[:shown])
		if unicode.Is(unicode.Han, last) || unicode.Is(unicode.Hiragana, last) || unicode.Is(unicode.Katakana, last) {
			return shown
		}
		for shown > 0 {
			r, n := utf8.DecodeLastRuneInString(source[:shown])
			if unicode.IsSpace(r) {
				break
			}
			shown -= n
		}
		return shown
	}
	prefix := source[:shown]
	if unit == ByLine {
		return strings.LastIndex(prefix, "\n") + 1
	}
	if unit == ByParagraph {
		if n := strings.LastIndex(prefix, "\n\n"); n >= 0 {
			return n + 2
		}
		return 0
	}
	if unit == BySentence {
		last := 0
		for i, r := range prefix {
			if strings.ContainsRune(".!?。！？\n", r) {
				last = i + utf8.RuneLen(r)
			}
		}
		return last
	}
	return shown
}
