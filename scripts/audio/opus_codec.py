"""Optional host-only Opus ABI and causal one-packet-lookahead decoder."""
import ctypes as C
import hashlib
import sys
from pathlib import Path

RATE = 8000
FRAME = 160
MAX_PACKETS = 3004
MAX_PACKET_BYTES = 1275
NB = 1101


class Opus:
    def __init__(self, path, expected_sha):
        path = Path(path)
        if path.is_symlink() or hashlib.sha256(path.read_bytes()).hexdigest() != expected_sha:
            raise ValueError("Opus library hash mismatch/symlink")
        if sys.byteorder != "little":
            raise ValueError("PCM fixture requires little-endian host")
        self.lib = C.CDLL(str(path.resolve()))
        self._bind("opus_get_version_string", C.c_char_p, [])
        self.version = self.lib.opus_get_version_string().decode("ascii")
        if self.version != "libopus 1.5.2":
            raise ValueError("requires the pinned libopus 1.5.2")
        pointer, integer = C.c_void_p, C.c_int
        self._bind("opus_encoder_create", pointer, [integer, integer, integer, C.POINTER(integer)])
        self._bind("opus_encoder_destroy", None, [pointer])
        self._bind("opus_encoder_ctl", integer, [pointer, integer])
        self._bind("opus_encode", integer, [pointer, pointer, integer, pointer, C.c_int32])
        self._bind("opus_decoder_create", pointer, [integer, integer, C.POINTER(integer)])
        self._bind("opus_decoder_destroy", None, [pointer])
        self._bind("opus_decode", integer, [pointer, pointer, C.c_int32, pointer, integer, integer])
        self._bind("opus_packet_has_lbrr", integer, [pointer, C.c_int32])
        self._bind("opus_packet_get_nb_samples", integer, [pointer, C.c_int32, integer])
        self._bind("opus_packet_get_nb_channels", integer, [pointer])
        self._bind("opus_packet_get_bandwidth", integer, [pointer])

    def _bind(self, name, result, args):
        function = getattr(self.lib, name)
        function.restype, function.argtypes = result, args

    def check_packet(self, packet):
        if not isinstance(packet, bytes) or not 1 <= len(packet) <= MAX_PACKET_BYTES:
            raise ValueError("invalid Opus packet length/type")
        if (self.lib.opus_packet_get_nb_samples(packet, len(packet), RATE) != FRAME or
                self.lib.opus_packet_get_nb_channels(packet) != 1 or
                self.lib.opus_packet_get_bandwidth(packet) != NB):
            raise ValueError("packet is not mono NB / 20 ms")
        flag = self.lib.opus_packet_has_lbrr(packet, len(packet))
        if flag not in (0, 1):
            raise ValueError("invalid LBRR packet")
        return bool(flag)

    def encode(self, pcm, fec, bitrate=8000):
        if type(bitrate) is not int or bitrate not in (8000, 24000):
            raise ValueError("only 8000 and test-only 24000 bit/s are supported")
        if type(fec) is not bool or len(pcm) % 2 or not RATE*2 <= len(pcm) <= RATE*120:
            raise ValueError("need bounded 1..60 second PCM16 and boolean FEC")
        error = C.c_int()
        state = self.lib.opus_encoder_create(RATE, 1, 2048, C.byref(error))
        if not state or error.value:
            raise ValueError(f"encoder create: {error.value}")
        try:
            # CTL fixed arguments are typed explicitly for the Apple ARM64 vararg ABI.
            settings = {4002: bitrate, 4004: NB, 4008: NB, 4006: 1, 4020: 1,
                        4010: 10, 4012: int(fec), 4014: 30 if fec else 0,
                        4016: 0, 4024: 3001}
            for request, value in settings.items():
                code = self.lib.opus_encoder_ctl(state, request, C.c_int(value))
                if code:
                    raise ValueError(f"encoder CTL {request}: {code}")
            lookahead = C.c_int()
            if self.lib.opus_encoder_ctl(state, 4027, C.byref(lookahead)):
                raise ValueError("cannot obtain encoder lookahead")
            if not 0 <= lookahead.value <= RATE:
                raise ValueError("unexpected codec delay")
            samples = len(pcm)//2
            count = (samples + lookahead.value + FRAME - 1)//FRAME
            if count > MAX_PACKETS:
                raise ValueError("packet count exceeds bound")
            padded = pcm + bytes(count*FRAME*2-len(pcm))
            packets, lbrr = [], []
            for offset in range(0, len(padded), FRAME*2):
                frame = (C.c_int16 * FRAME).from_buffer_copy(padded[offset:offset+FRAME*2])
                output = (C.c_ubyte * MAX_PACKET_BYTES)()
                size = self.lib.opus_encode(state, frame, FRAME, output, MAX_PACKET_BYTES)
                if not 1 <= size <= MAX_PACKET_BYTES:
                    raise ValueError(f"encode failed: {size}")
                packet = bytes(output[:size])
                lbrr.append(self.check_packet(packet))
                packets.append(packet)
            return packets, {"lookahead_samples": lookahead.value, "input_samples": samples,
                             "lbrr": lbrr, "packet_lengths": [len(p) for p in packets]}
        finally:
            self.lib.opus_encoder_destroy(state)

    def decode_received(self, received, use_fec):
        if type(use_fec) is not bool or not 1 <= len(received) <= MAX_PACKETS:
            raise ValueError("invalid decoder configuration/bound")
        lbrr = [False if p is None else self.check_packet(p) for p in received]
        error = C.c_int()
        state = self.lib.opus_decoder_create(RATE, 1, C.byref(error))
        if not state or error.value:
            raise ValueError(f"decoder create: {error.value}")
        try:
            pcm, actions = bytearray(), []
            for i, packet in enumerate(received):
                action, fec = "normal", 0
                if packet is None:
                    action = "plc"
                    if use_fec and i+1 < len(received) and received[i+1] is not None and lbrr[i+1]:
                        packet, fec, action = received[i+1], 1, "fec"
                output = (C.c_int16 * FRAME)()
                size = self.lib.opus_decode(state, packet, len(packet) if packet is not None else 0,
                                            output, FRAME, fec)
                if size != FRAME:
                    raise ValueError(f"decode failed or changed duration: {size}")
                pcm.extend(bytes(output))
                actions.append(action)
            return bytes(pcm), actions
        finally:
            self.lib.opus_decoder_destroy(state)


def loss_mask(count, condition, seed, sample_id):
    if type(count) is not int or not 1 <= count <= MAX_PACKETS or type(seed) is not int or not 0 <= seed < 2**64:
        raise ValueError("invalid mask bound/seed")
    if condition not in ("clean", "loss05", "loss20", "loss30", "burst240"):
        raise ValueError("unknown loss condition")
    prefix = f"opus-erasure-v1:{seed}:{sample_id}:".encode("ascii")
    if condition == "burst240":
        length = min(12, count)
        start = int.from_bytes(hashlib.sha256(prefix+b"burst").digest()[:8], "big") % (count-length+1)
        return [start <= i < start+length for i in range(count)]
    percent = {"clean": 0, "loss05": 5, "loss20": 20, "loss30": 30}[condition]
    return [int.from_bytes(hashlib.sha256(prefix+str(i).encode()).digest()[:8], "big") < percent*2**64//100
            for i in range(count)]


def erase(packets, mask):
    if len(packets) != len(mask) or not mask or any(type(x) is not bool for x in mask):
        raise ValueError("mask/packet length or type mismatch")
    return [None if lost else packet for packet, lost in zip(packets, mask)]


def align(decoded, meta):
    start = meta["lookahead_samples"] * 2
    end = start + meta["input_samples"] * 2
    if start < 0 or end <= start or len(decoded) < end:
        raise ValueError("decode too short for alignment")
    return decoded[start:end]
