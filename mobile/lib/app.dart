import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'screens/home_shell.dart';
import 'screens/login_screen.dart';
import 'state/providers.dart';
import 'theme.dart';
import 'widgets/kinkit_logo.dart';

final _navigatorKey = GlobalKey<NavigatorState>();

class KinkitApp extends ConsumerWidget {
  const KinkitApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // On logout / expired session / server switch, close any pushed screens,
    // sheets and dialogs so the login screen is what's visible.
    ref.listen(meProvider, (prev, next) {
      if (prev != null && next == null) {
        _navigatorKey.currentState?.popUntil((route) => route.isFirst);
      }
    });
    return MaterialApp(
      navigatorKey: _navigatorKey,
      title: 'Kinkit',
      debugShowCheckedModeBanner: false,
      theme: buildTheme(Brightness.light),
      darkTheme: buildTheme(Brightness.dark),
      themeMode: ref.watch(themeModeProvider),
      home: const AuthGate(),
    );
  }
}

/// Shows the login screen or the app depending on the session.
class AuthGate extends ConsumerWidget {
  const AuthGate({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authProvider);
    // Keep the current screen while re-checking (e.g. after login).
    if (auth.isLoading && !auth.hasValue && !auth.hasError) {
      return const _Splash();
    }
    // Errors win over stale data (e.g. after switching to an unreachable server).
    final me = auth.hasError ? null : auth.valueOrNull;
    if (me == null) {
      return LoginScreen(startupError: auth.hasError ? auth.error.toString() : null);
    }
    return HomeShell(key: ValueKey('shell-${me.accountId}-${me.memberId}'));
  }
}

class _Splash extends StatelessWidget {
  const _Splash();

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    return Scaffold(
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const KinkitLogo(size: 64),
            const SizedBox(height: 12),
            Text('kinkit', style: TextStyle(fontSize: 28, fontWeight: FontWeight.w800, color: p.text)),
            const SizedBox(height: 24),
            const CircularProgressIndicator(),
          ],
        ),
      ),
    );
  }
}
