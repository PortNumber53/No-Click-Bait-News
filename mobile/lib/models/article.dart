class ArticleVersion {
  final String modelName;
  final String title;
  final String summary;
  final String? content;
  final String? rewriteId;
  final bool isOriginal;

  const ArticleVersion({
    required this.modelName,
    required this.title,
    required this.summary,
    this.content,
    this.rewriteId,
    this.isOriginal = false,
  });
}

class Article {
  final String id;
  final String title;
  final String summary;
  final String? content;
  final String? originalContent;
  final String rewriteStatus;
  final String sourceName;
  final String sourceUrl;
  final String? imageUrl;
  final String? category;
  final List<String> categories;
  final int llmRewriteVersion;
  final DateTime publishedAt;
  final bool isPremium;
  final int viewCount;
  final List<ArticleVersion> versions;

  const Article({
    required this.id,
    required this.title,
    required this.summary,
    this.content,
    this.originalContent,
    this.rewriteStatus = 'complete',
    required this.sourceName,
    required this.sourceUrl,
    this.imageUrl,
    this.category,
    this.categories = const [],
    this.llmRewriteVersion = 0,
    required this.publishedAt,
    required this.isPremium,
    required this.viewCount,
    this.versions = const [],
  });

  Article withVersions(List<ArticleVersion> newVersions) {
    return Article(
      id: id,
      title: title,
      summary: summary,
      content: content,
      originalContent: originalContent,
      rewriteStatus: rewriteStatus,
      sourceName: sourceName,
      sourceUrl: sourceUrl,
      imageUrl: imageUrl,
      category: category,
      categories: categories,
      llmRewriteVersion: llmRewriteVersion,
      publishedAt: publishedAt,
      isPremium: isPremium,
      viewCount: viewCount,
      versions: newVersions,
    );
  }

  factory Article.fromJson(Map<String, dynamic> json) {
    final title = json['title'] as String;
    final summary = json['summary'] as String;
    final content = json['content'] as String?;
    final originalContent = json['original_content'] as String?;
    final baseVersion = ArticleVersion(
      modelName: 'Original',
      title: title,
      summary: summary,
      content: originalContent ?? content,
      isOriginal: true,
    );
    final rawVersions = (json['versions'] ?? json['rewrites']) as List?;
    final versions = <ArticleVersion>[
      baseVersion,
      ...?rawVersions?.map((value) {
        final version = value as Map<String, dynamic>;
        return ArticleVersion(
          modelName: version['model_name'] as String? ?? 'Model',
          title: version['title'] as String,
          summary: version['summary'] as String,
          content: version['content'] as String?,
          rewriteId: (version['id'] ?? version['rewrite_id']) as String?,
        );
      }),
    ];

    return Article(
      id: json['id'] as String,
      title: title,
      summary: summary,
      content: content,
      originalContent: originalContent,
      rewriteStatus: json['rewrite_status'] as String? ?? 'complete',
      sourceName: json['source_name'] as String,
      sourceUrl: json['source_url'] as String,
      imageUrl: json['image_url'] as String?,
      category: json['category'] as String?,
      categories: (json['categories'] as List<dynamic>?)
              ?.map((value) => value.toString())
              .toList() ??
          const [],
      llmRewriteVersion: json['llm_rewrite_version'] as int? ?? 0,
      publishedAt: DateTime.parse(json['published_at'] as String),
      isPremium: json['is_premium'] as bool? ?? false,
      viewCount: json['view_count'] as int? ?? 0,
      versions: versions,
    );
  }
}

class ArticleFeed {
  final List<Article> articles;
  final int page;
  final int pageSize;
  final bool hasMore;

  const ArticleFeed({
    required this.articles,
    required this.page,
    required this.pageSize,
    required this.hasMore,
  });

  factory ArticleFeed.fromJson(Map<String, dynamic> json) {
    return ArticleFeed(
      articles: (json['articles'] as List)
          .map((value) => Article.fromJson(value as Map<String, dynamic>))
          .toList(),
      page: json['page'] as int,
      pageSize: json['page_size'] as int,
      hasMore: json['has_more'] as bool,
    );
  }
}
