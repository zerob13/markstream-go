package markstream

import "strings"

// pendingFenceTail suppresses only an unfinished closing-marker prefix. It is
// a display projection: the original source stays intact and is restored at EOF.
func pendingFenceTail(content, raw string, streaming bool) string {
	if !streaming || strings.HasSuffix(raw, "\n") {
		return content
	}
	opener := strings.TrimLeft(raw, " \t")
	if len(opener) < 3 || opener[0] != '`' && opener[0] != '~' {
		return content
	}
	marker := opener[0]
	run := 0
	for run < len(opener) && opener[run] == marker {
		run++
	}
	if run < 3 {
		return content
	}
	body := strings.TrimSuffix(content, "\n")
	at := strings.LastIndexByte(body, '\n') + 1
	tail := strings.TrimLeft(body[at:], " ")
	if len(tail) == 0 || len(tail) >= run {
		return content
	}
	for i := range tail {
		if tail[i] != marker {
			return content
		}
	}
	return body[:at]
}
