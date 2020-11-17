package prometheus

// Copyright (c) 2019 Uber Technologies, Inc.
// [Apache License 2.0](licenses/m3.license.txt)

// Fork from https://github.com/m3db/m3/blob/e098969502ff32abe4d6be536cc7c1cf06885a85/src/query/graphite/graphite/glob.go

import (
	"bytes"
	"fmt"
	"strings"
)

const (
	ValidIdentifierRunes = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz" + "0123456789" + "$-_'|<>%#/:"
)

type Pattern struct {
	buff         bytes.Buffer
	eval         rune
	lastWriteLen int
}

func (p *Pattern) String() string {
	return p.buff.String()
}

func (p *Pattern) Evaluate(r rune) {
	p.eval = r
}

func (p *Pattern) LastEvaluate() rune {
	return p.eval
}

func (p *Pattern) WriteRune(r rune) {
	p.buff.WriteRune(r)
	p.lastWriteLen = 1
}

func (p *Pattern) WriteString(str string) {
	p.buff.WriteString(str)
	p.lastWriteLen = len(str)
}

func (p *Pattern) UnwriteLast() {
	p.buff.Truncate(p.buff.Len() - p.lastWriteLen)
	p.lastWriteLen = 0
}

func globToRegexPattern(glob string) (string, bool, error) {
	var (
		pattern  Pattern
		escaping = false
		regexed  = false
	)

	groupStartStack := []rune{rune(0)} // rune(0) indicates pattern is not in a group
	for i, r := range glob {
		pattern.Evaluate(r)

		if escaping {
			pattern.WriteRune(r)
			escaping = false
			continue
		}

		switch r {
		case '\\':
			escaping = true
			pattern.WriteRune('\\')
		case '.':
			// Match hierarchy separator
			pattern.WriteString("\\.+")
			regexed = true
		case '?':
			// Match anything except the hierarchy separator
			pattern.WriteString("[^.]")
			regexed = true
		case '*':
			// Match everything up to the next hierarchy separator
			pattern.WriteString("[^.]*")
			regexed = true
		case '{':
			// Begin non-capturing group
			pattern.WriteString("(")
			groupStartStack = append(groupStartStack, r)
			regexed = true
		case '}':
			// End non-capturing group
			priorGroupStart := groupStartStack[len(groupStartStack)-1]
			if priorGroupStart != '{' {
				return "", false, fmt.Errorf("invalid '}' at %d, no prior for '{' in %s", i, glob)
			}

			pattern.WriteRune(')')
			groupStartStack = groupStartStack[:len(groupStartStack)-1]
		case '[':
			// Begin character range
			pattern.WriteRune('[')
			groupStartStack = append(groupStartStack, r)
			regexed = true
		case ']':
			// End character range
			priorGroupStart := groupStartStack[len(groupStartStack)-1]
			if priorGroupStart != '[' {
				return "", false, fmt.Errorf("invalid ']' at %d, no prior for '[' in %s", i, glob)
			}

			pattern.WriteRune(']')
			groupStartStack = groupStartStack[:len(groupStartStack)-1]
		case '<', '>', '\'', '$':
			pattern.WriteRune('\\')
			pattern.WriteRune(r)
		case '|':
			pattern.WriteRune(r)
			regexed = true
		case ',':
			// Commas are part of the pattern if they appear in a group
			if groupStartStack[len(groupStartStack)-1] == '{' {
				pattern.WriteRune('|')
			} else {
				return "", false, fmt.Errorf("invalid ',' outside of matching group at pos %d in %s", i, glob)
			}
		default:
			if !strings.ContainsRune(ValidIdentifierRunes, r) {
				return "", false, fmt.Errorf("invalid character %c at pos %d in %s", r, i, glob)
			}
			pattern.WriteRune(r)
		}
	}

	if len(groupStartStack) > 1 {
		return "", false, fmt.Errorf("unbalanced '%c' in %s", groupStartStack[len(groupStartStack)-1], glob)
	}

	return pattern.buff.String(), regexed, nil
}
