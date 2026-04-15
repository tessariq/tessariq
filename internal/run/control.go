package run

// ContainsControlChar reports whether s contains any ASCII control byte
// (0x00–0x1F) or DEL (0x7F). These bytes are unsafe in filesystem paths,
// commit trailers, and proxy directives because newlines and other control
// characters can be used to inject forged lines into structured text
// (git trailers, Squid config, allowlist entries).
//
// Space is intentionally not rejected here — task paths and titles may
// legitimately contain spaces. Callers that must also reject space (for
// example proxy hostnames) use containsControlOrSpace.
func ContainsControlChar(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b <= 0x1f || b == 0x7f {
			return true
		}
	}
	return false
}
