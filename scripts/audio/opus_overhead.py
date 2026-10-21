"""Native packing and necessary-only airtime bounds; never service acceptance."""
import argparse
import hashlib
import json
from pathlib import Path

from common import digest, save
from compare_codecs import adapters, verify
from opus_generate import ROOT, child
from opus_repacketize import Repacketizer

BASE = ROOT / ".artifacts/opus-overhead-20260908"
PACK = ROOT / ".artifacts/codec-comparison-20260906/pack-reviewed-r1"
BUDGET = ROOT / ".artifacts/codec-comparison-20260906/all-budgets-reviewed"
PINS = {
    "opus-6000": ("e267bb7124516563875e978bd7710bbad5926724c2a33804965f93a7d71ed78e",
                  "f64345f137dcf462eebbd20c20585a026affca3f1d5defdad54dcbf7f9b2e70b"),
    "opus-8000": ("a5803206720eeaa52269f1b6447718f730f73956b29a9870882e4e905c6e0eca",
                  "52db8617809572e36102d2952b43abcdc1a0dcdd7649781707094d153f4ecee8"),
}
SHAPES = ((0, 1), (1, 1), (2, 1), (3, 1), (1, 2), (1, 4))


def ceil_div(n, d):
    return (n+d-1)//d


def bound(sizes, period, rate, opportunities, header=20, framing=48, guard=4):
    if (not sizes or len(sizes)>3004 or any(type(n) is not int or not 1<=n<=12000 for n in sizes)
            or period not in (20,40,80,160) or rate not in (4800,19200,38400)
            or type(opportunities) is not int or not 1<=opportunities<=5
            or header not in (0,20) or framing not in (0,48) or guard not in (0,4)):
        raise ValueError("unsupported arithmetic input")
    # Matches VoiceBudgetProfile.Measure: FEC2/3, ceil each duration to ms.
    bits = [(n+header)*12+framing for n in sizes]
    cycles = [opportunities*(ceil_div(b*1000,rate)+guard) for b in bits]
    available = period//opportunities-guard
    return {
        "payload_min":min(sizes), "payload_max":max(sizes), "payload_total":sum(sizes),
        "wire_header_bytes":header,"framing_bits":framing,"guard_ms":guard,
        "fec_numerator":2,"fec_denominator":3,
        "protected_bits_total":sum(bits),"peak_protected_bits":max(bits),
        "peak_reserved_cycle_ms":max(cycles),
        "mean_reserved_cycle_ms":sum(cycles)/len(cycles),
        "groups":len(sizes),"group_deadline_misses":sum(c>period for c in cycles),
        "arithmetic_peak_fits_not_service":max(cycles)<=period,
        "average_serialization_min_bps":ceil_div(sum(bits)*opportunities*1000,len(sizes)*period),
        "peak_reservation_min_bps":ceil_div(max(bits)*1000,available) if available>0 else None,
        "runtime_executed":False,"hardware_executed":False,"product_accepted":False,
    }


def pinned(path, sha):
    if path.is_symlink() or not path.is_file() or path.stat().st_size>2000000 or digest(path)["sha256"]!=sha:
        raise ValueError(f"pinned input mismatch: {path.name}")
    return json.loads(path.read_text())


def check_baseline(row, old):
    trial = old["envelope"]
    checks = ((row["payload_max"],trial["payload"]["max_bytes"]),
              (row["payload_total"],trial["payload"]["total_bytes"]),
              (row["peak_reserved_cycle_ms"],trial["reserved_cycle_ms"]),
              (row["protected_bits_total"],trial["payload"]["total_protected_bits"]),
              (row["peak_reservation_min_bps"],trial["peak_reservation_min_bps"]))
    if any(a!=b for a,b in checks):
        raise ValueError("Python arithmetic does not match pinned Go baseline")


def generate(output):
    output = Path(output).absolute()
    if not output.is_relative_to(BASE) or output==BASE:
        raise ValueError("fresh output required under overhead artifact root")
    child(ROOT,str(output.relative_to(ROOT)))
    if output.exists():
        raise ValueError("will not overwrite historical run")
    pinned(PACK/"manifest.json","7c938d3304d2df1f89122f449fbed63c1624de9245d47ed78e2e4a22f02b29c0")
    verify(PACK)
    _,_,opus,_ = adapters()
    native = Repacketizer(opus)
    rows, streams = [], []
    baseline_checks = 0
    inputs = []
    for codec,(trace_sha,report_sha) in PINS.items():
        trace_path, report_path = PACK/f"traces/{codec}.json", BUDGET/f"{codec}-r1.json"
        trace, old = pinned(trace_path,trace_sha), pinned(report_path,report_sha)
        inputs += [{"path":str(p.relative_to(ROOT)),**digest(p)} for p in (trace_path,report_path)]
        old_rows = {(r["envelope"]["sample_id"],r["envelope"]["group_count"],
                     r["envelope"]["trial"]["gross_bitrate_bps"],r["envelope"]["trial"]["relay_depth"],
                     r["envelope"]["trial"]["branch_width"]):r for r in old["rows"]
                    if r["envelope"]["trial"]["setup_allowance_ms"]==500}
        for sample in trace["samples"]:
            packets = [bytes.fromhex(p) for p in sample["packets_hex"]]
            original_pcm,_ = opus.decode_received(packets,False)
            for group in (1,2,4,8):
                grouped = native.group(packets,group)
                packed_pcm = native.decode([p for g in grouped for p in g])
                if packed_pcm!=original_pcm:
                    raise ValueError("repacketization changed decoded PCM")
                baseline, packed, bodies = [], [], []
                for start,chunks in zip(range(0,len(packets),group),grouped):
                    original = packets[start:start+group]
                    baseline.append(sum(len(p)+(2 if group>1 else 0) for p in original))
                    packed.append(sum(len(p)+(2 if group>1 else 0) for p in chunks))
                    bodies.append(sum(native.bodies(p) for p in original))
                streams.append({"codec":codec,"sample":sample["id"],"group":group,
                    "original_packets":len(packets),"input_samples":sample["input_samples"],
                    "decoded_samples_with_flush":len(packed_pcm)//2,
                    "decoded_pcm_sha256":hashlib.sha256(packed_pcm).hexdigest(),
                    "clean_pcm_identical":True,"groups_hex":[[p.hex() for p in g] for g in grouped],
                    "original_payload_bytes":sum(baseline),"packed_payload_bytes":sum(packed)})
                for rate in (4800,19200,38400):
                    for depth,width in SHAPES:
                        opportunities = 1+depth*width
                        policies = (("baseline",baseline,20,48,4),("native_repacketized",packed,20,48,4),
                                    ("zero_adhr_header_bound",packed,0,48,4),
                                    ("all_framing_removed_bound",bodies,0,0,4),
                                    ("body_fec_only_ideal",bodies,0,0,0))
                        for policy,sizes,header,framing,guard in policies:
                            row = {"codec":codec,"sample":sample["id"],"group":group,"period_ms":group*20,
                                   "gross_bps":rate,"rate_class":"unverified_sensitivity" if rate>19200 else "assumption",
                                   "depth":depth,"width":width,"opportunities":opportunities,"policy":policy,
                                   **bound(sizes,group*20,rate,opportunities,header,framing,guard)}
                            if policy=="baseline":
                                prior = old_rows[(sample["id"],group,rate,depth,width)]
                                check_baseline(row,prior)
                                row["historical_service_outcome"] = prior["envelope"]["trial"]["service_outcome"]
                                baseline_checks += 1
                            rows.append(row)
    report = {"version":1,"evidence":"native_pcm_identity_and_arithmetic_necessary_bounds",
        "baseline_go_rows_matched":baseline_checks,"inputs":inputs,"rows":rows,
        "limits":["No new scheduler/control execution or physical measurement; no service acceptance",
                  "Native packing is lossless but not an adopted ADHR wire format;160ms split into <=120ms chunks",
                  "Zero-header/body-only policies discard required identity/integrity/framing; impossible protocol bounds, not implementations",
                  "FEC2/3 retained even in body-only ideal; no spatial reuse and every branch transmitter charged",
                  "Full final period including partial group and codec flush tail; mean rates slightly optimistic for finite clips",
                  "No losses/jitter/retries/control airtime in necessary bound; passing it cannot imply reliable delivery",
                  "Observed maxima are not enforced future encoder ceilings; batching and6.5ms lookahead precede network playout"]}
    output.mkdir(parents=True,exist_ok=False)
    save(output/"packing.json",{"version":1,"streams":streams})
    save(output/"report.json",report)
    source_files = [Path(__file__),Path(__file__).with_name("opus_repacketize.py")]
    save(output/"manifest.json",{"version":1,"inputs":inputs,
        "sources":[{"path":str(p.relative_to(ROOT)),**digest(p)} for p in source_files],
        "files":[{"path":p.name,**digest(p)} for p in sorted(output.iterdir())]})
    return {"rows":len(rows),"streams":len(streams),"baseline_checks":baseline_checks,
            "manifest":digest(output/"manifest.json")}


if __name__=="__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output",required=True,type=Path)
    print(json.dumps(generate(parser.parse_args().output),indent=2))
