#!/usr/bin/env python3
"""Independent channel-capacity recomputation for the ad-hoc-radio review.

All inputs are explicit assumptions; nothing here is a measurement.
Model: one shared half-duplex channel, no spatial reuse (all nodes hear each
other, as in a basement/stairwell group), store-and-forward of audio blocks.
Airtime per speech second = (n+1) * S_c / R_net, rho > 1 means the channel is
slower than real time and the tail after PTT release grows as T*(rho-1).
"""
import math

# ---- DMR direct-mode burst arithmetic (ETSI TS 102 361-1, own recomputation)
slot_ms = 30.0
burst_bits = 264            # 108 + 48 (sync/EMB) + 108
payload_bits = 216          # 27 bytes = 3 x 72-bit AMBE+2 frames
tdma_period_ms = 60.0       # one MS burst per own timeslot every 60 ms
print("== DMR direct mode ==")
print("gross channel rate  : %.0f bit/s (4800 baud x 2 bit)" % (4800*2))
print("on-air per own slot : %.2f kbit/s (264 b / 60 ms)" % (burst_bits/tdma_period_ms))
print("payload per own slot: %.2f kbit/s (216 b / 60 ms)" % (payload_bits/tdma_period_ms))
print("payload both slots  : %.2f kbit/s (only if one MS may use both TS - unproven)" % (2*payload_bits/tdma_period_ms))
for hdr, name in ((14, "ADHR short voice header 14B"), (6+14, "ADHR hdr 14 + bind 6"), (4, "hypothetical 4B header")):
    pl = 27 - hdr
    print("  27B frame with %-26s -> %2d B codec payload = %.2f kbit/s per hop (no FEC)" % (name, pl, pl*8/0.060/1000))
print("  with ADHR 2/3 FEC on top (payload bits share 2/3): x0.667")
print("  36-byte project voice packet (ADR-0021) does NOT fit 27 B: ADR-0021 L61-62")

# ---- Candidate PHYs: (name, gross bit/s, eff = framing/preamble efficiency, per-block overhead ms)
phys = [
    ("DMR L2 27B/60ms (HR_C6000)", 3600.0, 1.0, 40+10),   # raw payload bits already; turnaround 2x20 + sync 10
    ("4FSK 4800 Bd, 12.5 kHz (M17-like, MCU modem)", 9600.0, 0.85, 40+10),
    ("FM voice-band soft modem (FreeDV 2400B class)", 2400.0, 0.90, 40+10),
    ("Hypothetical 19.2 kbit/s (project assumption)", 19200.0, 0.85, 40+10),
    ("CC1101 GFSK 100 kBd", 100000.0, 0.90, 0.1+0.3),
    ("CC1101 GFSK 250 kBd", 250000.0, 0.90, 0.1+0.3),
]
fecs = [("no FEC", 1.0), ("FEC 2/3", 2.0/3.0)]

codecs = [
    ("Synthetic 1600 (ADR-0021)", 1600),
    ("Codec2 3200 (rejected by user)", 3200),
    ("AMBE+2 2450 (+FEC1150 = 3600 on air)", 2450),
    ("Lyra 3200 / Opus 6k", 6000),
    ("Opus 8k", 8000),
]

def rho(n, codec_bps, r_net):
    return (n+1)*codec_bps/r_net

def max_utt(n, codec_bps, r_net, blk_ms, tail_s=2.0, block_s=0.4):
    """Longest utterance T [s] with completion tail <= tail_s after PTT release.
    tail = first_block_wait + T*(rho-1) + extra per-block overhead; blocks of block_s."""
    r = rho(n, codec_bps, r_net)
    per_s_overhead = (n+1)*(blk_ms/1000.0)/block_s   # s of overhead per s of speech
    load = r + per_s_overhead
    if load <= 1.0:
        return float('inf'), load
    # Playback can start after first block; steady-state backlog grows as T*(load-1)
    t = (tail_s - block_s) / (load-1)
    return max(t, 0.0), load

print()
print("== rho = airtime / speech-time, load incl. per-block TX/RX overhead (block 0.4 s) ==")
print("   inf = sustainable for any PTT length; number = longest PTT [s] with <=2.0 s tail")
for pname, gross, eff, blk in phys:
    for fname, fec in fecs:
        if pname.startswith("DMR L2"):
            if fname != "no FEC":
                # C6000 layer-2 voice path: 216 payload bits already include whatever coding the project adds
                pass
        # Header/CRC/ID overhead per block is amortised: assume 6 % for block-level framing
        r_net = gross*eff*fec*0.94
        line = "%-48s %-7s R_net=%7.2f kbit/s | " % (pname, fname, r_net/1000)
        parts = []
        for n in (0, 1, 3):
            row = []
            for cname, cbps in (("Opus6k", 6000), ("Opus8k", 8000), ("AMBE2450", 2450), ("C2-3200", 3200)):
                t, load = max_utt(n, cbps, r_net, blk)
                row.append("%s:%s" % (cname, ("inf" if t == float('inf') else "%.1fs" % t)))
            parts.append("n=%d [%s]" % (n, " ".join(row)))
        print(line + " ; ".join(parts))
    print()

print("== simple rho table (no per-block overhead), n=1 and n=3 ==")
for pname, gross, eff, blk in phys:
    for fname, fec in fecs:
        r_net = gross*eff*fec*0.94
        print("%-48s %-7s R_net=%7.2f kbit/s" % (pname, fname, r_net/1000), end="  ")
        for cname, cbps in (("Opus6k", 6000), ("Opus8k", 8000), ("AMBE2450", 2450)):
            print("%s rho(n=1)=%.2f rho(n=3)=%.2f" % (cname, rho(1, cbps, r_net), rho(3, cbps, r_net)), end="  ")
        print()

print()
print("== minimal gross rate needed (no FEC / FEC2/3), eff=0.85*0.94, rho<=0.8 ==")
for cname, cbps in (("Opus6k", 6000), ("Opus8k", 8000), ("AMBE2450", 2450), ("Codec2 3200", 3200)):
    for n in (1, 3):
        for fname, fec in fecs:
            need = (n+1)*cbps/(0.8*0.85*0.94*fec)
            print("%-12s n=%d %-7s needs gross >= %6.1f kbit/s" % (cname, n, fname, need/1000))

print()
print("== project's own control-plane bytes (opus-relay-budget-20260906.md L136-142): setup airtime at 19.2 kbit/s ==")
for relays, nbytes in ((1, 715), (2, 1608), (3, 2934)):
    bits = nbytes*8
    print("%d relay(s): %4d B -> %.0f ms (no FEC), %.0f ms (FEC 2/3), at 19.2 kbit/s; at 9.6k: %.0f / %.0f ms" % (
        relays, nbytes, bits/19.2, bits*1.5/19.2, bits/9.6, bits*1.5/9.6))

print()
print("== ADR-0021 integrated profile re-costed with turnaround evidence from research/015 ==")
air = 480/19.2   # ms: 36 B protected + framing at 19.2 kbit/s
for tta, label in ((1, "assumed 1 ms (ADR-0021)"), (20, "20 ms (best same-class DMR handheld RX->TX, QST via research/015)"),
                   (56, "56 ms (TYT TH-UVF1 RX->TX, QST via research/015)")):
    guard = 2*tta + 2   # RX->TX + TX->RX + processing + uncertainty(1+1)
    opp = math.ceil(air) + guard
    print("%-70s opportunity=%3d ms; 1 relay=%3d ms; 3 relays=%3d ms (cadence 80 ms)" % (label, opp, 2*opp, 4*opp))

print()
print("== Opus 8k through CC1101 hops (own arithmetic, 20 ms frames grouped x4=80 ms) ==")
for rate in (38400, 100000, 250000):
    # 8 kbit/s => 80 B per 80 ms; header 10 B, FEC 2/3 -> (90*8)*1.5 = 1080 bit + preamble/sync 6 B =48 bit
    bits = (90*8)*1.5 + 48
    t_air = bits/rate*1000.0
    for n in (1, 3):
        cycle = (n+1)*(t_air + 0.4)
        print("rate %6d bit/s: per-hop airtime %.1f ms; chain n=%d cycle %.1f ms per 80 ms (%.0f %% duty)" % (rate, t_air, n, cycle, 100*cycle/80))
