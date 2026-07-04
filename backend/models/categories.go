package models

import "strings"

var AllowedArticleCategories = []string{
	"Technology",
	"Science",
	"Business",
	"Health",
	"Sports",
	"World",
}

var allowedArticleCategoryByLower = func() map[string]string {
	allowed := make(map[string]string, len(AllowedArticleCategories))
	for _, category := range AllowedArticleCategories {
		allowed[strings.ToLower(category)] = category
	}
	return allowed
}()

func NormalizeArticleCategories(categories []string) []string {
	normalized := make([]string, 0, 3)
	seen := make(map[string]bool)

	for _, category := range categories {
		canonical, ok := allowedArticleCategoryByLower[strings.ToLower(strings.TrimSpace(category))]
		if !ok || seen[canonical] {
			continue
		}
		normalized = append(normalized, canonical)
		seen[canonical] = true
		if len(normalized) == 3 {
			break
		}
	}

	return normalized
}

func PrimaryArticleCategory(categories []string, fallback *string) *string {
	normalized := NormalizeArticleCategories(categories)
	if len(normalized) > 0 {
		return &normalized[0]
	}
	if fallback == nil {
		return nil
	}
	value := strings.TrimSpace(*fallback)
	if value == "" {
		return nil
	}
	return &value
}
