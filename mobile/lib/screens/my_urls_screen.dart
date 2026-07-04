import 'package:flutter/material.dart';

import '../models/article.dart';
import '../services/api_service.dart';
import '../widgets/article_card.dart';
import '../widgets/shimmer_card.dart';
import 'article_detail_screen.dart';

class MyUrlsScreen extends StatefulWidget {
  const MyUrlsScreen({super.key});

  @override
  State<MyUrlsScreen> createState() => _MyUrlsScreenState();
}

class _MyUrlsScreenState extends State<MyUrlsScreen> {
  final _scrollController = ScrollController();
  final List<Article> _articles = [];
  bool _isLoading = false;
  bool _hasMore = true;
  String? _error;
  int _page = 1;

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
    _load(refresh: true);
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 300) {
      _load();
    }
  }

  Future<void> _load({bool refresh = false}) async {
    if (_isLoading) return;
    if (!refresh && !_hasMore) return;

    if (refresh) {
      _page = 1;
      _hasMore = true;
      _articles.clear();
    }

    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final data = await ApiService.getMyArticles(page: _page);
      final feed = ArticleFeed.fromJson(data);
      if (!mounted) return;
      setState(() {
        _articles.addAll(feed.articles);
        _hasMore = feed.hasMore;
        _page++;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _error = e.message);
    } catch (_) {
      if (!mounted) return;
      setState(() => _error = 'Failed to load submitted URLs');
    } finally {
      if (mounted) setState(() => _isLoading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: const Text('My URLs')),
      body: Builder(
        builder: (context) {
          if (_articles.isEmpty && _isLoading) {
            return ListView.builder(
              padding: const EdgeInsets.all(16),
              itemCount: 5,
              itemBuilder: (_, __) => const ShimmerCard(),
            );
          }

          if (_articles.isEmpty && _error != null) {
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.error_outline,
                      size: 48, color: theme.colorScheme.error),
                  const SizedBox(height: 16),
                  Text(_error!),
                  const SizedBox(height: 16),
                  FilledButton.tonal(
                    onPressed: () => _load(refresh: true),
                    child: const Text('Retry'),
                  ),
                ],
              ),
            );
          }

          if (_articles.isEmpty) {
            return const Center(child: Text('No submitted URLs found'));
          }

          return RefreshIndicator(
            onRefresh: () => _load(refresh: true),
            child: ListView.builder(
              controller: _scrollController,
              padding: const EdgeInsets.all(16),
              itemCount: _articles.length + (_hasMore ? 1 : 0),
              itemBuilder: (context, index) {
                if (index == _articles.length) {
                  return const Padding(
                    padding: EdgeInsets.all(24),
                    child: Center(child: CircularProgressIndicator()),
                  );
                }
                final article = _articles[index];
                return ArticleCard(
                  article: article,
                  onTap: () {
                    Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (_) => ArticleDetailScreen(article: article),
                      ),
                    );
                  },
                );
              },
            ),
          );
        },
      ),
    );
  }
}
