import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('bundles the NoClickBait News app-bar logo', () async {
    final logo = await rootBundle.load(
      'assets/branding/noclickbait-news-logo.png',
    );

    expect(logo.lengthInBytes, greaterThan(0));
  });
}
