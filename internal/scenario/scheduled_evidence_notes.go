package scenario

func scheduledEvidenceNotes(result ScheduledRuntimeResult) string {
	note := "scheduled audio evidence v2: latency_ms is creation-to-playout; null audio summaries mean no samples; expected membership is the union over the PTT and each expected member owes the full call; no concealment/audio quality is measured"
	if result.ControlCounters == nil {
		note += "; control transport unmodeled (legacy skeleton)"
	} else {
		note += "; directed peer-local control with conservative serialized control medium (not an RF MAC)"
	}
	if result.Wire == nil {
		note += "; voice byte budget unmodeled (legacy skeleton)"
	} else {
		note += "; real short-wire encoding of synthetic size fixtures, assumed backend timings, not RF or codec acceptance"
	}
	return note
}
