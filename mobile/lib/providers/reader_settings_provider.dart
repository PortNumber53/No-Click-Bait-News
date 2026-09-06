import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class ReaderSettingsProvider extends ChangeNotifier {
  ReaderSettingsProvider({
    FlutterSecureStorage? storage,
    bool persistChanges = true,
    bool enableHardwareControls = true,
  })  : _storage = storage ?? const FlutterSecureStorage(),
        _persistChanges = persistChanges,
        _enableHardwareControls = enableHardwareControls {
    if (_persistChanges) {
      unawaited(_restore());
    }
    if (_enableHardwareControls) {
      _readerChannel.setMethodCallHandler(_handleReaderControl);
      unawaited(_setVolumeKeyControl(true));
    }
  }

  static const double minimumScale = 0.8;
  static const double maximumScale = 1.5;
  static const double scaleStep = 0.1;
  static const String _storageKey = 'reader_font_scale';
  static const MethodChannel _readerChannel =
      MethodChannel('co.truvis.ncbnews/reader_controls');

  final FlutterSecureStorage _storage;
  final bool _persistChanges;
  final bool _enableHardwareControls;
  Timer? _saveTimer;
  double _fontScale = 1;
  int _hardwareChangeSerial = 0;

  double get fontScale => _fontScale;
  int get fontScalePercent => (_fontScale * 100).round();
  bool get canDecrease => _fontScale > minimumScale;
  bool get canIncrease => _fontScale < maximumScale;
  int get hardwareChangeSerial => _hardwareChangeSerial;

  void increaseFontSize() => setFontScale(_fontScale + scaleStep);

  void decreaseFontSize() => setFontScale(_fontScale - scaleStep);

  void resetFontSize() => setFontScale(1);

  void setFontScale(double value) {
    final next = _roundToStep(value.clamp(minimumScale, maximumScale));
    if (next == _fontScale) return;

    _fontScale = next;
    notifyListeners();
    if (_persistChanges) {
      _saveTimer?.cancel();
      _saveTimer = Timer(const Duration(milliseconds: 250), () {
        unawaited(_save());
      });
    }
  }

  @override
  void dispose() {
    _saveTimer?.cancel();
    if (_persistChanges) {
      unawaited(_save());
    }
    if (_enableHardwareControls) {
      unawaited(_setVolumeKeyControl(false));
      _readerChannel.setMethodCallHandler(null);
    }
    super.dispose();
  }

  Future<void> _handleReaderControl(MethodCall call) async {
    if (call.method != 'fontSizeDelta') return;
    final delta = call.arguments as int? ?? 0;
    final previousScale = _fontScale;
    _hardwareChangeSerial++;
    if (delta > 0) {
      increaseFontSize();
    } else if (delta < 0) {
      decreaseFontSize();
    } else {
      return;
    }
    if (_fontScale == previousScale) {
      notifyListeners();
    }
  }

  Future<void> _setVolumeKeyControl(bool enabled) async {
    try {
      await _readerChannel.invokeMethod<void>(
        'setVolumeKeyFontControl',
        enabled,
      );
    } on MissingPluginException {
      // Physical volume-key control is an Android enhancement.
    } on PlatformException {
      // Keep the on-screen controls available if the native bridge fails.
    }
  }

  Future<void> _restore() async {
    try {
      final stored =
          double.tryParse(await _storage.read(key: _storageKey) ?? '');
      if (stored == null) return;
      final next = _roundToStep(stored.clamp(minimumScale, maximumScale));
      if (next == _fontScale) return;
      _fontScale = next;
      notifyListeners();
    } catch (_) {
      // Reader preferences are optional; use the default if storage is unavailable.
    }
  }

  Future<void> _save() async {
    try {
      await _storage.write(key: _storageKey, value: _fontScale.toString());
    } catch (_) {
      // Keep the in-memory preference even if the device cannot persist it.
    }
  }

  double _roundToStep(double value) =>
      (value / scaleStep).roundToDouble() * scaleStep;
}
