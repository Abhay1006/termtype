package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestSplitLines(t *testing.T) {
	in := "\n  hello   world  \n\n\tfoo\t\tbar\r\n   \nlast\n"
	got := splitLines(in)
	want := []string{"hello world", "foo bar", "last"}
	if !slices.Equal(got, want) {
		t.Fatalf("splitLines() = %#v, want %#v", got, want)
	}

	if got := splitLines("  \n\t\n "); len(got) != 0 {
		t.Fatalf("splitLines(blank) = %#v, want empty", got)
	}
}

func TestGenerateText(t *testing.T) {
	got := generateText(25, false, false)
	fields := strings.Fields(got)
	if len(fields) != 25 {
		t.Fatalf("generateText(25, false, false) returned %d words, want 25", len(fields))
	}

	punct := generateText(30, true, false)
	if !strings.HasSuffix(punct, ".") {
		t.Fatalf("generateText(30, true, false) = %q, want trailing '.'", punct)
	}
	first, _ := utf8.DecodeRuneInString(punct)
	if !unicode.IsUpper(first) {
		t.Fatalf("generateText(30, true, false) = %q, want capitalised start", punct)
	}

	// Non-ASCII custom words should capitalise the first rune cleanly.
	origWords := words
	t.Cleanup(func() { words = origWords })
	words = []string{"über"}
	utf8Text := generateText(3, true, false)
	if !strings.HasPrefix(utf8Text, "Über") {
		t.Fatalf("generateText with UTF-8 word = %q, want prefix 'Über'", utf8Text)
	}
	if !utf8.ValidString(utf8Text) {
		t.Fatalf("generateText produced invalid UTF-8: %q", utf8Text)
	}
}

func TestLoadWordsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "words.txt")
	if err := os.WriteFile(path, []byte("alpha  beta\n\tgamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := loadWordsFile(path)
	if err != nil {
		t.Fatalf("loadWordsFile: %v", err)
	}
	want := []string{"alpha", "beta", "gamma"}
	if !slices.Equal(got, want) {
		t.Fatalf("loadWordsFile() = %#v, want %#v", got, want)
	}

	emptyPath := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(emptyPath, []byte("  \n\t "), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadWordsFile(emptyPath); err == nil {
		t.Fatal("expected error for empty words file")
	}
}

func TestReadCustomText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("  first\tline \n\n  second   line.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readCustomText(path, nil)
	if err != nil {
		t.Fatalf("readCustomText(file): %v", err)
	}
	if got != "first line second line." {
		t.Fatalf("readCustomText(file) = %q, want %q", got, "first line second line.")
	}

	stdinGot, err := readCustomText("-", strings.NewReader("from\tstdin\nnow\n"))
	if err != nil {
		t.Fatalf("readCustomText(stdin): %v", err)
	}
	if stdinGot != "from stdin now" {
		t.Fatalf("readCustomText(stdin) = %q, want %q", stdinGot, "from stdin now")
	}

	if _, err := readCustomText("-", strings.NewReader("   \n\t")); err == nil {
		t.Fatal("expected error for blank stdin text")
	}
}
