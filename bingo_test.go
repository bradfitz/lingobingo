package main

import (
	"testing"
)

func TestLetters(t *testing.T) {
	seen := map[letter]bool{}
	for _, s := range slides {
		if s.letter != 0 {
			seen[s.letter] = true
		}
	}
	for l := letter('A'); l <= 'Z'; l++ {
		if !seen[l] {
			t.Errorf("letter %q not found", l)
		}
	}
	for l := letter('0'); l <= '9'; l++ {
		if !seen[l] {
			t.Errorf("number %q not found", l)
		}
	}
	t.Logf("%d letters/numbers found", len(seen))
}
