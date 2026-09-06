import 'package:flutter_test/flutter_test.dart';
import 'package:no_click_bait_news/providers/reader_settings_provider.dart';

void main() {
  group('ReaderSettingsProvider', () {
    late ReaderSettingsProvider settings;

    setUp(() {
      settings = ReaderSettingsProvider(
        persistChanges: false,
        enableHardwareControls: false,
      );
    });

    test('starts at the default text size', () {
      expect(settings.fontScale, 1);
      expect(settings.fontScalePercent, 100);
    });

    test('changes text size in ten percent steps', () {
      settings.increaseFontSize();
      expect(settings.fontScalePercent, 110);

      settings.decreaseFontSize();
      expect(settings.fontScalePercent, 100);
    });

    test('clamps text size to the supported range', () {
      for (var i = 0; i < 20; i++) {
        settings.increaseFontSize();
      }
      expect(settings.fontScale, ReaderSettingsProvider.maximumScale);
      expect(settings.canIncrease, isFalse);

      for (var i = 0; i < 20; i++) {
        settings.decreaseFontSize();
      }
      expect(settings.fontScale, ReaderSettingsProvider.minimumScale);
      expect(settings.canDecrease, isFalse);
    });

    test('rounds slider values and can reset the preference', () {
      settings.setFontScale(1.24);
      expect(settings.fontScalePercent, 120);

      settings.resetFontSize();
      expect(settings.fontScale, 1);
    });
  });
}
