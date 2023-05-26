// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strings"

	"tailscale.com/tsnet"
)

var (
	addr = flag.String("addr", ":443", "address to listen on")
)

func main() {
	flag.Parse()
	s := new(tsnet.Server)
	defer s.Close()
	ln, err := s.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	lc, err := s.LocalClient()
	if err != nil {
		log.Fatal(err)
	}

	if *addr == ":443" {
		ln = tls.NewListener(ln, &tls.Config{
			GetCertificate: lc.GetCertificate,
		})
	}
	log.Fatal(http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		who, err := lc.WhoIs(r.Context(), r.RemoteAddr)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_ = who
		hdr, _ := os.ReadFile("bingo.html")
		num := 0
		out := rxCell.ReplaceAllStringFunc(string(hdr), func(s string) string {
			num++
			row := (num - 1) / 5
			col := (num - 1) % 5

			letter := 'A' + rand.Intn(26)
			cellText := string(rune(letter))

			var class string
			freeSquare := row == 2 && col == 2
			if freeSquare {
				class = "word"
				cellText = "Free Square"
			}
			marked := rand.Intn(2) == 0 || freeSquare
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
	})))
}

var rxCell = regexp.MustCompile(`<td>\?</td>`)

//       <td id="c3r3" class="word">WireGuard<br>(Free Square)</td>

func firstLabel(s string) string {
	s, _, _ = strings.Cut(s, ".")
	return s
}
