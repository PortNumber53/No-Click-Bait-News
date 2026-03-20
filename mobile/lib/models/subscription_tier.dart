class SubscriptionTier {
  final int id;
  final String name;
  final double priceMonthly;
  final int maxArticlesPerDay;
  final bool hasPremiumAccess;

  const SubscriptionTier({
    required this.id,
    required this.name,
    required this.priceMonthly,
    required this.maxArticlesPerDay,
    required this.hasPremiumAccess,
  });

  factory SubscriptionTier.fromJson(Map<String, dynamic> json) {
    return SubscriptionTier(
      id: json['id'],
      name: json['name'],
      priceMonthly: (json['price_monthly'] as num).toDouble(),
      maxArticlesPerDay: json['max_articles_per_day'],
      hasPremiumAccess: json['has_premium_access'] ?? false,
    );
  }
}
