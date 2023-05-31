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
	if x, y, ok := b.find('2'); !ok || x != 0 || y != 3 {
		t.Errorf("find = %v, %v, %v", x, y, ok)
	}
	if x, y, ok := b.find('S'); !ok || x != 2 || y != 2 {
		t.Errorf("find = %v, %v, %v", x, y, ok)
	}

	b3 := NewBoard("42979a59a485a5b3")
	t.Logf("another board:\n%s", b3)
	bs := &bingoServer{
		letterSeen: map[letter]bool{},
	}
	for _, v := range goodLettersWithBingo {
		bs.letterSeen[v] = true
	}
	line := bs.boardWinLine(b)
	if len(line) == 0 {
		t.Errorf("no win line")
	}
	t.Logf("line = %v", line)

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
