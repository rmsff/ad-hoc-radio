import unittest
from merge_codec_comparison import restore


class LyraTimelineTests(unittest.TestCase):
    def test_missing_frames_are_silence_not_compressed_time(self):
        first,second=b"ab"*160,b"cd"*160
        self.assertEqual(restore(first+second,[False,True,False]),first+bytes(320)+second)
        self.assertEqual(restore(b"",[True,True]),bytes(640))

    def test_reject_bad_count_and_mask(self):
        for data,mask in ((b"x",[False]),(bytes(320),[True]),(b"",[]),(b"",[1])):
            with self.assertRaises(ValueError):restore(data,mask)


if __name__=="__main__":unittest.main()
