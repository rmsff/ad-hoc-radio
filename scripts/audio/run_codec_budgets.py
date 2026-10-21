"""Run all local comparison traces twice through the unchanged simulator runtime."""
import argparse
import json
from pathlib import Path
import subprocess

from common import digest, save
from opus_generate import ROOT, child


def run(pack, output):
    pack=Path(pack).resolve();output=Path(output).absolute()
    base=ROOT/".artifacts/codec-comparison-20260906"
    if not pack.is_relative_to(base) or not output.is_relative_to(base) or output==base or output.is_relative_to(pack):
        raise ValueError("paths must stay in separate comparison directories")
    child(ROOT,str(output.relative_to(ROOT)));output.mkdir(parents=True,exist_ok=False)
    traces=sorted((pack/"traces").glob("*.json"))
    if not 1<=len(traces)<=8:raise ValueError("trace count")
    rows=[]
    for trace in traces:
        outputs=[]
        for repeat in (1,2):
            target=output/f"{trace.stem}-r{repeat}.json"
            subprocess.run(["go","run","./cmd/adhr-sim","codec-budget","--input",str(trace),"--report",str(target)],cwd=ROOT,check=True,timeout=120)
            outputs.append(target)
        if outputs[0].read_bytes()!=outputs[1].read_bytes():raise ValueError("non-deterministic budget")
        data=json.loads(outputs[0].read_text())
        rows.append({"codec":data["codec"],"trace":digest(trace),"report":digest(outputs[0]),"rows":len(data["rows"]),
                     "pass":sum(r["envelope"]["trial"]["service_outcome"]=="pass" for r in data["rows"]),
                     "partial":sum(r["envelope"]["trial"]["admission"]=="admitted" and r["envelope"]["trial"]["service_outcome"]!="pass" for r in data["rows"])})
    save(output/"summary.json",{"reports":rows,"evidence":"simulation_only_not_hardware_or_audio_acceptance"});return rows


if __name__=="__main__":
    p=argparse.ArgumentParser();p.add_argument("--pack",type=Path,required=True);p.add_argument("--output",type=Path,required=True);a=p.parse_args();print(json.dumps(run(a.pack,a.output),indent=2))
