package handlers

import "testing"

func TestPickPairIsDeterministicAndDistinct(t *testing.T) {
	a1, b1 := pickPair("article-user", 7)
	a2, b2 := pickPair("article-user", 7)
	if a1 != a2 || b1 != b2 {
		t.Fatalf("pair changed between calls: (%d,%d) and (%d,%d)", a1, b1, a2, b2)
	}
	if a1 == b1 || a1 < 0 || b1 < 0 || a1 >= 7 || b1 >= 7 {
		t.Fatalf("invalid pair: (%d,%d)", a1, b1)
	}
}
