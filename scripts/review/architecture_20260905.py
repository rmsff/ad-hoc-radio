#!/usr/bin/env python3
"""Characterize review findings without asserting that current bugs are correct."""

import argparse
import copy
import json
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]


def cases():
    baseline = json.loads(
        (ROOT / "configs/scenarios/relay-tree/one-relay.json").read_text()
    )
    yield "baseline", baseline

    loss = copy.deepcopy(baseline)
    for link in loss["initial_plan"]["graph"]["links"]:
        link["loss"] = 1
    yield "selected_links_loss_100_percent", loss

    for name, duration, guard in (
        ("dmr_inspired_30ms_zero_guard", 30, 0),
        ("synthetic_27ms_plus_3ms_guard_not_dmr", 27, 3),
    ):
        cfg = copy.deepcopy(baseline)
        cfg["initial_plan"]["cost_model"].update(
            source_opportunity_ms=duration,
            relay_opportunity_ms=duration,
            guard_after_ms=guard,
            frame_period_ms=60,
        )
        cfg["runtime"]["opportunity_profile"].update(
            source_duration_ms=duration,
            relay_duration_ms=duration,
            guard_after_ms=guard,
            max_cycle_ms=60,
        )
        cfg["runtime"]["allow_cycle_timing_stress"] = True
        yield name, cfg

    late = copy.deepcopy(baseline)
    late["initial_plan"]["graph"]["links"][1]["delay_ms"] = 300
    late["initial_plan"]["constraints"]["max_playout_latency_ms"] = 1000
    late["initial_plan"]["cost_model"]["frame_period_ms"] = 400
    late["runtime"]["opportunity_profile"]["max_cycle_ms"] = 400
    yield "accepted_but_late_for_playout", late


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path)
    args = parser.parse_args()
    results = []
    with tempfile.TemporaryDirectory(prefix="adhr-review-") as directory:
        scratch = Path(directory)
        binary = scratch / "adhr-sim"
        subprocess.run(
            ["go", "build", "-o", str(binary), "./cmd/adhr-sim"],
            cwd=ROOT,
            check=True,
        )
        for name, config in cases():
            config_path = scratch / f"{name}.json"
            metrics_path = scratch / f"{name}.metrics.json"
            config_path.write_text(json.dumps(config))
            subprocess.run(
                [str(binary), "run", str(config_path), "--metrics", str(metrics_path)],
                cwd=ROOT,
                check=True,
                capture_output=True,
                text=True,
            )
            metrics = json.loads(metrics_path.read_text())
            results.append({
                "case": name,
                "input": config,
                "outcome": metrics["outcome"],
                "receiver": metrics["receiver"],
                "latency_ms": metrics["latency_ms"],
                "scheduled_relay_tree": metrics["scheduled_relay_tree"],
                "links": metrics["links"],
            })
    output = json.dumps({
        "evidence_grade": "host_simulator_characterization_only",
        "rf_claim": False,
        "cases": results,
    }, indent=2) + "\n"
    if args.out:
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(output)
    for row in results:
        print(json.dumps({
            "case": row["case"],
            "outcome": row["outcome"],
            "receiver": row["receiver"],
            "latency_ms": row["latency_ms"],
            "missed_opportunities": row["scheduled_relay_tree"]["missed_opportunity_count"],
        }))


if __name__ == "__main__":
    main()
