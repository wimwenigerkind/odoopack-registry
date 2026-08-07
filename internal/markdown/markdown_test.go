package markdown

import "testing"

func TestRenderSanitizes(t *testing.T) {
	in := "# Title\n\n<script>alert(1)</script>\n\n[click](javascript:alert(2))\n\n<img src=x onerror=alert(3)>\n\n**bold** and `code`"
	out, err := Render(in)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("OUT: %q", out)
	for _, bad := range []string{"<script", "javascript:", "onerror"} {
		if contains(out, bad) {
			t.Errorf("unsanitized: %q present in output", bad)
		}
	}
	for _, good := range []string{"<h1", "<strong>bold</strong>", "<code>code</code>"} {
		if !contains(out, good) {
			t.Errorf("expected %q in output", good)
		}
	}
}

func TestRenderEmpty(t *testing.T) {
	out, err := Render("")
	if err != nil || out != "" {
		t.Fatalf("want empty, got %q err %v", out, err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
