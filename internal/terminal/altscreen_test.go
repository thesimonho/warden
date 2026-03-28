package terminal

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestAltScreenFilter_Passthrough(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
	}{
		{"plain text", "hello world"},
		{"ANSI colors", "\x1b[1m\x1b[31mbold red\x1b[0m"},
		{"cursor movement", "\x1b[H\x1b[2J\x1b[10;20H"},
		{"SGR mouse reporting", "\x1b[?1006h\x1b[?1006l"},
		{"cursor show/hide", "\x1b[?25h\x1b[?25l"},
		{"synchronized output", "\x1b[?2026h\x1b[?2026l"},
		{"standard CSI", "\x1b[K\x1b[2K\x1b[1A\x1b[5B"},
		{"empty input", ""},
		{"single ESC in text", "foo\x1b bar"},
		{"ESC bracket no question mark", "\x1b[1049h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := readAll(t, NewAltScreenFilter(strings.NewReader(tt.input)))
			if got != tt.input {
				t.Errorf("want %q, got %q", tt.input, got)
			}
		})
	}
}

func TestAltScreenFilter_StripSimpleSequences(t *testing.T) {
	t.Parallel()
	sequences := []string{
		"\x1b[?1049h", "\x1b[?1049l", // DECSET/DECRST 1049
		"\x1b[?47h", "\x1b[?47l", // DECSET/DECRST 47
		"\x1b[?1047h", "\x1b[?1047l", // DECSET/DECRST 1047
	}

	for _, seq := range sequences {
		t.Run(seq, func(t *testing.T) {
			t.Parallel()
			input := "before" + seq + "after"
			got := readAll(t, NewAltScreenFilter(strings.NewReader(input)))
			want := "beforeafter"
			if got != want {
				t.Errorf("want %q, got %q", want, got)
			}
		})
	}
}

func TestAltScreenFilter_StripCombinedSequences(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "alt screen + SGR mouse",
			input: "\x1b[?1049;1006h",
			want:  "\x1b[?1006h",
		},
		{
			name:  "SGR mouse + alt screen",
			input: "\x1b[?1006;1049h",
			want:  "\x1b[?1006h",
		},
		{
			name:  "alt screen + cursor + mouse",
			input: "\x1b[?1049;25;1006h",
			want:  "\x1b[?25;1006h",
		},
		{
			name:  "all alt screen params",
			input: "\x1b[?47;1047;1049h",
			want:  "",
		},
		{
			name:  "reset combined",
			input: "\x1b[?1049;1006l",
			want:  "\x1b[?1006l",
		},
		{
			name:  "multiple non-alt params preserved",
			input: "\x1b[?25;1006;2026h",
			want:  "\x1b[?25;1006;2026h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := readAll(t, NewAltScreenFilter(strings.NewReader(tt.input)))
			if got != tt.want {
				t.Errorf("want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestAltScreenFilter_MultipleSequencesInOneRead(t *testing.T) {
	t.Parallel()
	input := "start\x1b[?1049h\x1b[1mmiddle\x1b[?1049l\x1b[0mend"
	want := "start\x1b[1mmiddle\x1b[0mend"
	got := readAll(t, NewAltScreenFilter(strings.NewReader(input)))
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestAltScreenFilter_SplitAcrossReads(t *testing.T) {
	t.Parallel()
	// Test splitting "\x1b[?1049h" at every possible boundary.
	seq := "\x1b[?1049h"
	prefix := "before"
	suffix := "after"
	full := prefix + seq + suffix

	for splitAt := 1; splitAt < len(full); splitAt++ {
		splitAt := splitAt
		t.Run(strings.ReplaceAll(
			full[:splitAt]+"|"+full[splitAt:], "\x1b", "ESC"),
			func(t *testing.T) {
				t.Parallel()
				r := &splitReader{
					chunks: [][]byte{
						[]byte(full[:splitAt]),
						[]byte(full[splitAt:]),
					},
				}
				got := readAll(t, NewAltScreenFilter(r))
				want := "beforeafter"
				if got != want {
					t.Errorf("split at %d: want %q, got %q", splitAt, want, got)
				}
			})
	}
}

func TestAltScreenFilter_PartialEscAtEnd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		chunks []string
		want   string
	}{
		{
			name:   "lone ESC then continuation",
			chunks: []string{"hello\x1b", "[?1049hworld"},
			want:   "helloworld",
		},
		{
			name:   "ESC bracket then continuation",
			chunks: []string{"hello\x1b[", "?1049hworld"},
			want:   "helloworld",
		},
		{
			name:   "ESC bracket question then continuation",
			chunks: []string{"hello\x1b[?", "1049hworld"},
			want:   "helloworld",
		},
		{
			name:   "partial param then continuation",
			chunks: []string{"hello\x1b[?104", "9hworld"},
			want:   "helloworld",
		},
		{
			name:   "lone ESC then non-DECSET",
			chunks: []string{"hello\x1b", "[1mworld"},
			want:   "hello\x1b[1mworld",
		},
		{
			name:   "lone ESC at very end",
			chunks: []string{"hello\x1b"},
			want:   "hello\x1b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			chunks := make([][]byte, len(tt.chunks))
			for i, c := range tt.chunks {
				chunks[i] = []byte(c)
			}
			got := readAll(t, NewAltScreenFilter(&splitReader{chunks: chunks}))
			if got != tt.want {
				t.Errorf("want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestAltScreenFilter_PreservesOtherDECSET(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
	}{
		{"cursor visible", "\x1b[?25h"},
		{"cursor invisible", "\x1b[?25l"},
		{"SGR mouse", "\x1b[?1006h"},
		{"bracketed paste", "\x1b[?2004h"},
		{"synchronized output", "\x1b[?2026h"},
		{"focus tracking", "\x1b[?1004h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input := "x" + tt.input + "y"
			got := readAll(t, NewAltScreenFilter(strings.NewReader(input)))
			if got != input {
				t.Errorf("want %q, got %q", input, got)
			}
		})
	}
}

func TestAltScreenFilter_LargeThroughput(t *testing.T) {
	t.Parallel()
	// 64KB of text with alt-screen sequences scattered.
	var buf bytes.Buffer
	chunk := bytes.Repeat([]byte("abcdefghij"), 100) // 1000 bytes
	for i := 0; i < 64; i++ {
		buf.Write(chunk)
		if i%10 == 0 {
			buf.WriteString("\x1b[?1049h")
		}
	}
	buf.WriteString("\x1b[?1049l")

	input := buf.String()
	got := readAll(t, NewAltScreenFilter(strings.NewReader(input)))

	// Verify no alt-screen sequences remain.
	if strings.Contains(got, "\x1b[?1049h") || strings.Contains(got, "\x1b[?1049l") {
		t.Error("alt-screen sequences still present in output")
	}

	// Verify text content is preserved.
	expectedTextLen := 64 * 1000
	gotTextLen := len(got)
	if gotTextLen != expectedTextLen {
		t.Errorf("expected %d text bytes, got %d", expectedTextLen, gotTextLen)
	}
}

func TestAltScreenFilter_UnrecognizedTerminator(t *testing.T) {
	t.Parallel()
	// Unknown terminator — the sequence should pass through.
	input := "\x1b[?1049x"
	got := readAll(t, NewAltScreenFilter(strings.NewReader(input)))
	if got != input {
		t.Errorf("want %q, got %q", input, got)
	}
}

func TestAltScreenFilter_PartialDECSETThenEOF(t *testing.T) {
	t.Parallel()
	// A partial DECSET with no terminator followed by EOF should flush as-is.
	chunks := [][]byte{
		[]byte("hello\x1b[?1049"),
	}
	got := readAll(t, NewAltScreenFilter(&splitReader{chunks: chunks}))
	want := "hello\x1b[?1049"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestAltScreenFilter_EmptyParamLeadingSemicolon(t *testing.T) {
	t.Parallel()
	// Malformed sequence with leading semicolon: "\x1b[?;1049h"
	// Should strip 1049 and the empty param, producing nothing.
	input := "\x1b[?;1049h"
	got := readAll(t, NewAltScreenFilter(strings.NewReader(input)))
	want := ""
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestAltScreenFilter_EmptyParamWithKeptParam(t *testing.T) {
	t.Parallel()
	// Malformed "\x1b[?;1006;1049h" — empty param + mouse + alt screen.
	// Should keep only 1006 without a leading semicolon.
	input := "\x1b[?;1006;1049h"
	got := readAll(t, NewAltScreenFilter(strings.NewReader(input)))
	want := "\x1b[?1006h"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestFilterAltScreen_DirectCoverage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "false start ESC bracket digit",
			input: "\x1b[1049h still here",
			want:  "\x1b[1049h still here",
		},
		{
			name:  "consecutive alt screen sequences",
			input: "\x1b[?1049h\x1b[?47h\x1b[?1047h",
			want:  "",
		},
		{
			name:  "alt screen between colors",
			input: "\x1b[31m\x1b[?1049h\x1b[32m",
			want:  "\x1b[31m\x1b[32m",
		},
		{
			name:  "bare terminator no params",
			input: "\x1b[?h",
			want:  "\x1b[?h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var carry []byte
			got := string(filterAltScreen([]byte(tt.input), &carry))
			if got != tt.want {
				t.Errorf("want %q, got %q", tt.want, got)
			}
		})
	}
}

// --- helpers ---

// readAll reads all data from an AltScreenFilter and returns it as a string.
func readAll(t *testing.T, r io.Reader) string {
	t.Helper()
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	return buf.String()
}

// splitReader delivers data in predetermined chunks, simulating read
// boundaries at specific positions. Each Read returns exactly one chunk.
type splitReader struct {
	chunks [][]byte
	idx    int
}

func (r *splitReader) Read(p []byte) (int, error) {
	if r.idx >= len(r.chunks) {
		return 0, io.EOF
	}
	n := copy(p, r.chunks[r.idx])
	r.idx++
	if r.idx >= len(r.chunks) {
		return n, io.EOF
	}
	return n, nil
}
