import 'package:flutter_test/flutter_test.dart';
import 'package:no_click_bait_news/models/subscription_tier.dart';

void main() {
  test('subscription tier parses unlimited reading entitlement', () {
    final tier = SubscriptionTier.fromJson({
      'id': 2,
      'name': 'premium',
      'price_monthly': 14,
      'max_articles_per_day': 0,
      'max_articles_per_month': 0,
      'has_premium_access': true,
      'unlimited_reading': true,
      'is_current': true,
    });

    expect(tier.priceMonthly, 14);
    expect(tier.unlimitedReading, isTrue);
    expect(tier.maxArticlesPerMonth, 0);
    expect(tier.isCurrent, isTrue);
  });

  test('subscription tier parses monthly reading allowance', () {
    final tier = SubscriptionTier.fromJson({
      'id': 3,
      'name': 'standard',
      'price_monthly': 9.99,
      'max_articles_per_day': 0,
      'max_articles_per_month': 60,
      'has_premium_access': false,
      'unlimited_reading': false,
      'is_current': false,
    });

    expect(tier.priceMonthly, 9.99);
    expect(tier.maxArticlesPerMonth, 60);
    expect(tier.unlimitedReading, isFalse);
  });
}
