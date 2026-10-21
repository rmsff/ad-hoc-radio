"""Combine verified native codec packs; Lyra erasures use explicit frozen-state silence."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess

from common import digest, pcm_stats, read_pcm, save, write_wav
from compare_codecs import BASE, CASES, verify
from compare_codec_support import grouped_mask
from lyra_verify import verify as verify_lyra
from opus_generate import ROOT, child

LYRA=BASE/"lyra/final-1"
CLOSURE="a0d0b8f9fb0e0062fea4a0ca4497bfb6f48def150be02de4664b4da043981db6"
TRACE="cd6743c766cc52067196b443ab481c0cb4dcd9c693c5702162b28128becbdcea"


def restore(decoded, mask):
    if not mask or any(type(x) is not bool for x in mask) or len(decoded)!=mask.count(False)*320:
        raise ValueError("Lyra received timeline mismatch")
    out,cursor=bytearray(),0
    for lost in mask:
        if lost:out.extend(bytes(320))
        else:out.extend(decoded[cursor:cursor+320]);cursor+=320
    return bytes(out)


def generate(pack, output):
    pack=Path(pack).resolve();output=Path(output).absolute()
    if not pack.is_relative_to(BASE) or not output.is_relative_to(BASE) or output==BASE or output.is_relative_to(pack):
        raise ValueError("comparison paths must be separate")
    verify(pack);verify_lyra(LYRA,BASE/"lyra/final-2")
    if digest(LYRA/"library-closure.json")["sha256"]!=CLOSURE or digest(LYRA/"packet-trace.json")["sha256"]!=TRACE:
        raise ValueError("Lyra pinned native closure/trace changed")
    child(ROOT,str(output.relative_to(ROOT)));output.mkdir(parents=True,exist_ok=False)
    for p in sorted(pack.rglob("*")):
        if p.is_file() and p.name not in ("index.html","listening-data.json","manifest.json","host-timing.json"):
            dest=output/p.relative_to(pack);dest.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(p,dest)
    data=json.loads((pack/"listening-data.json").read_text())
    data["pack_id"]="codec-comparison-with-lyra-20260906"
    data["codecs"].append("lyra-v2-3200")
    data["notice"]="Lyra: задержка не измерена; потери заменены тишиной, без PLC/FEC. Модель пока не подходит для прямого размещения во внутренней flash TYT."
    trace=json.loads((LYRA/"packet-trace.json").read_text())
    shutil.copyfile(LYRA/"packet-trace.json",output/"traces/lyra-v2-3200.json")
    shutil.copyfile(LYRA/"LYRA-LICENSE.txt",output/"LYRA-LICENSE.txt")
    shutil.copyfile(LYRA/"library-closure.json",output/"evidence/lyra-library-closure.json")
    attribution=(pack/"ATTRIBUTION.txt").read_text()+"\n"+(LYRA/"ATTRIBUTION.txt").read_text()+"\nAdditional Lyra loss derivatives: whole groups removed before decoding; decoder state frozen across gaps, silence inserted at missing frame times. Output truncated to source sample count, no algorithmic delay compensation or delayed-tail preservation claim.\n"
    (output/"ATTRIBUTION.txt").write_text(attribution)
    rows=[]
    for sample in trace["samples"]:
        sid=sample["id"];target=next(s for s in data["samples"] if s["id"]==sid)
        packets=[bytes.fromhex(p) for p in sample["packets_hex"]]
        for group,condition in CASES:
            mask,group_mask=grouped_mask(len(packets),group,condition,sid)
            received=b"".join(p for p,lost in zip(packets,mask) if not lost)
            stem=f"{sid}-lyra-v2-3200-g{group}-{condition}"
            encoded=output/f"evidence/{stem}.lyra";encoded.write_bytes(received)
            native=output/f"evidence/{stem}_decoded.wav"
            command=[str(LYRA/"bin/decoder_main"),f"--encoded_path={encoded}",f"--output_dir={output/'evidence'}",f"--model_path={LYRA/'model_coeffs'}",
                "--sample_rate_hz=8000","--packet_loss_rate=0","--randomize_num_samples_requested=false","--bitrate=3200"]
            if received:
                subprocess.run(command,stdout=subprocess.DEVNULL,stderr=subprocess.PIPE,check=True,timeout=120)
                decoded,_=read_pcm(native)
            else:
                decoded=b""
            full=restore(decoded,mask)[:sample["input_samples"]*2]
            wav=f"wav/{stem}.wav";write_wav(output/wav,full)
            if condition=="clean":
                clean,_=read_pcm(LYRA/f"{sid}-padded8k_decoded.wav")
                if full!=clean[:len(full)]:raise ValueError("native clean replay changed")
            label="lyra-v2-3200 · без потерь · задержка не компенсирована" if condition=="clean" else f"lyra-v2-3200 · {group*20} мс · {condition} · {sum(group_mask)*100/len(group_mask):.1f}% · пропуски тишиной, без PLC"
            target["tracks"].append({"codec":"lyra-v2-3200","condition":condition,"group":group,"label":label,"path":wav})
            row={"sample":sid,"group":group,"condition":condition,"frame_mask":mask,"group_mask":group_mask,
                "received_packets":[None if lost else p.hex() for p,lost in zip(packets,mask)],"wav":wav,"pcm":pcm_stats(full),
                "recovery":"native decode of retained frames; frozen-state silence insertion, not PLC","lookahead_us":None}
            save(output/f"evidence/{stem}-received.json",row);rows.append(row)
    audio_set=[{"path":str(p.relative_to(output)),**digest(p)} for p in sorted((output/"wav").glob("*.wav"))]
    data["audio_set_sha256"]=hashlib.sha256(json.dumps(audio_set,sort_keys=True).encode()).hexdigest()
    save(output/"listening-data.json",data)
    template=Path(__file__).with_name("codec_comparison.html").read_text()
    template=template.replace("<h1>Сравнение кодеков</h1>","<h1>Сравнение кодеков</h1><p class=\"note\">"+data["notice"]+"</p>")
    encoded=json.dumps(data,ensure_ascii=False,sort_keys=True).replace("<","\\u003c").replace("&","\\u0026")
    (output/"index.html").write_text(template.replace("__PACK_DATA__",encoded))
    manifest={"version":1,"base_manifest_sha256":digest(pack/"manifest.json")["sha256"],"lyra_trace_sha256":TRACE,"lyra_closure_sha256":CLOSURE,
        "lyra_records":rows,"audio_set_sha256":data["audio_set_sha256"],"quality_status":"pending-user-listening","hardware_executed":False,
        "generator":{"path":str(Path(__file__).relative_to(ROOT)),**digest(Path(__file__))},
        "files":[{"path":str(p.relative_to(output)),**digest(p)} for p in sorted(output.rglob("*")) if p.is_file()]}
    save(output/"manifest.json",manifest)
    return {"pack":str(output),"lyra_records":len(rows),"total_audio":len(audio_set),"manifest_sha256":digest(output/"manifest.json")["sha256"]}


if __name__=="__main__":
    p=argparse.ArgumentParser();p.add_argument("--pack",type=Path,required=True);p.add_argument("--output",type=Path,required=True);a=p.parse_args();print(json.dumps(generate(a.pack,a.output),indent=2))
