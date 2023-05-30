package main

import (
	"fmt"
	"testing"
)

func TestLetters(t *testing.T) {
	t.Logf("  used: %2d: %q", len(goodLetters), goodLetters)
	t.Logf("unused: %2d: %q", len(deadLetters), deadLetters)
	t.Logf("  path: %2d: %q", len(preWinLetters), preWinLetters)
	t.Logf("   win: %s", string(winLetter))
}

func TestNewBoard(t *testing.T) {
	b := NewBoard("hi")
	b2 := NewBoard("hi")
	if b != b2 {
		t.Fatal("NewBoard not deterministic")
	}

	const N = 1000
	saw := map[board]bool{}
	for i := 0; i < N; i++ {
		saw[NewBoard(fmt.Sprint(i))] = true
	}
	t.Logf("in %d builds, saw %d unique boards", N, len(saw))
	if false {
		for b := range saw {
			t.Logf("board\n%s\n", b)
		}
	}
}
