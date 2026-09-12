import 'package:flutter_test/flutter_test.dart';
import 'package:no_click_bait_news/utils/time_ago.dart';

void main() {
  final now = DateTime.utc(2026, 9, 12, 21);

  test('future publication dates display as just now', () {
    expect(
      formatTimeAgo(now.add(const Duration(days: 2)), now: now),
      'Just now',
    );
  });

  test('past publication dates retain compact elapsed time', () {
    expect(
      formatTimeAgo(now.subtract(const Duration(minutes: 12)), now: now),
      '12m ago',
    );
  });
}
