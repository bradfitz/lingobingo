package main

import (
	"fmt"
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
			t.Logf("letter %q not found", l)
		}
	}
	for l := letter('0'); l <= '9'; l++ {
		if !seen[l] {
			t.Logf("number %q not found", l)
		}
	}
	t.Logf("%d letters/numbers found", len(seen))
}

func TestNewBoard(t *testing.T) {
	b := NewBoard("hi")
	b2 := NewBoard("hi")
	if b != b2 {
		t.Fatal("NewBoard not deterministic")
	}
	t.Logf("got:\n%s", b)

	const N = 1000
	saw := map[board]bool{}
	for i := 0; i < N; i++ {
		saw[NewBoard(fmt.Sprint(i))] = true
	}
	t.Logf("in %d builds, saw %d unique boards", N, len(saw))

}
