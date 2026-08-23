import 'package:flutter_test/flutter_test.dart';
import 'package:no_click_bait_news/models/article.dart';

void main() {
  test('article parser preserves original and rewrite versions', () {
    final article = Article.fromJson({
      'id': 'article-1',
      'title': 'Original title',
      'summary': 'Original summary',
      'content': 'Primary rewrite',
      'original_content': 'Original body',
      'rewrite_status': 'complete',
      'source_name': 'Example',
      'source_url': 'https://example.com/story',
      'categories': ['Technology', 'Business'],
      'published_at': '2026-08-22T12:00:00Z',
      'is_premium': false,
      'view_count': 1,
      'rewrites': [
        {
          'id': 'rewrite-1',
          'model_name': 'Model A',
          'title': 'Direct title',
          'summary': 'Direct summary',
          'content': 'Direct content',
        },
      ],
    });

    expect(article.categories, ['Technology', 'Business']);
    expect(article.versions, hasLength(2));
    expect(article.versions.first.isOriginal, isTrue);
    expect(article.versions.last.rewriteId, 'rewrite-1');
  });
}
