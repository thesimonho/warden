package engine

// DefaultDisconnectKey is the default key to disconnect from a terminal.
const DefaultDisconnectKey = "ctrl+\\"

// DisconnectKeyToByte converts a "ctrl+<char>" string to its control character byte.
// Returns 0 if the format is invalid. Supports:
//   - ctrl+a through ctrl+z (0x01–0x1a)
//   - ctrl+[ (0x1b, ESC) — not recommended
//   - ctrl+\ (0x1c, default)
//   - ctrl+] (0x1d)
//   - ctrl+^ (0x1e)
//   - ctrl+_ (0x1f)
func DisconnectKeyToByte(key string) byte {
	if len(key) < 6 || key[:5] != "ctrl+" {
		return 0
	}
	ch := key[5:]
	if len(ch) != 1 {
		// Handle escaped backslash: "ctrl+\\" in Go source = "ctrl+\" at runtime.
		if ch == "\\" {
			return 0x1c
		}
		return 0
	}
	b := ch[0]
	// ctrl+a=0x01 ... ctrl+z=0x1a
	if b >= 'a' && b <= 'z' {
		return b - 'a' + 1
	}
	// ctrl+[=0x1b, ctrl+\=0x1c, ctrl+]=0x1d, ctrl+^=0x1e, ctrl+_=0x1f
	if b >= '[' && b <= '_' {
		return b - '@'
	}
	return 0
}
