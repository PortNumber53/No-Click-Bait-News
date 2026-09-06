import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

class MarkdownContent extends StatelessWidget {
  final String markdown;
  final TextStyle? baseStyle;

  const MarkdownContent({
    super.key,
    required this.markdown,
    this.baseStyle,
  });

  @override
  Widget build(BuildContext context) {
    final blocks = _parseBlocks(markdown);
    final style = baseStyle ?? Theme.of(context).textTheme.bodyMedium!;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (final block in blocks)
          _MarkdownBlockView(block: block, style: style),
      ],
    );
  }
}

class _MarkdownBlock {
  final String type;
  final int level;
  final String text;
  final List<String> items;

  const _MarkdownBlock.heading(this.level, this.text)
      : type = 'heading',
        items = const [];
  const _MarkdownBlock.paragraph(this.text)
      : type = 'paragraph',
        level = 0,
        items = const [];
  const _MarkdownBlock.list(this.items)
      : type = 'list',
        level = 0,
        text = '';
  const _MarkdownBlock.quote(this.text)
      : type = 'quote',
        level = 0,
        items = const [];
}

List<_MarkdownBlock> _parseBlocks(String markdown) {
  final lines = markdown.replaceAll('\r\n', '\n').split('\n');
  final blocks = <_MarkdownBlock>[];
  var i = 0;

  while (i < lines.length) {
    final line = lines[i].trim();
    if (line.isEmpty) {
      i++;
      continue;
    }

    final heading = RegExp(r'^(#{1,6})\s+(.+)$').firstMatch(line);
    if (heading != null) {
      blocks.add(_MarkdownBlock.heading(
        heading.group(1)!.length,
        heading.group(2)!.trim(),
      ));
      i++;
      continue;
    }

    if (RegExp(r'^[-*+]\s+').hasMatch(line)) {
      final items = <String>[];
      while (
          i < lines.length && RegExp(r'^[-*+]\s+').hasMatch(lines[i].trim())) {
        items.add(lines[i].trim().replaceFirst(RegExp(r'^[-*+]\s+'), ''));
        i++;
      }
      blocks.add(_MarkdownBlock.list(items));
      continue;
    }

    if (line.startsWith('>')) {
      final quotes = <String>[];
      while (i < lines.length && lines[i].trim().startsWith('>')) {
        quotes.add(lines[i].trim().replaceFirst(RegExp(r'^>\s?'), ''));
        i++;
      }
      blocks.add(_MarkdownBlock.quote(quotes.join(' ')));
      continue;
    }

    final paragraph = <String>[];
    while (i < lines.length &&
        lines[i].trim().isNotEmpty &&
        !_isBlockStart(lines[i].trim())) {
      paragraph.add(lines[i].trim());
      i++;
    }
    blocks.add(_MarkdownBlock.paragraph(paragraph.join(' ')));
  }

  return blocks;
}

bool _isBlockStart(String line) {
  return RegExp(r'^(#{1,6})\s+').hasMatch(line) ||
      RegExp(r'^[-*+]\s+').hasMatch(line) ||
      line.startsWith('>');
}

class _MarkdownBlockView extends StatelessWidget {
  final _MarkdownBlock block;
  final TextStyle style;

  const _MarkdownBlockView({required this.block, required this.style});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    switch (block.type) {
      case 'heading':
        final size = (style.fontSize ?? 16) * (block.level <= 2 ? 1.25 : 1.1);
        return Padding(
          padding: const EdgeInsets.only(top: 12, bottom: 6),
          child: Text(
            _stripInline(block.text),
            style: style.copyWith(
              fontSize: size,
              fontWeight: FontWeight.w700,
              color: theme.colorScheme.onSurface,
            ),
          ),
        );
      case 'list':
        return Padding(
          padding: const EdgeInsets.only(bottom: 12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              for (final item in block.items)
                Padding(
                  padding: const EdgeInsets.only(bottom: 6),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('• ', style: style),
                      Expanded(
                        child: _InlineMarkdown(text: item, style: style),
                      ),
                    ],
                  ),
                ),
            ],
          ),
        );
      case 'quote':
        return Container(
          margin: const EdgeInsets.only(bottom: 12),
          padding: const EdgeInsets.fromLTRB(14, 10, 12, 10),
          decoration: BoxDecoration(
            color: theme.colorScheme.primaryContainer.withValues(alpha: 0.45),
            borderRadius: BorderRadius.circular(12),
            border: Border(
              left: BorderSide(
                color: theme.colorScheme.primary,
                width: 4,
              ),
            ),
          ),
          child: _InlineMarkdown(text: block.text, style: style),
        );
      default:
        return Padding(
          padding: const EdgeInsets.only(bottom: 12),
          child: _InlineMarkdown(text: block.text, style: style),
        );
    }
  }
}

class _InlineMarkdown extends StatelessWidget {
  final String text;
  final TextStyle style;

  const _InlineMarkdown({required this.text, required this.style});

  @override
  Widget build(BuildContext context) {
    final spans = <InlineSpan>[];
    final regex = RegExp(
      r'\[([^\]]+)\]\(([^)]+)\)|\*\*([^*]+)\*\*|__([^_]+)__|`([^`]+)`',
    );
    var last = 0;

    for (final match in regex.allMatches(text)) {
      if (match.start > last) {
        spans.add(TextSpan(text: text.substring(last, match.start)));
      }
      if (match.group(1) != null) {
        final label = match.group(1)!;
        final href = match.group(2)!;
        spans.add(WidgetSpan(
          alignment: PlaceholderAlignment.baseline,
          baseline: TextBaseline.alphabetic,
          child: GestureDetector(
            onTap: () async {
              final uri = Uri.tryParse(href);
              if (uri != null && await canLaunchUrl(uri)) {
                await launchUrl(uri, mode: LaunchMode.externalApplication);
              }
            },
            child: Text(
              label,
              style: style.copyWith(
                color: Theme.of(context).colorScheme.primary,
                decoration: TextDecoration.underline,
              ),
            ),
          ),
        ));
      } else if (match.group(3) != null || match.group(4) != null) {
        spans.add(TextSpan(
          text: match.group(3) ?? match.group(4),
          style: style.copyWith(fontWeight: FontWeight.w700),
        ));
      } else if (match.group(5) != null) {
        spans.add(TextSpan(
          text: match.group(5),
          style: style.copyWith(fontFamily: 'monospace'),
        ));
      }
      last = match.end;
    }

    if (last < text.length) {
      spans.add(TextSpan(text: text.substring(last)));
    }

    return Text.rich(TextSpan(style: style, children: spans));
  }
}

String _stripInline(String value) {
  return value
      .replaceAllMapped(
        RegExp(r'\*\*([^*]+)\*\*'),
        (match) => match.group(1)!,
      )
      .replaceAllMapped(RegExp(r'__([^_]+)__'), (match) => match.group(1)!)
      .replaceAllMapped(RegExp(r'`([^`]+)`'), (match) => match.group(1)!);
}
