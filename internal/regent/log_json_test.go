package regent

import (
	"testing"
)

const sampleLogJSON = `{
  "session_id": "claude_code:abc-123",
  "steps": [
    {
      "hash": "deadbeefcafebabe1234567890abcdef",
      "timestamp": "2026-05-18T17:21:38+02:00",
      "origin": "claude_code",
      "causes": [
        {"tool": "Read", "args": {"file_path": "/abs/foo.go", "limit": 60}},
        {"tool": "Write", "args": {"file_path": "/abs/bar.go", "content": "..."}}
      ],
      "messages": [
        {"type": "user", "message": {"role": "user", "content": "do the thing"}},
        {"type": "assistant", "message": {"role": "assistant", "content": [
          {"type": "text", "text": "Done."},
          {"type": "tool_use", "id": "x", "name": "Read", "input": {}}
        ]}}
      ]
    },
    {
      "hash": "1234567890abcdef",
      "timestamp": "2026-05-18T17:25:00+02:00",
      "origin": "claude_code",
      "causes": [],
      "messages": [
        {"type": "user", "message": {"role": "user", "content": "another"}}
      ]
    }
  ]
}`

func TestParseLogJSON(t *testing.T) {
	sessionID, steps, err := parseLogJSON([]byte(sampleLogJSON))
	if err != nil {
		t.Fatalf("parseLogJSON: %v", err)
	}
	if sessionID != "claude_code:abc-123" {
		t.Errorf("sessionID = %q, want claude_code:abc-123", sessionID)
	}
	if len(steps) != 2 {
		t.Fatalf("got %d steps, want 2", len(steps))
	}

	s0 := steps[0]
	if s0.ShortHash() != "deadbeef" {
		t.Errorf("steps[0].ShortHash = %q, want deadbeef", s0.ShortHash())
	}
	if s0.HumanPrompt != "do the thing" {
		t.Errorf("steps[0].HumanPrompt = %q, want %q", s0.HumanPrompt, "do the thing")
	}
	if s0.AgentReply != "Done." {
		t.Errorf("steps[0].AgentReply = %q, want %q (text block only, no tool_use)", s0.AgentReply, "Done.")
	}
	if len(s0.Tools) != 2 {
		t.Fatalf("steps[0].Tools = %d, want 2", len(s0.Tools))
	}
	if s0.Tools[0].Name != "Read" || s0.Tools[0].Args["file_path"] != "/abs/foo.go" {
		t.Errorf("steps[0].Tools[0] = %+v, want Read /abs/foo.go", s0.Tools[0])
	}
}

func TestParseLogJSON_StringAssistantContent(t *testing.T) {
	// Some agents (codex, opencode) may emit assistant content as a
	// plain string instead of the Claude content-block array. Both must
	// extract as the visible text.
	in := `{
	  "session_id": "s",
	  "steps": [{
		"hash": "h",
		"timestamp": "2026-05-18T00:00:00Z",
		"messages": [
		  {"type": "assistant", "message": {"role": "assistant", "content": "plain reply"}}
		]
	  }]
	}`
	_, steps, err := parseLogJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].AgentReply != "plain reply" {
		t.Errorf("AgentReply = %q, want %q", steps[0].AgentReply, "plain reply")
	}
}

func TestParseLogJSON_UserContentAsBlocks(t *testing.T) {
	// User content can also be an array of blocks. Concatenate the
	// text-type ones; ignore the rest.
	in := `{
	  "steps": [{
		"hash": "h",
		"messages": [
		  {"type": "user", "message": {"role": "user", "content": [
			{"type": "text", "text": "part 1"},
			{"type": "image", "url": "x"},
			{"type": "text", "text": "part 2"}
		  ]}}
		]
	  }]
	}`
	_, steps, err := parseLogJSON([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].HumanPrompt != "part 1\n\npart 2" {
		t.Errorf("HumanPrompt = %q", steps[0].HumanPrompt)
	}
}

func TestParseLogJSON_NoSteps(t *testing.T) {
	_, steps, err := parseLogJSON([]byte(`{"session_id": "s", "steps": []}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 0 {
		t.Errorf("len(steps) = %d, want 0", len(steps))
	}
}

func TestParseLogJSON_MalformedReturnsError(t *testing.T) {
	if _, _, err := parseLogJSON([]byte("{not json")); err == nil {
		t.Error("parseLogJSON did not error on malformed JSON")
	}
}

func TestShortHash(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"deadbeefcafebabe", "deadbeef"},
		{"short", "short"},
		{"", ""},
		{"12345678", "12345678"},
	}
	for _, tt := range cases {
		s := Step{Hash: tt.in}
		if got := s.ShortHash(); got != tt.want {
			t.Errorf("Step{Hash:%q}.ShortHash() = %q, want %q", tt.in, got, tt.want)
		}
	}
}
