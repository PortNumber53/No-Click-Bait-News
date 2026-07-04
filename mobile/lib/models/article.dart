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
  });

  factory Article.fromJson(Map<String, dynamic> json) {
    return Article(
      id: json['id'],
      title: json['title'],
      summary: json['summary'],
      content: json['content'],
      originalContent: json['original_content'],
      rewriteStatus: json['rewrite_status'] ?? 'complete',
      sourceName: json['source_name'],
      sourceUrl: json['source_url'],
      imageUrl: json['image_url'],
      category: json['category'],
      categories: (json['categories'] as List<dynamic>?)
              ?.map((category) => category.toString())
              .toList() ??
          const [],
      llmRewriteVersion: json['llm_rewrite_version'] ?? 0,
      publishedAt: DateTime.parse(json['published_at']),
      isPremium: json['is_premium'] ?? false,
      viewCount: json['view_count'] ?? 0,
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
          .map((a) => Article.fromJson(a))
          .toList(),
      page: json['page'],
      pageSize: json['page_size'],
      hasMore: json['has_more'],
    );
  }
}
