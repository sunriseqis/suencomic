package downloader

import "testing"

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		`one/piece`:   "one_piece",
		`a:b*c?"d<e>`: "a_b_c__d_e_",
		"  spaces  ":  "spaces",
		`back\slash`:  "back_slash",
		"鬼滅之刃 第1话":    "鬼滅之刃 第1话",
	}
	for in, want := range cases {
		if got := sanitizeFilename(in); got != want {
			t.Fatalf("sanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTaskCloneIsDeepCopy(t *testing.T) {
	src := &Task{ID: "t1", Logs: []string{"a", "b"}}
	c := src.clone()
	c.Logs[0] = "mutated"

	if src.Logs[0] != "a" {
		t.Fatalf("clone did not deep-copy Logs: original mutated to %q", src.Logs[0])
	}
	if c.ctx != nil || c.cancelFunc != nil {
		t.Fatal("clone must not carry live ctx/cancelFunc")
	}
}
