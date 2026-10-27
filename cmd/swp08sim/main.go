// Command swp08sim is a minimal fake SW-P-08 router for development.
// It answers name requests, tally dumps, interrogates and extended connects
// on any matrix, and broadcasts Connected to every client. With
// SIM_PANEL_SECONDS it also makes routes of its own, like a control panel;
// with SIM_PANEL_SILENT=1 those aren't announced, as the Ultrix sometimes
// doesn't, so they only show in a tally dump or an interrogate.
package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

type sim struct {
	mu      sync.Mutex
	srcs    []string
	dsts    []string
	tally   map[int][]int // wire level → dst → src
	clients map[net.Conn]*sync.Mutex
	deaf    map[int]bool  // dsts whose connects are ACKed but never carried out
	silent  map[int]bool  // dsts whose connects are carried out but never announced
	follows map[int][]int // dst → dsts that follow it, like a multiviewer PiP
}

func envInt(k string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return n
	}
	return def
}

func main() {
	addr := ":" + strconv.Itoa(envInt("SIM_PORT", 8910))
	ns, nd, nl := envInt("SIM_SOURCES", 200), envInt("SIM_DESTS", 120), envInt("SIM_LEVELS", 17)

	s := &sim{tally: map[int][]int{}, clients: map[net.Conn]*sync.Mutex{}, deaf: map[int]bool{}, silent: map[int]bool{}, follows: map[int][]int{}}
	// SIM_IGNORE_DSTS=5,7 makes connects to those (0-based) destinations
	// silently fail, so Sode's timeout and retry paths can be exercised.
	for _, f := range strings.Split(os.Getenv("SIM_IGNORE_DSTS"), ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
			s.deaf[n] = true
		}
	}
	// SIM_SILENT_DSTS=5,7 makes those destinations switch without a
	// Connected, as the Ultrix sometimes does: Sode only learns of it from
	// the interrogate it sends when nothing was announced.
	for _, f := range strings.Split(os.Getenv("SIM_SILENT_DSTS"), ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
			s.silent[n] = true
		}
	}
	// SIM_FOLLOWS=2>40,3>41 (0-based): when dst 2 changes, dst 40 follows
	// it to the same source a moment later, as a router's destination
	// follow does.
	for _, f := range strings.Split(os.Getenv("SIM_FOLLOWS"), ",") {
		from, to, ok := strings.Cut(strings.TrimSpace(f), ">")
		a, err1 := strconv.Atoi(from)
		b, err2 := strconv.Atoi(to)
		if ok && err1 == nil && err2 == nil {
			s.follows[a] = append(s.follows[a], b)
		}
	}
	named := []string{"Black", "Bars", "CG1 Fill", "CG1 Key", "CG2 Fill", "CG2 Key", "CG3 Fill", "CG3 Key", "Replay A", "Replay B", "Zoom 1", "Zoom 2"}
	for i := 0; i < ns; i++ {
		switch {
		case i < len(named):
			s.srcs = append(s.srcs, named[i])
		case i < len(named)+12:
			s.srcs = append(s.srcs, fmt.Sprintf("CAM %d", i-len(named)+1))
		default:
			s.srcs = append(s.srcs, fmt.Sprintf("SRC %03d", i+1))
		}
	}
	dnamed := []string{"PGM", "PVW", "DSM1", "DSM2", "DSM3", "DSM4", "DSM5", "Zoom Return 1", "Zoom Return 2", "TX 1", "TX 2", "Rec A", "Rec B"}
	for i := 0; i < nd; i++ {
		if i < len(dnamed) {
			s.dsts = append(s.dsts, dnamed[i])
		} else {
			s.dsts = append(s.dsts, fmt.Sprintf("MV %d", i-len(dnamed)+1))
		}
	}
	for l := 0; l < 256 && l < nl+1; l++ {
		t := make([]int, nd)
		for d := range t {
			t[d] = d % ns
		}
		s.tally[l] = t
	}

	// SIM_PANEL_SECONDS=5 routes something every 5s as if from a panel:
	// one or a few destinations, all levels or a few. SIM_PANEL_DST (0-based,
	// default 3) gets every other punch, so repeats on a monitor show up.
	if n := envInt("SIM_PANEL_SECONDS", 0); n > 0 {
		go s.panel(time.Duration(n)*time.Second, envInt("SIM_PANEL_DST", 3), nl+1, envInt("SIM_PANEL_SILENT", 0) == 1)
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("swp08sim on %s: %d sources, %d destinations, levels 0-%d", addr, ns, nd, nl)
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go s.serve(c)
	}
}

func (s *sim) panel(every time.Duration, monitor, levels int, silent bool) {
	for i := 0; ; i++ {
		time.Sleep(every)
		dsts := []int{monitor}
		if i%2 == 1 {
			dsts = []int{rand.IntN(len(s.dsts))}
			for range rand.IntN(3) {
				dsts = append(dsts, rand.IntN(len(s.dsts)))
			}
		}
		ls := []int{}
		if rand.IntN(3) == 0 {
			for range 1 + rand.IntN(3) {
				ls = append(ls, rand.IntN(levels))
			}
		} else {
			for l := 0; l < levels; l++ {
				ls = append(ls, l)
			}
		}
		for _, dst := range dsts {
			src := rand.IntN(min(len(s.srcs), 24))
			log.Printf("panel: dst %d ← src %d on levels %v (silent %t)", dst, src, ls, silent)
			for _, l := range ls {
				s.route(0, l, dst, src, !silent)
			}
		}
	}
}

// route makes a crosspoint and, if it changed and announce is set, tells
// every client.
func (s *sim) route(mx, l, dst, src int, announce bool) {
	s.mu.Lock()
	t, ok := s.tally[l]
	valid := ok && dst < len(t) && src < len(s.srcs)
	changed := valid && t[dst] != src
	if valid {
		t[dst] = src
	}
	var all []net.Conn
	for cc := range s.clients {
		all = append(all, cc)
	}
	s.mu.Unlock()
	if !valid {
		log.Printf("connect out of range: level %d dst %d src %d", l, dst, src)
		return
	}
	// Like a real router, only a change is announced.
	if !changed {
		log.Printf("connect level %d: dst %d already on src %d (no Connected)", l, dst, src)
		return
	}
	log.Printf("connect level %d: dst %d ← src %d", l, dst, src)
	if announce {
		for _, cc := range all {
			s.write(cc, swp08.Encode(swp08.ExtConnected(mx, l, dst, src)))
		}
	}
	for _, f := range s.follows[dst] {
		time.AfterFunc(30*time.Millisecond, func() { s.route(mx, l, f, src, announce) })
	}
}

func (s *sim) write(c net.Conn, b []byte) {
	s.mu.Lock()
	wmu := s.clients[c]
	s.mu.Unlock()
	if wmu == nil {
		return
	}
	wmu.Lock()
	c.Write(b)
	wmu.Unlock()
}

func (s *sim) serve(c net.Conn) {
	log.Printf("client %s connected", c.RemoteAddr())
	s.mu.Lock()
	s.clients[c] = &sync.Mutex{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, c)
		s.mu.Unlock()
		c.Close()
		log.Printf("client %s gone", c.RemoteAddr())
	}()

	var dec swp08.Decoder
	buf := make([]byte, 4096)
	for {
		n, err := c.Read(buf)
		if err != nil {
			return
		}
		dec.Feed(buf[:n], func(f swp08.Frame) {
			switch f.Kind {
			case swp08.KindBad:
				s.write(c, swp08.NakFrame)
			case swp08.KindMsg:
				s.write(c, swp08.AckFrame)
				s.handle(c, f.Msg)
			}
		})
	}
}

func (s *sim) handle(c net.Conn, m []byte) {
	reply := func(msg []byte) { s.write(c, swp08.Encode(msg)) }
	switch m[0] {
	case swp08.CmdExtConnect:
		if len(m) < 7 {
			return
		}
		mx, l := int(m[1]), int(m[2])
		dst, src := int(m[3])<<8|int(m[4]), int(m[5])<<8|int(m[6])
		if s.deaf[dst] {
			log.Printf("connect level %d: dst %d ← src %d ignored (SIM_IGNORE_DSTS)", l, dst, src)
			return
		}
		s.route(mx, l, dst, src, !s.silent[dst])

	case swp08.CmdExtInterrogate:
		if len(m) < 5 {
			return
		}
		mx, l, dst := int(m[1]), int(m[2]), int(m[3])<<8|int(m[4])
		s.mu.Lock()
		t := s.tally[l]
		src := -1
		if dst < len(t) {
			src = t[dst]
		}
		s.mu.Unlock()
		if src >= 0 {
			reply(swp08.ExtTally(mx, l, dst, src))
		}

	case swp08.CmdExtTallyDumpReq:
		if len(m) < 3 {
			return
		}
		mx, l := int(m[1]), int(m[2])
		s.mu.Lock()
		t := append([]int(nil), s.tally[l]...)
		s.mu.Unlock()
		for i := 0; i < len(t); i += 64 {
			reply(swp08.ExtTallyDump(mx, l, i, t[i:min(i+64, len(t))]))
		}

	case swp08.CmdExtProtectDumpReq:
		// Destination 12 ("Rec B") is protected by device 9 on every level,
		// packed as the Ultrix does: state in bits 4-5, then the device.
		if len(m) < 3 {
			return
		}
		mx, l := int(m[1]), int(m[2])
		for i := 0; i < len(s.dsts); i += 64 {
			n := min(64, len(s.dsts)-i)
			msg := []byte{swp08.CmdExtProtectDump, byte(mx), byte(l), byte(n), byte(i >> 8), byte(i)}
			for d := i; d < i+n; d++ {
				if d == 12 {
					msg = append(msg, 0x20, 0x09)
				} else {
					msg = append(msg, 0, 0)
				}
			}
			reply(msg)
		}

	case swp08.CmdSrcNamesReq, swp08.CmdDstNamesReq, swp08.CmdExtSrcNamesReq, swp08.CmdExtDstNamesReq:
		dest := m[0] == swp08.CmdDstNamesReq || m[0] == swp08.CmdExtDstNamesReq
		ext := m[0] >= 0x80
		code := m[len(m)-1] & 3
		list := s.srcs
		if dest {
			list = s.dsts
		}
		per := swp08.NamesPerMsg(code)
		for i := 0; i < len(list); i += per {
			reply(swp08.NamesResp(dest, ext, 0, 0, code, i, list[i:min(i+per, len(list))]))
		}

	default:
		log.Printf("ignored cmd 0x%02X", m[0])
	}
}
