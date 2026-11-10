package markstream

import (
	"fmt"
	"github.com/egoist/mygo/ui/markdown/parser"
	"strings"
	"testing"
)

const corpus = "# 流式 Markdown\n\n**bold**, *italic*, ~~deleted~~, `code`, [Go](https://go.dev) and $x_i$.\n\n" +
	"- [x] ready\n- [ ] pending\n  - nested\n\n> [!TIP]\n> Keep old blocks.\n\n" +
	"| Name | Value |\n| :--- | ---: |\n| a | 42 |\n\n```go\nfmt.Println(\"你好\")\n```\n\n" +
	"Text[^1] and [later][ref].\n\n[^1]: footnote\n\n[ref]: https://go.dev\n\n$$\nx^2 + y^2\n$$\n\n---\n"

func shape(d *Document) string {
	var b strings.Builder
	for i := 0; i < d.Len(); i++ {
		block := d.Block(i)
		fmt.Fprintf(&b, "%v %d %d %d %v %q %q\n", block.Kind, block.Parent, block.Next, block.Level, block.Task, d.Lines(i), block.Info())
		for _, in := range d.Inlines(i) {
			fmt.Fprintf(&b, "%v:%q:%q:%t;", in.Kind, in.Text, in.Dest, in.Pending)
		}
	}
	return b.String()
}
func TestIncrementalMatchesFinalAtEverySplit(t *testing.T) {
	want := shape(Parse(corpus))
	for split := 0; split <= len(corpus); split++ {
		d := NewDocument(0)
		d.SetStreaming(true)
		d.Append(corpus[:split])
		shape(d)
		d.Append(corpus[split:])
		d.SetStreaming(false)
		if got := shape(d); got != want {
			t.Fatalf("split %d differs\n%s\nwant\n%s", split, got, want)
		}
	}
}
func TestHealingAndStrictFinalization(t *testing.T) {
	d := NewDocument(0)
	d.SetStreaming(true)
	d.Append("Hello **world")
	strong := false
	for _, in := range d.Inlines(1) {
		strong = strong || in.Kind == parser.Strong
	}
	if !strong {
		t.Fatal("incomplete strong not healed")
	}
	d.SetStreaming(false)
	if got, want := shape(d), shape(Parse("Hello **world")); got != want {
		t.Fatalf("%s != %s", got, want)
	}
}
func TestSourceReplacementAndReferenceInvalidation(t *testing.T) {
	d := NewDocument(0)
	d.SetStreaming(true)
	d.Set("[Go][ref]\n\n")
	d.Inlines(1)
	d.Append("[ref]: https://go.dev\n")
	d.SetStreaming(false)
	if got, want := shape(d), shape(Parse(d.Source())); got != want {
		t.Fatal("reference cache stale")
	}
	d.Set("# replacement\n\n- one\n- two\n")
	if shape(d) != shape(Parse(d.Source())) {
		t.Fatal("replacement cache stale")
	}
}
func BenchmarkIncrementalDocument(b *testing.B) {
	src := strings.Repeat(corpus, 20)
	b.ReportAllocs()
	b.SetBytes(int64(len(src)))
	for b.Loop() {
		d := NewDocument(0)
		d.SetStreaming(true)
		for n := 0; n < len(src); n += 64 {
			d.Append(src[n:min(n+64, len(src))])
		}
		d.SetStreaming(false)
	}
}
func BenchmarkReparseDocument(b *testing.B) {
	src := strings.Repeat(corpus, 20)
	b.ReportAllocs()
	b.SetBytes(int64(len(src)))
	for b.Loop() {
		for n := 64; n < len(src); n += 64 {
			Parse(src[:n])
		}
		Parse(src)
	}
}
