package markstream

import "github.com/egoist/mygo/ui/markdown/parser"

// Document is MyGo's incremental Markdown syntax tree. Block indices and spans
// remain valid until the next mutation. It is owned by one goroutine; Session is
// the concurrent transport. Reusing a document preserves completed-block caches.
type Document = parser.Document

// Extensions selects syntax beyond CommonMark. NewDocument(0) uses the defaults.
type Extensions = parser.Extensions

const (
	GFM               = parser.GFM
	DefaultExtensions = parser.Default | parser.ExtDollarMath
	Tables            = parser.ExtTables
	TaskLists         = parser.ExtTaskLists
	Strikethrough     = parser.ExtStrikethrough
	Autolinks         = parser.ExtAutolinks
	Footnotes         = parser.ExtFootnotes
	Math              = parser.ExtMath
	DollarMath        = parser.ExtDollarMath
	Alerts            = parser.ExtAlerts
)

func NewDocument(extensions Extensions) *Document {
	if extensions == 0 {
		extensions = DefaultExtensions
	}
	return parser.New(extensions)
}
func Parse(source string) *Document { d := NewDocument(0); d.Set(source); return d }
