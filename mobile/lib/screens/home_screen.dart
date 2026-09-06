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

  static const _categoryFilters = [
    _CategoryFilter(null, 'For you', Icons.auto_awesome_rounded),
    _CategoryFilter('Technology', 'Tech', Icons.memory_rounded),
    _CategoryFilter('Science', 'Science', Icons.science_rounded),
    _CategoryFilter('Business', 'Business', Icons.trending_up_rounded),
    _CategoryFilter('Health', 'Health', Icons.favorite_rounded),
    _CategoryFilter('Sports', 'Sports', Icons.sports_basketball_rounded),
    _CategoryFilter('World', 'World', Icons.public_rounded),
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
        MaterialPageRoute(
            builder: (_) => ArticleDetailScreen(article: article)),
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

  Future<void> _openPlans() async {
    await Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => const SubscriptionScreen()),
    );
    if (mounted) await context.read<AuthProvider>().refreshUser();
  }

  Future<void> _openNewsTools() async {
    final theme = Theme.of(context);
    final tier = context.read<AuthProvider>().user?.subscriptionTier;
    final isUnlimited = tier != null && tier != 'free';
    final action = await showModalBottomSheet<_NewsToolsAction>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (sheetContext) => SafeArea(
        child: SingleChildScrollView(
          padding: EdgeInsets.fromLTRB(
            20,
            0,
            20,
            20 + MediaQuery.viewInsetsOf(sheetContext).bottom,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Container(
                    width: 44,
                    height: 44,
                    decoration: BoxDecoration(
                      gradient: const LinearGradient(
                        colors: [Color(0xFF087F74), Color(0xFF20A58E)],
                      ),
                      borderRadius: BorderRadius.circular(14),
                    ),
                    child: const Icon(
                      Icons.add_link_rounded,
                      color: Colors.white,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('News tools', style: theme.textTheme.titleLarge),
                        Text(
                          'Submit a link or manage your reading plan',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 18),
              Container(
                padding: const EdgeInsets.fromLTRB(14, 10, 8, 10),
                decoration: BoxDecoration(
                  color: isUnlimited
                      ? theme.colorScheme.tertiaryContainer
                      : theme.colorScheme.secondaryContainer,
                  borderRadius: BorderRadius.circular(14),
                ),
                child: Row(
                  children: [
                    Icon(
                      isUnlimited
                          ? Icons.all_inclusive_rounded
                          : Icons.today_rounded,
                      color: isUnlimited
                          ? theme.colorScheme.onTertiaryContainer
                          : theme.colorScheme.onSecondaryContainer,
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Text(
                        isUnlimited
                            ? 'Unlimited reading'
                            : 'Free · 1 story per category today',
                        style: theme.textTheme.labelLarge,
                      ),
                    ),
                    TextButton(
                      onPressed: () => Navigator.pop(
                        sheetContext,
                        _NewsToolsAction.plans,
                      ),
                      child: const Text('Plans'),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 20),
              TextField(
                controller: _urlController,
                autofocus: true,
                keyboardType: TextInputType.url,
                textInputAction: TextInputAction.go,
                onSubmitted: (_) => Navigator.pop(
                  sheetContext,
                  _NewsToolsAction.fetch,
                ),
                decoration: const InputDecoration(
                  labelText: 'News article URL',
                  hintText: 'https://…',
                  prefixIcon: Icon(Icons.link_rounded),
                ),
              ),
              const SizedBox(height: 14),
              SizedBox(
                width: double.infinity,
                child: FilledButton.icon(
                  onPressed: () => Navigator.pop(
                    sheetContext,
                    _NewsToolsAction.fetch,
                  ),
                  icon: const Icon(Icons.auto_fix_high_rounded),
                  label: const Text('Fetch and clean article'),
                ),
              ),
            ],
          ),
        ),
      ),
    );
    if (!mounted) return;
    switch (action) {
      case _NewsToolsAction.fetch:
        await _fetchUrl();
      case _NewsToolsAction.plans:
        await _openPlans();
      case null:
        break;
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final tier = context.watch<AuthProvider>().user?.subscriptionTier;
    final isUnlimited = tier != null && tier != 'free';

    return Scaffold(
      appBar: AppBar(
        title: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 36,
              height: 36,
              decoration: BoxDecoration(
                gradient: const LinearGradient(
                  colors: [Color(0xFF087F74), Color(0xFF20A58E)],
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                ),
                borderRadius: BorderRadius.circular(11),
              ),
              child: const Icon(
                Icons.newspaper_rounded,
                color: Colors.white,
                size: 20,
              ),
            ),
            const SizedBox(width: 10),
            const Text(
              'No Clickbait',
              style: TextStyle(fontWeight: FontWeight.w800),
            ),
          ],
        ),
        actions: [
          IconButton.filledTonal(
            onPressed: _isFetchingUrl ? null : _openNewsTools,
            tooltip: 'News tools',
            icon: _isFetchingUrl
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : Icon(
                    isUnlimited
                        ? Icons.all_inclusive_rounded
                        : Icons.add_link_rounded,
                  ),
          ),
          PopupMenuButton<_HomeAction>(
            tooltip: 'Account and links',
            icon: const Icon(Icons.account_circle_outlined),
            onSelected: (action) {
              switch (action) {
                case _HomeAction.myUrls:
                  Navigator.push(
                    context,
                    MaterialPageRoute(builder: (_) => const MyUrlsScreen()),
                  );
                  break;
                case _HomeAction.plans:
                  _openPlans();
                  break;
                case _HomeAction.signOut:
                  context.read<AuthProvider>().logout();
                  break;
              }
            },
            itemBuilder: (context) => const [
              PopupMenuItem(
                value: _HomeAction.myUrls,
                child: ListTile(
                  leading: Icon(Icons.link_rounded),
                  title: Text('My submitted links'),
                  contentPadding: EdgeInsets.zero,
                ),
              ),
              PopupMenuItem(
                value: _HomeAction.plans,
                child: ListTile(
                  leading: Icon(Icons.workspace_premium_outlined),
                  title: Text('Plans and access'),
                  contentPadding: EdgeInsets.zero,
                ),
              ),
              PopupMenuDivider(),
              PopupMenuItem(
                value: _HomeAction.signOut,
                child: ListTile(
                  leading: Icon(Icons.logout_rounded),
                  title: Text('Sign out'),
                  contentPadding: EdgeInsets.zero,
                ),
              ),
            ],
          ),
          const SizedBox(width: 4),
        ],
      ),
      body: Column(
        children: [
          Consumer<NewsProvider>(
            builder: (context, news, _) {
              return SizedBox(
                height: 54,
                child: ListView.builder(
                  scrollDirection: Axis.horizontal,
                  padding: const EdgeInsets.symmetric(horizontal: 12),
                  itemCount: _categoryFilters.length,
                  itemBuilder: (context, index) {
                    final filter = _categoryFilters[index];
                    final isSelected = news.selectedCategory == filter.value;
                    return Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 4),
                      child: FilterChip(
                        avatar: Icon(filter.icon, size: 17),
                        label: Text(filter.label),
                        selected: isSelected,
                        showCheckmark: false,
                        selectedColor: theme.colorScheme.primaryContainer,
                        onSelected: (_) {
                          news.setCategory(filter.value);
                        },
                      ),
                    );
                  },
                ),
              );
            },
          ),
          Consumer<NewsProvider>(
            builder: (context, news, _) => Padding(
              padding: const EdgeInsets.fromLTRB(18, 8, 18, 4),
              child: Row(
                children: [
                  Text(
                    news.selectedCategory ?? 'Latest stories',
                    style: theme.textTheme.titleMedium,
                  ),
                  const Spacer(),
                  Icon(
                    Icons.swipe_rounded,
                    size: 17,
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                  const SizedBox(width: 5),
                  Text(
                    'Swipe to compare',
                    style: theme.textTheme.labelSmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
          ),
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
                    padding: const EdgeInsets.only(top: 2, bottom: 24),
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

enum _HomeAction { myUrls, plans, signOut }

enum _NewsToolsAction { fetch, plans }

class _CategoryFilter {
  final String? value;
  final String label;
  final IconData icon;

  const _CategoryFilter(this.value, this.label, this.icon);
}
