package handlers

import "testing"

func TestCanonicalReadCategory(t *testing.T) {
	if got := canonicalReadCategory("  Technology "); got != "technology" {
		t.Fatalf("canonicalReadCategory() = %q, want technology", got)
	}
	if got := canonicalReadCategory(" "); got != "general" {
		t.Fatalf("canonicalReadCategory() empty = %q, want general", got)
	}
}

func TestArticleReadCategory(t *testing.T) {
	primary := " Science "
	if got := articleReadCategory(&primary, []string{"Technology"}); got != "Science" {
		t.Fatalf("articleReadCategory() = %q, want Science", got)
	}
	if got := articleReadCategory(nil, []string{"", "Health"}); got != "Health" {
		t.Fatalf("articleReadCategory() fallback = %q, want Health", got)
	}
	if got := articleReadCategory(nil, nil); got != "General" {
		t.Fatalf("articleReadCategory() default = %q, want General", got)
	}
}
