"""Focused offline audit tests, including the actual pinned models.

Skips only when the local pinned models are absent (fresh clone)."""
import hashlib
import json
from pathlib import Path
import tempfile
import unittest

from local_data import require_local
import lyra_target_audit as audit


def state_graph(index=0, shape=None):
    shape = [1, 2] if shape is None else shape
    return {"index": index,
            "tensors": [{"type": "RESOURCE", "shape": [], "static_bytes": None},
                        {"type": "FLOAT32", "shape": shape,
                         "static_bytes": audit.tensor_bytes(shape, "FLOAT32")}],
            "operators": [
                {"index": 0, "name": "VAR_HANDLE", "inputs": [], "outputs": [0],
                 "resource_key": ["", "state"]},
                {"index": 1, "name": "READ_VARIABLE", "inputs": [0], "outputs": [1]},
                {"index": 2, "name": "ASSIGN_VARIABLE", "inputs": [0, 1], "outputs": []}]}


class AuditUnitTests(unittest.TestCase):
    def test_dense_scalar_zero_and_dynamic(self):
        self.assertEqual(audit.tensor_bytes([], "INT32"), 4)
        self.assertEqual(audit.tensor_bytes([1, 320], "FLOAT32"), 1280)
        self.assertEqual(audit.tensor_bytes([0], "FLOAT32"), 0)
        self.assertIsNone(audit.tensor_bytes([1], "FLOAT32", [-1]))
        self.assertIsNone(audit.tensor_bytes([], "FLOAT32", has_rank=False))
        self.assertIsNone(audit.tensor_bytes([], "RESOURCE"))

    def test_reject_invalid_and_oversized_shapes(self):
        for shape in ([True], [-2], [1.5], [2] * 9, [1048576, 1048576]):
            with self.subTest(shape=shape), self.assertRaises(ValueError):
                audit.tensor_bytes(shape, "FLOAT32")
        with self.assertRaises(ValueError):
            audit.tensor_bytes([1], "STRING")

    def test_constants_count_storage_once_across_types(self):
        constants = {}
        audit.add_constant(constants, 1, "FLOAT32", 4)
        audit.add_constant(constants, 1, "INT32", 4)
        audit.add_constant(constants, 1, "FLOAT32", 4)
        self.assertEqual(constants, {1: {"type": "FLOAT32|INT32", "bytes": 4}})
        with self.assertRaises(ValueError):
            audit.add_constant(constants, 1, "FLOAT32", 8)

    def test_resources_deduplicated_across_subgraphs(self):
        result = audit.resource_summary([state_graph(), state_graph(1)])
        self.assertEqual(len(result), 1)
        self.assertEqual(result[0]["bytes"], 8)
        self.assertEqual(len(result[0]["reads"]), 2)

    def test_resource_namespaces_are_distinct(self):
        other = state_graph(1)
        other["operators"][0]["resource_key"][0] = "other"
        self.assertEqual(len(audit.resource_summary([state_graph(), other])), 2)

    def test_reject_inconsistent_or_dynamic_state(self):
        with self.assertRaises(ValueError):
            audit.resource_summary([state_graph(), state_graph(1, [1, 3])])
        with self.assertRaises(ValueError):
            audit.resource_summary([state_graph(shape=[-1])])
        graph = state_graph()
        graph["operators"].pop()
        with self.assertRaises(ValueError):
            audit.resource_summary([graph])

    def test_paths_pins_bounded_reads_and_exclusive_outputs(self):
        require_local(audit.ROOT / audit.AUDIT)
        with tempfile.TemporaryDirectory(dir=audit.ROOT / audit.AUDIT) as temp:
            root = Path(temp).resolve()
            (root / audit.AUDIT).mkdir(parents=True)
            source = root / "sample"
            source.write_bytes(b"test")
            expected = hashlib.sha256(b"test").hexdigest()
            self.assertEqual(audit.pinned_read(root, "sample", expected), b"test")
            for path in ("../sample", str(source), ""):
                with self.subTest(path=path), self.assertRaises(ValueError):
                    audit.safe_path(root, path)
            with self.assertRaises(ValueError):
                audit.pinned_read(root, "sample", expected, 3)
            source.write_bytes(b"fail")
            with self.assertRaises(ValueError):
                audit.pinned_read(root, "sample", expected)
            (root / "link").symlink_to(source)
            with self.assertRaises(ValueError):
                audit.safe_path(root, "link")
            for label in ("../old", "x/y", "Upper", "x" * 49, "", "тест"):
                with self.subTest(label=label), self.assertRaises(ValueError):
                    audit.write_report(root, label, {})
            dest = audit.write_report(root, "test-1", {"value": 1})
            before = dest.read_bytes()
            with self.assertRaises(FileExistsError):
                audit.write_report(root, "test-1", {"value": 2})
            self.assertEqual(dest.read_bytes(), before)


class PinnedModelTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        require_local(audit.ROOT / audit.AUDIT)
        cls.schema = audit.load_schema()
        cls.report = audit.build_report(cls.schema)

    def test_repeat_is_deterministic(self):
        self.assertEqual(json.dumps(self.report, sort_keys=True),
                         json.dumps(audit.build_report(self.schema), sort_keys=True))

    def test_actual_resource_payloads_and_bounds(self):
        for name, count, size, bound in (
                ("soundstream_encoder.tflite", 14, 55232, 75712),
                ("lyragan.tflite", 18, 51648, 72128),
                ("quantizer.tflite", 0, 0, 256)):
            with self.subTest(name=name):
                m = self.report["models"][name]
                self.assertEqual(len(m["resource_states"]), count)
                self.assertEqual(m["persistent_state_payload_bytes"], size)
                self.assertEqual(m["state_plus_largest_main_tensor_lower_bound_bytes"], bound)
                self.assertIsNone(m["peak_arena_bytes"])
                self.assertIsNone(m["mcu_cycles_per_frame"])

    def test_mixed_models_and_quantizer_graph(self):
        for name in ("soundstream_encoder.tflite", "lyragan.tflite"):
            m = self.report["models"][name]
            self.assertGreater(m["unique_convolution_filter_bytes_by_type"]["FLOAT32"], 0)
            self.assertGreater(m["unique_convolution_filter_bytes_by_type"]["INT8"], 0)
            self.assertTrue(all(not t["is_variable"] for g in m["graphs"] for t in g["tensors"]))
        q = self.report["models"]["quantizer.tflite"]["graphs"][1]
        self.assertEqual(q["operator_counts"]["ONE_HOT"], 46)
        self.assertEqual(q["largest_static_nonconstant_bytes"], 4096)
        signatures = self.report["models"]["quantizer.tflite"]["signatures"]
        self.assertEqual({s["key"]: s["subgraph"] for s in signatures},
                         {"encode": 1, "decode": 0})
        self.assertEqual(self.report["tflm_registration_name_check"]["missing_by_model"],
                         {"lyragan.tflite": [], "soundstream_encoder.tflite": [],
                          "quantizer.tflite": ["ONE_HOT"]})


if __name__ == "__main__":
    unittest.main()
