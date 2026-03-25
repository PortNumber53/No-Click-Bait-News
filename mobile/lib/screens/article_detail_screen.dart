import 'dart:async';

import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:intl/intl.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';

import '../models/article.dart';
import '../providers/auth_provider.dart';
import '../services/api_service.dart';
import '../widgets/markdown_content.dart';

class ArticleDetailScreen extends StatefulWidget {
  final Article article;
  final int initialVersion;

  const ArticleDetailScreen({
    super.key,
    required this.article,
    this.initialVersion = 0,
  });

  @override
  State<ArticleDetailScreen> createState() => _ArticleDetailScreenState();
}

class _ArticleDetailScreenState extends State<ArticleDetailScreen> {
  late Article _article = widget.article;
  late final PageController _pageController;
  late int _currentVersion;
  Timer? _pollTimer;
  // rewriteId of the version the user voted for (null = not yet voted)
  String? _votedForId;
  bool _isVoting = false;

  @override
  void initState() {
    super.initState();
    _currentVersion = widget.initialVersion.clamp(
      0,
      (_article.versions.length - 1).clamp(0, double.maxFinite.toInt()),
    );
    _pageController = PageController(initialPage: _currentVersion);
    _loadArticle();
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    _pageController.dispose();
    super.dispose();
  }

  // Label shown in tabs — never exposes model name, mirrors frontend A/B labeling
  String _tabLabel(int index, ArticleVersion version) {
    if (version.isOriginal) return 'Original';
    const letters = ['A', 'B', 'C', 'D'];
    final llmIndex = _article.versions
        .take(index)
        .where((v) => !v.isOriginal)
        .length;
    final letter = llmIndex < letters.length ? letters[llmIndex] : '${llmIndex + 1}';
    return 'Version $letter';
  }

  Future<void> _vote(String chosenId, String otherId) async {
    if (_isVoting || _votedForId != null) return;
    setState(() => _isVoting = true);
    try {
      await ApiService.submitVote(
        articleId: _article.id,
        chosenRewriteId: chosenId,
        otherRewriteId: otherId,
      );
      setState(() => _votedForId = chosenId);
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.message)),
        );
      }
    } finally {
      if (mounted) setState(() => _isVoting = false);
    }
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
    final article = _article;
    final hasMultiple = article.versions.length > 1;

    return Scaffold(
      body: NestedScrollView(
        headerSliverBuilder: (context, innerBoxIsScrolled) => [
          SliverAppBar(
            expandedHeight: article.imageUrl != null ? 240 : 0,
            pinned: true,
            forceElevated: innerBoxIsScrolled,
            flexibleSpace: article.imageUrl != null
                ? FlexibleSpaceBar(
                    background: CachedNetworkImage(
                      imageUrl: article.imageUrl!,
                      fit: BoxFit.cover,
                      errorWidget: (_, __, ___) => Container(
                        color: theme.colorScheme.surfaceContainerHighest,
                        child: Icon(Icons.image_not_supported_outlined,
                            size: 48,
                            color: theme.colorScheme.onSurfaceVariant),
                      ),
                    ),
                  )
                : null,
            bottom: hasMultiple
                ? PreferredSize(
                    preferredSize: const Size.fromHeight(48),
                    child: _VersionTabBar(
                      versions: article.versions,
                      current: _currentVersion,
                      labelFor: _tabLabel,
                      onTap: (i) {
                        setState(() => _currentVersion = i);
                        _pageController.animateToPage(
                          i,
                          duration: const Duration(milliseconds: 300),
                          curve: Curves.easeInOut,
                        );
                      },
                      theme: theme,
                    ),
                  )
                : null,
          ),
        ],
        body: PageView.builder(
          controller: _pageController,
          itemCount: article.versions.length,
          onPageChanged: (i) => setState(() => _currentVersion = i),
          itemBuilder: (context, i) {
            final version = article.versions[i];
            // Collect the two LLM rewrite IDs for voting
            final llmVersions =
                article.versions.where((v) => !v.isOriginal).toList();
            return _VersionBody(
              article: article,
              version: version,
              theme: theme,
              votedForId: _votedForId,
              isVoting: _isVoting,
              llmVersions: llmVersions,
              isAuthenticated:
                  context.read<AuthProvider>().isAuthenticated,
              onVote: _vote,
            );
          },
        ),
      ),
    );
  }
}

// ── Tab bar ──────────────────────────────────────────────────────────────────

class _VersionTabBar extends StatelessWidget {
  final List<ArticleVersion> versions;
  final int current;
  final String Function(int, ArticleVersion) labelFor;
  final void Function(int) onTap;
  final ThemeData theme;

  const _VersionTabBar({
    required this.versions,
    required this.current,
    required this.labelFor,
    required this.onTap,
    required this.theme,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      color: theme.colorScheme.surface,
      height: 48,
      child: ListView.builder(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        itemCount: versions.length,
        itemBuilder: (context, i) {
          final isSelected = i == current;
          final version = versions[i];
          return Padding(
            padding: const EdgeInsets.only(right: 8),
            child: GestureDetector(
              onTap: () => onTap(i),
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 200),
                padding:
                    const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
                decoration: BoxDecoration(
                  color: isSelected
                      ? theme.colorScheme.primaryContainer
                      : theme.colorScheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(20),
                  border: isSelected
                      ? Border.all(
                          color: theme.colorScheme.primary, width: 1.5)
                      : null,
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(
                      version.isOriginal
                          ? Icons.article_outlined
                          : Icons.auto_awesome_rounded,
                      size: 12,
                      color: isSelected
                          ? theme.colorScheme.primary
                          : theme.colorScheme.onSurfaceVariant,
                    ),
                    const SizedBox(width: 5),
                    Text(
                      labelFor(i, version),
                      style: theme.textTheme.labelSmall?.copyWith(
                        color: isSelected
                            ? theme.colorScheme.primary
                            : theme.colorScheme.onSurfaceVariant,
                        fontWeight: isSelected
                            ? FontWeight.bold
                            : FontWeight.normal,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}

// ── Version body ─────────────────────────────────────────────────────────────

class _VersionBody extends StatelessWidget {
  final Article article;
  final ArticleVersion version;
  final ThemeData theme;
  final String? votedForId;
  final bool isVoting;
  final bool isAuthenticated;
  final List<ArticleVersion> llmVersions;
  final Future<void> Function(String chosenId, String otherId) onVote;

  const _VersionBody({
    required this.article,
    required this.version,
    required this.theme,
    required this.votedForId,
    required this.isVoting,
    required this.isAuthenticated,
    required this.llmVersions,
    required this.onVote,
  });

  @override
  Widget build(BuildContext context) {
    final dateFormat = DateFormat.yMMMd().add_jm();
    final hasVoted = votedForId != null;

    return SingleChildScrollView(
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Meta chips (category + premium only — no model name)
          if (article.category != null || article.isPremium)
            Wrap(
              spacing: 8,
              runSpacing: 4,
              children: [
                if (article.category != null)
                  Chip(
                    label: Text(article.category!),
                    visualDensity: VisualDensity.compact,
                  ),
                if (article.isPremium)
                  Chip(
                    avatar: const Icon(Icons.star_rounded, size: 16),
                    label: const Text('Premium'),
                    backgroundColor: theme.colorScheme.tertiaryContainer,
                    labelStyle: TextStyle(
                        color: theme.colorScheme.onTertiaryContainer),
                    visualDensity: VisualDensity.compact,
                  ),
              ],
            ),
          if (article.category != null || article.isPremium)
            const SizedBox(height: 16),
          // Title
          Text(
            version.title,
            style: theme.textTheme.headlineSmall?.copyWith(
              fontWeight: FontWeight.bold,
              height: 1.3,
            ),
          ),
          const SizedBox(height: 12),
          // Source & date row
          Row(
            children: [
              Icon(Icons.source_outlined,
                  size: 15, color: theme.colorScheme.primary),
              const SizedBox(width: 4),
              Flexible(
                child: Text(
                  article.sourceName,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.primary,
                    fontWeight: FontWeight.w500,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const Spacer(),
              Text(
                dateFormat.format(article.publishedAt),
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
          const Divider(height: 32),
          // Summary
          MarkdownContent(
            markdown: version.summary,
            baseStyle: theme.textTheme.bodyLarge?.copyWith(
              fontWeight: FontWeight.w500,
              height: 1.65,
            ),
          ),
          // Full content
          if (version.content != null && version.content!.isNotEmpty) ...[
            const SizedBox(height: 20),
            MarkdownContent(
              markdown: version.content!,
              baseStyle: theme.textTheme.bodyMedium?.copyWith(height: 1.8),
            ),
          ],
          const SizedBox(height: 32),
          // ── Vote section (only for LLM versions, not original) ──
          if (!version.isOriginal && llmVersions.length == 2) ...[
            _VoteSection(
              theme: theme,
              version: version,
              llmVersions: llmVersions,
              votedForId: votedForId,
              isVoting: isVoting,
              isAuthenticated: isAuthenticated,
              hasVoted: hasVoted,
              onVote: onVote,
            ),
            const SizedBox(height: 24),
          ],
          // ── Read original source ──
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: () async {
                final uri = Uri.parse(article.sourceUrl);
                if (await canLaunchUrl(uri)) {
                  await launchUrl(uri, mode: LaunchMode.externalApplication);
                }
              },
              icon: const Icon(Icons.open_in_new_rounded),
              label: const Text('Read Original Source'),
            ),
          ),
          const SizedBox(height: 16),
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
          markdown: article.originalContent!,
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
    if (article.rewriteStatus == 'pending') {
      return _ProcessingPanel(theme: theme);
    }
    if (article.rewriteStatus == 'failed') {
      return _FailedPanel(theme: theme);
    }
    if (!hasContent) {
      return const SizedBox.shrink();
    }

    return MarkdownContent(
      markdown: article.content!,
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

// ── Vote section ─────────────────────────────────────────────────────────────

class _VoteSection extends StatelessWidget {
  final ThemeData theme;
  final ArticleVersion version;
  final List<ArticleVersion> llmVersions;
  final String? votedForId;
  final bool isVoting;
  final bool isAuthenticated;
  final bool hasVoted;
  final Future<void> Function(String, String) onVote;

  const _VoteSection({
    required this.theme,
    required this.version,
    required this.llmVersions,
    required this.votedForId,
    required this.isVoting,
    required this.isAuthenticated,
    required this.hasVoted,
    required this.onVote,
  });

  @override
  Widget build(BuildContext context) {
    final a = llmVersions[0];
    final b = llmVersions[1];
    final aId = a.rewriteId;
    final bId = b.rewriteId;
    final canVote = isAuthenticated && aId != null && bId != null;

    if (!canVote && !isAuthenticated) {
      return Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: theme.colorScheme.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(12),
        ),
        child: Row(
          children: [
            Icon(Icons.how_to_vote_outlined,
                color: theme.colorScheme.onSurfaceVariant),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                'Sign in to vote for your preferred version',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ),
          ],
        ),
      );
    }

    if (hasVoted) {
      // After voting, reveal the model names
      final chosenVersion =
          votedForId == aId ? a : b;
      final otherVersion =
          votedForId == aId ? b : a;
      return Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: theme.colorScheme.primaryContainer,
          borderRadius: BorderRadius.circular(12),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.check_circle_rounded,
                    color: theme.colorScheme.primary, size: 20),
                const SizedBox(width: 8),
                Text(
                  'You preferred: ${chosenVersion.modelName}',
                  style: theme.textTheme.titleSmall?.copyWith(
                    color: theme.colorScheme.onPrimaryContainer,
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              'Other version: ${otherVersion.modelName}',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onPrimaryContainer,
              ),
            ),
          ],
        ),
      );
    }

    // Pre-vote: show "Prefer this version?" button on the current LLM version
    final isCurrentChosen = version.rewriteId == aId || version.rewriteId == bId;
    if (!isCurrentChosen || version.rewriteId == null) return const SizedBox.shrink();

    final otherId = version.rewriteId == aId ? bId! : aId!;

    return SizedBox(
      width: double.infinity,
      child: FilledButton.icon(
        onPressed: isVoting
            ? null
            : () => onVote(version.rewriteId!, otherId),
        icon: isVoting
            ? const SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            : const Icon(Icons.thumb_up_alt_outlined, size: 18),
        label: const Text('Prefer this version'),
      ),
    );
  }
}
