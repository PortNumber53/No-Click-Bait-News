import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../models/article.dart';
import '../providers/auth_provider.dart';
import '../providers/news_provider.dart';
import '../services/api_service.dart';
import '../widgets/article_version_card.dart';
import '../widgets/shimmer_card.dart';
import 'article_detail_screen.dart';
import 'my_urls_screen.dart';
import 'subscription_screen.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  final _scrollController = ScrollController();
  final _urlController = TextEditingController();
  bool _isFetchingUrl = false;

  static const _categories = [
    null,
    'Technology',
    'Science',
    'Business',
    'Health',
    'Sports',
    'World',
  ];

  static const _categoryLabels = [
    'All',
    'Technology',
    'Science',
    'Business',
    'Health',
    'Sports',
    'World',
  ];

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      context.read<NewsProvider>().loadArticles(refresh: true);
    });
  }

  @override
  void dispose() {
    _scrollController.dispose();
    _urlController.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 400) {
      context.read<NewsProvider>().loadArticles();
    }
  }

  Future<void> _refresh() async {
    await context.read<NewsProvider>().loadArticles(refresh: true);
  }

  Future<void> _fetchUrl() async {
    final url = _urlController.text.trim();
    if (url.isEmpty || _isFetchingUrl) return;

    setState(() => _isFetchingUrl = true);
    try {
      final data = await ApiService.fetchArticleUrl(url);
      final article = Article.fromJson(data);
      if (!mounted) return;
      _urlController.clear();
      Navigator.push(
        context,
        MaterialPageRoute(builder: (_) => ArticleDetailScreen(article: article)),
      );
    } on ApiException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.message)),
      );
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Failed to fetch URL')),
      );
    } finally {
      if (mounted) setState(() => _isFetchingUrl = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.newspaper_rounded,
                color: theme.colorScheme.primary, size: 22),
            const SizedBox(width: 8),
            const Text(
              'No-Click Bait News',
              style: TextStyle(fontWeight: FontWeight.bold),
            ),
          ],
        ),
        actions: [
          IconButton(
            icon: const Icon(Icons.link),
            tooltip: 'My URLs',
            onPressed: () {
              Navigator.push(
                context,
                MaterialPageRoute(builder: (_) => const MyUrlsScreen()),
              );
            },
          ),
          IconButton(
            icon: const Icon(Icons.workspace_premium_outlined),
            tooltip: 'Subscriptions',
            onPressed: () {
              Navigator.push(
                context,
                MaterialPageRoute(builder: (_) => const SubscriptionScreen()),
              );
            },
          ),
          IconButton(
            icon: const Icon(Icons.logout_rounded),
            tooltip: 'Sign Out',
            onPressed: () => context.read<AuthProvider>().logout(),
          ),
        ],
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _urlController,
                    enabled: !_isFetchingUrl,
                    keyboardType: TextInputType.url,
                    textInputAction: TextInputAction.go,
                    onSubmitted: (_) => _fetchUrl(),
                    decoration: const InputDecoration(
                      hintText: 'Paste news URL',
                      prefixIcon: Icon(Icons.add_link),
                      border: OutlineInputBorder(),
                      isDense: true,
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                IconButton.filled(
                  tooltip: 'Fetch URL',
                  onPressed: _isFetchingUrl ? null : _fetchUrl,
                  icon: _isFetchingUrl
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.add),
                ),
              ],
            ),
          ),
          // Category filter chips
          Consumer<NewsProvider>(
            builder: (context, news, _) {
              return SizedBox(
                height: 50,
                child: ListView.builder(
                  scrollDirection: Axis.horizontal,
                  padding: const EdgeInsets.symmetric(horizontal: 12),
                  itemCount: _categories.length,
                  itemBuilder: (context, index) {
                    final isSelected =
                        news.selectedCategory == _categories[index];
                    return Padding(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 4, vertical: 8),
                      child: FilterChip(
                        label: Text(_categoryLabels[index]),
                        selected: isSelected,
                        onSelected: (_) {
                          news.setCategory(_categories[index]);
                        },
                      ),
                    );
                  },
                ),
              );
            },
          ),
          const Divider(height: 1),
          // Article feed with infinite scroll
          Expanded(
            child: Consumer<NewsProvider>(
              builder: (context, news, _) {
                if (news.articles.isEmpty && news.isLoading) {
                  return ListView.builder(
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    itemCount: 5,
                    itemBuilder: (_, __) => const ShimmerCard(),
                  );
                }

                if (news.articles.isEmpty && news.error != null) {
                  return Center(
                    child: Padding(
                      padding: const EdgeInsets.all(32),
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(Icons.wifi_off_rounded,
                              size: 56,
                              color: theme.colorScheme.onSurfaceVariant),
                          const SizedBox(height: 16),
                          Text(
                            'Could not load articles',
                            style: theme.textTheme.titleMedium,
                          ),
                          const SizedBox(height: 8),
                          Text(
                            news.error!,
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: theme.colorScheme.onSurfaceVariant,
                            ),
                            textAlign: TextAlign.center,
                          ),
                          const SizedBox(height: 24),
                          FilledButton.icon(
                            onPressed: _refresh,
                            icon: const Icon(Icons.refresh_rounded),
                            label: const Text('Retry'),
                          ),
                        ],
                      ),
                    ),
                  );
                }

                if (news.articles.isEmpty) {
                  return const Center(child: Text('No articles found'));
                }

                return RefreshIndicator(
                  onRefresh: _refresh,
                  child: ListView.builder(
                    controller: _scrollController,
                    padding: const EdgeInsets.only(top: 8, bottom: 24),
                    itemCount: news.articles.length + (news.hasMore ? 1 : 0),
                    itemBuilder: (context, index) {
                      if (index == news.articles.length) {
                        return const Padding(
                          padding: EdgeInsets.symmetric(vertical: 32),
                          child: Center(child: CircularProgressIndicator()),
                        );
                      }

                      final article = news.articles[index];
                      return ArticleVersionCard(
                        key: ValueKey(article.id),
                        article: article,
                        onTap: () {
                          Navigator.push(
                            context,
                            MaterialPageRoute(
                              builder: (_) =>
                                  ArticleDetailScreen(article: article),
                            ),
                          );
                        },
                      );
                    },
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
