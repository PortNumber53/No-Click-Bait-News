class SubscriptionTier {
  final int id;
  final String name;
  final double priceMonthly;
  final int maxArticlesPerDay;
  final int maxArticlesPerMonth;
  final bool hasPremiumAccess;
  final bool unlimitedReading;
  final bool isCurrent;

  const SubscriptionTier({
    required this.id,
    required this.name,
    required this.priceMonthly,
    required this.maxArticlesPerDay,
    required this.maxArticlesPerMonth,
    required this.hasPremiumAccess,
    required this.unlimitedReading,
    required this.isCurrent,
  });

  factory SubscriptionTier.fromJson(Map<String, dynamic> json) {
    return SubscriptionTier(
      id: json['id'] as int,
      name: json['name'] as String,
      priceMonthly: (json['price_monthly'] as num).toDouble(),
      maxArticlesPerDay: json['max_articles_per_day'] as int,
      maxArticlesPerMonth: json['max_articles_per_month'] as int? ?? 0,
      hasPremiumAccess: json['has_premium_access'] as bool? ?? false,
      unlimitedReading: json['unlimited_reading'] as bool? ?? false,
      isCurrent: json['is_current'] as bool? ?? false,
    );
  }
}
