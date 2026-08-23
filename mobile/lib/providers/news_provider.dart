import 'dart:async';

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
  int _generation = 0;

  List<Article> get articles => List.unmodifiable(_articles);
  bool get isLoading => _isLoading;
  bool get hasMore => _hasMore;
  String? get selectedCategory => _selectedCategory;
  String? get error => _error;

  Future<void> loadArticles({bool refresh = false}) async {
    if (_isLoading && !refresh) return;
    if (!refresh && !_hasMore) return;
    final generation = refresh ? ++_generation : _generation;

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
      if (generation != _generation) return;
      final startIndex = _articles.length;
      _articles.addAll(feed.articles);
      _hasMore = feed.hasMore;
      _currentPage++;
      notifyListeners();

      final articleIds = _articles
          .skip(startIndex)
          .take(feed.articles.length)
          .map((article) => article.id)
          .toList();
      unawaited(_fetchComparisons(articleIds, generation));
    } on ApiException catch (e) {
      if (generation != _generation) return;
      _error = e.message;
    } catch (e) {
      if (generation != _generation) return;
      _error = 'Failed to load articles';
    } finally {
      if (generation == _generation) {
        _isLoading = false;
        notifyListeners();
      }
    }
  }

  Future<void> _fetchComparisons(
      List<String> articleIds, int generation) async {
    final futures = articleIds
        .map((articleId) => _fetchComparison(articleId, generation))
        .toList();
    await Future.wait(futures, eagerError: false);
  }

  Future<void> _fetchComparison(String articleId, int generation) async {
    try {
      final initialIndex =
          _articles.indexWhere((article) => article.id == articleId);
      if (initialIndex < 0) return;
      final article = _articles[initialIndex];
      final data = await ApiService.getComparison(articleId);
      if (data == null) return;
      if (generation != _generation) return;

      final versionA = data['version_a'] as Map<String, dynamic>?;
      final versionB = data['version_b'] as Map<String, dynamic>?;
      if (versionA == null || versionB == null) return;

      // Original is always the first tab so readers can compare honestly
      final original =
          article.versions.isNotEmpty ? article.versions.first : null;
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

      final currentIndex =
          _articles.indexWhere((candidate) => candidate.id == articleId);
      if (currentIndex >= 0) {
        _articles[currentIndex] =
            _articles[currentIndex].withVersions(versions);
        notifyListeners();
      }
    } catch (_) {
      // Silently ignore — article still shows with original single version
    }
  }

  void setCategory(String? category) {
    if (_selectedCategory == category) return;
    _selectedCategory = category;
    unawaited(loadArticles(refresh: true));
  }
}
