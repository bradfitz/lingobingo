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

	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"
)

func main() {
	flag.Parse()
	s := new(tsnet.Server)
	defer s.Close()

	ln80, err := s.Listen("tcp", ":80")
	if err != nil {
		log.Fatal(err)
	}
	defer ln80.Close()

	// ln443, err := s.Listen("tcp", ":443")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer ln443.Close()

	lc, err := s.LocalClient()
	if err != nil {
		log.Fatal(err)
	}

	lnFunnel, err := s.ListenFunnel("tcp", ":443")
	if err != nil {
		log.Fatal(err)
	}
	defer lnFunnel.Close()

	// lnTLS := tls.NewListener(ln443, &tls.Config{
	// 	GetCertificate: lc.GetCertificate,
	// })

	bs := &bingoServer{
		lc: lc,
	}

	errc := make(chan error, 1)
	go func() { errc <- http.Serve(lnFunnel, bs) }()
	//	go func() { errc <- http.ServeTLS(lnTLS, bs, "", "") }()
	go func() { errc <- http.Serve(ln80, bs) }()

	log.Fatal(<-errc)
}

type bingoServer struct {
	lc *tailscale.LocalClient
}

var crc64Table = crc64.MakeTable(crc64.ISO)

func (s *bingoServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	who, _ := s.lc.WhoIs(r.Context(), r.RemoteAddr)
	var gameBoard string
	if who != nil {
		if firstLabel(who.Node.ComputedName) == "funnel-ingress-node" {
			log.Printf("Funnel headers: %q", r.Header)
		}
		gameBoard = who.UserProfile.LoginName
	} else {
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err != nil {
			gameBoard = host
		} else {
			gameBoard = r.RemoteAddr
		}
	}

	log.Printf("TLS=%v, Board: %q", r.TLS != nil, gameBoard)

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
