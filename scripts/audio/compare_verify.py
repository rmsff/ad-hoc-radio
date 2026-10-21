"""Semantic verification, independent of a pack's self-reported inventory."""
import hashlib
import json
from pathlib import Path

from common import digest, pcm_stats, read_pcm
from compare_codec_support import grouped_mask
from opus_codec import erase
from opus_generate import ROOT, child, load_json


def require(value, message):
    if not value:raise ValueError(message)


def verify_pack(output):
    from compare_codecs import adapters, encode, decode, CODECS, CASES, CONFIG
    output=Path(output).resolve()
    m=load_json(child(output,"manifest.json"))
    cfg,source,opus,c2=adapters()
    require(m["version"]==1 and m["source_manifest_sha256"]==cfg["source_manifest_sha256"],"source identity")
    names=[f["path"] for f in m["files"]]
    actual={str(p.relative_to(output)) for p in output.rglob("*") if p.is_file() and p.name not in ("manifest.json","host-timing.json")}
    require(len(names)==len(set(names)) and set(names)==actual,"incomplete/duplicate inventory")
    for f in m["files"]:
        require(digest(child(output,f["path"]))=={k:f[k] for k in ("sha256","size")},"file hash mismatch")
    modules=("compare_codecs.py","compare_verify.py","compare_codec_support.py","comparison_opus.py","opus_codec.py","common.py","opus_generate.py","codec_comparison.html")
    expected_inputs=[{"path":f"scripts/audio/{n}",**digest(Path(__file__).with_name(n))} for n in modules]
    require(m["generator_inputs"]==expected_inputs,"generator source drift")
    require((output/"evidence/opus-config.json").read_bytes()==CONFIG.read_bytes(),"config pin")
    require((output/"evidence/source-manifest.json").read_bytes()==(source/"manifest.json").read_bytes(),"source receipt pin")
    require((output/"evidence/source-config.json").read_bytes()==(source/"evidence/source-config.json").read_bytes(),"source config pin")
    attribution=(source/"ATTRIBUTION.txt").read_text()+"\nAdditional derivatives: Opus6k/8k and Codec2 2400/3200; no denoising, gain adjustment or normalization. Opus lookahead trimmed; Codec2 delay not corrected or measured. Grouped erasure comparison, no endorsement.\n"
    require((output/"ATTRIBUTION.txt").read_text()==attribution,"attribution mismatch")
    require(m["quality_status"]=="pending-user-listening" and m["hardware_executed"] is False and m["codec_selected"] is False,"acceptance claim")
    expected_keys={(sid,codec,g,c) for sid in cfg["samples"] for codec in CODECS for g,c in CASES}
    keys=[(r["sample"],r["codec"],r["group"],r["condition"]) for r in m["records"]]
    require(len(keys)==len(set(keys)) and set(keys)==expected_keys,"missing/duplicate comparison cases")
    samples=[]
    for sid in cfg["samples"]:
        pcm,_=read_pcm(source/f"wav/{sid}-input8k.wav")
        sample={"id":sid,"duration_seconds":len(pcm)/16000,"tracks":[]}
        for kind,label in (("original","Оригинал"),("input8k","Исходник 8 кГц")):
            name=f"wav/{sid}-{kind}.wav"
            require((source/name).read_bytes()==(output/name).read_bytes(),"reference audio mismatch")
            sample["tracks"].append({"codec":"reference","condition":"clean","group":1,"label":label,"path":name})
        for codec in CODECS:
            trace=load_json(child(output,f"traces/{codec}.json"))
            header={"version":1,"codec":codec,"packet_ms":20,"lookahead_us":6500 if codec.startswith("opus") else None,
                "flush_samples_8k":52 if codec.startswith("opus") else 0,"source_manifest_sha256":cfg["source_manifest_sha256"],
                "library_sha256":cfg["library"]["sha256"] if codec.startswith("opus") else c2.closure,
                "license":"CC-BY-SA-4.0","attribution":attribution}
            require({k:v for k,v in trace.items() if k!="samples"}==header,"trace metadata mismatch")
            require([s["id"] for s in trace["samples"]]==cfg["samples"],"trace sample identities")
            packets,meta=encode(codec,pcm,opus,c2)
            expected_sample={"id":sid,"input_samples":len(pcm)//2,"packets_hex":[p.hex() for p in packets]}
            require(trace["samples"][cfg["samples"].index(sid)]==expected_sample,"trace source/packets mismatch")
            require(load_json(child(output,f"evidence/{sid}-{codec}.json"))=={**expected_sample,"meta":meta},"encoded evidence mismatch")
            for group,condition in CASES:
                r=m["records"][keys.index((sid,codec,group,condition))]
                mask,groups=grouped_mask(len(packets),group,condition,sid)
                expected_received=[p.hex() if p is not None else None for p in erase(packets,mask)]
                stem=f"{sid}-{codec}-g{group}-{condition}"
                evidence=load_json(child(output,f"evidence/{stem}-received.json"))
                require(evidence=={"received":expected_received,"frame_mask":mask,"group_mask":groups},"loss evidence mismatch")
                received=[bytes.fromhex(p) if p is not None else None for p in evidence["received"]]
                replay=decode(codec,received,meta,opus,c2)
                wav=f"wav/{stem}.wav";actual_pcm,_=read_pcm(child(output,wav))
                require(replay==actual_pcm,"WAV replay mismatch")
                recovery="PLC" if codec.startswith("opus") else "пропуски тишиной, без PLC"
                label=f"{codec} · без потерь" if condition=="clean" else f"{codec} · {group*20} мс · {condition} · {sum(groups)*100/len(groups):.1f}% · {recovery}"
                track={"codec":codec,"condition":condition,"group":group,"label":label,"path":wav}
                require(r=={"sample":sid,**track,"pcm":pcm_stats(replay),"lost_groups":sum(groups),"groups":len(groups),"lost_frames":sum(mask),"frames":len(mask),"meta":meta},"record metadata mismatch")
                sample["tracks"].append(track)
        samples.append(sample)
    audio_set=[{"path":str(p.relative_to(output)),**digest(p)} for p in sorted((output/"wav").glob("*.wav"))]
    audio_sha=hashlib.sha256(json.dumps(audio_set,sort_keys=True).encode()).hexdigest()
    data={"pack_id":"codec-comparison-20260906","audio_set_sha256":audio_sha,"samples":samples,"codecs":list(CODECS)}
    require(load_json(child(output,"listening-data.json"))==data,"UI data mismatch")
    embedded=json.dumps(data,ensure_ascii=False,sort_keys=True).replace("<","\\u003c").replace("&","\\u0026")
    template=Path(__file__).with_name("codec_comparison.html").read_text()
    require((output/"index.html").read_text()==template.replace("__PACK_DATA__",embedded),"UI binding mismatch")
    return {"records_reencoded_and_replayed":len(keys),"status":"verified-not-human-accepted","audio_set_sha256":audio_sha}
