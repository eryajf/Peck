package app

import (
	"fmt"
	"testing"

	"github.com/eryajf/Peck/internal/core"
)

func TestReceiveFileQueuesColdStartApplicationAndNormalizesPath(t *testing.T) {
	a := &application{}

	a.receiveFile("/Applications/Test.app/")

	if len(a.pending) != 1 {
		t.Fatalf("pending requests = %d, want 1", len(a.pending))
	}
	if got := a.pending[0].path; got != "/Applications/Test.app" {
		t.Fatalf("queued path = %q, want normalized application path", got)
	}
	if a.inspection != nil || a.trusted || a.state != pageIdle {
		t.Fatalf("cold-start file request changed active state: %+v", a)
	}
}

func TestReceiveFilePreservesCurrentConfirmation(t *testing.T) {
	inspection := &core.Inspection{Path: "/Applications/Current.app", Name: "Current"}
	a := &application{
		state:      pageConfirm,
		inspection: inspection,
		trusted:    true,
	}

	a.receiveFile("/Applications/Next.app")

	if a.state != pageConfirm || a.inspection != inspection || !a.trusted {
		t.Fatal("incoming file replaced the current confirmation")
	}
	if len(a.pending) != 1 || a.pending[0].path != "/Applications/Next.app" {
		t.Fatalf("pending requests = %+v, want the new application queued", a.pending)
	}
}

func TestReceiveFileDeduplicatesAndBoundsQueue(t *testing.T) {
	a := &application{}
	for index := 0; index < 6; index++ {
		a.receiveFile(fmt.Sprintf("/Applications/Test-%d.app", index))
	}

	if len(a.pending) != 5 {
		t.Fatalf("pending requests = %d, want queue limit 5", len(a.pending))
	}

	a.receiveFile("/Applications/Test-0.app")
	if len(a.pending) != 5 {
		t.Fatalf("duplicate file expanded the queue to %d entries", len(a.pending))
	}
}

func TestReceiveFileRejectsNonApplicationPath(t *testing.T) {
	for _, path := range []string{
		"",
		"/Applications/Notes.txt",
		"/Downloads/Test.dmg",
		"/Applications/Test.app/Contents/Info.plist",
	} {
		t.Run(path, func(t *testing.T) {
			a := &application{}
			a.receiveFile(path)

			if len(a.pending) != 0 || a.inspection != nil || a.state != pageIdle {
				t.Fatalf("invalid path entered the processing flow: %+v", a)
			}
			if a.notice == "" {
				t.Fatal("invalid path did not produce a notice")
			}
		})
	}
}
