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

func TestArticleReadLimitMessage(t *testing.T) {
	monthly := readingEntitlement{MonthlyReadLimit: 60}
	if got := articleReadLimitMessage(monthly, "Technology"); got != "Your plan includes 60 articles per month. Upgrade for unlimited reading." {
		t.Fatalf("monthly limit message = %q", got)
	}

	free := readingEntitlement{}
	if got := articleReadLimitMessage(free, "Technology"); got != "Your free plan includes one Technology article per day. Upgrade for more reading." {
		t.Fatalf("free limit message = %q", got)
	}
}

func TestArticleAccessRetention(t *testing.T) {
	if articleAccessRetentionDays != 7 {
		t.Fatalf("article access retention = %d days, want 7", articleAccessRetentionDays)
	}
}
