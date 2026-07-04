import 'dart:async';

import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:intl/intl.dart';
import 'package:url_launcher/url_launcher.dart';

import '../models/article.dart';
import '../services/api_service.dart';
import '../widgets/markdown_content.dart';

class ArticleDetailScreen extends StatefulWidget {
  final Article article;

  const ArticleDetailScreen({super.key, required this.article});

  @override
  State<ArticleDetailScreen> createState() => _ArticleDetailScreenState();
}

class _ArticleDetailScreenState extends State<ArticleDetailScreen> {
  late Article _article = widget.article;
  Timer? _pollTimer;

  @override
  void initState() {
    super.initState();
    _loadArticle();
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    super.dispose();
  }

  Future<void> _loadArticle() async {
    try {
      final data = await ApiService.getArticle(_article.id);
      if (!mounted) return;
      setState(() => _article = Article.fromJson(data));
      _schedulePollingIfNeeded();
    } catch (_) {
      if (!mounted) return;
      _schedulePollingIfNeeded();
    }
  }

  void _schedulePollingIfNeeded() {
    _pollTimer?.cancel();
    if (_article.rewriteStatus != 'pending') return;
    _pollTimer = Timer(const Duration(seconds: 5), _loadArticle);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final dateFormat = DateFormat.yMMMd().add_jm();
    final hasOriginal = _article.originalContent?.trim().isNotEmpty == true;
    final hasContent = _article.content?.trim().isNotEmpty == true;
    final categories = _article.categories.isNotEmpty
        ? _article.categories
        : _article.category != null
            ? [_article.category!]
            : const <String>[];

    return Scaffold(
      body: CustomScrollView(
        slivers: [
          SliverAppBar(
            expandedHeight: _article.imageUrl != null ? 250 : 0,
            pinned: true,
            flexibleSpace: _article.imageUrl != null
                ? FlexibleSpaceBar(
                    background: CachedNetworkImage(
                      imageUrl: _article.imageUrl!,
                      fit: BoxFit.cover,
                      errorWidget: (_, __, ___) => Container(
                        color: theme.colorScheme.surfaceContainerHighest,
                        child: const Icon(Icons.image_not_supported, size: 48),
                      ),
                    ),
                  )
                : null,
          ),
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.all(20),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      for (final category in categories.take(3))
                        Chip(
                          label: Text(category),
                          visualDensity: VisualDensity.compact,
                        ),
                      if (_article.isPremium)
                        Chip(
                          avatar: const Icon(Icons.star, size: 16),
                          label: const Text('Premium'),
                          backgroundColor: theme.colorScheme.tertiaryContainer,
                          visualDensity: VisualDensity.compact,
                        ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  Text(
                    _article.title,
                    style: theme.textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      Icon(Icons.source_outlined,
                          size: 16,
                          color: theme.colorScheme.onSurfaceVariant),
                      const SizedBox(width: 4),
                      Expanded(
                        child: Text(
                          _article.sourceName,
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        dateFormat.format(_article.publishedAt),
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                  const Divider(height: 32),
                  MarkdownContent(
                    markdown: _article.summary,
                    baseStyle: theme.textTheme.bodyLarge?.copyWith(
                      fontWeight: FontWeight.w500,
                      height: 1.6,
                    ),
                  ),
                  if (hasOriginal ||
                      hasContent ||
                      _article.rewriteStatus == 'pending') ...[
                    const SizedBox(height: 20),
                    _buildContentSection(theme, hasOriginal, hasContent),
                  ],
                  const SizedBox(height: 24),
                  SizedBox(
                    width: double.infinity,
                    child: OutlinedButton.icon(
                      onPressed: () async {
                        final uri = Uri.parse(_article.sourceUrl);
                        if (await canLaunchUrl(uri)) {
                          await launchUrl(uri,
                              mode: LaunchMode.externalApplication);
                        }
                      },
                      icon: const Icon(Icons.open_in_new),
                      label: const Text('Read Original Source'),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildContentSection(
    ThemeData theme,
    bool hasOriginal,
    bool hasContent,
  ) {
    if (!hasOriginal) {
      return _buildRewriteContent(theme, hasContent);
    }

    final aiSection = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionTitle('AI Rewrite'),
        _buildRewriteContent(theme, hasContent),
      ],
    );
    final originalSection = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionTitle('Original'),
        MarkdownContent(
          markdown: _article.originalContent!,
          baseStyle: theme.textTheme.bodyMedium?.copyWith(
            height: 1.8,
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );

    return LayoutBuilder(
      builder: (context, constraints) {
        if (constraints.maxWidth >= 720) {
          return Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(child: aiSection),
              const SizedBox(width: 20),
              Expanded(child: originalSection),
            ],
          );
        }

        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            aiSection,
            const SizedBox(height: 24),
            originalSection,
          ],
        );
      },
    );
  }

  Widget _buildRewriteContent(ThemeData theme, bool hasContent) {
    if (_article.rewriteStatus == 'pending') {
      return _ProcessingPanel(theme: theme);
    }
    if (_article.rewriteStatus == 'failed') {
      return _FailedPanel(theme: theme);
    }
    if (!hasContent) {
      return const SizedBox.shrink();
    }

    return MarkdownContent(
      markdown: _article.content!,
      baseStyle: theme.textTheme.bodyMedium?.copyWith(height: 1.8),
    );
  }
}

class _SectionTitle extends StatelessWidget {
  final String text;

  const _SectionTitle(this.text);

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Text(
        text.toUpperCase(),
        style: theme.textTheme.labelMedium?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
          fontWeight: FontWeight.bold,
        ),
      ),
    );
  }
}

class _ProcessingPanel extends StatelessWidget {
  final ThemeData theme;

  const _ProcessingPanel({required this.theme});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        border: Border.all(color: theme.colorScheme.outlineVariant),
        borderRadius: BorderRadius.circular(8),
      ),
      child: const Row(
        children: [
          SizedBox(
            width: 20,
            height: 20,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
          SizedBox(width: 12),
          Expanded(child: Text('Processing AI rewrite...')),
        ],
      ),
    );
  }
}

class _FailedPanel extends StatelessWidget {
  final ThemeData theme;

  const _FailedPanel({required this.theme});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        border: Border.all(color: theme.colorScheme.error),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        'AI rewrite failed. The original article is still available.',
        style: TextStyle(color: theme.colorScheme.error),
      ),
    );
  }
}
