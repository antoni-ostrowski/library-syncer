package handlers

import (
	"testing"
)

func TestZipLayout(t *testing.T) {
	top, kept, rels, _, ok := zipLayout(
		[]string{"Tape/disc1/01.mp3", "Tape/disc2/02.mp3", "Tape/", "Tape/disc1/"},
		"Tape.zip",
	)
	if !ok || top != "Tape" || len(kept) != 2 || len(rels) != 2 {
		t.Fatalf("nested zip: top=%q kept=%v rels=%v ok=%v", top, kept, rels, ok)
	}
	if rels[0] != "disc1/01.mp3" {
		t.Fatalf("rels[0] = %q, want disc1/01.mp3", rels[0])
	}

	top, kept, rels, _, ok = zipLayout(
		[]string{"disc1/01.mp3", "cover.jpg", "__MACOSX/._01.mp3", ".DS_Store"},
		"MyTape",
	)
	if !ok || top != "MyTape" || len(kept) != 2 {
		t.Fatalf("flat zip: top=%q kept=%v ok=%v, want MyTape + 2 kept", top, kept, ok)
	}
	if rels[0] != "disc1/01.mp3" || rels[1] != "cover.jpg" {
		t.Fatalf("flat zip rels = %v", rels)
	}

	for name, args := range map[string]struct {
		names []string
		top   string
	}{
		"zip slip":      {[]string{"Tape/../../evil.mp3"}, "Tape"},
		"absolute":      {[]string{"/etc/passwd"}, "Tape"},
		"bad fallback":  {[]string{"01.mp3"}, ".."},
		"only metadata": {[]string{"__MACOSX/._01.mp3", ".DS_Store"}, "Tape"},
	} {
		_, kept, _, _, ok := zipLayout(args.names, args.top)
		if ok && len(kept) > 0 {
			t.Fatalf("%s: usable files kept, want rejected/empty", name)
		}
	}
}
