// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

/*
TODO:
if you add transition: 1s all on both tr td and #marked it’ll do an animation when the class changes
12:47
and then you need a tiny bit of JS to add/remove classes

*/

package main

import (
	"flag"
	"fmt"
	"hash/crc64"
	"html"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"
	"tailscale.com/types/logger"
)

var (
	verbose     = flag.Bool("verbose", false, "verbose")
	presentOnly = flag.Bool("present", false, "present only")
	startSlide  = flag.Int("slide", 0, "slide to start on (1-based)")
)

type slide struct {
	msg string
	id  slideID // set for certain key slides
}

type slideID string

func ss(msg string, arg ...any) slide {
	s := slide{msg: msg}
	for _, a := range arg {
		switch v := a.(type) {
		case slideID:
			s.id = v
		default:
			panic(fmt.Sprintf("unknown type %T", a))
		}
	}
	return s
}

var slides = []slide{
	ss("A lightning talk\n⚡️\nBrad Fitzpatrick\n"),
	ss("oh hi\nintros later"),
	ss("talks talks talks"),
	ss("talks are fun"),
	ss("but let's play a game\n🎮🎲?"),
	ss("BINGO"),
	ss("more specifically,"),
	ss("LINGO BINGO"),
	ss("Bingo?\n🤨"),
	ss("🧗"),
	ss("🚢"),
	ss("Ⓑ  Ⓘ  Ⓝ  Ⓖ  Ⓞ  \n⚪ ⚪ ⚪ ⚪ ⚪\n⚪ ⚪ ⚪ ⚪ ⚪\n⚪ ⚪ ⚪ ⚪ ⚪\n⚪ ⚪ ⚪ ⚪ ⚪\n⚪ ⚪ ⚪ ⚪ ⚪\n"),
	ss("Ⓑ  Ⓘ  Ⓝ  Ⓖ  Ⓞ  \n⚪ 🔴 ⚪ ⚪ ⚪\n⚪ 🔴 ⚪ ⚪ ⚪\n⚪ 🔴 ⚪ ⚪ ⚪\n⚪ 🔴 ⚪ ⚪ ⚪\n⚪ 🔴 ⚪ ⚪ ⚪\n"),
	ss("Ⓑ  Ⓘ  Ⓝ  Ⓖ  Ⓞ  \n⚪ ⚪ ⚪ ⚪ ⚪\n⚪ ⚪ ⚪ ⚪ ⚪\n⚪ ⚪ ⚪ ⚪ ⚪\n🔴 🔴 🔴 🔴 🔴\n⚪ ⚪ ⚪ ⚪ ⚪\n"),
	ss("Ⓑ  Ⓘ  Ⓝ  Ⓖ  Ⓞ  \n🔴 ⚪ ⚪ ⚪ ⚪\n⚪ 🔴 ⚪ ⚪ ⚪\n⚪ ⚪ 🔴 ⚪ ⚪\n⚪ ⚪ ⚪ 🔴 ⚪\n⚪ ⚪ ⚪ ⚪ 🔴\n"),
	ss("s/🔢/🔤/g"),
	ss("s/🔢/Tailscale lingo/g"),
	ss("lingo: noun\nthe vocabulary or jargon\n of a particular\nsubject or group of people"),
	ss("i.e."),
	ss("I say $GIBBERISH,\n"),
	ss("I say $G\u0332IBBERISH,\nyou get G\u0332"),
	ss("got it?"),
	ss("play.bingo.ts.net"),
	ss("⚡️ 😬⚡️"),
	ss("play.bingo.ts.net\n\n⏳ -:--\n ", slideID("timer")),
	ss("play.bingo.ts.net\n\n⏳ -:--\n✨ Tailscale Funnel ✨"),
	ss("play.bingo.ts.net\n\n⏳ -:--\n🍄 (accept the share) 🍄"),
	ss("play.bingo.ts.net\n\n⏳ -:--\nintro? right."),
	ss("play.bingo.ts.net\n\n⏳ -:--\nGopher"),
	ss("play.bingo.ts.net\n\n⏳ -:--\nI write code at Tailscale"),
	ss("play.bingo.ts.net\n\n⏳ -:--\nfew years now"),
	ss("play.bingo.ts.net\n\n⏳ -:--\n(I know where the bodies are)"),
	ss("play.bingo.ts.net\n\n⏳ -:--\nI used to ❤️ travel + speak"),
	ss("play.bingo.ts.net\n\n⏳ -:--\nthen pandemic + kids 😅"),
	ss("play.bingo.ts.net\n\n⏳ -:--\n🌲🏡🏔️"),
	ss("play.bingo.ts.net\n\n⏳ -:--\n"),
	ss("Game on! 🏒"),
	ss(""),
}

func main() {
	flag.Parse()

	bs := &bingoServer{
		gameEv: make(chan any, 8),
	}
	if *startSlide > 0 {
		bs.slide = *startSlide - 1
	}

	if *presentOnly {
		log.Fatal(bs.present())
	}
	s := &tsnet.Server{
		Logf: logger.Discard,
	}
	if *verbose {
		s.Logf = log.Printf
	}
	defer s.Close()

	ln80, err := s.Listen("tcp", ":80")
	if err != nil {
		log.Fatal(err)
	}
	defer ln80.Close()

	bs.lc, err = s.LocalClient()
	if err != nil {
		log.Fatal(err)
	}

	lnFunnel, err := s.ListenFunnel("tcp", ":443")
	if err != nil {
		log.Fatal(err)
	}
	defer lnFunnel.Close()

	errc := make(chan error, 1)
	go func() { errc <- http.Serve(lnFunnel, bs) }()
	go func() { errc <- http.Serve(ln80, bs) }()
	go func() { errc <- bs.present() }()

	log.Fatal(<-errc)
}

func (bs *bingoServer) advanceSlide(delta int) {
	next := bs.slide + delta
	if next < 0 {
		next = 0
	}
	if next >= len(slides) {
		next = len(slides) - 1
	}
	bs.setSlide(next)
}

func (bs *bingoServer) setSlide(n int) {
	bs.slide = n
	bs.render()
}

func (bs *bingoServer) writeString(x, y int, s string, optStyle ...tcell.Style) {
	style := tcell.StyleDefault
	if len(optStyle) > 0 {
		if len(optStyle) > 1 {
			panic("too many styles")
		}
		style = optStyle[0]
	}

	var lastX int
	var lastR rune
	for _, r := range s {
		wid := runewidth.RuneWidth(r)
		if wid == 0 {
			if r == '\u0332' {
				bs.sc.SetContent(lastX, y, lastR, nil, style.Foreground(tcell.ColorRed))
			}
			continue
		}
		bs.sc.SetContent(x, y, r, nil, style)
		lastX = x
		lastR = r
		x += wid
	}
}

func (bs *bingoServer) render() {
	curSlide := slides[bs.slide]
	msg := curSlide.msg

	if strings.Contains(msg, "⏳") {
		if bs.clock != nil {
			msg = strings.ReplaceAll(msg, "-:--", fmt.Sprintf("%1d:%02d", bs.secRemain/60, bs.secRemain%60))
		}
	}

	bs.paintWithMsg(msg)
	width, height := bs.sc.Size()
	if bs.slide > 3 {
		bs.writeString(0, height-1, "🔤")
	}
	if bs.slide > 0 {
		bs.writeString(width-5, height-1, fmt.Sprintf("🛝%d", bs.slide+1), tcell.StyleDefault.Foreground(tcell.ColorDarkGray))
	}
	bs.sc.Sync()
}

func (bs *bingoServer) paintWithMsg(msg string) {
	sc := bs.sc
	sc.Fill(' ', tcell.StyleDefault)

	width, height := sc.Size()

	lines := strings.Split(msg, "\n")
	midy := int(float64(height)/2.0 - float64(len(lines))/2 + 0.5)
	if midy < 0 {
		midy = 0
	}
	for y, lineMsg := range lines {
		midx := width / 2
		start := midx - runewidth.StringWidth(lineMsg)/2
		if start < 0 {
			start = 0
		}
		bs.writeString(start, midy+y, lineMsg)
	}
}

func (bs *bingoServer) present() error {
	var err error
	bs.sc, err = tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := bs.sc.Init(); err != nil {
		return err
	}

	bs.render()

	evc := make(chan tcell.Event, 8)
	quitc := make(chan struct{})
	go func() {
		bs.sc.Clear()

		for {
			select {
			case ev := <-bs.gameEv:
				switch ev := ev.(type) {
				case string:
					bs.paintWithMsg(fmt.Sprintf("Player: %q", ev))
					bs.sc.Sync()
				case func():
					ev()
					continue
				default:
					panic(fmt.Sprintf("unknown game event: %T", ev))
				}
			case ev := <-evc:
				switch ev := ev.(type) {
				case *tcell.EventKey:
					k := ev.Key()
					switch k {
					case tcell.KeyDown, tcell.KeyRight:
						bs.advanceSlide(+1)
					case tcell.KeyUp, tcell.KeyLeft:
						bs.advanceSlide(-1)
					case tcell.KeyRune:
						r := ev.Rune()
						switch r {
						case '.':
							bs.advanceSlide(0)
							continue
						case ' ':
							bs.advanceSlide(1)
							continue
						case '0':
							bs.setSlide(0)
							continue
						case 'c':
							bs.startClock()
							continue
						}
						bs.paintWithMsg(fmt.Sprintf("Rune: %q", r))
						bs.sc.Sync()
						if r == 'q' {
							bs.sc.Fini()
							os.Exit(0)
						}
					default:
						bs.paintWithMsg(fmt.Sprintf("Key: %d", k))
						bs.sc.Sync()
					}
				case *tcell.EventResize:
					bs.render()
				default:
					bs.paintWithMsg(fmt.Sprintf("ev: %T: %v", ev, ev))
					bs.sc.Sync()
				}
			}
		}
	}()
	bs.sc.ChannelEvents(evc, quitc)

	select {}
}

type bingoServer struct {
	lc     *tailscale.LocalClient
	gameEv chan any

	sc    tcell.Screen
	slide int

	secRemain int
	clock     *time.Timer
}

func (s *bingoServer) startClock() {
	s.secRemain = 30
	if s.clock != nil {
		s.clock.Stop()
	}

	s.clock = time.AfterFunc(time.Second, func() {
		s.gameEv <- s.tickClock
	})
	s.render()
}

func (s *bingoServer) tickClock() {
	s.secRemain--
	s.render()
	if s.secRemain > 0 {
		s.clock = time.AfterFunc(time.Second, func() {
			s.gameEv <- s.tickClock
		})
	}
}

var crc64Table = crc64.MakeTable(crc64.ISO)

func (s *bingoServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	who, _ := s.lc.WhoIs(r.Context(), r.RemoteAddr)
	var gameBoard string
	if who != nil {
		if firstLabel(who.Node.ComputedName) == "funnel-ingress-node" {
			//log.Printf("Funnel headers: %q", r.Header)
		}
		gameBoard = who.UserProfile.LoginName

		s.gameEv <- who.UserProfile.LoginName
	} else {
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err != nil {
			gameBoard = host
		} else {
			gameBoard = r.RemoteAddr
		}
	}

	//log.Printf("TLS=%v, Board: %q", r.TLS != nil, gameBoard)

	letters := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")

	rnd := rand.New(rand.NewSource(int64(crc64.Checksum([]byte(gameBoard), crc64Table))))
	rnd.Shuffle(len(letters), reflect.Swapper(letters))

	hdr, _ := os.ReadFile("bingo.html")
	num := 0
	out := rxCell.ReplaceAllStringFunc(string(hdr), func(s string) string {
		row, col := num/5, num%5
		letter := letters[num]
		num++

		cellText := string(rune(letter))

		var class string
		freeSquare := row == 2 && col == 2
		if freeSquare {
			class = "word"
			cellText = "Free Square"
		}
		marked := rnd.Intn(2) == 0 || freeSquare
		if marked {
			class += " marked"
		}
		return fmt.Sprintf("<td class='%s'>%s</td>", class, cellText)
	})

	io.WriteString(w, out)

	fmt.Fprintf(w, "<p>You are <b>%s</b> from <b>%s</b> (%s)</p>",
		html.EscapeString(who.UserProfile.LoginName),
		html.EscapeString(firstLabel(who.Node.ComputedName)),
		r.RemoteAddr)

}

var rxCell = regexp.MustCompile(`<td>\?</td>`)

//       <td id="c3r3" class="word">WireGuard<br>(Free Square)</td>

func firstLabel(s string) string {
	s, _, _ = strings.Cut(s, ".")
	return s
}
