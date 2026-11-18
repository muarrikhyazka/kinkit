import 'package:cookie_jar/cookie_jar.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kinkit_mobile/app.dart';
import 'package:kinkit_mobile/state/providers.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  testWidgets('app boots to the login screen when there is no session', (tester) async {
    SharedPreferences.setMockInitialValues({'kinkit.baseUrl': 'http://localhost:9'});
    final prefs = await SharedPreferences.getInstance();
    await tester.pumpWidget(ProviderScope(
      overrides: [
        sharedPrefsProvider.overrideWithValue(prefs),
        cookieJarProvider.overrideWithValue(CookieJar()),
      ],
      child: const KinkitApp(),
    ));
    // The /api/me call fails in the test environment (no network), so the
    // AuthGate settles on the login screen.
    await tester.runAsync(() => Future<void>.delayed(const Duration(milliseconds: 500)));
    await tester.pumpAndSettle();
    expect(find.text('kinkit'), findsOneWidget);
    expect(find.text('Log in'), findsWidgets);
    expect(find.text('Server: http://localhost:9'), findsOneWidget);
    // The unreachable server opens the server field automatically.
    expect(find.text('Save & test connection'), findsOneWidget);
  });
}
