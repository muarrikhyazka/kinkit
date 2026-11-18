import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

import 'config.dart';

/// Thin wrapper around shared_preferences for pairing + offline cache.
class TvStore {
  TvStore(this._prefs);

  static Future<TvStore> open() async => TvStore(await SharedPreferences.getInstance());

  final SharedPreferences _prefs;

  static const _kServer = 'kinkit_tv.server';
  static const _kToken = 'kinkit_tv.token';
  static const _kCache = 'kinkit_tv.cache_json';
  static const _kCacheAt = 'kinkit_tv.cache_at';
  static const _kFx = 'kinkit_tv.fx_json';

  String get lastServer => _prefs.getString(_kServer) ?? defaultServer;

  /// Server pre-filled on first pairing: `--dart-define=KINKIT_SERVER=https://...`.
  static const String defaultServer = String.fromEnvironment('KINKIT_SERVER');

  PairingConfig? get config {
    final s = _prefs.getString(_kServer);
    final t = _prefs.getString(_kToken);
    if (s == null || s.isEmpty || t == null || t.isEmpty) return null;
    return PairingConfig(server: s, token: t);
  }

  Future<void> saveConfig(PairingConfig c) async {
    final changed = config != c;
    await _prefs.setString(_kServer, c.server);
    await _prefs.setString(_kToken, c.token);
    if (changed) await clearCache();
  }

  /// Revoked token / "forget": keep the server so re-pairing is quicker.
  Future<void> clearToken() async {
    await _prefs.remove(_kToken);
    await clearCache();
  }

  Future<void> clearCache() async {
    await _prefs.remove(_kCache);
    await _prefs.remove(_kCacheAt);
  }

  String? get cachedJson => _prefs.getString(_kCache);

  DateTime? get cachedAt {
    final ms = _prefs.getInt(_kCacheAt);
    return ms == null ? null : DateTime.fromMillisecondsSinceEpoch(ms);
  }

  Future<void> saveCache(String json, DateTime at) async {
    await _prefs.setString(_kCache, json);
    await _prefs.setInt(_kCacheAt, at.millisecondsSinceEpoch);
  }

  Map<String, double>? get fxRates {
    final raw = _prefs.getString(_kFx);
    if (raw == null) return null;
    try {
      final m = jsonDecode(raw);
      if (m is! Map) return null;
      return m.map((k, v) => MapEntry(k.toString(), (v as num).toDouble()));
    } catch (_) {
      return null;
    }
  }

  Future<void> saveFxRates(Map<String, double> rates) => _prefs.setString(_kFx, jsonEncode(rates));
}
