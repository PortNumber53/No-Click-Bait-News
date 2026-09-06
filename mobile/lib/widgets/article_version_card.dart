import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';

import '../models/article.dart';

class ArticleVersionCard extends StatefulWidget {
  final Article article;
  final VoidCallback onTap;

  const ArticleVersionCard({
    super.key,
    required this.article,
    required this.onTap,
  });

  @override
  State<ArticleVersionCard> createState() => _ArticleVersionCardState();
}

class _ArticleVersionCardState extends State<ArticleVersionCard> {
  // Index into the LLM-only versions list (never shows original on feed)
  int _llmIndex = 0;

  List<ArticleVersion> _llmVersions(Article article) =>
      article.versions.where((v) => !v.isOriginal).toList();

  void _onHorizontalDrag(DragEndDetails details) {
    final llm = _llmVersions(widget.article);
    if (llm.length <= 1) return;
    if (details.primaryVelocity == null) return;
    // Lower threshold for easier swiping (was 200)
    if (details.primaryVelocity! < -100) {
      setState(() => _llmIndex = (_llmIndex + 1) % llm.length);
    } else if (details.primaryVelocity! > 100) {
      setState(() => _llmIndex = (_llmIndex - 1 + llm.length) % llm.length);
    }
  }

  String _labelFor(int llmIdx) {
    const labels = ['A', 'B', 'C', 'D'];
    return 'Version ${llmIdx < labels.length ? labels[llmIdx] : llmIdx + 1}';
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final article = widget.article;
    final llmVersions = _llmVersions(article);
    final hasMultipleVersions = llmVersions.length >= 2;
    // Clamp in case versions list shrinks on rebuild
    final safeIndex =
        llmVersions.isEmpty ? 0 : _llmIndex.clamp(0, llmVersions.length - 1);
    final activeVersion = llmVersions.isNotEmpty
        ? llmVersions[safeIndex]
        : (article.versions.isNotEmpty ? article.versions.first : null);

    return Semantics(
      button: true,
      label: 'Read ${activeVersion?.title ?? article.title}',
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: widget.onTap,
        onHorizontalDragEnd: _onHorizontalDrag,
        child: Container(
          margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          decoration: BoxDecoration(
            color: theme.colorScheme.surface,
            borderRadius: BorderRadius.circular(16),
            border: Border.all(
              color: theme.colorScheme.outlineVariant.withValues(alpha: 0.65),
            ),
            boxShadow: [
              BoxShadow(
                color: theme.colorScheme.shadow.withValues(alpha: 0.08),
                blurRadius: 12,
                offset: const Offset(0, 4),
              ),
            ],
          ),
          clipBehavior: Clip.antiAlias,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                height: 4,
                decoration: const BoxDecoration(
                  gradient: LinearGradient(
                    colors: [Color(0xFF087F74), Color(0xFFF06449)],
                  ),
                ),
              ),
              if (article.imageUrl != null)
                AspectRatio(
                  aspectRatio: 16 / 9,
                  child: CachedNetworkImage(
                    imageUrl: article.imageUrl!,
                    fit: BoxFit.cover,
                    placeholder: (_, __) => Container(
                      color: theme.colorScheme.surfaceContainerHighest,
                      child: const Center(child: CircularProgressIndicator()),
                    ),
                    errorWidget: (_, __, ___) => Container(
                      color: theme.colorScheme.surfaceContainerHighest,
                      child: Icon(
                        Icons.image_not_supported_outlined,
                        size: 40,
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ),
                ),
              if (activeVersion != null)
                // Render all LLM versions offstage simultaneously so
                // IntrinsicHeight always reserves the tallest version's space.
                IntrinsicHeight(
                  child: Stack(
                    children: [
                      for (int i = 0; i < llmVersions.length; i++)
                        Offstage(
                          offstage: true,
                          child: _VersionContent(
                            article: article,
                            version: llmVersions[i],
                            versionLabel: _labelFor(i),
                            theme: theme,
                          ),
                        ),
                      _VersionContent(
                        article: article,
                        version: activeVersion,
                        versionLabel: llmVersions.isNotEmpty
                            ? _labelFor(safeIndex)
                            : null,
                        theme: theme,
                      ),
                    ],
                  ),
                ),
              if (hasMultipleVersions)
                _VersionIndicator(
                  count: llmVersions.length,
                  current: safeIndex,
                  theme: theme,
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class _VersionContent extends StatelessWidget {
  final Article article;
  final ArticleVersion version;
  final ThemeData theme;
  final String? versionLabel;

  const _VersionContent({
    required this.article,
    required this.version,
    required this.theme,
    this.versionLabel,
  });

  @override
  Widget build(BuildContext context) {
    final categoryColor = _categoryColor(article.category);

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 14, 16, 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              if (article.category != null)
                Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                  decoration: BoxDecoration(
                    color: categoryColor.withValues(alpha: 0.16),
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: Text(
                    article.category!,
                    style: theme.textTheme.labelSmall?.copyWith(
                      color: categoryColor,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
              if (article.isPremium) ...[
                const SizedBox(width: 6),
                Icon(Icons.star_rounded,
                    size: 16, color: theme.colorScheme.tertiary),
              ],
              if (versionLabel != null) ...[
                const Spacer(),
                Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                  decoration: BoxDecoration(
                    color: theme.colorScheme.secondaryContainer,
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: Text(
                    versionLabel!,
                    style: theme.textTheme.labelSmall?.copyWith(
                      color: theme.colorScheme.onSecondaryContainer,
                      fontWeight: FontWeight.bold,
                      fontSize: 10,
                    ),
                  ),
                ),
              ],
            ],
          ),
          const SizedBox(height: 10),
          Text(
            version.title,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.bold,
              height: 1.3,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            version.summary,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              height: 1.45,
            ),
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              Icon(Icons.source_outlined,
                  size: 14, color: theme.colorScheme.primary),
              const SizedBox(width: 4),
              Text(
                article.sourceName,
                style: theme.textTheme.labelMedium?.copyWith(
                  color: theme.colorScheme.primary,
                  fontWeight: FontWeight.w500,
                ),
              ),
              const Spacer(),
              Text(
                _timeAgo(article.publishedAt),
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  String _timeAgo(DateTime dateTime) {
    final diff = DateTime.now().difference(dateTime);
    if (diff.inMinutes < 60) return '${diff.inMinutes}m ago';
    if (diff.inHours < 24) return '${diff.inHours}h ago';
    if (diff.inDays < 7) return '${diff.inDays}d ago';
    if (diff.inDays < 30) return '${diff.inDays}d ago';
    return '${diff.inDays ~/ 30}mo ago';
  }

  Color _categoryColor(String? category) {
    final color = switch (category?.toLowerCase()) {
      'technology' => const Color(0xFF4776D0),
      'science' => const Color(0xFF7756C7),
      'business' => const Color(0xFF087F74),
      'health' => const Color(0xFFD94E67),
      'sports' => const Color(0xFFE0782F),
      'world' => const Color(0xFF2F78A4),
      _ => theme.colorScheme.primary,
    };
    return theme.brightness == Brightness.dark
        ? Color.lerp(color, Colors.white, 0.28)!
        : color;
  }
}

class _VersionIndicator extends StatelessWidget {
  final int count;
  final int current;
  final ThemeData theme;

  const _VersionIndicator({
    required this.count,
    required this.current,
    required this.theme,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12, top: 2),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.chevron_left_rounded,
              size: 16, color: theme.colorScheme.onSurfaceVariant),
          const SizedBox(width: 4),
          ...List.generate(count, (i) {
            final isActive = i == current;
            return AnimatedContainer(
              duration: const Duration(milliseconds: 200),
              margin: const EdgeInsets.symmetric(horizontal: 3),
              width: isActive ? 18 : 6,
              height: 6,
              decoration: BoxDecoration(
                color: isActive
                    ? theme.colorScheme.primary
                    : theme.colorScheme.outlineVariant,
                borderRadius: BorderRadius.circular(3),
              ),
            );
          }),
          const SizedBox(width: 4),
          Icon(Icons.chevron_right_rounded,
              size: 16, color: theme.colorScheme.onSurfaceVariant),
        ],
      ),
    );
  }
}
