import 'package:flutter/material.dart';
import '../models/article.dart';
import '../services/api_service.dart';

class NewsProvider extends ChangeNotifier {
  final List<Article> _articles = [];
  bool _isLoading = false;
  bool _hasMore = true;
  int _currentPage = 1;
  String? _selectedCategory;
  String? _error;

  List<Article> get articles => List.unmodifiable(_articles);
  bool get isLoading => _isLoading;
  bool get hasMore => _hasMore;
  String? get selectedCategory => _selectedCategory;
  String? get error => _error;

  Future<void> loadArticles({bool refresh = false}) async {
    if (_isLoading) return;
    if (!refresh && !_hasMore) return;

    if (refresh) {
      _currentPage = 1;
      _articles.clear();
      _hasMore = true;
    }

    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final data = await ApiService.getFeed(
        page: _currentPage,
        category: _selectedCategory,
      );
      final feed = ArticleFeed.fromJson(data);
      final startIndex = _articles.length;
      _articles.addAll(feed.articles);
      _hasMore = feed.hasMore;
      _currentPage++;
      _isLoading = false;
      notifyListeners();

      // Fetch LLM versions for each new article concurrently — fire and forget
      _fetchComparisons(startIndex, feed.articles.length);
    } on ApiException catch (e) {
      _error = e.message;
      _isLoading = false;
      notifyListeners();
    } catch (e) {
      _error = 'Failed to load articles';
      _isLoading = false;
      notifyListeners();
    }
  }

  Future<void> _fetchComparisons(int startIndex, int count) async {
    final futures = <Future<void>>[];
    for (int i = startIndex; i < startIndex + count; i++) {
      futures.add(_fetchComparison(i));
    }
    await Future.wait(futures, eagerError: false);
  }

  Future<void> _fetchComparison(int index) async {
    try {
      final article = _articles[index];
      final data = await ApiService.getComparison(article.id);
      if (data == null) return;

      final versionA = data['version_a'] as Map<String, dynamic>?;
      final versionB = data['version_b'] as Map<String, dynamic>?;
      if (versionA == null || versionB == null) return;

      // Original is always the first tab so readers can compare honestly
      final original = article.versions.isNotEmpty ? article.versions.first : null;
      final originalVersion = original != null
          ? ArticleVersion(
              modelName: 'Original',
              title: original.title,
              summary: original.summary,
              content: original.content,
              isOriginal: true,
            )
          : null;

      final versions = [
        if (originalVersion != null) originalVersion,
        ArticleVersion(
          modelName: versionA['model_name'] as String? ?? 'Model A',
          title: versionA['title'] as String,
          summary: versionA['summary'] as String,
          content: versionA['content'] as String?,
          rewriteId: versionA['id'] as String?,
        ),
        ArticleVersion(
          modelName: versionB['model_name'] as String? ?? 'Model B',
          title: versionB['title'] as String,
          summary: versionB['summary'] as String,
          content: versionB['content'] as String?,
          rewriteId: versionB['id'] as String?,
        ),
      ];

      if (index < _articles.length) {
        _articles[index] = _articles[index].withVersions(versions);
        notifyListeners();
      }
    } catch (_) {
      // Silently ignore — article still shows with original single version
    }
  }

  void setCategory(String? category) {
    if (_selectedCategory == category) return;
    _selectedCategory = category;
    loadArticles(refresh: true);
  }
}
