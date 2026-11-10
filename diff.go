package markstream

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type DiffKind uint8

const (
	DiffContext DiffKind = iota
	DiffAdded
	DiffRemoved
	DiffMeta
	DiffHunk
)

type DiffLine struct {
	Kind       DiffKind
	Text       string
	Old, New   int
	SourceLine int
}
type FilePatch struct {
	OldName, NewName string
	Lines            []DiffLine
	Binary           bool
}

var hunkHeader = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@`)

// ParseDiff accepts multi-file unified/Git patches, including partial streamed
// hunks, zero-length ranges, rename metadata and no-newline markers. It preserves
// source line identity so a growing patch can update its native rows in place.
func ParseDiff(source string) []FilePatch {
	var files []FilePatch
	current := -1
	old, newLine := 1, 1
	loose := !strings.Contains(source, "@@ ") && !strings.Contains(source, "diff --git ") && !strings.Contains(source, "--- ")
	inHunk := loose
	ensure := func() {
		if current < 0 {
			files = append(files, FilePatch{})
			current = 0
		}
	}
	for n, line := range strings.Split(strings.TrimSuffix(source, "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "diff --git ") {
			files = append(files, FilePatch{})
			current = len(files) - 1
			inHunk = false
			// Header paths are only labels; ---/+++ below provide canonical paths.
			parts := strings.Fields(strings.TrimPrefix(line, "diff --git "))
			if len(parts) >= 2 {
				files[current].OldName = strings.TrimPrefix(parts[0], "a/")
				files[current].NewName = strings.TrimPrefix(parts[1], "b/")
			}
			continue
		}
		ensure()
		file := &files[current]
		if strings.HasPrefix(line, "--- ") && !inHunk {
			file.OldName = diffPath(line[4:])
			continue
		}
		if strings.HasPrefix(line, "+++ ") && !inHunk {
			file.NewName = diffPath(line[4:])
			continue
		}
		row := DiffLine{Text: line, Kind: DiffMeta, SourceLine: n}
		if m := hunkHeader.FindStringSubmatch(line); m != nil {
			old, _ = strconv.Atoi(m[1])
			newLine, _ = strconv.Atoi(m[3])
			row.Kind = DiffHunk
			inHunk = true
		} else if inHunk && len(line) > 0 {
			switch line[0] {
			case ' ':
				row.Kind = DiffContext
				row.Text = line[1:]
				row.Old = old
				row.New = newLine
				old++
				newLine++
			case '-':
				row.Kind = DiffRemoved
				row.Text = line[1:]
				row.Old = old
				old++
			case '+':
				row.Kind = DiffAdded
				row.Text = line[1:]
				row.New = newLine
				newLine++
			}
		}
		if loose && row.Kind == DiffMeta {
			row.Kind = DiffContext
			row.Old = old
			row.New = newLine
			old++
			newLine++
		}
		if strings.HasPrefix(line, "Binary files ") || line == "GIT binary patch" {
			file.Binary = true
		}
		file.Lines = append(file.Lines, row)
	}
	return files
}
func diffPath(s string) string {
	if i := strings.IndexByte(s, '\t'); i >= 0 {
		s = s[:i]
	}
	if unquoted, e := strconv.Unquote(s); e == nil {
		s = unquoted
	}
	return strings.TrimPrefix(strings.TrimPrefix(s, "a/"), "b/")
}

type DiffMode uint8

const (
	Unified DiffMode = iota
	Split
)

type diffRow struct {
	Left, Right *DiffLine
	Kind        DiffKind
	Text        string
	Key         string
	Hidden      []DiffLine
}

func diffRows(files []FilePatch, mode DiffMode, fold bool, contextLines int, expanded map[string]bool) []diffRow {
	if contextLines <= 0 {
		contextLines = 3
	}
	var out []diffRow
	for fileIndex, file := range files {
		name := file.NewName
		if name == "" {
			name = file.OldName
		}
		if name != "" {
			out = append(out, diffRow{Kind: DiffMeta, Text: name, Key: fmt.Sprintf("file:%d", fileIndex)})
		}
		for i := 0; i < len(file.Lines); {
			line := file.Lines[i]
			if fold && line.Kind == DiffContext {
				end := i
				for end < len(file.Lines) && file.Lines[end].Kind == DiffContext {
					end++
				}
				if end-i > contextLines*2+1 {
					key := fmt.Sprintf("fold:%d:%d", fileIndex, line.SourceLine)
					for k := i; k < i+contextLines; k++ {
						row := file.Lines[k]
						out = append(out, diffRow{Left: &row, Right: &row, Key: fmt.Sprintf("%d:%d", fileIndex, row.SourceLine)})
					}
					if !expanded[key] {
						out = append(out, diffRow{Text: fmt.Sprintf("Expand %d unchanged lines", end-i-2*contextLines), Key: key, Hidden: file.Lines[i+contextLines : end-contextLines]})
					} else {
						for k := i + contextLines; k < end-contextLines; k++ {
							row := file.Lines[k]
							out = append(out, diffRow{Left: &row, Right: &row, Key: fmt.Sprintf("%d:%d", fileIndex, row.SourceLine)})
						}
					}
					i = end - contextLines
					continue
				}
			}
			if mode == Split && (line.Kind == DiffRemoved || line.Kind == DiffAdded) {
				var removed, added []DiffLine
				for i < len(file.Lines) && (file.Lines[i].Kind == DiffRemoved || file.Lines[i].Kind == DiffAdded) {
					row := file.Lines[i]
					if row.Kind == DiffRemoved {
						removed = append(removed, row)
					} else {
						added = append(added, row)
					}
					i++
				}
				for k := 0; k < max(len(removed), len(added)); k++ {
					row := diffRow{Key: fmt.Sprintf("pair:%d:%d:%d", fileIndex, line.SourceLine, k)}
					if k < len(removed) {
						r := removed[k]
						row.Left = &r
					}
					if k < len(added) {
						r := added[k]
						row.Right = &r
					}
					out = append(out, row)
				}
				continue
			}
			row := diffRow{Kind: line.Kind, Key: fmt.Sprintf("%d:%d", fileIndex, line.SourceLine)}
			if line.Kind == DiffMeta || line.Kind == DiffHunk {
				row.Text = line.Text
			} else {
				row.Left = &line
				row.Right = &line
			}
			out = append(out, row)
			i++
		}
	}
	return out
}
