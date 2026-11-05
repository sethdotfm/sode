# Verified against the real Ultrix (2026-10-22, 172.17.60.101:8910)

Moved from AGENTS.md unchanged.

- UI level 1 = wire 0 (SODE_LEVEL_WIRE_OFFSET=-1), display number = wire + 1 (SODE_NUM_OFFSET=1).
- Standard name requests (100/102) with 12-char names work. About 2690 sources, 720 destinations.
- Extended tally dump (0x95 to 0x97, 64 per message) works on all 17 levels; source 0xFFFF = nothing routed. Extended connect (0x82) is confirmed by 0x84.
- Routers send Connected only when a crosspoint changes; a matching connect gets no reply, so take() interrogates (0x81 to 0x83) levels already on the source, and levels unconfirmed at the 3s timeout get a 1s interrogate grace; those are reported already. The simulator mimics this.
- The router does not announce every change: 2026-10-23 KiPro 4 went UDX 04 to UDX 03 with no 0x84 (the next entry's prev showed it), hence found entries and the pre-take check. Also 2026-10-23 18:01:40 a 0x84 arrived for a route Sode didn't make (PIX 03 to DSM 03 and MV08 PIP015), logged as an ext burst, source unknown.
- Extended protect dump (0x93 matrix, level, startDst(2) to 0x94 matrix, level, count, firstDst(2), count x 2 bytes): state in bits 4-5 of byte 1, device in the rest. Dest 65 was protected by device 9 on all levels. Single protect change messages (0x8B/0x8D/0x8F) not seen live.
- 2026-11-02 (branch spike/salvo-probe, cmd/swp08probe): 51 ordinary extended connects take about 840 ms (~16 ms each, one dst after another); pipelining saves under 10%. Group salvos work despite the spec saying XD and Eclipse only: 120 is answered by 122 echoing its 5 data bytes, 121 by 123 (00 set, 01 cleared, 02 nothing to do; clearing an empty group answers 02), in 5-7 ms. Standard layout: 4-bit level, numbers under 1024, group 0-127; probe used 127.
- 0x84 after a Go is unreliable: the spec (SW-P-88 Issue 4b 8.1.5) says none, uninstrumented runs saw 0 and 3, instrumented runs got one 0x84 per staged crosspoint ~3-5 ms apart, from ~100-130 ms after the Go to ~270-340 ms for 48-51 crosspoints. So a Go announces crosspoints about 4 ms apart (51 in ~210 ms) against ~16 ms for ordinary connects; whether the switch itself is simultaneous is unproven (needs a scope). Sode must work either way.
- Extended stage 248 (F8 matrix level dstH dstL srcH srcL group) is acked by 250 (FA + same 7 bytes), is held in the group, and a standard Go (121) fires extended-staged crosspoints too (#154). So level 17 and numbers >= 1024 can ride a salvo; a Go-clear clears both kinds; they may be announced with an ordinary 0x84 after the Go.

