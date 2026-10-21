#!/usr/bin/env python3
"""Offline, pinned Lyra graph audit; payload bounds are NOT peak arena sizes.

Run with python3 -B -S. Requires the retained local audit/input-pins.json and
generated schema; deliberately accepts no arbitrary model or source paths.
"""
import argparse
from collections import Counter
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import stat
import sys

ROOT = Path(__file__).resolve().parents[2]
AUDIT = ".artifacts/lyra-target-audit-20260908"
OLD = ".artifacts/codec-comparison-20260906/lyra"
BAZEL = OLD + "/bazel-user/<output-base>"
RUNTIME = BAZEL + "/external/flatbuffers/python"
PINS_SHA = "39c664e61ad34d575708dd3b84461538a20c21477180c3b45306f82d82f2e6e9"
MODEL_PINS = {
    "lyragan.tflite": "50e7839cda4f30599a3a34cfaddcd9c98449b049eee884e1a74295780448ad06",
    "quantizer.tflite": "841ee05989687d32d1f616a8ad1419fc7eda37599d3c05c7cf5a0d60ccf8ac5c",
    "soundstream_encoder.tflite": "237da0b16226fb0a68d49551c81bebf614e42142af72764448359f68581e88f1",
}
WIDTHS = {"FLOAT32": 4, "INT8": 1, "INT32": 4, "BOOL": 1}
MAX_FILE = 16 * 1024 * 1024


def safe_path(root, relative):
    p = Path(relative)
    if p.is_absolute() or not p.parts or ".." in p.parts:
        raise ValueError("expected a contained relative path")
    target = root / p
    if target.resolve() != target:
        raise ValueError("symlink or noncanonical path rejected")
    return target


def pinned_read(root, relative, expected, limit=MAX_FILE):
    path = safe_path(root, relative)
    info = path.stat()
    if not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= limit:
        raise ValueError("input is not a bounded regular file")
    with path.open("rb") as f:
        data = f.read(limit + 1)
    if len(data) > limit or hashlib.sha256(data).hexdigest() != expected:
        raise ValueError("input hash mismatch: " + relative)
    return data


def verify_inputs():
    rows = json.loads(pinned_read(ROOT, AUDIT + "/input-pins.json", PINS_SHA))
    if not isinstance(rows, list) or not 1 <= len(rows) <= 128:
        raise ValueError("invalid pin inventory")
    seen = set()
    for row in rows:
        if set(row) != {"path", "bytes", "sha256"} or row["path"] in seen:
            raise ValueError("invalid or duplicate pin")
        seen.add(row["path"])
        data = pinned_read(ROOT, row["path"], row["sha256"])
        if len(data) != row["bytes"]:
            raise ValueError("input size mismatch")
    return rows


def load_schema():
    # Check code before importing it; -S keeps optional host packages out.
    verify_inputs()
    sys.dont_write_bytecode = True
    if "flatbuffers" in sys.modules:
        origin = Path(sys.modules["flatbuffers"].__file__).resolve()
        if origin != ROOT / RUNTIME / "flatbuffers/__init__.py":
            raise ValueError("unexpected preloaded flatbuffers runtime")
    sys.path.insert(0, str(ROOT / RUNTIME))
    spec = importlib.util.spec_from_file_location(
        "lyra_audit_schema", ROOT / AUDIT / "schema_generated.py")
    schema = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(schema)
    return schema


def vector(obj, field, maximum=4096):
    n = getattr(obj, field + "Length")()
    if not 0 <= n <= maximum:
        raise ValueError("oversized vector: " + field)
    return [getattr(obj, field)(i) for i in range(n)]


def string(value):
    if value is None:
        return ""
    if len(value) > 4096:
        raise ValueError("oversized name")
    return value.decode("utf-8", errors="strict")


def tensor_bytes(shape, kind, signature=(), has_rank=True):
    if kind == "RESOURCE":
        return None  # Handle size is not the size of its backing state.
    if kind not in WIDTHS or len(shape) > 8 or len(signature) > 8:
        raise ValueError("unsupported tensor representation")
    if any(type(d) is not int or d < -1 or d > 1048576
           for d in (*shape, *signature)):
        raise ValueError("invalid tensor dimension")
    if not has_rank or -1 in shape or -1 in signature:
        return None
    size = WIDTHS[kind]
    for d in shape:
        size *= d
        if size > MAX_FILE:
            raise ValueError("oversized tensor")
    return size


def enum_names(cls):
    return {v: k for k, v in vars(cls).items() if type(v) is int}


def add_constant(constants, buffer_id, kind, size):
    row = constants.setdefault(buffer_id, {"type": kind, "bytes": size})
    if row["bytes"] != size:
        raise ValueError("inconsistent shared constant size")
    row["type"] = "|".join(sorted(set(row["type"].split("|")) | {kind}))


def resource_summary(graphs):
    resources = {}
    for graph in graphs:
        handles = {}
        for op in graph["operators"]:
            if op["name"] != "VAR_HANDLE":
                continue
            key = tuple(op["resource_key"])
            if not key[1] or len(op["outputs"]) != 1:
                raise ValueError("unnamed or malformed resource")
            handles[op["outputs"][0]] = key
            resources.setdefault(key, {"container": key[0], "shared_name": key[1],
                                       "reads": [], "writes": []})
        for op in graph["operators"]:
            name = op["name"]
            if name not in ("READ_VARIABLE", "ASSIGN_VARIABLE"):
                continue
            ins, outs = op["inputs"], op["outputs"]
            if ((name == "READ_VARIABLE" and (len(ins), len(outs)) != (1, 1))
                    or (name == "ASSIGN_VARIABLE" and (len(ins), len(outs)) != (2, 0))):
                raise ValueError("malformed resource operation")
            row = resources[handles[ins[0]]]
            ti = outs[0] if name == "READ_VARIABLE" else ins[1]
            t = graph["tensors"][ti]
            representation = (t["type"], t["shape"], t["static_bytes"])
            if t["static_bytes"] is None:
                raise ValueError("dynamic resource payload cannot be bounded here")
            if "representation" in row and row["representation"] != representation:
                raise ValueError("resource representation changes")
            row["representation"] = representation
            row["reads" if name == "READ_VARIABLE" else "writes"].append(
                {"graph": graph["index"], "op": op["index"], "tensor": ti})
    result = []
    for key, row in sorted(resources.items()):
        if not row["reads"] or not row["writes"]:
            raise ValueError("incomplete persistent-state evidence")
        kind, shape, size = row.pop("representation")
        result.append({**row, "type": kind, "shape": shape, "bytes": size})
    return result


def inspect_model(schema, name):
    relative = OLD + "/upstream/lyra/model_coeffs/" + name
    data = pinned_read(ROOT, relative, MODEL_PINS[name], 2 * 1024 * 1024)
    if data[4:8] != b"TFL3":
        raise ValueError("invalid TFLite identifier")
    m = schema.Model.GetRootAsModel(data, 0)
    if m.Version() != 3 or not 1 <= m.SubgraphsLength() <= 8:
        raise ValueError("unsupported model version or graph count")
    op_names, type_names = enum_names(schema.BuiltinOperator), enum_names(schema.TensorType)
    codes = []
    for c in vector(m, "OperatorCodes", 128):
        code = max(c.BuiltinCode(), c.DeprecatedBuiltinCode())
        if code not in op_names or op_names[code] == "CUSTOM":
            raise ValueError("unsupported operator code")
        codes.append({"name": op_names[code], "version": c.Version()})
    buffers = [b.DataLength() for b in vector(m, "Buffers")]
    graphs, constants, filters = [], {}, {}
    for gi, g in enumerate(vector(m, "Subgraphs", 8)):
        tensors = []
        for ti, t in enumerate(vector(g, "Tensors")):
            kind = type_names[t.Type()]
            shape, signature = vector(t, "Shape", 8), vector(t, "ShapeSignature", 8)
            size = tensor_bytes(shape, kind, signature, bool(t.HasRank()))
            if not 0 <= t.Buffer() < len(buffers) or t.Sparsity() or t.IsVariable():
                raise ValueError("unsupported buffer, sparsity or inline variable")
            stored = buffers[t.Buffer()]
            if stored and stored != size:
                raise ValueError("constant size does not match dense shape")
            if stored:
                add_constant(constants, t.Buffer(), kind, stored)
            q = t.Quantization()
            scales = vector(q, "Scale") if q else []
            zero_points = vector(q, "ZeroPoint") if q else []
            tensors.append({"index": ti, "name": string(t.Name()), "type": kind,
                            "shape": shape, "shape_signature": signature,
                            "has_rank": bool(t.HasRank()), "static_bytes": size,
                            "buffer": t.Buffer(), "stored_bytes": stored,
                            "is_variable": bool(t.IsVariable()),
                            "quantization": {"scale": scales, "zero_point": zero_points,
                                             "dimension": q.QuantizedDimension() if q else None}})
        operators = []
        for oi, o in enumerate(vector(g, "Operators")):
            if not 0 <= o.OpcodeIndex() < len(codes):
                raise ValueError("invalid operator index")
            inputs, outputs = vector(o, "Inputs"), vector(o, "Outputs")
            if any(i < -1 or i >= len(tensors) for i in inputs) or any(
                    i < 0 or i >= len(tensors) for i in outputs):
                raise ValueError("invalid tensor reference")
            op = {"index": oi, **codes[o.OpcodeIndex()], "inputs": inputs, "outputs": outputs}
            if op["name"] == "VAR_HANDLE":
                table = o.BuiltinOptions()
                options = schema.VarHandleOptions()
                options.Init(table.Bytes, table.Pos)
                op["resource_key"] = [string(options.Container()), string(options.SharedName())]
            if op["name"] == "CALL_ONCE":
                table = o.BuiltinOptions()
                options = schema.CallOnceOptions()
                options.Init(table.Bytes, table.Pos)
                op["init_subgraph"] = options.InitSubgraphIndex()
            if op["name"] in ("CONV_2D", "DEPTHWISE_CONV_2D", "TRANSPOSE_CONV"):
                wi = inputs[1]
                activation = inputs[2] if op["name"] == "TRANSPOSE_CONV" else inputs[0]
                op["convolution_types"] = [tensors[activation]["type"], tensors[wi]["type"],
                                           tensors[outputs[0]]["type"]]
                weight = tensors[wi]
                if not weight["stored_bytes"]:
                    raise ValueError("nonconstant convolution filter")
                filters[weight["buffer"]] = {"type": weight["type"], "bytes": weight["stored_bytes"]}
            operators.append(op)
        io = {k.lower(): vector(g, k) for k in ("Inputs", "Outputs")}
        if any(i < 0 or i >= len(tensors) for ids in io.values() for i in ids):
            raise ValueError("invalid graph interface")
        live = [t for t in tensors if not t["stored_bytes"] and t["static_bytes"] is not None]
        largest = max(live, key=lambda t: t["static_bytes"], default=None)
        graphs.append({"index": gi, "name": string(g.Name()), **io,
                       "tensor_type_counts": dict(Counter(t["type"] for t in tensors)),
                       "operator_counts": dict(Counter(o["name"] for o in operators)),
                       "largest_static_nonconstant_tensor": largest["index"] if largest else None,
                       "largest_static_nonconstant_bytes": largest["static_bytes"] if largest else 0,
                       "tensors": tensors, "operators": operators})
    states = resource_summary(graphs)
    state_bytes = sum(r["bytes"] for r in states)
    signatures = []
    for signature in vector(m, "SignatureDefs", 16):
        gi = signature.SubgraphIndex()
        if not 0 <= gi < len(graphs):
            raise ValueError("invalid signature graph")
        row = {"key": string(signature.SignatureKey()), "subgraph": gi}
        for field in ("Inputs", "Outputs"):
            mappings = vector(signature, field, 16)
            if any(x.TensorIndex() >= len(graphs[gi]["tensors"]) for x in mappings):
                raise ValueError("invalid signature tensor")
            row[field.lower()] = {string(x.Name()): x.TensorIndex() for x in mappings}
        signatures.append(row)
    def histogram(rows):
        counts = Counter()
        for row in rows.values():
            counts[row["type"]] += row["bytes"]
        return dict(counts)
    return {"path": relative, "sha256": MODEL_PINS[name], "file_bytes": len(data),
            "operator_codes": codes, "graphs": graphs, "signatures": signatures,
            "resource_states": states,
            "unique_constant_buffer_bytes_by_type": histogram(constants),
            "unique_convolution_filter_bytes_by_type": histogram(filters),
            "persistent_state_payload_bytes": state_bytes,
            "state_plus_largest_main_tensor_lower_bound_bytes":
                state_bytes + graphs[0]["largest_static_nonconstant_bytes"],
            "peak_arena_bytes": None, "mcu_cycles_per_frame": None}


def build_report(schema):
    pins = verify_inputs()
    models = {name: inspect_model(schema, name) for name in sorted(MODEL_PINS)}
    inventory = json.loads((ROOT / OLD / "inventory.json").read_text())
    resolver_path = (ROOT / AUDIT / "primary-sources/tflite-micro/tensorflow/lite/micro/"
                     "micro_mutable_op_resolver.h")
    registered = set(re.findall(r"AddBuiltin\(BuiltinOperator_([A-Z0-9_]+)",
                                resolver_path.read_text()))
    return {"version": 1, "upstream_commit": inventory["upstream_commit"],
            "input_pins_sha256": PINS_SHA, "input_pins": pins,
            "measurement_boundary": "static dense payload bounds; not arena, heap, cycles or MCU execution",
            "target": inventory["target"], "models": models,
            "tflm_registration_name_check": {
                "commit": "bfeb43aa795adad4044bbf2e709062b1b989a98a",
                "not_a_type_version_or_execution_check": True,
                "missing_by_model": {n: sorted({o["name"] for o in m["operator_codes"]} - registered)
                                     for n, m in models.items()}},
            "bound_assumptions": [
                "Resource key is (container, shared_name), deduplicated across subgraphs.",
                "Every read and assignment must agree on dense type and static shape.",
                "Persistent buffers are separate from READ_VARIABLE copies in inspected runtimes.",
                "State plus one largest main-graph tensor excludes scratch, other live tensors, metadata, alignment and application memory.",
                "No sum of all tensors is presented as peak; transformed/tiled/recomputed models are outside this bound.",
                "Encoder and decoder bounds are separate; half duplex does not require summing both arenas."]}


def write_report(root, label, report):
    if not re.fullmatch(r"[a-z][a-z0-9-]{0,47}", label):
        raise ValueError("label must be 1-48 lowercase ASCII letters/digits/hyphens")
    target = safe_path(root, AUDIT + "/" + label + ".json")
    with target.open("x", encoding="utf-8") as f:
        json.dump(report, f, indent=2, sort_keys=True, allow_nan=False)
        f.write("\n")
    return target


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--label", default="audit")
    args = parser.parse_args()
    if not re.fullmatch(r"[a-z][a-z0-9-]{0,47}", args.label):
        parser.error("invalid output label")
    report = build_report(load_schema())
    print(write_report(ROOT, args.label, report).relative_to(ROOT))


if __name__ == "__main__":
    main()
