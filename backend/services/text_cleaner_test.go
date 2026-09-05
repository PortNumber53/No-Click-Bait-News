package services

import "testing"

func TestSummarizeTextStripsFeedHTML(t *testing.T) {
	got := summarizeText(`<p>Lead <a href="https://example.com">story</a>.</p><ul><li>First &amp; second</li></ul>`, 500)
	want := "Lead story. First & second"
	if got != want {
		t.Fatalf("summarizeText() = %q, want %q", got, want)
	}
}
