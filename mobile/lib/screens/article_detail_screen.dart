import 'dart:async';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';

import '../models/article.dart';
import '../providers/auth_provider.dart';
import '../providers/reader_settings_provider.dart';
import '../services/api_service.dart';
import '../widgets/markdown_content.dart';
import 'subscription_screen.dart';

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
  bool _isCheckingAccess = true;
  bool _hasLoadedArticle = false;
  ApiException? _accessError;
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

  void _showTextSizeSheet() {
    showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      builder: (context) => const _TextSizeSheet(),
    );
  }

  // Label shown in tabs — never exposes model name, mirrors frontend A/B labeling
  String _tabLabel(int index, ArticleVersion version) {
    if (version.isOriginal) return 'Original';
    const letters = ['A', 'B', 'C', 'D'];
    final llmIndex =
        _article.versions.take(index).where((v) => !v.isOriginal).length;
    final letter =
        llmIndex < letters.length ? letters[llmIndex] : '${llmIndex + 1}';
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

  Future<void> _loadArticle({bool showLoading = false}) async {
    if (showLoading && mounted) {
      setState(() {
        _isCheckingAccess = true;
        _accessError = null;
      });
    }
    try {
      final data = await ApiService.getArticle(_article.id);
      if (!mounted) return;
      setState(() {
        _article = Article.fromJson(data);
        _isCheckingAccess = false;
        _hasLoadedArticle = true;
        _accessError = null;
      });
      _schedulePollingIfNeeded();
    } on ApiException catch (e) {
      if (!mounted) return;
      if (!_hasLoadedArticle ||
          e.statusCode == 401 ||
          e.statusCode == 403 ||
          e.statusCode == 429) {
        setState(() {
          _isCheckingAccess = false;
          _accessError = e;
        });
        return;
      }
      _schedulePollingIfNeeded();
    } catch (_) {
      if (!mounted) return;
      if (!_hasLoadedArticle) {
        setState(() {
          _isCheckingAccess = false;
          _accessError = const ApiException(
            0,
            'Could not connect to the news service',
          );
        });
        return;
      }
      _schedulePollingIfNeeded();
    }
  }

  Future<void> _openPlans() async {
    await Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => const SubscriptionScreen()),
    );
    if (mounted) await _loadArticle(showLoading: true);
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
    if (_isCheckingAccess) {
      return Scaffold(
        appBar: AppBar(title: const Text('Opening story')),
        body: const Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              CircularProgressIndicator(),
              SizedBox(height: 16),
              Text('Checking your daily reading access…'),
            ],
          ),
        ),
      );
    }
    if (_accessError case final error?) {
      return _ArticleAccessScreen(
        article: article,
        error: error,
        onViewPlans: _openPlans,
        onRetry: () => _loadArticle(showLoading: true),
      );
    }
    final hasMultiple = article.versions.length > 1;
    final fontScale = context.watch<ReaderSettingsProvider>().fontScale;

    return Scaffold(
      body: NestedScrollView(
        headerSliverBuilder: (context, innerBoxIsScrolled) => [
          SliverAppBar(
            expandedHeight: article.imageUrl != null ? 240 : 0,
            pinned: true,
            forceElevated: innerBoxIsScrolled,
            foregroundColor: article.imageUrl != null ? Colors.white : null,
            actions: [
              IconButton(
                onPressed: _showTextSizeSheet,
                tooltip: 'Text size',
                icon: const Icon(Icons.text_fields_rounded),
              ),
              const SizedBox(width: 4),
            ],
            flexibleSpace: article.imageUrl != null
                ? FlexibleSpaceBar(
                    background: Stack(
                      fit: StackFit.expand,
                      children: [
                        CachedNetworkImage(
                          imageUrl: article.imageUrl!,
                          fit: BoxFit.cover,
                          errorWidget: (_, __, ___) => Container(
                            color: theme.colorScheme.surfaceContainerHighest,
                            child: Icon(
                              Icons.image_not_supported_outlined,
                              size: 48,
                              color: theme.colorScheme.onSurfaceVariant,
                            ),
                          ),
                        ),
                        const DecoratedBox(
                          decoration: BoxDecoration(
                            gradient: LinearGradient(
                              begin: Alignment.topCenter,
                              end: Alignment.bottomCenter,
                              colors: [
                                Color(0x66000000),
                                Colors.transparent,
                                Color(0x55087F74),
                              ],
                              stops: [0, 0.55, 1],
                            ),
                          ),
                        ),
                      ],
                    ),
                  )
                : null,
            bottom: hasMultiple
                ? PreferredSize(
                    preferredSize: const Size.fromHeight(52),
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
              fontScale: fontScale,
              votedForId: _votedForId,
              isVoting: _isVoting,
              llmVersions: llmVersions,
              isAuthenticated: context.read<AuthProvider>().isAuthenticated,
              onVote: _vote,
            );
          },
        ),
      ),
    );
  }
}

class _ArticleAccessScreen extends StatelessWidget {
  final Article article;
  final ApiException error;
  final Future<void> Function() onViewPlans;
  final VoidCallback onRetry;

  const _ArticleAccessScreen({
    required this.article,
    required this.error,
    required this.onViewPlans,
    required this.onRetry,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isPlanLimit = error.statusCode == 403 || error.statusCode == 429;

    return Scaffold(
      appBar: AppBar(
          title: Text(isPlanLimit ? 'Daily reading' : 'Story unavailable')),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(28),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: 84,
                  height: 84,
                  decoration: BoxDecoration(
                    gradient: LinearGradient(
                      colors: isPlanLimit
                          ? const [Color(0xFFF2B134), Color(0xFFF06449)]
                          : [
                              theme.colorScheme.primaryContainer,
                              theme.colorScheme.secondaryContainer,
                            ],
                    ),
                    borderRadius: BorderRadius.circular(28),
                  ),
                  child: Icon(
                    isPlanLimit
                        ? Icons.auto_stories_rounded
                        : Icons.cloud_off_rounded,
                    size: 40,
                    color: isPlanLimit
                        ? Colors.white
                        : theme.colorScheme.onPrimaryContainer,
                  ),
                ),
                const SizedBox(height: 24),
                Text(
                  isPlanLimit
                      ? 'Your ${article.category ?? 'news'} pick is set for today'
                      : 'We couldn’t open this story',
                  style: theme.textTheme.headlineSmall,
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 12),
                Text(
                  error.message,
                  style: theme.textTheme.bodyLarge?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    height: 1.5,
                  ),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 28),
                if (isPlanLimit) ...[
                  SizedBox(
                    width: double.infinity,
                    child: FilledButton.icon(
                      onPressed: onViewPlans,
                      icon: const Icon(Icons.all_inclusive_rounded),
                      label: const Text('Read unlimited — \$14/month'),
                    ),
                  ),
                  const SizedBox(height: 12),
                  Text(
                    'Secure monthly billing. Cancel anytime.',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                    textAlign: TextAlign.center,
                  ),
                ] else
                  FilledButton.icon(
                    onPressed: onRetry,
                    icon: const Icon(Icons.refresh_rounded),
                    label: const Text('Try again'),
                  ),
              ],
            ),
          ),
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
      height: 52,
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
                      ? Border.all(color: theme.colorScheme.primary, width: 1.5)
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
                        fontWeight:
                            isSelected ? FontWeight.bold : FontWeight.normal,
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
  final double fontScale;
  final String? votedForId;
  final bool isVoting;
  final bool isAuthenticated;
  final List<ArticleVersion> llmVersions;
  final Future<void> Function(String chosenId, String otherId) onVote;

  const _VersionBody({
    required this.article,
    required this.version,
    required this.theme,
    required this.fontScale,
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
      padding: EdgeInsets.fromLTRB(
        20,
        20,
        20,
        20 + MediaQuery.viewPaddingOf(context).bottom,
      ),
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
                    labelStyle:
                        TextStyle(color: theme.colorScheme.onTertiaryContainer),
                    visualDensity: VisualDensity.compact,
                  ),
              ],
            ),
          if (article.category != null || article.isPremium)
            const SizedBox(height: 16),
          // Title
          Text(
            version.title,
            style: _scaledStyle(theme.textTheme.headlineSmall, fontScale)
                ?.copyWith(
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
            baseStyle:
                _scaledStyle(theme.textTheme.bodyLarge, fontScale)?.copyWith(
              fontWeight: FontWeight.w500,
              height: 1.65,
            ),
          ),
          // Full content
          if (version.content != null && version.content!.isNotEmpty) ...[
            const SizedBox(height: 20),
            MarkdownContent(
              markdown: version.content!,
              baseStyle: _scaledStyle(theme.textTheme.bodyMedium, fontScale)
                  ?.copyWith(height: 1.8),
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
}

TextStyle? _scaledStyle(TextStyle? style, double scale) {
  if (style == null) return null;
  return style.copyWith(fontSize: (style.fontSize ?? 16) * scale);
}

class _TextSizeSheet extends StatelessWidget {
  const _TextSizeSheet();

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final settings = context.watch<ReaderSettingsProvider>();

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(24, 0, 24, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 42,
                  height: 42,
                  decoration: BoxDecoration(
                    color: theme.colorScheme.primaryContainer,
                    borderRadius: BorderRadius.circular(13),
                  ),
                  child: Icon(
                    Icons.format_size_rounded,
                    color: theme.colorScheme.primary,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Reading text', style: theme.textTheme.titleLarge),
                      Text(
                        '${settings.fontScalePercent}% size',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
                TextButton(
                  onPressed:
                      settings.fontScale == 1 ? null : settings.resetFontSize,
                  child: const Text('Reset'),
                ),
              ],
            ),
            const SizedBox(height: 20),
            Row(
              children: [
                IconButton.filledTonal(
                  onPressed:
                      settings.canDecrease ? settings.decreaseFontSize : null,
                  tooltip: 'Smaller text',
                  icon: const Icon(Icons.remove_rounded),
                ),
                Expanded(
                  child: Slider(
                    value: settings.fontScale,
                    min: ReaderSettingsProvider.minimumScale,
                    max: ReaderSettingsProvider.maximumScale,
                    divisions: 7,
                    label: '${settings.fontScalePercent}%',
                    onChanged: settings.setFontScale,
                  ),
                ),
                IconButton.filled(
                  onPressed:
                      settings.canIncrease ? settings.increaseFontSize : null,
                  tooltip: 'Larger text',
                  icon: const Icon(Icons.add_rounded),
                ),
              ],
            ),
            const SizedBox(height: 16),
            Container(
              padding: const EdgeInsets.all(14),
              decoration: BoxDecoration(
                color: theme.colorScheme.secondaryContainer,
                borderRadius: BorderRadius.circular(14),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.volume_up_rounded,
                    color: theme.colorScheme.onSecondaryContainer,
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      'Tip: use the volume buttons while reading or scrolling the news to change text size.',
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: theme.colorScheme.onSecondaryContainer,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
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
      final chosenVersion = votedForId == aId ? a : b;
      final otherVersion = votedForId == aId ? b : a;
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
    final isCurrentChosen =
        version.rewriteId == aId || version.rewriteId == bId;
    if (!isCurrentChosen || version.rewriteId == null) {
      return const SizedBox.shrink();
    }

    final otherId = version.rewriteId == aId ? bId! : aId!;

    return SizedBox(
      width: double.infinity,
      child: FilledButton.icon(
        onPressed: isVoting ? null : () => onVote(version.rewriteId!, otherId),
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
