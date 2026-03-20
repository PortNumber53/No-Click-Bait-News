import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../providers/auth_provider.dart';
import '../providers/news_provider.dart';
import '../widgets/article_card.dart';
import '../widgets/shimmer_card.dart';
import 'article_detail_screen.dart';
import 'subscription_screen.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  final _scrollController = ScrollController();

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
    // Load initial articles
    WidgetsBinding.instance.addPostFrameCallback((_) {
      context.read<NewsProvider>().loadArticles(refresh: true);
    });
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 300) {
      context.read<NewsProvider>().loadArticles();
    }
  }

  Future<void> _refresh() async {
    await context.read<NewsProvider>().loadArticles(refresh: true);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text(
          'No-Click Bait News',
          style: TextStyle(fontWeight: FontWeight.bold),
        ),
        actions: [
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
            icon: const Icon(Icons.logout),
            tooltip: 'Sign Out',
            onPressed: () => context.read<AuthProvider>().logout(),
          ),
        ],
      ),
      body: Column(
        children: [
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
          // Article list with infinite scroll
          Expanded(
            child: Consumer<NewsProvider>(
              builder: (context, news, _) {
                if (news.articles.isEmpty && news.isLoading) {
                  return ListView.builder(
                    padding: const EdgeInsets.all(16),
                    itemCount: 5,
                    itemBuilder: (_, __) => const ShimmerCard(),
                  );
                }

                if (news.articles.isEmpty && news.error != null) {
                  return Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.error_outline,
                            size: 48, color: theme.colorScheme.error),
                        const SizedBox(height: 16),
                        Text(news.error!),
                        const SizedBox(height: 16),
                        FilledButton.tonal(
                          onPressed: _refresh,
                          child: const Text('Retry'),
                        ),
                      ],
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
                    padding: const EdgeInsets.all(16),
                    itemCount: news.articles.length + (news.hasMore ? 1 : 0),
                    itemBuilder: (context, index) {
                      if (index == news.articles.length) {
                        return const Padding(
                          padding: EdgeInsets.all(24),
                          child: Center(child: CircularProgressIndicator()),
                        );
                      }

                      final article = news.articles[index];
                      return ArticleCard(
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
