// Package markstream provides native MyGo components for streaming Markdown.
package markstream

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"unicode/utf8"
)

// State is the lifecycle of a streaming reply. Its zero value is Receiving.
type State uint8

const (
	Receiving State = iota
	Completed
	Canceled
	Failed
)

func (s State) String() string {
	switch s {
	case Receiving:
		return "receiving"
	case Completed:
		return "completed"
	case Canceled:
		return "canceled"
	case Failed:
		return "failed"
	}
	return "unknown"
}

var (
	ErrClosed      = errors.New("markstream: stream is closed")
	ErrInvalidUTF8 = errors.New("markstream: invalid or incomplete UTF-8")
	ErrLimit       = errors.New("markstream: source byte limit exceeded")
	ErrReset       = errors.New("markstream: session reset during read")
)

// Snapshot is immutable and safe to retain while a producer writes more data.
// Source contains complete UTF-8 only; PendingBytes counts a split rune held back.
type Snapshot struct {
	Source       string
	State        State
	Err          error
	Revision     uint64
	Generation   uint64
	PendingBytes int
}

func (s Snapshot) Done() bool { return s.State != Receiving }

// Session accepts io.Writer chunks from background goroutines. Render calls
// Snapshot once per frame, coalescing multiple writes into one UI invalidation.
// The zero value is ready to use. Do not copy a Session after use.
type Session struct {
	mu                   sync.Mutex
	text                 strings.Builder
	pending              string
	state                State
	err                  error
	revision, generation uint64
	limit                int
	dirty                bool
	next                 uint64
	listeners            map[uint64]func()
}

// NewSession limits incoming bytes when maxBytes > 0. Zero means unlimited.
func NewSession(maxBytes int) *Session { return &Session{limit: maxBytes} }
func (s *Session) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirty = false
	return Snapshot{s.text.String(), s.state, s.err, s.revision, s.generation, len(s.pending)}
}

// Subscribe registers a nonblocking wake-up callback. It may run on the writer's
// goroutine and must not build UI. MyGo Services.Invalidate is suitable. The
// returned function removes the subscription; an already-running call may finish.
func (s *Session) Subscribe(wake func()) func() {
	s.mu.Lock()
	s.next++
	id := s.next
	if s.listeners == nil {
		s.listeners = make(map[uint64]func())
	}
	s.listeners[id] = wake
	s.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { s.mu.Lock(); delete(s.listeners, id); s.mu.Unlock() }) }
}
func (s *Session) changed() []func() {
	s.revision++
	if s.dirty {
		return nil
	}
	s.dirty = true
	calls := make([]func(), 0, len(s.listeners))
	for _, f := range s.listeners {
		if f != nil {
			calls = append(calls, f)
		}
	}
	return calls
}
func wake(calls []func()) {
	for _, f := range calls {
		f()
	}
}

func (s *Session) Write(p []byte) (int, error) { return s.write(p, 0, false) }
func (s *Session) Append(chunk string) error   { _, err := s.Write([]byte(chunk)); return err }
func (s *Session) write(p []byte, generation uint64, guard bool) (int, error) {
	s.mu.Lock()
	if guard && generation != s.generation {
		s.mu.Unlock()
		return 0, ErrReset
	}
	if s.state != Receiving {
		s.mu.Unlock()
		return 0, ErrClosed
	}
	if len(p) == 0 {
		s.mu.Unlock()
		return 0, nil
	}
	if s.limit > 0 && len(p) > s.limit-s.text.Len()-len(s.pending) {
		s.mu.Unlock()
		return 0, ErrLimit
	}
	chunk := string(p)
	if s.pending != "" {
		chunk = s.pending + chunk
	}
	end := 0
	for end < len(chunk) {
		if !utf8.FullRuneInString(chunk[end:]) {
			break
		}
		r, n := utf8.DecodeRuneInString(chunk[end:])
		if r == utf8.RuneError && n == 1 {
			s.mu.Unlock()
			return 0, ErrInvalidUTF8
		}
		end += n
	}
	s.text.WriteString(chunk[:end])
	s.pending = chunk[end:]
	calls := s.changed()
	s.mu.Unlock()
	wake(calls)
	return len(p), nil
}

// Finish marks EOF and flushes the parser to strict Markdown on the next frame.
// An incomplete final UTF-8 rune fails the stream instead of displaying corruption.
func (s *Session) Finish() error { return s.finish(0, false) }
func (s *Session) finish(generation uint64, guard bool) error {
	s.mu.Lock()
	if guard && generation != s.generation {
		s.mu.Unlock()
		return ErrReset
	}
	if s.state != Receiving {
		err := s.err
		s.mu.Unlock()
		return err
	}
	if s.pending != "" {
		s.state = Failed
		s.err = ErrInvalidUTF8
	} else {
		s.state = Completed
	}
	err := s.err
	calls := s.changed()
	s.mu.Unlock()
	wake(calls)
	return err
}

// Cancel stops receiving while retaining all complete text. It is idempotent.
func (s *Session) Cancel() { s.stop(Canceled, context.Canceled, 0, false) }

// Fail reports a producer error while retaining all complete text.
func (s *Session) Fail(err error) {
	if err != nil {
		s.stop(Failed, err, 0, false)
	}
}
func (s *Session) stop(state State, err error, generation uint64, guard bool) {
	s.mu.Lock()
	if s.state != Receiving || guard && generation != s.generation {
		s.mu.Unlock()
		return
	}
	s.state = state
	s.err = err
	calls := s.changed()
	s.mu.Unlock()
	wake(calls)
}

// Reset starts a new reply. Reads started for an older generation cannot write to it.
func (s *Session) Reset() {
	s.mu.Lock()
	s.text.Reset()
	s.pending = ""
	s.state = Receiving
	s.err = nil
	s.generation++
	calls := s.changed()
	s.mu.Unlock()
	wake(calls)
}

// Consume reads raw Markdown (not SSE framing) until EOF, then calls Finish.
// Cancellation is checked between reads: to interrupt a blocked Read, cancel the
// HTTP request context or close the reader. The reader is owned by the caller.
func (s *Session) Consume(ctx context.Context, r io.Reader) error {
	s.mu.Lock()
	generation := s.generation
	s.mu.Unlock()
	buf := make([]byte, 16*1024)
	empty := 0
	fail := func(err error) error {
		state := Failed
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			state = Canceled
		}
		s.stop(state, err, generation, true)
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		n, err := r.Read(buf)
		if n > 0 {
			empty = 0
			if _, e := s.write(buf[:n], generation, true); e != nil {
				return fail(e)
			}
		} else if err == nil {
			empty++
			if empty >= 100 {
				return fail(io.ErrNoProgress)
			}
		}
		if err == io.EOF {
			return s.finish(generation, true)
		}
		if err != nil {
			return fail(err)
		}
	}
}
