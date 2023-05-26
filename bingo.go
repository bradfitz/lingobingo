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

	"github.com/gdamore/tcell/v2"
	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"
	"tailscale.com/types/logger"
)

var (
	verbose     = flag.Bool("verbose", false, "verbose")
	presentOnly = flag.Bool("present", false, "present only")
)

var slides = []string{
	"Lingo Bingo\n\nBrad Fitzpatrick",
	"play.bingo.ts.net",
	"oh hi",
}

func main() {
	flag.Parse()

	bs := &bingoServer{
		gameEv: make(chan any, 8),
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
	bs.msg = slides[n]
	bs.render()
}

func (bs *bingoServer) render() {
	bs.paintWithMsg(bs.msg)
}

func (bs *bingoServer) paintWithMsg(msg string) {
	sc := bs.sc
	sc.Fill(' ', tcell.StyleDefault)
	width, height := sc.Size()
	mid := width / 2
	start := mid - len(msg)/2
	if start < 0 {
		start = 0
	}
	midy := height/2 - 3
	if midy < 0 {
		midy = 0
	}
	for i, r := range msg {
		sc.SetContent(start+i, midy, r, nil, tcell.StyleDefault)
	}
	sc.Show()
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

	bs.setSlide(0)

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
						bs.paintWithMsg(fmt.Sprintf("Rune: %q", r))
						if r == 'q' {
							bs.sc.Fini()
							os.Exit(0)
						}
					default:
						bs.paintWithMsg(fmt.Sprintf("Key: %d", k))
					}
				case *tcell.EventResize:
					bs.render()
				default:
					bs.paintWithMsg(fmt.Sprintf("ev: %T: %v", ev, ev))
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
	msg   string
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
