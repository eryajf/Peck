package version

import "testing"

func TestFromManifest(t *testing.T) {
	got, err := FromManifest("[app]\nname = \"Peck\"\nversion = \"0.2.0\"\n")
	if err != nil || got != "0.2.0" {
		t.Fatalf("FromManifest() = %q, %v", got, err)
	}
}

func TestFromManifestRejectsMissingVersion(t *testing.T) {
	if _, err := FromManifest("[app]\nname = \"Peck\"\n"); err == nil {
		t.Fatal("missing version unexpectedly succeeded")
	}
}
