package services

import "time"

// ClampPublishedAt prevents bad publisher timestamps from sorting articles into
// the future. Small clock differences are also safer to present as newly
// published than as a negative age.
func ClampPublishedAt(publishedAt time.Time) time.Time {
	return clampPublishedAt(publishedAt, time.Now().UTC())
}

func clampPublishedAt(publishedAt, now time.Time) time.Time {
	publishedAt = publishedAt.UTC()
	now = now.UTC()
	if publishedAt.After(now) {
		return now
	}
	return publishedAt
}
