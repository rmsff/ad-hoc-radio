"""Candidate comparison guards and actual native replay; no quality scoring."""
import tempfile
import unittest
import json
import shutil
from pathlib import Path

from common import read_pcm, digest, save
from local_data import LISTENING_PACK, require_local
from compare_codecs import BASE, adapters, decode, encode, generate, verify
from compare_codec_support import Codec2, grouped_mask
from opus_codec import erase


class GroupingTests(unittest.TestCase):
    def test_whole_groups_and_partial_tail(self):
        frames,groups=grouped_mask(51,4,"loss30","test")
        self.assertEqual(len(frames),51)
        self.assertEqual(len(groups),13)
        self.assertEqual(frames,[groups[i//4] for i in range(51)])
        self.assertEqual((frames,groups),grouped_mask(51,4,"loss30","test"))

    def test_burst_and_invalid(self):
        frames,_=grouped_mask(500,4,"burst240","ru01")
        indices=[i for i,v in enumerate(frames) if v]
        self.assertEqual(indices,list(range(indices[0],indices[0]+12)))
        for count,group,condition in ((0,4,"clean"),(3005,4,"clean"),(50,3,"clean"),(50,8,"burst240"),(50,4,"bad")):
            with self.assertRaises(ValueError):grouped_mask(count,group,condition,"s")


class NativeComparisonTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        require_local(LISTENING_PACK)
        cls.cfg,cls.source,cls.opus,cls.c2=adapters()
        cls.pcm,_=read_pcm(cls.source/"wav/ru01-input8k.wav")

    def test_native_packet_replay_all_candidates(self):
        for codec,size in (("opus-6000",None),("opus-8000",None),("codec2-2400",6),("codec2-3200",8)):
            with self.subTest(codec=codec):
                packets,meta=encode(codec,self.pcm,self.opus,self.c2)
                repeat,again=encode(codec,self.pcm,self.opus,self.c2)
                self.assertEqual(packets,repeat);self.assertEqual(meta,again)
                if size:self.assertTrue(all(len(p)==size for p in packets))
                mask,_=grouped_mask(len(packets),4,"loss20","ru01")
                received=erase(packets,mask)
                pcm=decode(codec,received,meta,self.opus,self.c2)
                self.assertEqual(len(pcm),len(self.pcm))
                self.assertEqual(pcm,decode(codec,received,meta,self.opus,self.c2))

    def test_codec2_all_lost_is_explicit_silence(self):
        self.assertEqual(self.c2.decode([None]*5,2400),bytes(1600))
        with self.assertRaises(ValueError):self.c2.decode([b"short"],2400)

    def test_opus6k_uses_real_packets(self):
        packets,meta=self.opus.encode(self.pcm,False,6000)
        self.assertEqual(meta["lookahead_samples"],52)
        self.assertEqual(sum(meta["lbrr"]),0)
        self.assertNotEqual(packets,self.opus.encode(self.pcm,False,8000)[0])


class ComparisonVerifierTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        require_local(LISTENING_PACK)
        BASE.mkdir(parents=True,exist_ok=True)
        cls.directory=tempfile.TemporaryDirectory(prefix="verify-test-",dir=BASE)
        cls.pack=Path(cls.directory.name)/"pack"
        generate(cls.pack)

    @classmethod
    def tearDownClass(cls):cls.directory.cleanup()

    def test_complete_native_pack(self):
        self.assertEqual(verify(self.pack)["records_reencoded_and_replayed"],56)

    def test_semantic_tampering_after_resigning(self):
        for case in ("inventory","source","delay","sample_count","duplicates","counters","received","ui","generator"):
            with self.subTest(case=case), tempfile.TemporaryDirectory(prefix="tamper-test-",dir=BASE) as directory:
                pack=Path(directory)/"pack";shutil.copytree(self.pack,pack)
                m=json.loads((pack/"manifest.json").read_text())
                if case in ("source","delay","sample_count"):
                    p=pack/"traces/codec2-2400.json";v=json.loads(p.read_text())
                    if case=="source":v["library_sha256"]="0"*64
                    if case=="delay":v["lookahead_us"]=0
                    if case=="sample_count":v["samples"][0]["input_samples"]+=1
                    save(p,v)
                if case=="duplicates":m["records"]=[m["records"][0]]*56
                if case=="counters":m["records"][1]["lost_frames"]=0
                if case=="received":
                    p=pack/"evidence/ru01-opus-6000-g4-loss20-received.json";v=json.loads(p.read_text());v["frame_mask"][0]=not v["frame_mask"][0];save(p,v)
                if case=="ui":
                    p=pack/"listening-data.json";v=json.loads(p.read_text());v["samples"][0]["tracks"][2]["label"]="false label";save(p,v)
                if case=="generator":m["generator_inputs"][0]["sha256"]="0"*64
                m["files"]=[{"path":f["path"],**digest(pack/f["path"])} for f in m["files"]]
                if case=="inventory":m["files"]=[f for f in m["files"] if f["path"]!="traces/codec2-2400.json"]
                save(pack/"manifest.json",m)
                with self.assertRaises(ValueError):verify(pack)


if __name__=="__main__":unittest.main()
