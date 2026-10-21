"""Generate and independently replay bounded real codec comparison fixtures."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import time

from common import digest, pcm_stats, read_pcm, save, write_wav
from compare_codec_support import Codec2, grouped_mask
from opus_codec import align, erase
from comparison_opus import ComparisonOpus
from opus_generate import ROOT, child, config, source_inputs

BASE = ROOT / ".artifacts/codec-comparison-20260906"
CONFIG = ROOT / "configs/audio/opus-listening-20260906.json"
CODECS = ("opus-6000", "opus-8000", "codec2-2400", "codec2-3200")
CASES = ((1,"clean"),(4,"loss05"),(4,"loss20"),(4,"loss30"),(4,"burst240"),(8,"loss20"),(8,"loss30"))


def adapters():
    cfg = config(CONFIG)
    source, _ = source_inputs(cfg)
    return cfg, source, ComparisonOpus(cfg["library"]["path"], cfg["library"]["sha256"]), Codec2()


def encode(codec, pcm, opus, c2):
    if codec not in CODECS:
        raise ValueError("unknown codec")
    if codec.startswith("opus-"):
        return opus.encode(pcm, False, int(codec.split("-")[1]))
    if codec not in CODECS:
        raise ValueError("unknown codec")
    return c2.encode(pcm, int(codec.split("-")[1])), {"input_samples": len(pcm)//2, "lookahead_samples": 0}


def decode(codec, received, meta, opus, c2):
    if codec.startswith("opus-"):
        pcm, _ = opus.decode_received(received, False)
        return align(pcm, meta)
    return c2.decode(received, int(codec.split("-")[1]))[:meta["input_samples"]*2]


def generate(output):
    cfg, source, opus, c2 = adapters()
    output = Path(output).absolute()
    if not output.is_relative_to(BASE) or output == BASE:
        raise ValueError("need fresh output under codec comparison artifact root")
    child(ROOT, str(output.relative_to(ROOT)))
    output.mkdir(parents=True, exist_ok=False)
    for name in ("wav", "evidence", "traces"):
        (output/name).mkdir()
    attribution = (source/"ATTRIBUTION.txt").read_text() + "\nAdditional derivatives: Opus6k/8k and Codec2 2400/3200; no denoising, gain adjustment or normalization. Opus lookahead trimmed; Codec2 delay not corrected or measured. Grouped erasure comparison, no endorsement.\n"
    (output/"ATTRIBUTION.txt").write_text(attribution)
    shutil.copyfile(CONFIG, output/"evidence/opus-config.json")
    shutil.copyfile(source/"manifest.json", output/"evidence/source-manifest.json")
    shutil.copyfile(source/"evidence/source-config.json", output/"evidence/source-config.json")
    shutil.copyfile(Path(cfg["library"]["license_path"]), output/"OPUS-COPYING.txt")
    tools = ROOT/".artifacts/audio-listening-20260906/tools-r1"
    shutil.copyfile(tools/"provenance.json", output/"evidence/codec2-provenance.json")
    license_path = next((tools/"source").glob("codec2-*/COPYING"))
    shutil.copyfile(license_path, output/"CODEC2-COPYING.txt")
    traces = {codec: {"version":1,"codec":codec,"packet_ms":20,
        "lookahead_us":6500 if codec.startswith("opus") else None,
        "flush_samples_8k":52 if codec.startswith("opus") else 0,
        "source_manifest_sha256":cfg["source_manifest_sha256"],
        "library_sha256":cfg["library"]["sha256"] if codec.startswith("opus") else c2.closure,
        "license":"CC-BY-SA-4.0","attribution":attribution,"samples":[]} for codec in CODECS}
    samples, records, costs = [], [], []
    for sid in cfg["samples"]:
        pcm,_ = read_pcm(source/f"wav/{sid}-input8k.wav")
        sample = {"id":sid,"duration_seconds":len(pcm)/16000,"tracks":[]}
        for kind,label in (("original","Оригинал"),("input8k","Исходник 8 кГц")):
            ref=f"wav/{sid}-{kind}.wav"
            shutil.copyfile(source/ref,output/ref)
            sample["tracks"].append({"codec":"reference","condition":"clean","group":1,"label":label,"path":ref})
        for codec in CODECS:
            start=time.perf_counter()
            packets,meta=encode(codec,pcm,opus,c2)
            costs.append({"sample":sid,"codec":codec,"encode_host_seconds":time.perf_counter()-start})
            trace_sample={"id":sid,"input_samples":len(pcm)//2,"packets_hex":[p.hex() for p in packets]}
            traces[codec]["samples"].append(trace_sample)
            save(output/f"evidence/{sid}-{codec}.json", {**trace_sample,"meta":meta})
            for group,condition in CASES:
                mask,groups=grouped_mask(len(packets),group,condition,sid)
                received=erase(packets,mask)
                start=time.perf_counter()
                decoded=decode(codec,received,meta,opus,c2)
                costs.append({"sample":sid,"codec":codec,"condition":condition,"group":group,"decode_host_seconds":time.perf_counter()-start})
                stem=f"{sid}-{codec}-g{group}-{condition}"
                wav=f"wav/{stem}.wav"
                write_wav(output/wav,decoded)
                recovery="PLC" if codec.startswith("opus") else "пропуски тишиной, без PLC"
                label=f"{codec} · без потерь" if condition=="clean" else f"{codec} · {group*20} мс · {condition} · {sum(groups)*100/len(groups):.1f}% · {recovery}"
                track={"codec":codec,"condition":condition,"group":group,"label":label,"path":wav}
                sample["tracks"].append(track)
                save(output/f"evidence/{stem}-received.json",{"received":[p.hex() if p is not None else None for p in received],"frame_mask":mask,"group_mask":groups})
                records.append({"sample":sid,**track,"pcm":pcm_stats(decoded),"lost_groups":sum(groups),"groups":len(groups),"lost_frames":sum(mask),"frames":len(mask),"meta":meta})
        samples.append(sample)
    for codec,trace in traces.items():
        save(output/f"traces/{codec}.json",trace)
    audio_set=[{"path":str(p.relative_to(output)),**digest(p)} for p in sorted((output/"wav").glob("*.wav"))]
    audio_sha=hashlib.sha256(json.dumps(audio_set,sort_keys=True).encode()).hexdigest()
    data={"pack_id":"codec-comparison-20260906","audio_set_sha256":audio_sha,"samples":samples,"codecs":list(CODECS)}
    save(output/"listening-data.json",data)
    template=Path(__file__).with_name("codec_comparison.html").read_text()
    embedded=json.dumps(data,ensure_ascii=False,sort_keys=True).replace("<","\\u003c").replace("&","\\u0026")
    (output/"index.html").write_text(template.replace("__PACK_DATA__",embedded))
    # Host measurements are deliberately separate from deterministic audio/packet evidence.
    save(output/"host-timing.json", {"measurements":costs,"limits":["host process/ABI time; includes startup for Codec2","not MCU timing or algorithmic delay"]})
    inputs=[Path(__file__),Path(__file__).with_name("compare_verify.py"),Path(__file__).with_name("compare_codec_support.py"),Path(__file__).with_name("comparison_opus.py"),Path(__file__).with_name("opus_codec.py"),Path(__file__).with_name("common.py"),Path(__file__).with_name("opus_generate.py"),Path(__file__).with_name("codec_comparison.html")]
    manifest={"version":1,"records":records,"source_manifest_sha256":cfg["source_manifest_sha256"],
        "generator_inputs":[{"path":str(p.relative_to(ROOT)),**digest(p)} for p in inputs],
        "files":[{"path":str(p.relative_to(output)),**digest(p)} for p in sorted(output.rglob("*")) if p.is_file() and p.name!="host-timing.json"],
        "quality_status":"pending-user-listening","hardware_executed":False,"codec_selected":False,
        "limits":["Codec2 3200 is a reference, not claimed improved quality","Codec2 decoder skips absent frames with frozen state, then inserts silence; no PLC","No new resampling/noise suppression; Codec2 algorithmic delay unknown","Two literary recordings, one speaker; not operational numbers/callsign acceptance","External FEC not simulated in WAVs; losses applied after hypothetical transport"]}
    save(output/"manifest.json",manifest)
    return {"output":str(output),"records":len(records),"traces":len(traces),"manifest_sha256":digest(output/"manifest.json")["sha256"]}


def verify(output):
    from compare_verify import verify_pack
    return verify_pack(output)


if __name__=="__main__":
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument("action",choices=("generate","verify"));p.add_argument("--output",required=True,type=Path)
    a=p.parse_args();print(json.dumps(generate(a.output) if a.action=="generate" else verify(a.output),indent=2))
