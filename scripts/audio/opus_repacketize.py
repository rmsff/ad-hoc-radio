"""Pinned native Opus regrouping; no codec re-encoding or new radio format."""
import ctypes as C

from opus_codec import FRAME, MAX_PACKET_BYTES, RATE


class Repacketizer:
    def __init__(self, opus):
        self.opus = opus
        self.lib = opus.lib
        p, i = C.c_void_p, C.c_int
        for name, result, args in (
            ("opus_repacketizer_create", p, []),
            ("opus_repacketizer_destroy", None, [p]),
            ("opus_repacketizer_cat", i, [p, p, C.c_int32]),
            ("opus_repacketizer_out", C.c_int32, [p, p, C.c_int32]),
            ("opus_packet_parse", i, [p, C.c_int32, p, p, p, p]),
        ):
            opus._bind(name, result, args)

    def bodies(self, packet):
        """Count actual compressed frame bodies excluding ALL Opus framing."""
        self.opus.check_packet(packet)
        sizes = (C.c_int16 * 48)()
        pointers = (C.c_void_p * 48)()
        toc, offset = C.c_ubyte(), C.c_int()
        n = self.lib.opus_packet_parse(packet, len(packet), C.byref(toc),
                                     pointers, sizes, C.byref(offset))
        if not 1 <= n <= 48 or any(sizes[j] < 0 for j in range(n)):
            raise ValueError("invalid native packet parse")
        return sum(sizes[:n])

    def join(self, packets):
        if not 1 <= len(packets) <= 6:
            raise ValueError("native chunk must contain1..6 twenty-ms packets")
        for p in packets:
            self.opus.check_packet(p)
        state = self.lib.opus_repacketizer_create()
        if not state:
            raise ValueError("native repacketizer allocation")
        try:
            # cat retains pointers: the immutable input bytes stay alive here.
            for p in packets:
                if self.lib.opus_repacketizer_cat(state, p, len(p)):
                    raise ValueError("incompatible native Opus chunk")
            out = (C.c_ubyte * (6 * MAX_PACKET_BYTES + 64))()
            n = self.lib.opus_repacketizer_out(state, out, len(out))
            if not 1 <= n <= len(out):
                raise ValueError("native repacketizer output")
            data = bytes(out[:n])
            if self.lib.opus_packet_get_nb_samples(data, n, RATE) != len(packets)*FRAME:
                raise ValueError("native duration mismatch")
            return data
        finally:
            self.lib.opus_repacketizer_destroy(state)

    def group(self, packets, group):
        if type(group) is not int or group not in (1, 2, 4, 8):
            raise ValueError("outer group must be1/2/4/8")
        if not 1 <= len(packets) <= 3004:
            raise ValueError("input packet count")
        for p in packets:
            self.opus.check_packet(p)
        result = []
        for start in range(0, len(packets), group):
            chunks, pending = [], []
            for p in packets[start:start+group]:
                if pending and (len(pending) == 6 or p[0] >> 2 != pending[0][0] >> 2):
                    chunks.append(self.join(pending))
                    pending = []
                pending.append(p)
            if pending:
                chunks.append(self.join(pending))
            result.append(chunks)
        return result

    def decode(self, chunks):
        if not 1 <= len(chunks) <= 3004:
            raise ValueError("native decode packet count")
        for p in chunks:
            if not isinstance(p, bytes) or not 1 <= len(p) <= 6*MAX_PACKET_BYTES+64:
                raise ValueError("native decode bytes")
            samples = self.lib.opus_packet_get_nb_samples(p, len(p), RATE)
            if samples not in (160, 320, 480, 640, 800, 960):
                raise ValueError("native decode duration")
        error = C.c_int()
        state = self.lib.opus_decoder_create(RATE, 1, C.byref(error))
        if not state or error.value:
            raise ValueError("native decoder allocation")
        try:
            pcm = bytearray()
            for p in chunks:
                out = (C.c_int16 * (6*FRAME))()
                n = self.lib.opus_decode(state, p, len(p), out, len(out), 0)
                expected = self.lib.opus_packet_get_nb_samples(p, len(p), RATE)
                if n != expected:
                    raise ValueError("native decode failed")
                pcm.extend(bytes(out)[:n*2])
            return bytes(pcm)
        finally:
            self.lib.opus_decoder_destroy(state)
