package sim

import "testing"

// TestCanonicalJSON_ExactBytes pins the exact §17 canonical JSON of an
// event. The expected strings are the bytes produced by the original
// map-based encoder, so any change to them changes every event_hash.
func TestCanonicalJSON_ExactBytes(t *testing.T) {
	cases := []struct {
		name  string
		event Event
		want  string
	}{
		{
			name: "no optional fields",
			event: Event{
				TimeMs: 100, EventPriority: PrioritySourceTick, EventID: 7, EventType: "source_tick",
				SourceStationID: 1, DestinationStationID: 0,
			},
			want: `{"destination_station_id":0,"event_id":7,"event_priority":20,"event_type":"source_tick",` +
				`"source_station_id":1,"time_ms":100}`,
		},
		{
			name: "all optional fields with zero and false values",
			event: Event{
				EventPriority: PriorityFrameDelivery, EventType: "frame_delivery",
				OriginKey: &OriginKey{}, CopyKey: &CopyKey{}, Delivery: &FrameDeliveryMeta{},
			},
			want: `{"copy_key":{"call_id":0,"hop":0,"net_id":0,"relay_id":0,"seq":0,"source_id":0},` +
				`"corrupted":false,"destination_station_id":0,"event_id":0,"event_priority":30,` +
				`"event_type":"frame_delivery","is_duplicate":false,` +
				`"origin_key":{"call_id":0,"net_id":0,"seq":0,"source_id":0},` +
				`"original_event_id":0,"payload_len":0,"source_station_id":0,"time_ms":0}`,
		},
		{
			name: "all optional fields with distinct values",
			event: Event{
				TimeMs: 1500, EventPriority: PriorityFrameDelivery, EventID: 12, EventType: "frame_delivery",
				SourceStationID: 2, DestinationStationID: 3,
				OriginKey: &OriginKey{NetID: 4, SourceID: 5, CallID: 6, Seq: 7},
				CopyKey:   &CopyKey{NetID: 8, SourceID: 9, RelayID: 10, CallID: 11, Seq: 13, Hop: 14},
				Delivery:  &FrameDeliveryMeta{Corrupted: true, IsDuplicate: true, OriginalEventID: 15, PayloadLen: 16},
			},
			want: `{"copy_key":{"call_id":11,"hop":14,"net_id":8,"relay_id":10,"seq":13,"source_id":9},` +
				`"corrupted":true,"destination_station_id":3,"event_id":12,"event_priority":30,` +
				`"event_type":"frame_delivery","is_duplicate":true,` +
				`"origin_key":{"call_id":6,"net_id":4,"seq":7,"source_id":5},` +
				`"original_event_id":15,"payload_len":16,"source_station_id":2,"time_ms":1500}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := canonicalEventJSON(tc.event)
			if err != nil {
				t.Fatalf("canonicalEventJSON: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("canonical JSON mismatch\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}
