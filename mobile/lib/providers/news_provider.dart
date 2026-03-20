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
      _articles.addAll(feed.articles);
      _hasMore = feed.hasMore;
      _currentPage++;
    } on ApiException catch (e) {
      _error = e.message;
    } catch (e) {
      _error = 'Failed to load articles';
    }

    _isLoading = false;
    notifyListeners();
  }

  void setCategory(String? category) {
    if (_selectedCategory == category) return;
    _selectedCategory = category;
    loadArticles(refresh: true);
  }
}
