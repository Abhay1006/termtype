package main

import (
	_ "embed"
	"math/rand/v2"
	"strconv"
	"strings"
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
				w = strings.ToUpper(w[:1]) + w[1:]
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
