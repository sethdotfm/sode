// Command swp08sim is a fake SW-P-08 router for development. Group salvos (120-123, extended stage 248/250)
// mimic the Ultrix. All clients share one staged list per group.
// Protect messages (138-143) mimic the Ultrix. Each message covers one level. Replies have 7 bytes: matrix, level,
// details, dst(2), device(2). Details is the kind: 1 a Protect, 2 a Router lock. 0x8D and 0x8F go to every client
// that has sent a message. Only the device that protected a level can clear it. A connect to a Protected level gets
// an ACK and no other reply, and the route stays. Dst 12 ("Rec B") starts with a Router lock by device 9 on every
// level; SIM_PROTECTS sets others.
package main

import (
	"cmp"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

type sim struct {
	mu         sync.Mutex
	srcs       []string
	dsts       []string
	tally      map[int][]int // wire level → dst → src
	clients    map[net.Conn]*sync.Mutex
	spoke      map[net.Conn]bool     // clients that have sent a message; only they get 0x8D and 0x8F
	protect    map[[2]int]hold       // wire level, dst → who protected it
	deaf       map[int]bool          // dsts: the simulator ACKs a connect but does not do it
	silent     map[int]bool          // dsts: the simulator does a connect but does not announce it
	slow       map[int]time.Duration // dsts: the simulator does a connect but announces it late
	follows    map[int][]int         // dst → dsts that follow it, like a multiviewer PiP
	staged     map[int][]swp08.Staged
	noSalvo    bool
	announceGo bool
	namesFile  string
}

// hold is one protected level of one dst. Details is byte 3 of the Ultrix's 0x8B. It is 1 after a 0x8C. It is 2
// for a hold from a panel or DashBoard, as on dst 12. The dump packs it in bits 4-5.
type hold struct{ details, device int }

const salvoDelay = 6 * time.Millisecond

func envInt(k string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return n
	}
	return def
}

func main() {
	addr := ":" + strconv.Itoa(envInt("SIM_PORT", 8910))
	ns, nd, nl := envInt("SIM_SOURCES", 200), envInt("SIM_DESTS", 120), envInt("SIM_LEVELS", 17)

	s := newSim(ns, nd, nl)
	s.noSalvo, s.announceGo, s.namesFile = os.Getenv("SIM_NO_SALVO") == "1", os.Getenv("SIM_SALVO_ANNOUNCE") == "1", os.Getenv("SIM_NAMES_FILE")
	// SIM_IGNORE_DSTS=5,7 (0-based): the simulator ACKs a connect but does not do it, to test timeout and retry.
	for _, f := range strings.Split(os.Getenv("SIM_IGNORE_DSTS"), ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
			s.deaf[n] = true
		}
	}
	// SIM_PROTECTS=12:2:9,20:1:112 (0-based dst:kind:device): that dst starts Protected on every level. Kind 1 is a
	// Protect, 2 a Router lock, 0 clears dst 12's default.
	for _, f := range strings.Split(os.Getenv("SIM_PROTECTS"), ",") {
		if dst, kind, dev, ok := protectSpec(f); ok {
			s.holdAll(dst, kind, dev)
		}
	}
	// SIM_SILENT_DSTS=5,7: a connect sends no Connected, so Sode learns of it only from its interrogate.
	for _, f := range strings.Split(os.Getenv("SIM_SILENT_DSTS"), ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
			s.silent[n] = true
		}
	}
	// SIM_SLOW_DSTS=9:6000,11:8000 (0-based dst:ms): the connect is immediate and its Connected comes after the delay.
	// Interrogate replies stay immediate.
	for _, f := range strings.Split(os.Getenv("SIM_SLOW_DSTS"), ",") {
		dst, ms, ok := strings.Cut(strings.TrimSpace(f), ":")
		n, err1 := strconv.Atoi(dst)
		d, err2 := strconv.Atoi(strings.TrimSpace(ms))
		if ok && err1 == nil && err2 == nil && d > 0 {
			s.slow[n] = time.Duration(d) * time.Millisecond
		}
	}
	// SIM_EXTERNAL_FOLLOW=2>40,3>41 (0-based): dst 40 follows dst 2 to the same source a moment later.
	for _, f := range strings.Split(os.Getenv("SIM_EXTERNAL_FOLLOW"), ",") {
		from, to, ok := strings.Cut(strings.TrimSpace(f), ">")
		a, err1 := strconv.Atoi(from)
		b, err2 := strconv.Atoi(to)
		if ok && err1 == nil && err2 == nil {
			s.follows[a] = append(s.follows[a], b)
		}
	}
	// SIM_EXTERNAL_SECONDS=5 makes a route every 5 s, as a panel does. Each second one goes to SIM_EXTERNAL_DST
	// (0-based, default 3).
	if n := envInt("SIM_EXTERNAL_SECONDS", 0); n > 0 {
		go s.panel(time.Duration(n)*time.Second, envInt("SIM_EXTERNAL_DST", 3), nl+1, envInt("SIM_EXTERNAL_SILENT", 0) == 1)
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

func protectSpec(f string) (dst, kind, dev int, ok bool) {
	parts := strings.Split(strings.TrimSpace(f), ":")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 {
			return 0, 0, 0, false
		}
		n[i] = v
	}
	return n[0], n[1], n[2], n[1] <= 3
}

// holdAll sets dst's hold on every level; kind 0 clears it.
func (s *sim) holdAll(dst, kind, dev int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for l := range s.tally {
		if kind == 0 {
			delete(s.protect, [2]int{l, dst})
		} else {
			s.protect[[2]int{l, dst}] = hold{details: kind, device: dev}
		}
	}
}

// held: a connect to this level is refused. Caller holds mu.
func (s *sim) held(l, dst int) bool {
	_, ok := s.protect[[2]int{l, dst}]
	return ok
}

func newSim(ns, nd, nl int) *sim {
	s := &sim{tally: map[int][]int{}, clients: map[net.Conn]*sync.Mutex{}, spoke: map[net.Conn]bool{}, protect: map[[2]int]hold{}, deaf: map[int]bool{}, silent: map[int]bool{}, follows: map[int][]int{}, slow: map[int]time.Duration{}, staged: map[int][]swp08.Staged{}}
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
		if nd > 12 {
			s.protect[[2]int{l, 12}] = hold{details: 2, device: 9}
		}
	}
	return s
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
	// A real router announces only a change.
	if !changed {
		log.Printf("connect level %d: dst %d already on src %d (no Connected)", l, dst, src)
		return
	}
	if d, ok := s.slow[dst]; ok && announce {
		log.Printf("connect level %d: dst %d ← src %d (Connected in %v, SIM_SLOW_DSTS)", l, dst, src, d)
		time.AfterFunc(d, func() { s.announce(mx, l, dst, src) })
	} else {
		log.Printf("connect level %d: dst %d ← src %d", l, dst, src)
		if announce {
			for _, cc := range all {
				s.write(cc, swp08.Encode(swp08.ExtConnected(mx, l, dst, src)))
			}
		}
	}
	for _, f := range s.follows[dst] {
		time.AfterFunc(30*time.Millisecond, func() { s.route(mx, l, f, src, announce) })
	}
}

// announce reaches only clients connected now, so a delayed one goes to whoever is listening then.
func (s *sim) announce(mx, l, dst, src int) {
	s.mu.Lock()
	var all []net.Conn
	for cc := range s.clients {
		all = append(all, cc)
	}
	s.mu.Unlock()
	for _, cc := range all {
		s.write(cc, swp08.Encode(swp08.ExtConnected(mx, l, dst, src)))
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
		delete(s.spoke, c)
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
	s.mu.Lock()
	s.spoke[c] = true
	s.mu.Unlock()
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
		held := s.held(l, dst)
		s.mu.Unlock()
		if held {
			// The Ultrix refuses in silence, also to the session that set the Protect: it cannot see a SW-P-08
			// client as the owner.
			log.Printf("connect level %d: dst %d ← src %d refused: Protected (no reply)", l, dst, src)
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
		if len(m) < 3 {
			return
		}
		mx, l := int(m[1]), int(m[2])
		s.mu.Lock()
		var msgs [][]byte
		for i := 0; i < len(s.dsts); i += 64 {
			n := min(64, len(s.dsts)-i)
			msg := []byte{swp08.CmdExtProtectDump, byte(mx), byte(l), byte(n), byte(i >> 8), byte(i)}
			for d := i; d < i+n; d++ {
				h := s.protect[[2]int{l, d}]
				hi, lo := swp08.ProtectWord(h.details, h.device)
				msg = append(msg, hi, lo)
			}
			msgs = append(msgs, msg)
		}
		s.mu.Unlock()
		for _, msg := range msgs {
			reply(msg)
		}

	case swp08.CmdExtProtectInterrogate:
		if len(m) < 5 {
			return
		}
		mx, l, dst := int(m[1]), int(m[2]), int(m[3])<<8|int(m[4])
		s.mu.Lock()
		h := s.protect[[2]int{l, dst}]
		s.mu.Unlock()
		reply(swp08.ExtProtectChange(swp08.CmdExtProtectTally, mx, l, h.details, dst, h.device))

	case swp08.CmdExtProtectConnect, swp08.CmdExtProtectDisconnect:
		s.protectMsg(m)

	case swp08.CmdSrcNamesReq, swp08.CmdDstNamesReq, swp08.CmdExtSrcNamesReq, swp08.CmdExtDstNamesReq:
		dest := m[0] == swp08.CmdDstNamesReq || m[0] == swp08.CmdExtDstNamesReq
		ext := m[0] >= 0x80
		code := m[len(m)-1] & 3
		list := s.srcs
		if dest {
			list = s.dsts
		}
		list = s.renamed(list, dest)
		per := swp08.NamesPerMsg(code)
		for i := 0; i < len(list); i += per {
			reply(swp08.NamesResp(dest, ext, 0, 0, code, i, list[i:min(i+per, len(list))]))
		}

	case swp08.CmdConnectOnGo, swp08.CmdExtConnectOnGo, swp08.CmdGoSalvo:
		s.salvo(c, m)

	default:
		log.Printf("ignored cmd 0x%02X", m[0])
	}
}

// later writes msg after salvoDelay, queued behind earlier replies so their order holds.
func (s *sim) later(c net.Conn, msg []byte) {
	time.Sleep(salvoDelay)
	s.write(c, swp08.Encode(msg))
}

func (s *sim) salvo(c net.Conn, m []byte) {
	if s.noSalvo {
		log.Printf("ignored salvo cmd %d (SIM_NO_SALVO)", m[0])
		return
	}
	if x, ok := swp08.StagedOf(m); ok {
		s.mu.Lock()
		valid := x.Dst < len(s.dsts) && x.Src < len(s.srcs) && len(s.tally[x.Level]) > 0
		if valid {
			s.staged[x.Group] = append(s.staged[x.Group], x)
		}
		n := len(s.staged[x.Group])
		s.mu.Unlock()
		if !valid {
			log.Printf("salvo %d: crosspoint out of range: level %d dst %d src %d (not staged, no 122)", x.Group, x.Level, x.Dst, x.Src)
			return
		}
		log.Printf("salvo %d: staged level %d dst %d ← src %d (%d staged)", x.Group, x.Level, x.Dst, x.Src, n)
		s.later(c, swp08.StageAck(x))
		return
	}
	if len(m) < 3 || m[1] > 1 {
		log.Printf("salvo: bad Go % X", m)
		return
	}
	g := int(m[2] & 0x7f)
	s.mu.Lock()
	xs := s.staged[g]
	delete(s.staged, g)
	s.mu.Unlock()
	if m[1] == 1 {
		st := swp08.GoNothing
		if len(xs) > 0 {
			st = swp08.GoCleared
		}
		log.Printf("salvo %d: cleared %d staged crosspoints", g, len(xs))
		s.later(c, swp08.GoDoneAck(st, g))
		return
	}
	if len(xs) == 0 {
		log.Printf("salvo %d: Go with nothing staged", g)
		s.later(c, swp08.GoDoneAck(swp08.GoNothing, g))
		return
	}
	// The Ultrix announces a Go's crosspoints only sometimes, so the default is no Connected. SIM_SALVO_ANNOUNCE=1
	// sends one for
	// each changed crosspoint in dst and level order, 4 ms apart, from about 110 ms after the Go (measured).
	// Not proved on the Ultrix: what a Go does with a Protected destination. The sim leaves those crosspoints, as a
	// connect does, and sets the rest (Best Effort).
	var changed, free []swp08.Staged
	s.mu.Lock()
	for _, x := range xs {
		if s.held(x.Level, x.Dst) {
			log.Printf("salvo %d: level %d dst %d is Protected, left", g, x.Level, x.Dst)
			continue
		}
		free = append(free, x)
		if t := s.tally[x.Level]; x.Dst < len(t) && t[x.Dst] != x.Src {
			changed = append(changed, x)
		}
	}
	s.mu.Unlock()
	xs = free
	for _, x := range xs {
		s.route(x.Matrix, x.Level, x.Dst, x.Src, false)
	}
	if s.announceGo && len(changed) > 0 {
		slices.SortFunc(changed, func(a, b swp08.Staged) int { return cmp.Or(a.Dst-b.Dst, a.Level-b.Level) })
		go func() {
			time.Sleep(110 * time.Millisecond)
			for _, x := range changed {
				s.announce(x.Matrix, x.Level, x.Dst, x.Src)
				time.Sleep(4 * time.Millisecond)
			}
		}()
		log.Printf("salvo %d: Go set %d crosspoints (%d Connected to follow, SIM_SALVO_ANNOUNCE)", g, len(xs), len(changed))
	} else {
		log.Printf("salvo %d: Go set %d crosspoints (no Connected)", g, len(xs))
	}
	s.later(c, swp08.GoDoneAck(swp08.GoSet, g))
}

// Read again on every name request, so a test can rename while the simulator runs. Lines: "src 12 CAM C", "dst 0 PGM".
func (s *sim) renamed(list []string, dest bool) []string {
	if s.namesFile == "" {
		return list
	}
	b, err := os.ReadFile(s.namesFile)
	if err != nil {
		log.Printf("SIM_NAMES_FILE: %v", err)
		return list
	}
	out := slices.Clone(list)
	for _, line := range strings.Split(string(b), "\n") {
		side, rest, _ := strings.Cut(strings.TrimSpace(line), " ")
		num, name, _ := strings.Cut(rest, " ")
		n, err := strconv.Atoi(num)
		if err != nil || n < 0 || n >= len(out) || (side != "src" && side != "dst") || (side == "dst") != dest {
			continue
		}
		out[n] = name
	}
	return out
}

// protectMsg protects (0x8C) or clears (0x8E) one level. A 0x8E from a device that did not protect the level gets an
// ACK, no change and no reply, as on the Ultrix. A 0x8C on a level held by another device is treated the same (not
// tried on the Ultrix).
func (s *sim) protectMsg(m []byte) {
	if len(m) < 7 {
		return
	}
	set := m[0] == swp08.CmdExtProtectConnect
	mx, l, dst, dev := int(m[1]), int(m[2]), int(m[3])<<8|int(m[4]), int(m[5])<<8|int(m[6])
	k := [2]int{l, dst}
	s.mu.Lock()
	h, held := s.protect[k]
	_, levelOK := s.tally[l]
	ok := levelOK && dst < len(s.dsts) && (!held || h.device == dev)
	if ok && set {
		s.protect[k] = hold{details: 1, device: dev}
	} else if ok {
		delete(s.protect, k)
	}
	var to []net.Conn
	for cc := range s.spoke {
		to = append(to, cc)
	}
	s.mu.Unlock()
	if !ok {
		log.Printf("protect level %d dst %d device %d: no change (set %t, held by %+v)", l, dst, dev, set, h)
		return
	}
	reply := swp08.ExtProtectChange(swp08.CmdExtProtectConnected, mx, l, 1, dst, dev)
	if !set {
		reply = swp08.ExtProtectChange(swp08.CmdExtProtectDisconnected, mx, l, 0, dst, 0)
	}
	log.Printf("protect level %d dst %d device %d: set %t", l, dst, dev, set)
	for _, cc := range to {
		s.write(cc, swp08.Encode(reply))
	}
}
