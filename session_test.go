package markstream

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

func TestSessionSplitUTF8AndImmutableSnapshots(t *testing.T) {
	s := NewSession(0)
	text := "你好🙂 **stream**"
	var old []Snapshot
	for _, b := range []byte(text) {
		if n, e := s.Write([]byte{b}); n != 1 || e != nil {
			t.Fatalf("write %d %v", n, e)
		}
		snap := s.Snapshot()
		if !utf8.ValidString(snap.Source) {
			t.Fatal("partial rune leaked")
		}
		old = append(old, snap)
	}
	if e := s.Finish(); e != nil {
		t.Fatal(e)
	}
	if got := s.Snapshot(); got.Source != text || got.State != Completed || got.PendingBytes != 0 {
		t.Fatalf("%+v", got)
	}
	s.Reset()
	s.Append("replacement")
	for _, snap := range old {
		if !strings.HasPrefix(text, snap.Source) {
			t.Fatal("snapshot mutated")
		}
	}
}
func TestSessionAtomicValidation(t *testing.T) {
	s := NewSession(5)
	s.Append("abc")
	for _, p := range [][]byte{{0xff}, []byte("def")} {
		n, err := s.Write(p)
		if n != 0 || err == nil {
			t.Fatalf("write %d %v", n, err)
		}
		if s.Snapshot().Source != "abc" {
			t.Fatal("failed write changed source")
		}
	}
	s.Write([]byte{0xe4})
	if !errors.Is(s.Finish(), ErrInvalidUTF8) || s.Snapshot().State != Failed {
		t.Fatal("incomplete rune not failed")
	}
	if _, e := s.Write([]byte("x")); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
}
func TestCoalescedWakeupAndUnsubscribe(t *testing.T) {
	s := new(Session)
	calls := 0
	off := s.Subscribe(func() { calls++; _ = s.Snapshot() })
	// A reentrant snapshot must not deadlock.
	s.Append("a")
	if calls != 1 {
		t.Fatal(calls)
	}
	off()
	off()
	s.Append("b")
	if calls != 1 {
		t.Fatal(calls)
	}
	s.Snapshot()
	off = s.Subscribe(func() { calls++ })
	defer off()
	s.Append("c")
	s.Append("d")
	s.Finish()
	if calls != 2 {
		t.Fatal(calls)
	}
	s.Snapshot()
	s.Reset()
	if calls != 3 {
		t.Fatal(calls)
	}
}
func TestSessionLifecycle(t *testing.T) {
	s := new(Session)
	s.Append("partial")
	s.Cancel()
	s.Cancel()
	s.Fail(io.ErrUnexpectedEOF)
	got := s.Snapshot()
	if got.State != Canceled || !errors.Is(got.Err, context.Canceled) || got.Source != "partial" {
		t.Fatalf("%+v", got)
	}
	s.Reset()
	if got = s.Snapshot(); got.State != Receiving || got.Err != nil || got.Source != "" || got.Generation != 1 {
		t.Fatalf("%+v", got)
	}
	s.Finish()
	s.Finish()
	if s.Snapshot().State != Completed {
		t.Fatal("finish not idempotent")
	}
}
func TestConsume(t *testing.T) {
	s := new(Session)
	if e := s.Consume(context.Background(), strings.NewReader("# Hello\n\n你好")); e != nil {
		t.Fatal(e)
	}
	if got := s.Snapshot(); got.State != Completed || got.Source != "# Hello\n\n你好" {
		t.Fatalf("%+v", got)
	}
	s.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := s.Consume(ctx, strings.NewReader("ignored")); !errors.Is(e, context.Canceled) || s.Snapshot().State != Canceled {
		t.Fatal(e)
	}
	s.Reset()
	if e := s.Consume(context.Background(), errorReader{}); !errors.Is(e, io.ErrUnexpectedEOF) || s.Snapshot().State != Failed {
		t.Fatal(e)
	}
}

type errorReader struct{}

func (errorReader) Read(p []byte) (int, error) { copy(p, "ok"); return 2, io.ErrUnexpectedEOF }

type blockedReader struct{ entered, release chan struct{} }

func (r blockedReader) Read(p []byte) (int, error) {
	close(r.entered)
	<-r.release
	copy(p, "old")
	return 3, io.EOF
}
func TestResetRejectsOldProducer(t *testing.T) {
	s := new(Session)
	r := blockedReader{make(chan struct{}), make(chan struct{})}
	done := make(chan error, 1)
	go func() { done <- s.Consume(context.Background(), r) }()
	<-r.entered
	s.Reset()
	s.Append("new")
	close(r.release)
	if e := <-done; !errors.Is(e, ErrReset) {
		t.Fatal(e)
	}
	if snap := s.Snapshot(); snap.Source != "new" || snap.State != Receiving {
		t.Fatalf("%+v", snap)
	}
}
func TestConcurrentSession(t *testing.T) {
	s := new(Session)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 200 {
				s.Append("你")
				s.Snapshot()
			}
		})
	}
	wg.Wait()
	s.Finish()
	if got := s.Snapshot(); len([]rune(got.Source)) != 800 {
		t.Fatal(len(got.Source))
	}
}
func FuzzSessionUTF8(f *testing.F) {
	f.Add("你好🙂 **bold**", uint8(1))
	f.Add("a\r\n# hi", uint8(4))
	f.Fuzz(func(t *testing.T, src string, chunk uint8) {
		if !utf8.ValidString(src) || len(src) > 8192 {
			t.Skip()
		}
		s := new(Session)
		size := int(chunk)%31 + 1
		for n := 0; n < len(src); n += size {
			if _, err := s.Write([]byte(src[n:min(n+size, len(src))])); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.Finish(); err != nil {
			t.Fatal(err)
		}
		if s.Snapshot().Source != src {
			t.Fatal("source changed")
		}
	})
}

// A closed session must not start reading a potentially blocked transport.
type unreadableReader struct{}

func (unreadableReader) Read([]byte) (int, error) { panic("closed session read from transport") }
func TestConsumeClosedSessionDoesNotRead(t *testing.T) {
	s := new(Session)
	s.Finish()
	if e := s.Consume(context.Background(), unreadableReader{}); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
}

type cancelOnRead struct{ cancel context.CancelFunc }

func (r cancelOnRead) Read(p []byte) (int, error) { r.cancel(); copy(p, "late"); return 4, io.EOF }
func TestConsumeCancellationDuringFinalRead(t *testing.T) {
	s := new(Session)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if e := s.Consume(ctx, cancelOnRead{cancel}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if got := s.Snapshot(); got.State != Canceled || got.Source != "" {
		t.Fatalf("%+v", got)
	}
}
