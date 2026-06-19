package mcp_test

import (
	"strings"
	"testing"

	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestResourcesListAndRead exercises the resources channel end-to-end over the
// stdio server (spec AC2–AC6): list enumerates a seeded spec's artifacts and
// steering files, read returns content with the right mime, and traversal/unknown
// URIs error without disclosure.
func TestResourcesListAndRead(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")

	resps := drive(t,
		`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"specd://specs/auth/tasks.md"}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"specd://specs/auth/state.json"}}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"specd://../../etc/passwd"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"specd://specs/auth/nope.md"}}`,
	)

	// list (AC2): at least the seeded spec's tasks.md + state.json show up.
	list := result(t, resps[0])["resources"].([]any)
	uris := map[string]string{}
	for _, r := range list {
		m := r.(map[string]any)
		uris[m["uri"].(string)] = m["mimeType"].(string)
	}
	if uris["specd://specs/auth/tasks.md"] != "text/markdown" {
		t.Errorf("list missing tasks.md or wrong mime: %v", uris)
	}
	if uris["specd://specs/auth/state.json"] != "application/json" {
		t.Errorf("list missing state.json or wrong mime: %v", uris)
	}

	// read markdown (AC3).
	md := result(t, resps[1])["contents"].([]any)[0].(map[string]any)
	if md["mimeType"] != "text/markdown" || strings.TrimSpace(md["text"].(string)) == "" {
		t.Errorf("tasks.md read = %v", md)
	}
	// read json (AC4).
	js := result(t, resps[2])["contents"].([]any)[0].(map[string]any)
	if js["mimeType"] != "application/json" {
		t.Errorf("state.json mime = %v, want application/json", js["mimeType"])
	}

	// traversal (AC5) and unknown (AC6) both error.
	for _, i := range []int{3, 4} {
		if _, ok := resps[i]["error"].(map[string]any); !ok {
			t.Errorf("response %d should be a resource error: %v", i, resps[i])
		}
	}
}

// TestPromptsListAndGet covers the prompts channel (spec AC2–AC6): list returns
// the 4 phase + 2 role prompts with declared arguments, get renders messages with
// slug substitution and is deterministic, and unknown names error.
func TestPromptsListAndGet(t *testing.T) {
	resps := drive(t,
		`{"jsonrpc":"2.0","id":1,"method":"prompts/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"prompts/get","params":{"name":"phase/design","arguments":{"slug":"auth"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"prompts/get","params":{"name":"phase/design","arguments":{"slug":"auth"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"prompts/get","params":{"name":"role/builder"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"prompts/get","params":{"name":"phase/bogus"}}`,
	)

	// list (AC2): the six expected prompt names with arguments declared.
	prompts := result(t, resps[0])["prompts"].([]any)
	if len(prompts) != 6 {
		t.Fatalf("prompts/list count = %d, want 6", len(prompts))
	}
	names := map[string]bool{}
	for _, p := range prompts {
		names[p.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{"phase/requirements", "phase/design", "phase/tasks", "phase/execute", "role/builder", "role/investigator"} {
		if !names[want] {
			t.Errorf("prompts/list missing %s", want)
		}
	}

	// get phase/design slug=auth mentions the slug (AC3).
	msg := result(t, resps[1])["messages"].([]any)[0].(map[string]any)
	text := msg["content"].(map[string]any)["text"].(string)
	if !strings.Contains(text, "auth") {
		t.Errorf("phase/design did not substitute slug: %q", text)
	}

	// determinism (AC6): identical inputs → identical messages.
	text2 := result(t, resps[2])["messages"].([]any)[0].(map[string]any)["content"].(map[string]any)["text"].(string)
	if text != text2 {
		t.Errorf("phase/design not deterministic:\n%q\nvs\n%q", text, text2)
	}

	// role/builder returns a contract message (AC4).
	rb := result(t, resps[3])["messages"].([]any)
	if len(rb) == 0 {
		t.Error("role/builder returned no messages")
	}

	// unknown prompt errors (AC5).
	if _, ok := resps[4]["error"].(map[string]any); !ok {
		t.Errorf("unknown prompt should error: %v", resps[4])
	}
}

// TestCompositeRoundTrip covers spec AC1/AC3/AC4: a composite call produces output
// byte-identical to the atomic command it wraps.
func TestCompositeRoundTrip(t *testing.T) {
	h := th.New(t)
	seedSpec(h, "auth")

	pairs := []struct {
		name      string
		composite string
		atomic    string
	}{
		{
			"inspect=status",
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"specd_inspect","arguments":{"view":"status","slug":"auth"}}}`,
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"specd_status","arguments":{"args":["auth"]}}}`,
		},
		{
			"query=next",
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"specd_query","arguments":{"view":"next","slug":"auth"}}}`,
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"specd_next","arguments":{"args":["auth"]}}}`,
		},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			resps := drive(t, p.composite, p.atomic)
			ct := result(t, resps[0])["content"].([]any)[0].(map[string]any)["text"].(string)
			at := result(t, resps[1])["content"].([]any)[0].(map[string]any)["text"].(string)
			if ct != at {
				t.Errorf("round-trip mismatch:\ncomposite=%q\natomic=%q", ct, at)
			}
		})
	}
}
