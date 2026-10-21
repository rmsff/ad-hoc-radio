"""Separate 6k encoder adapter: preserve the historical pinned Opus source."""
import ctypes as C

from opus_codec import FRAME, MAX_PACKET_BYTES, MAX_PACKETS, NB, RATE, Opus


class ComparisonOpus(Opus):
    def encode(self, pcm, fec, bitrate=8000):
        if bitrate != 6000:
            return super().encode(pcm, fec, bitrate)
        if type(bitrate) is not int or fec is not False or len(pcm)%2 or not RATE*2 <= len(pcm) <= RATE*120:
            raise ValueError("6k comparison requires bounded PCM and FEC off")
        error=C.c_int()
        state=self.lib.opus_encoder_create(RATE,1,2048,C.byref(error))
        if not state or error.value:
            raise ValueError(f"encoder create: {error.value}")
        try:
            settings={4002:6000,4004:NB,4008:NB,4006:1,4020:1,4010:10,
                      4012:0,4014:0,4016:0,4024:3001}
            for request,value in settings.items():
                code=self.lib.opus_encoder_ctl(state,request,C.c_int(value))
                if code:raise ValueError(f"encoder CTL {request}: {code}")
            lookahead=C.c_int()
            if self.lib.opus_encoder_ctl(state,4027,C.byref(lookahead)) or lookahead.value!=52:
                raise ValueError("unexpected 6k encoder lookahead")
            samples=len(pcm)//2
            count=(samples+lookahead.value+FRAME-1)//FRAME
            if count>MAX_PACKETS:raise ValueError("packet count exceeds bound")
            padded=pcm+bytes(count*FRAME*2-len(pcm))
            packets,lbrr=[],[]
            for offset in range(0,len(padded),FRAME*2):
                frame=(C.c_int16*FRAME).from_buffer_copy(padded[offset:offset+FRAME*2])
                output=(C.c_ubyte*MAX_PACKET_BYTES)()
                size=self.lib.opus_encode(state,frame,FRAME,output,MAX_PACKET_BYTES)
                if not 1<=size<=MAX_PACKET_BYTES:raise ValueError(f"encode failed: {size}")
                packet=bytes(output[:size]);lbrr.append(self.check_packet(packet));packets.append(packet)
            return packets,{"lookahead_samples":lookahead.value,"input_samples":samples,
                            "lbrr":lbrr,"packet_lengths":[len(p) for p in packets]}
        finally:
            self.lib.opus_encoder_destroy(state)
