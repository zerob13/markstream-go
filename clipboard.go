package markstream

import (
	"html"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

type selectionFragment struct {
	text, markdown, link string
	style                inlineStyle
}

func escapeMarkdown(text string) string {
	return strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "`", "\\`", "~", "\\~").Replace(text)
}
func fragmentFormats(f selectionFragment, text string) (string, string) {
	md, ht := escapeMarkdown(text), html.EscapeString(text)
	if f.markdown != "" {
		return f.markdown, ht
	}
	wrap := func(marker, tag string) { md = marker + md + marker; ht = "<" + tag + ">" + ht + "</" + tag + ">" }
	if f.style.code {
		fence := "`"
		for strings.Contains(text, fence) {
			fence += "`"
		}
		padding := ""
		if strings.HasPrefix(text, "`") || strings.HasSuffix(text, "`") {
			padding = " "
		}
		md = fence + padding + text + padding + fence
		ht = "<code>" + ht + "</code>"
	} else {
		if f.style.weight >= 600 {
			wrap("**", "strong")
		}
		if f.style.italic {
			wrap("*", "em")
		}
		if f.style.strike {
			wrap("~~", "del")
		}
		if f.style.mark {
			wrap("==", "mark")
		}
		if f.style.underline {
			wrap("++", "ins")
		}
		if f.style.shift > 0 {
			wrap("^", "sup")
		} else if f.style.shift < 0 {
			wrap("~", "sub")
		}
	}
	if f.link != "" && SafeLink(f.link) {
		md = "[" + md + "](<" + strings.ReplaceAll(f.link, ">", "%3E") + ">)"
		ht = "<a href=\"" + html.EscapeString(f.link) + "\">" + ht + "</a>"
	}
	return md, ht
}
func selectedFragments(fragments []selectionFragment, start, end int) (string, string) {
	var md, ht strings.Builder
	at := 0
	for _, fragment := range fragments {
		runes := []rune(fragment.text)
		a, b := max(0, start-at), min(len(runes), end-at)
		if a < b {
			m, h := fragmentFormats(fragment, string(runes[a:b]))
			md.WriteString(m)
			ht.WriteString(h)
		}
		at += len(runes)
	}
	return md.String(), ht.String()
}
func (v *viewState) copySelection(parts []ui.SelectedText) transfer.Data {
	var plain, markup, markdown, code strings.Builder
	codeID, language := "", ""
	separator := func() {
		if markdown.Len() > 0 {
			markdown.WriteString("\n\n")
		}
	}
	flushCode := func() {
		if codeID == "" {
			return
		}
		text := code.String()
		fence := "```"
		for strings.Contains(text, fence) {
			fence += "`"
		}
		separator()
		markdown.WriteString(fence + language + "\n" + text + "\n" + fence)
		markup.WriteString("<pre><code>" + html.EscapeString(text) + "</code></pre>")
		code.Reset()
		codeID = ""
	}
	for _, part := range parts {
		runes := []rune(part.Text)
		a, b := max(0, part.Start), min(len(runes), part.End)
		if a >= b {
			continue
		}
		text := string(runes[a:b])
		if plain.Len() > 0 {
			plain.WriteByte('\n')
		}
		plain.WriteString(text)
		region := v.sources[part.ID]
		md, ht := selectedFragments(region.fragments, a, b)
		if len(region.fragments) == 0 {
			md = text
			ht = html.EscapeString(text)
		}
		if a == 0 && b == utf8.RuneCountInString(part.Text) && region.start >= 0 && region.end <= len(region.source) && region.end > region.start {
			md = strings.TrimRight(region.source[region.start:region.end], "\r\n")
		}
		if region.node != nil && region.node.Type == "code_block" {
			if codeID != region.node.ID {
				flushCode()
				codeID = region.node.ID
				language = strings.NewReplacer("`", "", "\n", "", "\r", "").Replace(region.node.Language)
			}
			if code.Len() > 0 {
				code.WriteByte('\n')
			}
			code.WriteString(md)
			continue
		}
		flushCode()
		if region.node != nil && region.node.Type == "heading" && (a != 0 || b != len(runes)) {
			md = strings.Repeat("#", min(6, max(1, region.node.Level))) + " " + md
		}
		separator()
		markdown.WriteString(md)
		markup.WriteString("<p>" + ht + "</p>")
	}
	flushCode()
	return selectionData(plain.String(), markup.String(), markdown.String())
}

func (v *viewState) selectedMarkdown(parts []ui.SelectedText) string {
	data := v.copySelection(parts)
	items := data.Items()
	if len(items) == 0 {
		return ""
	}
	b, _ := items[0].Read(transfer.Markdown)
	return string(b)
}
