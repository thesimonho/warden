package terminal

import (
	"bytes"
	"io"
	"strings"
)

// DECSET/DECRST parameter numbers that switch to/from the alternate screen
// buffer. Stripping these forces applications (like Claude Code via Ink)
// to render in the normal buffer, where xterm.js scrollback works.
const (
	decsetAltScreen           = "47"   // alternate screen
	decsetAltScreenClearExit  = "1047" // alternate screen (clear on exit)
	decsetAltScreenSaveCursor = "1049" // save cursor + alternate screen
)

// AltScreenFilter wraps an io.Reader and strips escape sequences that
// switch to/from the alternate screen buffer. All other data — including
// colors, cursor movement, mouse reporting, and synchronized output —
// passes through unchanged.
//
// This is the same approach as xterm's titeInhibit resource.
//
// The filter handles sequences split across read boundaries by carrying
// partial escape data between reads.
type AltScreenFilter struct {
	src   io.Reader
	carry []byte // partial escape sequence from previous Read
	buf   []byte // reusable read buffer, grown as needed
}

// NewAltScreenFilter creates a filter that strips alternate screen escape
// sequences from src.
func NewAltScreenFilter(src io.Reader) *AltScreenFilter {
	return &AltScreenFilter{src: src}
}

// Read implements io.Reader. It reads from the underlying source, strips
// any alternate screen escape sequences, and writes the filtered result
// into p.
func (f *AltScreenFilter) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	for {
		// Ensure the scratch buffer is large enough.
		if len(f.buf) < len(p) {
			f.buf = make([]byte, len(p))
		}
		buf := f.buf[:len(p)]

		n, readErr := f.src.Read(buf)
		if n == 0 && readErr != nil {
			// Source is exhausted. Flush any carried bytes (partial escape
			// that never completed — deliver as-is since there's no more data).
			if len(f.carry) > 0 {
				carried := f.carry
				f.carry = nil
				copied := copy(p, carried)
				return copied, readErr
			}
			return 0, readErr
		}

		// Prepend any partial escape from the previous read.
		var data []byte
		if len(f.carry) > 0 {
			data = make([]byte, len(f.carry)+n)
			copy(data, f.carry)
			copy(data[len(f.carry):], buf[:n])
			f.carry = nil
		} else {
			data = buf[:n]
		}

		filtered := filterAltScreen(data, &f.carry)

		// If the source signaled an error (e.g. EOF), no more data will arrive,
		// so any carried partial escape will never complete. Flush it as-is.
		if readErr != nil && len(f.carry) > 0 {
			filtered = append(filtered, f.carry...)
			f.carry = nil
		}

		copied := copy(p, filtered)
		if copied < len(filtered) {
			// Can happen when carry bytes were prepended, making the data
			// larger than len(p). Stash the overflow for the next Read.
			f.carry = append(filtered[copied:], f.carry...)
		}

		// If we produced no output but the source gave us data (all stripped
		// or all carried), loop to try again so the caller doesn't see a
		// zero-length read (which signals EOF per io.Reader contract).
		if copied > 0 || readErr != nil {
			return copied, readErr
		}
		// All data was stripped or carried — loop for more.
	}
}

// filterAltScreen scans data for DECSET/DECRST escape sequences containing
// alternate screen parameters and removes them. If data ends with a partial
// escape sequence, the partial bytes are saved to carry for the next read.
//
// Returns the filtered output.
func filterAltScreen(data []byte, carry *[]byte) []byte {
	// Fast path: if there's no ESC byte at all, nothing can match.
	if bytes.IndexByte(data, '\x1b') == -1 {
		return data
	}

	out := make([]byte, 0, len(data))
	i := 0

	for i < len(data) {
		// Look for the next ESC character.
		escIdx := bytes.IndexByte(data[i:], '\x1b')
		if escIdx == -1 {
			// No more escapes — copy remainder.
			out = append(out, data[i:]...)
			break
		}

		// Copy everything before the ESC.
		out = append(out, data[i:i+escIdx]...)
		i += escIdx

		// Check if we have enough bytes for the DECSET prefix "\x1b[?".
		remaining := data[i:]
		if len(remaining) < 3 {
			// Partial escape at end of buffer — carry it.
			*carry = append(*carry, remaining...)
			return out
		}

		if remaining[1] != '[' || remaining[2] != '?' {
			// Not a DECSET/DECRST — pass through the ESC and continue.
			out = append(out, remaining[0])
			i++
			continue
		}

		// We have "\x1b[?" — now parse the parameter list and terminator.
		seqEnd, params, terminator := parseDECSET(remaining[3:])
		if seqEnd == -1 {
			// Incomplete sequence at end of buffer — carry it.
			*carry = append(*carry, remaining...)
			return out
		}

		// Total sequence length: 3 (prefix) + seqEnd (params + terminator)
		seqLen := 3 + seqEnd

		// Check if any params are alt-screen related.
		if hasAltScreenParam(params) {
			// Filter: rebuild without alt-screen params.
			kept := filterParams(params)
			if len(kept) > 0 {
				// Rebuild with remaining params.
				out = append(out, '\x1b', '[', '?')
				out = append(out, strings.Join(kept, ";")...)
				out = append(out, terminator)
			}
			// If all params were alt-screen, the entire sequence is dropped.
		} else {
			// No alt-screen params — pass through unchanged.
			out = append(out, remaining[:seqLen]...)
		}

		i += seqLen
	}

	return out
}

// parseDECSET parses the parameter list and terminator from data that
// follows the "\x1b[?" prefix. Returns the offset past the terminator
// (relative to data), the list of parameter strings, and the terminator
// byte ('h' or 'l'). Returns -1 if the sequence is incomplete.
func parseDECSET(data []byte) (end int, params []string, terminator byte) {
	// Parameters are digits separated by ';', terminated by 'h' or 'l'.
	paramStart := 0
	var paramList []string

	for j := 0; j < len(data); j++ {
		b := data[j]
		switch {
		case b >= '0' && b <= '9':
			// Part of a parameter number.
			continue
		case b == ';':
			paramList = append(paramList, string(data[paramStart:j]))
			paramStart = j + 1
		case b == 'h' || b == 'l':
			// Terminator found. Capture the last parameter.
			paramList = append(paramList, string(data[paramStart:j]))
			return j + 1, paramList, b
		default:
			// Unexpected character — not a DECSET we recognize. Treat as
			// non-matching and pass through. Return the sequence up to and
			// including the unexpected byte so we don't loop forever.
			return j + 1, nil, 0
		}
	}

	// Reached end of data without finding terminator — incomplete.
	return -1, nil, 0
}

// hasAltScreenParam returns true if any parameter in the list is an
// alternate screen parameter.
func hasAltScreenParam(params []string) bool {
	for _, p := range params {
		if isAltScreenParam(p) {
			return true
		}
	}
	return false
}

// isAltScreenParam returns true if the parameter number corresponds to
// an alternate screen mode.
func isAltScreenParam(p string) bool {
	return p == decsetAltScreen || p == decsetAltScreenClearExit || p == decsetAltScreenSaveCursor
}

// filterParams returns only the parameters that are NOT alt-screen related.
// Empty parameters (from malformed sequences like "\x1b[?;1049h") are
// also dropped to avoid producing invalid rebuilt sequences.
func filterParams(params []string) []string {
	var kept []string
	for _, p := range params {
		if p != "" && !isAltScreenParam(p) {
			kept = append(kept, p)
		}
	}
	return kept
}
