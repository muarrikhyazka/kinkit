import 'package:cookie_jar/cookie_jar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../api/api_client.dart';
import '../api/models.dart';
import '../config.dart';
import '../util/dates.dart';
import '../util/money.dart';

// Core app state (Riverpod). Feature data lives in data_providers.dart.

/// Overridden in main() once SharedPreferences is loaded.
final sharedPrefsProvider = Provider<SharedPreferences>((ref) {
  throw UnimplementedError('sharedPrefsProvider must be overridden in main()');
});

/// Overridden in main() with a PersistCookieJar in the app support dir.
final cookieJarProvider = Provider<CookieJar>((ref) => CookieJar());

const _kBaseUrl = 'kinkit.baseUrl';
const _kThemeMode = 'kinkit.themeMode';

// server URL

class BaseUrlNotifier extends Notifier<String> {
  @override
  String build() {
    final saved = ref.watch(sharedPrefsProvider).getString(_kBaseUrl);
    return AppConfig.normalizeBaseUrl(saved ?? AppConfig.defaultBaseUrl);
  }

  Future<void> set(String url) async {
    final v = AppConfig.normalizeBaseUrl(url);
    await ref.read(sharedPrefsProvider).setString(_kBaseUrl, v);
    state = v;
  }
}

final baseUrlProvider = NotifierProvider<BaseUrlNotifier, String>(BaseUrlNotifier.new);

// theme

class ThemeModeNotifier extends Notifier<ThemeMode> {
  @override
  ThemeMode build() {
    final v = ref.watch(sharedPrefsProvider).getString(_kThemeMode);
    return ThemeMode.values.firstWhere((m) => m.name == v, orElse: () => ThemeMode.system);
  }

  Future<void> set(ThemeMode mode) async {
    await ref.read(sharedPrefsProvider).setString(_kThemeMode, mode.name);
    state = mode;
  }
}

final themeModeProvider = NotifierProvider<ThemeModeNotifier, ThemeMode>(ThemeModeNotifier.new);

// API client

final apiProvider = Provider<KinkitApi>((ref) {
  final api = KinkitApi(baseUrl: ref.watch(baseUrlProvider), cookieJar: ref.watch(cookieJarProvider));
  // Read lazily (at call time) so there is no provider dependency cycle.
  api.onUnauthorized = () => ref.read(authProvider.notifier).handleUnauthorized();
  return api;
});

// auth

/// The signed-in member, or null when logged out.
class AuthNotifier extends AsyncNotifier<Me?> {
  @override
  Future<Me?> build() async {
    final api = ref.watch(apiProvider);
    final me = await api.me();
    return me.authenticated ? me : null;
  }

  Future<void> login(String username, String password) async {
    final api = ref.read(apiProvider);
    await api.login(username, password);
    await _reload(api);
  }

  Future<void> signup(String email, String password, String displayName) async {
    final api = ref.read(apiProvider);
    await api.signup(email, password, displayName);
    await _reload(api);
  }

  Future<void> logout() async {
    try {
      await ref.read(apiProvider).logout();
    } catch (_) {
      // Logging out locally is what matters; cookies are already cleared.
    }
    state = const AsyncData(null);
  }

  Future<void> _reload(KinkitApi api) async {
    final me = await api.me();
    if (!me.authenticated) {
      throw const ApiException('Signed in, but the session cookie was not kept. '
          'If the server uses COOKIE_SECURE=true, connect over https.');
    }
    state = AsyncData(me);
  }

  void handleUnauthorized() {
    if (state.valueOrNull != null) state = const AsyncData(null);
  }
}

final authProvider = AsyncNotifierProvider<AuthNotifier, Me?>(AuthNotifier.new);

/// Current member (null when logged out / loading).
final meProvider = Provider<Me?>((ref) {
  final auth = ref.watch(authProvider);
  return auth.hasError ? null : auth.valueOrNull;
});

// money formatting

final fxRatesProvider = FutureProvider<FxRates>((ref) async {
  try {
    return await ref.watch(apiProvider).fxRates();
  } catch (_) {
    return const FxRates();
  }
});

/// Formats IDR amounts in the household currency (from /api/me).
final moneyProvider = Provider<MoneyFormatter>((ref) {
  final currency = ref.watch(meProvider.select((m) => m?.currency)) ?? 'IDR';
  final rates = ref.watch(fxRatesProvider).valueOrNull ?? const FxRates();
  return MoneyFormatter(currency: currency, rates: rates.rates);
});

// selected month

/// Month shown by Dashboard / Spending / Income / Budget.
final selectedMonthProvider = StateProvider<YearMonth>((ref) => YearMonth.now());
