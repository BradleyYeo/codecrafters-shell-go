package main

import (
	"strings"
	"unicode"
)

// parseState defines lexical scanner modes.
type parseState int

const (
	stateNormal parseState = iota
	stateInSingleQuote
	stateInDoubleQuote
)

const (
	singleQuote = '\''
	doubleQuote = '"'
	backslash   = '\\'
	spaceChar   = ' '
	tabChar     = '\t'
)

// tokenize parses a raw command line into semantic argument tokens.
// Input: raw command string from standard input.
// Output: slice of parsed arguments with quotes stripped and escaped/quoted spaces preserved.
func tokenize(line string) []string {
	var tokens []string
	state := stateNormal
	inToken := false

	var current strings.Builder

	flush := func() {
		if inToken {
			tokens = append(tokens, current.String())
			current.Reset()
			inToken = false
		}
	}

	for _, r := range line {
		switch state {
		case stateInDoubleQuote:
			if r == doubleQuote {
				state = stateNormal
				continue
			}
		case stateInSingleQuote:
			if r == singleQuote {
				state = stateNormal
				continue
			}
			current.WriteRune(r)
		case stateNormal:
			if r == doubleQuote {
				state = stateInDoubleQuote
				inToken = true
				continue
			}
			if r == singleQuote {
				state = stateInSingleQuote
				inToken = true
				continue
			}
			if unicode.IsSpace(r) {
				flush()
				continue
			}
			current.WriteRune(r)
			inToken = true
		}
	}
	flush()
	return tokens
}
