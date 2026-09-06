import 'dart:async';

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../providers/reader_settings_provider.dart';

class ReaderFontFeedbackOverlay extends StatefulWidget {
  final Widget child;

  const ReaderFontFeedbackOverlay({
    super.key,
    required this.child,
  });

  @override
  State<ReaderFontFeedbackOverlay> createState() =>
      _ReaderFontFeedbackOverlayState();
}

class _ReaderFontFeedbackOverlayState extends State<ReaderFontFeedbackOverlay> {
  ReaderSettingsProvider? _settings;
  Timer? _hideTimer;
  int _lastHardwareChange = 0;
  bool _isVisible = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final settings = context.read<ReaderSettingsProvider>();
    if (identical(settings, _settings)) return;
    _settings?.removeListener(_onSettingsChanged);
    _settings = settings;
    _lastHardwareChange = settings.hardwareChangeSerial;
    settings.addListener(_onSettingsChanged);
  }

  void _onSettingsChanged() {
    final settings = _settings;
    if (settings == null ||
        settings.hardwareChangeSerial == _lastHardwareChange) {
      return;
    }
    _lastHardwareChange = settings.hardwareChangeSerial;
    _hideTimer?.cancel();
    if (mounted) setState(() => _isVisible = true);
    _hideTimer = Timer(const Duration(milliseconds: 1100), () {
      if (mounted) setState(() => _isVisible = false);
    });
  }

  @override
  void dispose() {
    _hideTimer?.cancel();
    _settings?.removeListener(_onSettingsChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final settings = context.watch<ReaderSettingsProvider>();
    final scheme = Theme.of(context).colorScheme;

    return Stack(
      children: [
        widget.child,
        Positioned(
          left: 0,
          right: 0,
          bottom: 28,
          child: SafeArea(
            child: IgnorePointer(
              child: AnimatedOpacity(
                opacity: _isVisible ? 1 : 0,
                duration: const Duration(milliseconds: 180),
                child: Center(
                  child: Material(
                    color: scheme.inverseSurface,
                    borderRadius: BorderRadius.circular(22),
                    elevation: 8,
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 18,
                        vertical: 11,
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            Icons.text_fields_rounded,
                            size: 20,
                            color: scheme.onInverseSurface,
                          ),
                          const SizedBox(width: 8),
                          Text(
                            'Text size  ${settings.fontScalePercent}%',
                            style: TextStyle(
                              color: scheme.onInverseSurface,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}
