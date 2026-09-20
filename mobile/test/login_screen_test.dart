import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:no_click_bait_news/providers/auth_provider.dart';
import 'package:no_click_bait_news/screens/login_screen.dart';
import 'package:provider/provider.dart';

void main() {
  testWidgets('remember me is an explicit opt-in', (tester) async {
    await tester.pumpWidget(
      ChangeNotifierProvider(
        create: (_) => AuthProvider(),
        child: const MaterialApp(home: LoginScreen()),
      ),
    );

    expect(find.text('Remember me'), findsOneWidget);
    expect(find.text('Keep this account signed in on this device'),
        findsOneWidget);
    expect(tester.widget<Checkbox>(find.byType(Checkbox)).value, isFalse);

    await tester.tap(find.text('Remember me'));
    await tester.pump();

    expect(tester.widget<Checkbox>(find.byType(Checkbox)).value, isTrue);
  });
}
