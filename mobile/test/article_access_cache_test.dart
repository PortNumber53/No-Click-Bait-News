import 'package:flutter_test/flutter_test.dart';
import 'package:no_click_bait_news/services/article_access_cache.dart';
import 'package:shared_preferences/shared_preferences.dart';

Map<String, dynamic> articleJson({required DateTime expiresAt}) => {
      'id': 'article-1',
      'title': 'Cached title',
      'summary': 'Cached summary',
      'content': 'Cached full story',
      'source_name': 'Example',
      'source_url': 'https://example.com/story',
      'categories': ['Technology'],
      'published_at': '2026-09-05T12:00:00Z',
      'is_premium': false,
      'view_count': 1,
      'access_expires_at': expiresAt.toUtc().toIso8601String(),
    };

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues({});
    ArticleAccessCache.resetForTesting();
  });

  test('granted article is immediately available and survives memory reset',
      () async {
    await ArticleAccessCache.put(
      'user-1',
      articleJson(
          expiresAt: DateTime.now().toUtc().add(const Duration(days: 7))),
    );

    expect(ArticleAccessCache.peek('user-1', 'article-1')?.content,
        'Cached full story');
    expect(ArticleAccessCache.peek('user-2', 'article-1'), isNull);

    ArticleAccessCache.resetForTesting();
    final restored = await ArticleAccessCache.get('user-1', 'article-1');
    expect(restored?.title, 'Cached title');
  });

  test('expired access is not cached', () async {
    await ArticleAccessCache.put(
      'user-1',
      articleJson(
          expiresAt:
              DateTime.now().toUtc().subtract(const Duration(seconds: 1))),
    );

    expect(await ArticleAccessCache.get('user-1', 'article-1'), isNull);
  });
}
