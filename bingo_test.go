package main

import (
	"fmt"
	"testing"
)

func TestLetters(t *testing.T) {
	t.Logf("good: %2d: %q + %s", len(goodLetters), goodLetters, string(bingoLetter))
	t.Logf("dead: %2d: %q", len(deadLetters), deadLetters)

	if len(goodLetters) < 20 { // 25 minus one Bingo minus 4 bad squares
		t.Errorf("only have %d good letters; need 20", len(goodLetters))
	}
	if len(deadLetters) < 4 {
		t.Errorf("only have %d dead letters; need 4", len(deadLetters))
	}
}

func TestNewBoard(t *testing.T) {
	b := NewBoard("hi")
	b2 := NewBoard("hi")
	if b != b2 {
		t.Fatal("NewBoard not deterministic")
	}
	t.Logf("example board:\n%s", b)

	const N = 150
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

func TestWidth(t *testing.T) {
	for _, r := range "🌪️" {
		t.Logf("rune %c (u+%04x)  = width %v", r, r, runeWidth(r))
	}
	for _, r := range []rune{'\U0001f32a'} {
		t.Logf("rune %c (u+%04x)  = width %v", r, r, runeWidth(r))
	}
}
