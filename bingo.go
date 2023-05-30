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
	"bytes"
	"context"
	crand "crypto/rand"
	"flag"
	"fmt"
	"hash/crc64"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"golang.org/x/exp/slices"
	"golang.org/x/net/websocket"
	"tailscale.com/client/tailscale"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn"
	"tailscale.com/tsnet"
	"tailscale.com/types/logger"
)

var (
	verbose      = flag.Bool("verbose", false, "verbose")
	doPresent    = flag.Bool("present", true, "present")
	useTailscale = flag.Bool("tailscale", true, "use tailscale")
	startSlide   = flag.Int("slide", 0, "slide to start on (1-based)")
)

type slide struct {
	idx    int // 0-based
	msg    string
	id     slideID // set for certain key slides
	letter letter  // non-zero for slides adding a letter
}

func (s *slide) OnOrAfter(id slideID) bool {
	target, ok := slideByID[id]
	if !ok {
		panic("unknown slide id: " + string(id))
	}
	return s.idx >= target.idx
}

type slideID string

type letter byte
type L = letter

var (
	slideGameOn = slideID("game-on")
	slideURL    = slideID("url")
)

// ss constructs a slide.
func ss(msg string, arg ...any) *slide {
	s := &slide{msg: msg}
	for _, a := range arg {
		switch v := a.(type) {
		case slideID:
			s.id = v
		case letter:
			s.letter = v
			if !strings.Contains(s.msg, "\u0332") {
				if strings.Contains(s.msg, string(s.letter)) {
					s.msg = strings.Replace(s.msg, string(s.letter), string(s.letter)+"\u0332", 1)
				} else {
					lower := strings.ToLower(string(s.letter))
					s.msg = strings.Replace(s.msg, lower, lower+"\u0332", 1)
				}
			}
		default:
			panic(fmt.Sprintf("unknown type %T", a))
		}
	}
	return s
}

var slides = []*slide{
	ss("A lightning talk\n⚡️\nBrad Fitzpatrick"),
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
	ss("play.bingo.ts.net", slideURL),
	ss("play.bingo.ts.net\n\n⚡️ 😬⚡️\n"),
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
	ss("Game on! 🏒", slideGameOn),
	ss("tailscaled"),
	ss("tailscaled\n\n* slaps hood *"),
	ss("tailscaled\n\n\"we can get so many protocols\nin this daemon...\""),
	ss("Wireguard", L('W')),
	ss("IP\n(the data plane)", L('I')),
	ss("IPv4", L('4')),
	ss("IPv6", L('6')),
	ss("HTTP client\n(the control plane)", L('H')),
	ss("HTTP server\n(PeerAPI, LocalAPI, ...)", L('H')),
	ss("HTTP server\n(Tailscale Funnel)", L('F')),
	ss("ACME", L('A')),
	ss("HTTP/1", L('1')),
	ss("HTTP/2", L('2')),
	ss("TLS", L('T')),
	ss("Noise", L('N')),
	ss("MagicDNS"),
	ss("DNS\n", L('D')),
	ss("DNS\nServer + Client", L('D')),
	ss("DoH\n", L('D')),
	ss("DoH\n1.1.1.1", L('1')),
	ss("DoH\n8.8.8.8", L('8')),
	ss("NAT Traversal"),
	ss("UDP", L('U')),
	ss("DERP", L('D')),
	ss("Disco", L('D')),
	ss("Disco pings", L('D')),
	ss("ICMP", L('I')),
	ss("TSMP\n", L('T')),
	ss("TSMP\nIANA proto 99", L('9')),
	ss("NAT-PMP", L('N')),
	ss("PCP", L('P')),
	ss("UPnP\n", L('U')),
	ss("UPnP\n(XML)", L('X')),
	ss("L3", L('3')),
	ss("TUN", L('T')),
	ss("L2", L('2')),
	ss("Ethernet", L('E')),
	ss("MAC", L('M')),
	ss("TAP", L('T')),
	ss("ARP", L('A')),
	ss("DHCP", L('D')),
	ss("No TUN/TAP?"),
	ss("Userspace mode"),
	ss("gVisor", L('V')),
	ss("netstack", L('N')),
	ss("Two outbound\nproxy options..."),
	ss("HTTP CONNECT", L('C')),
	ss("SOCKS5", L('5')),
	ss("Tailscale SSH", L('S')),
	ss("scp"),
	ss("SFTP", L('S')),

	ss("Platforms"),
	ss("Linux", L('L')),
	ss("macOS", L('M')),
	ss("Windows", L('W')),
	ss("iOS", L('I')),
	ss("Android", L('A')),
	ss("FreeBSD", L('F')),
	ss("OpenBSD", L('F')),
	ss("WASM", L('W')),
	ss("WebSockets", L('W')),

	ss("Synology", L('S')),
	ss("QNAP", L('Q')),
	ss("pfSense", L('P')),
	ss("OPNsense", L('O')),
	ss("TrueNAS", L('T')), // XXX check spelling

	ss("Clouds"),
	ss("AWS", L('A')),
	ss("Azure", L('A')),
	ss("GCP", L('G')),

	ss("Misc"),
	ss("Prometheus", L('P')),
	ss("BGP, bird", L('B')),

	ss("🤣"),
	ss("We all win with\n✨ Tailscale ✨"),
}

/*
ACME (LetsEncrypt certs)
ARP (TAP)
BGP/BIRD
CGNAT
CONNECT (outbound HTTP proxy)
DERP
DHCP (TAP)
DNS (client, server)
Ethernet (TAP)
Funnel (TLS SNI)
GRE
GCP
Happy Eyeballs
HTTP
HTTPS
HTTP/2
ICMP
IPv4, IPv6 (gvisor)
JSON
Jitter (on most protocols)
KeepAlive (TCP, DERP, …)
Key exchange
Logz
L4
LAN
Linux
Machine API
MTU
NaCl crypto_secretbox
NAT
NAT-PMP
Noise
Netlink
OAuth (tailscale up authkey)
PeerAPI
PCAP
PCP
Prometheus (metrics exporter)
QUIC (but maybe not yet)
RDP
Router Advertisements (IPv6)
TCP (gvisor, filter)
TSMP
SFTP
SOCKS5
SSH
SQLite
UDP (gvisor, filter)
UPnP
Virtio (UDP Offloading in WireGuard-go)
WebSockets (DERP)
WireGuard
XDG Base Directory Specification
YAML
Zeroconf (primarily ignoring it for now in MagicDNS server, want to forward it later)

Hold:
Z, Q, V (Virtio), J (JSON),
*/

var (
	slideByID            = map[slideID]*slide{}
	goodLettersWithBingo []letter
	goodLetters          []letter // good letters except the last one
	deadLetters          []letter
	bingoLetter          letter
)

func init() {
	saw := map[letter]int{}
	for i, s := range slides {
		s.idx = i
		if s.id != "" {
			slideByID[s.id] = s
		}
		if v := s.letter; v != 0 {
			saw[v]++
			if saw[v] == 1 {
				goodLettersWithBingo = append(goodLettersWithBingo, v)
			}
		}
	}

	for v := letter('A'); v <= 'Z'; v++ {
		if saw[v] == 0 {
			deadLetters = append(deadLetters, v)
		}
	}
	for v := letter('0'); v <= '9'; v++ {
		if saw[v] == 0 {
			deadLetters = append(deadLetters, v)
		}
	}
	// Last good letter must be unique.
	bingoLetter = goodLettersWithBingo[len(goodLettersWithBingo)-1]
	if saw[bingoLetter] > 1 {
		panic(fmt.Sprintf("final letter %q used multiple times", string(bingoLetter)))
	}
	goodLetters = goodLettersWithBingo[:len(goodLettersWithBingo)-1]
}

func main() {
	flag.Parse()

	bs := &bingoServer{
		gameEv:     make(chan any, 8),
		letterSeen: make(map[letter]bool),
	}
	if *startSlide > 0 {
		bs.slide = *startSlide - 1
	}

	if !*useTailscale {
		errc := make(chan error)
		if *doPresent {
			go func() { errc <- bs.present() }()
		}
		go func() { errc <- http.ListenAndServe(":5859", bs) }()
		log.Fatal(<-errc)
	}
	s := &tsnet.Server{
		Dir:      "/Users/bradfitz/Library/Application Support/tsnet-lingobingo",
		Hostname: "bingo",
		Logf:     logger.Discard,
	}
	if *verbose {
		s.Logf = log.Printf
	}
	defer s.Close()

	log.SetFlags(0)

	log.Printf("LC...")
	var err error
	bs.lc, err = s.LocalClient()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("80...")
	ln80, err := s.Listen("tcp", ":80")
	if err != nil {
		log.Fatal(err)
	}
	defer ln80.Close()

	log.Printf("443...")
	lnFunnel, err := s.ListenFunnel("tcp", ":443")
	if err != nil {
		log.Fatal(err)
	}
	defer lnFunnel.Close()

	log.Printf("Watch...")
	watcher, err := bs.lc.WatchIPNBus(context.Background(), ipn.NotifyWatchEngineUpdates|ipn.NotifyInitialNetMap|ipn.NotifyInitialState)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			not, err := watcher.Next()
			if err != nil {
				log.Printf("watcher: %v\n", err)
				return
			}
			bs.gameEv <- not
		}
	}()

	log.Printf("Showtime.")
	time.Sleep(500 * time.Millisecond)

	var httpServer http.Server
	httpServer.Handler = bs
	httpServer.ErrorLog = log.New(io.Discard, "", 0)
	if *verbose {
		httpServer.ErrorLog = nil // use default
	}

	errc := make(chan error, 1)
	go func() { errc <- httpServer.Serve(lnFunnel) }()
	go func() { errc <- httpServer.Serve(ln80) }()
	if *doPresent {
		go func() { errc <- bs.present() }()
	}

	log.Fatal(<-errc)
}

func (bs *bingoServer) advanceSlide(delta int) {
	bs.quitKey = nil
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
		wid := runeWidth(r)
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

func runeWidth(r rune) int {
	switch r {
	case '🛝':
		return 2
	case '\U0001f32a': // cyclone (Funnel)
		return 2
	}
	return runewidth.RuneWidth(r)
}

func stringCells(s string) (n int) {
	for _, r := range s {
		n += runeWidth(r)
	}
	return n
}

// render either does a render now, or schedules one soon if the most previous was was too recent.
func (bs *bingoServer) render() {
	if bs.renderPending {
		return
	}
	const tooQuick = 50 * time.Millisecond
	if time.Since(bs.lastRender) > tooQuick {
		bs.doRender()
		return
	}
	bs.renderPending = true
	time.AfterFunc(tooQuick, func() {
		bs.gameEv <- bs.doRender
	})
}

func (bs *bingoServer) doRender() {
	bs.lastRender = time.Now()
	bs.renderPending = false
	curSlide := slides[bs.slide]
	msg := curSlide.msg

	if strings.Contains(msg, "⏳") {
		if bs.clock != nil {
			msg = strings.ReplaceAll(msg, "-:--", fmt.Sprintf("%1d:%02d", bs.secRemain/60, bs.secRemain%60))
		}
	}

	bs.paintWithMsg(msg)
	width, height := bs.sc.Size()
	if curSlide.OnOrAfter(slideGameOn) {
		if letter := curSlide.letter; letter != 0 {
			bs.letterSeen[letter] = true
		}
		bs.writeString(width-10, height-1, fmt.Sprintf("🔤%d", len(bs.letterSeen)), tcell.StyleDefault)
	}
	if curSlide.OnOrAfter(slideURL) {
		bs.writeString(0, height-1,
			fmt.Sprintf("🎮%d (%v + 🌪️%d)",
				len(bs.players),
				bs.tsPlayers,
				len(bs.players)-bs.tsPlayers,
			),
			tcell.StyleDefault)
		if !curSlide.OnOrAfter(slideGameOn) {
			y := 0
			for i := len(bs.joined) - 1; i >= 0; i-- {
				bs.writeString(0, y, fmt.Sprintf("👋 %s", bs.joined[i]), tcell.StyleDefault)
				y++
				if y == 2 {
					break
				}
			}
		}
	}
	if bs.slide > 0 {
		bs.writeString(width-5, height-1, fmt.Sprintf("🛝%d", bs.slide+1), tcell.StyleDefault)
	}
	if bs.showSize {
		bs.writeString(0, 0, "┏", tcell.StyleDefault)
		bs.writeString(width-1, 0, "┓", tcell.StyleDefault)
		bs.writeString(0, height-1, "┗", tcell.StyleDefault)
		bs.writeString(width-1, height-1, "┛", tcell.StyleDefault)
		bs.writeString(width/2-4, 1, fmt.Sprintf("%dx%d", width, height), tcell.StyleDefault.Foreground(tcell.ColorDarkGray))

		stateIcon := fmt.Sprintf("%d %s 🔴", bs.notifies, bs.state)
		switch bs.state {
		case ipn.Running:
			stateIcon = "🟢"
		case ipn.Starting:
			stateIcon = "🟡"
		}

		bs.writeString(width-1-stringCells(stateIcon), 1, stateIcon, tcell.StyleDefault)
	}

	bs.sc.Sync()
}

func (bs *bingoServer) paintWithMsg(msg string) {
	sc := bs.sc
	sc.Fill(' ', tcell.StyleDefault)

	width, height := sc.Size()

	lines := strings.Split(msg, "\n")
	midy := int(float64(height)/2.0 - float64(len(lines))/2 - 0.5)
	if midy < 1 {
		midy = 1
	}
	for y, lineMsg := range lines {
		midx := width / 2
		start := midx - stringCells(lineMsg)/2
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
	go bs.loop(evc)
	bs.sc.ChannelEvents(evc, quitc)
	select {}
}

func (bs *bingoServer) loop(evc <-chan tcell.Event) {
	bs.sc.Clear()

	for {
		select {
		case ev := <-bs.gameEv:
			switch ev := ev.(type) {
			case func():
				ev()
				continue
			case ipn.Notify:
				bs.handleNotify(ev)
			case playerChangeEvent:
				bs.handlePlayerChange(ev)
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
					case 't':
						bs.troll()
						continue
					case 's':
						bs.showSize = !bs.showSize
						bs.render()
						continue
					case 'q':
						was := bs.quitKey
						bs.quitKey = bs.quitKey[:0]
						for _, t := range was {
							if time.Since(t) < 5*time.Second {
								bs.quitKey = append(bs.quitKey, t)
							}
						}
						bs.quitKey = append(bs.quitKey, time.Now())
						if len(bs.quitKey) == 5 {
							bs.sc.Fini()
							os.Exit(0)
						}
						continue
					}
					bs.paintWithMsg(fmt.Sprintf("Rune: %q", r))
					bs.sc.Sync()
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
}

type bingoServer struct {
	lc            *tailscale.LocalClient
	gameEv        chan any
	letterSeen    map[letter]bool
	lastRender    time.Time
	renderPending bool

	sc    tcell.Screen
	slide int

	state     ipn.State
	numPeers  int
	notifies  int
	quitKey   []time.Time
	secRemain int
	showSize  bool
	clock     *time.Timer
	players   map[*player]bool
	tsPlayers int      // number of players that are over Tailscale
	joined    []string // latest join last
}

func (s *bingoServer) addPlayer(p *player) {
	s.gameEv <- playerChangeEvent{p, true}
}

func (s *bingoServer) removePlayer(p *player) {
	s.gameEv <- playerChangeEvent{p, false}
}

type playerChangeEvent struct {
	p      *player
	online bool
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

func (s *bingoServer) handleNotify(n ipn.Notify) {
	s.notifies++
	if n.State != nil {
		s.state = *n.State
	}
	if n.NetMap != nil {
		// TODO
	}
}

func (s *bingoServer) handlePlayerChange(ev playerChangeEvent) {
	if ev.online {
		if s.players == nil {
			s.players = map[*player]bool{}
		}
		s.players[ev.p] = true
		if ev.p.emailAt != "" {
			s.tsPlayers++
		}
		if e := ev.p.emailAt; e != "" && !slices.Contains(s.joined, e) {
			s.joined = append(s.joined, e)
		}
	} else {
		delete(s.players, ev.p)
		if ev.p.emailAt != "" {
			s.tsPlayers--
		}
	}
	s.render()
}

func (s *bingoServer) troll() {
	for p := range s.players {
		select {
		case p.ch <- "document.location.href = 'https://upload.wikimedia.org/wikipedia/en/9/9a/Trollface_non-free.png';":
		default:
		}
	}
}

var crc64Table = crc64.MakeTable(crc64.ISO)

type player struct {
	ws      *websocket.Conn
	s       *bingoServer
	board   board
	ch      chan string // JS to eval :)
	emailAt string      // empty for funnel, else like "bradfitz@" for over Tailscale
}

func (s *bingoServer) serveWebSocket(ws *websocket.Conn) {
	defer ws.Close()
	req := ws.Request()

	var who *apitype.WhoIsResponse
	var emailAt string // or empty
	if s.lc != nil {
		who, _ = s.lc.WhoIs(req.Context(), req.RemoteAddr)
		if isOverTailscale(who) {
			emailAt = who.UserProfile.LoginName
			if i := strings.Index(emailAt, "@"); i != -1 {
				emailAt = emailAt[:i+1]
			}
		}
	}

	gc, _ := req.Cookie("game")
	var game string
	if gc != nil {
		game = gc.Value
	} else {
		return
	}

	ch := make(chan string, 128)
	done := make(chan bool)
	defer close(done)
	board := NewBoard(game)
	p := &player{
		ws:      ws,
		s:       s,
		board:   board,
		ch:      ch,
		emailAt: emailAt,
	}

	if *verbose {
		log.Printf("websocket from %v, game: %v", req.RemoteAddr, game)
		defer log.Printf("websocket GONE for game %v", game)
	}

	s.addPlayer(p)
	defer s.removePlayer(p)

	go func() {
		for {
			select {
			case <-done:
				return
			case m := <-p.ch:
				if err := websocket.Message.Send(ws, m); err != nil {
					return
				}
			}
		}
	}()

	var message string
	websocket.Message.Receive(ws, &message)
}

func (s *bingoServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(strings.ToLower(r.Header.Get("Upgrade")), "websocket") {
		websocket.Handler(s.serveWebSocket).ServeHTTP(w, r)
		return
	}
	c, _ := r.Cookie("game")
	if c == nil || r.URL.Query().Get("new") == "1" {
		buf := make([]byte, 8)
		crand.Read(buf)
		c = &http.Cookie{
			Name:  "game",
			Value: fmt.Sprintf("%x", buf),
		}
		http.SetCookie(w, c)
	}
	gameBoard := c.Value

	var who *apitype.WhoIsResponse
	if s.lc != nil {
		who, _ = s.lc.WhoIs(r.Context(), r.RemoteAddr)
	}

	//log.Printf("TLS=%v, Board: %q", r.TLS != nil, gameBoard)

	letters := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")

	rnd := rand.New(rand.NewSource(int64(crc64.Checksum([]byte(gameBoard), crc64Table))))
	rnd.Shuffle(len(letters), reflect.Swapper(letters))

	hdr, _ := os.ReadFile("bingo.html")
	js, _ := os.ReadFile("bingo.js")
	if *useTailscale == false {
		js = bytes.Replace(js, []byte("wss://play.bingo.ts.net/"), []byte("ws://127.0.0.1:5859/"), 1)
	}
	num := 0
	out := rxCell.ReplaceAllFunc(hdr, func(_ []byte) []byte {
		row, col := num/5, num%5
		letter := letters[num]
		num++

		cellText := string(rune(letter))

		var class string
		freeSquare := row == 2 && col == 2
		var marked bool
		if freeSquare {
			if isOverTailscale(who) {
				class = "word"
				cellText = "Free Square"
				marked = true
			} else {
				cellText = `<a class='bonus' href="https://login.tailscale.com/admin/invite/MmtyZnexxM1">🍄</a>`
			}
		}
		if marked {
			class += " marked"
		}
		return fmt.Appendf(nil, "<td class='%s'>%s</td>", class, cellText)
	})
	out = bytes.Replace(out, []byte("<script></script>"),
		fmt.Appendf(nil, "<script>\n%s\n</script>", js), 1)

	w.Write(out)

	// fmt.Fprintf(w, "<p>You are <b>%s</b> from <b>%s</b> (%s)</p>",
	// 	html.EscapeString(who.UserProfile.LoginName),
	// 	html.EscapeString(firstLabel(who.Node.ComputedName)),
	// 	r.RemoteAddr)
}

var rxCell = regexp.MustCompile(`<td>\?</td>`)

func firstLabel(s string) string {
	s, _, _ = strings.Cut(s, ".")
	return s
}

type board [5][5]letter // [y][x]

type pos struct{ x, y int }

var (
	center   = pos{2, 2}
	bingoPos = []pos{{1, 0}, {3, 0}, {0, 1}, {4, 1}, {0, 3}, {4, 3}, {1, 4}, {3, 4}}
	ex1      = []pos{{0, 0}, {1, 1}, {3, 3}, {4, 4}}
	ex2      = []pos{{4, 0}, {3, 1}, {1, 3}, {0, 4}}
)

func NewBoard(s string) board {
	rnd := rand.New(rand.NewSource(int64(crc64.Checksum([]byte(s), crc64Table))))
	var b board

	// First pass: mark certain squares with one of these.
	// They'll later be replaced with actual letters.
	const (
		cellBingo = 'B' // "Bingo!" (final win)
		cellWin   = 'w' // win path
		cellKill  = 'X'
	)

	// Pick final Bingo win position.
	bp := bingoPos[rnd.Intn(len(bingoPos))]
	// Mark column & row as not blocked, so they can win.
	for x := 0; x < 5; x++ {
		b[bp.y][x] = cellWin
	}
	for y := 0; y < 5; y++ {
		b[y][bp.x] = cellWin
	}
	b[bp.y][bp.x] = cellBingo

	// Eliminate a cell on both diagonals.
	var xkill [5]bool
	var ykill [5]bool
	for {
		p := ex1[rnd.Intn(len(ex1))]
		if b[p.y][p.x] != 0 {
			continue
		}
		b[p.y][p.x] = cellKill
		ykill[p.y] = true
		xkill[p.x] = true
		break
	}
	for {
		p := ex2[rnd.Intn(len(ex2))]
		if b[p.y][p.x] != 0 || xkill[p.x] || ykill[p.y] {
			continue
		}
		b[p.y][p.x] = cellKill
		ykill[p.y] = true
		xkill[p.x] = true
		break
	}
	// Now pick two more.
	for {
		p1 := pos{rnd.Intn(5), rnd.Intn(5)}
		p2 := pos{rnd.Intn(5), rnd.Intn(5)}
		if p1 == center || p2 == center ||
			p1.x == p2.x ||
			p1.y == p2.y ||
			b[p1.y][p1.x] != 0 ||
			b[p2.y][p2.x] != 0 ||
			xkill[p1.x] || ykill[p1.y] ||
			xkill[p2.x] || ykill[p2.y] {
			continue
		}
		b[p1.y][p1.x] = cellKill
		b[p2.y][p2.x] = cellKill
		break
	}

	// Now fill with letters.
	good := append([]letter(nil), goodLetters...)
	dead := append([]letter(nil), deadLetters...)
	rnd.Shuffle(len(good), reflect.Swapper(good))
	rnd.Shuffle(len(dead), reflect.Swapper(dead))

	for y := range b {
		row := &b[y]
		for x := range row {
			var n letter
			switch b[y][x] {
			case cellBingo:
				n = bingoLetter
			case cellKill:
				n = dead[0]
				dead = dead[1:]
			default:
				n = good[0]
				good = good[1:]
			}
			b[y][x] = n
		}
	}

	return b
}

func (b board) String() string {
	var buf strings.Builder
	for _, row := range b {
		for _, cell := range row {
			b := byte(cell)
			if b == 0 {
				b = '.'
			}
			buf.WriteByte(b)
		}
		buf.WriteByte('\n')
	}
	return buf.String()
}

// isOverTailscale reports whether who is over Tailscale (but not over Funnel).
func isOverTailscale(who *apitype.WhoIsResponse) bool {
	return who != nil && firstLabel(who.Node.ComputedName) != "funnel-ingress-node"
}
