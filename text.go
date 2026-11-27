package main

import (
	_ "embed"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

//go:embed words.txt
var wordsFile string

var words = strings.Fields(wordsFile)

//go:embed quotes.txt
var quotesFile string

// quotes holds one quote per non-empty line of quotes.txt.
var quotes = splitLines(quotesFile)

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		// Normalise whitespace so stray tabs or double spaces can't
		// make a quote impossible to type.
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func randomQuote() string {
	return quotes[rand.IntN(len(quotes))]
}

func loadWordsFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	loaded := strings.Fields(string(data))
	if len(loaded) == 0 {
		return nil, fmt.Errorf("%s: no words found", path)
	}
	return loaded, nil
}

func readCustomText(path string, stdin io.Reader) (string, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return "", err
	}
	text := strings.Join(splitLines(string(data)), " ")
	if text == "" {
		return "", fmt.Errorf("no text to type")
	}
	return text, nil
}

func capitalizeFirst(w string) string {
	if w == "" {
		return ""
	}
	_, size := utf8.DecodeRuneInString(w)
	return strings.ToUpper(w[:size]) + w[size:]
}

// generateText returns n random words. With punct, words are grouped into
// capitalised sentences with commas and other marks, always ending in a
// full stop. With nums, roughly one word in ten becomes a number.
func generateText(n int, punct, nums bool) string {
	picked := make([]string, n)
	newSentence := true
	for i := range picked {
		w := words[rand.IntN(len(words))]
		if nums && rand.IntN(10) == 0 {
			w = strconv.Itoa(rand.IntN(1000))
		}
		if punct {
			if newSentence {
				w = capitalizeFirst(w)
			}
			newSentence = false
			if i == n-1 {
				w += "."
			} else {
				switch r := rand.IntN(100); {
				case r < 8:
					w += "."
					newSentence = true
				case r < 10:
					w += "?"
					newSentence = true
				case r < 11:
					w += "!"
					newSentence = true
				case r < 19:
					w += ","
				case r < 20:
					w += ";"
				case r < 21:
					w += ":"
				}
			}
		}
		picked[i] = w
	}
	return strings.Join(picked, " ")
}
