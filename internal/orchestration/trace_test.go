package orchestration

import "testing"

func TestTraceParseNormalizesValid(t *testing.T) {
	raw := []byte(`{"run_id":"r1","event_id":"a","seq":1,"tool":"read","time":"2026-01-01T00:00:00Z","actor":"w1","paths":["a.go"]}
{"run_id":"r1","event_id":"b","seq":2,"tool":"edit","time":"2026-01-01T00:00:01Z","actor":"w1","paths":["b.go"]}`)
	events, err := ParseTrace(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d", len(events))
	}
	if TraceDigest(events) == "" {
		t.Fatal("empty digest")
	}
}

func TestTraceRejectsForbiddenAndBadOrder(t *testing.T) {
	cases := map[string][]byte{
		"TRACE_FORBIDDEN_FIELD":       []byte(`{"run_id":"r1","event_id":"a","seq":1,"tool":"read","time":"t","actor":"w","reasoning":"because"}`),
		"TRACE_SEQUENCE_NONMONOTONIC": []byte("{\"run_id\":\"r1\",\"event_id\":\"a\",\"seq\":2,\"tool\":\"read\",\"time\":\"t\",\"actor\":\"w\"}\n{\"run_id\":\"r1\",\"event_id\":\"b\",\"seq\":2,\"tool\":\"edit\",\"time\":\"t\",\"actor\":\"w\"}"),
		"TRACE_DUPLICATE_EVENT":       []byte("{\"run_id\":\"r1\",\"event_id\":\"a\",\"seq\":1,\"tool\":\"read\",\"time\":\"t\",\"actor\":\"w\"}\n{\"run_id\":\"r1\",\"event_id\":\"a\",\"seq\":2,\"tool\":\"edit\",\"time\":\"t\",\"actor\":\"w\"}"),
		"TRACE_RUN_MIXED":             []byte("{\"run_id\":\"r1\",\"event_id\":\"a\",\"seq\":1,\"tool\":\"read\",\"time\":\"t\",\"actor\":\"w\"}\n{\"run_id\":\"r2\",\"event_id\":\"b\",\"seq\":2,\"tool\":\"edit\",\"time\":\"t\",\"actor\":\"w\"}"),
		"TRACE_REQUIRED_FIELD":        []byte(`{"run_id":"r1","event_id":"a","seq":1,"time":"t","actor":"w"}`),
		"TRACE_UNKNOWN_FIELD":         []byte(`{"run_id":"r1","event_id":"a","seq":1,"tool":"read","time":"t","actor":"w","raw_result":"secret"}`),
		"TRACE_SEQUENCE_INVALID":      []byte(`{"run_id":"r1","event_id":"a","seq":0,"tool":"read","time":"t","actor":"w"}`),
	}
	for wantCode, raw := range cases {
		_, err := ParseTrace(raw)
		if err == nil {
			t.Fatalf("%s: accepted", wantCode)
		}
		if got := err.Error(); len(got) < len(wantCode) || got[:len(wantCode)] != wantCode {
			t.Fatalf("want %s, got %v", wantCode, err)
		}
	}
}
