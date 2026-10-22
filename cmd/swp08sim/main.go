// Command swp08sim is a minimal fake SW-P-08 router for development.
// It answers name requests, tally dumps, interrogates and extended connects
// on any matrix, and broadcasts Connected to every client.
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/sethdotfm/sode/internal/swp08"
)

type sim struct {
	mu      sync.Mutex
	srcs    []string
	dsts    []string
	tally   map[int][]int // wire level → dst → src
	clients map[net.Conn]*sync.Mutex
	deaf    map[int]bool // dsts whose connects are ACKed but never carried out
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

	s := &sim{tally: map[int][]int{}, clients: map[net.Conn]*sync.Mutex{}, deaf: map[int]bool{}}
	// SIM_IGNORE_DSTS=5,7 makes connects to those (0-based) destinations
	// silently fail, so Sode's timeout and retry paths can be exercised.
	for _, f := range strings.Split(os.Getenv("SIM_IGNORE_DSTS"), ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
			s.deaf[n] = true
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
		s.mu.Lock()
		t, ok := s.tally[l]
		valid := ok && dst < len(t) && src < len(s.srcs)
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
		log.Printf("connect level %d: dst %d ← src %d", l, dst, src)
		for _, cc := range all {
			s.write(cc, swp08.Encode(swp08.ExtConnected(mx, l, dst, src)))
		}

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
