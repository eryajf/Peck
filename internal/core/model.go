// Package core inspects and processes one explicitly confirmed macOS app.
package core

import (
	"context"
	"time"
)

const QuarantineAttribute = "com.apple.quarantine"

// Inspection is an immutable, short-lived plan. Only Inspect can create its
// private snapshot; changing display fields never changes the authorized target.
type Inspection struct {
	Path, Name, Version, BundleID string
	SignatureStatus               string // unsigned, adhoc, adhoc-invalid, valid, invalid, unknown
	QuarantineCount               int
	CanSign                       bool
	// AlreadyProcessed means the app has a verified existing signature, no
	// quarantine attributes remain, and macOS returned only a generic
	// assessment rejection without a concrete security finding. It offers a
	// no-op verification and lets the user try opening it; it does not claim
	// that the app is safe or that Launch Services will successfully start it.
	AlreadyProcessed          bool
	SignReason, BlockedReason string
	Assessment                string
	snapshot                  *snapshot
	executable                string
	created                   time.Time
}

type Options struct{ Sign bool }

type Progress struct {
	Stage, Message   string
	Descriptor       Message
	Completed, Total int
}

// Result keeps the three outcomes separate. CanOpen becomes true only after all
// requested operations and their verification succeeded.
type Result struct {
	Path, Name, Signature, Quarantine, Launch string
	Verification                              string
	Changed, Skipped, Failed                  int
	Error                                     error
	Cancelled, CanOpen                        bool
	post                                      *snapshot
}

func report(fn func(Progress), stage, message string, completed, total int) {
	if fn != nil {
		fn(Progress{Stage: stage, Message: message, Completed: completed, Total: total, Descriptor: Message{Key: stage}})
	}
}

func contextError(ctx context.Context) error { return ctx.Err() }
