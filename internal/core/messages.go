package core

// Message is a stable user-facing message descriptor. Core workers can attach
// a key and parameters without choosing a UI language; the app catalog formats
// it at paint time. Existing Message strings remain populated for compatibility
// with callers and older diagnostics.
type Message struct {
	Key  string
	Args []any
}
