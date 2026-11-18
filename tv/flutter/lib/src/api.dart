import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;

import 'config.dart';
import 'models.dart';

enum TvErrorKind { unauthorized, offline, server, badData }

class TvApiException implements Exception {
  const TvApiException(this.kind, this.message);
  final TvErrorKind kind;
  final String message;

  @override
  String toString() => 'TvApiException($kind, $message)';
}

/// Read-only client. Only ever issues GET requests:
///  - /api/tv/data?k=<token>  (account data, token-authenticated)
///  - /api/fx-rates           (public currency table, optional)
class TvApi {
  TvApi({http.Client? client}) : _client = client ?? http.Client();

  final http.Client _client;
  static const _timeout = Duration(seconds: 15);

  /// Returns the parsed payload plus the raw JSON string (cached for offline use).
  Future<(TvData, String)> fetchData(PairingConfig config) async {
    http.Response res;
    try {
      res = await _client.get(config.dataUri(), headers: const {'Accept': 'application/json'}).timeout(_timeout);
    } on TimeoutException {
      throw const TvApiException(TvErrorKind.offline, 'The server did not answer in time.');
    } catch (e) {
      throw TvApiException(TvErrorKind.offline, 'Cannot reach the server ($e).');
    }
    if (res.statusCode == 401 || res.statusCode == 403) {
      throw const TvApiException(TvErrorKind.unauthorized, 'This TV link is invalid or was revoked.');
    }
    if (res.statusCode != 200) {
      // 5xx / proxies / maintenance pages: treat like offline (keep showing last data).
      throw TvApiException(TvErrorKind.server, 'Server error (HTTP ${res.statusCode}).');
    }
    final body = utf8.decode(res.bodyBytes);
    return (parseTvData(body), body);
  }

  /// Best effort; null on any failure (amounts are then shown in IDR).
  Future<Map<String, double>?> fetchFxRates(PairingConfig config) async {
    try {
      final res = await _client.get(config.fxUri(), headers: const {'Accept': 'application/json'}).timeout(_timeout);
      if (res.statusCode != 200) return null;
      return _parseRates(utf8.decode(res.bodyBytes));
    } catch (_) {
      return null;
    }
  }

  void close() => _client.close();
}

/// Parses the /api/tv/data body; throws [TvApiException] (badData) on garbage.
TvData parseTvData(String body) {
  Object? json;
  try {
    json = jsonDecode(body);
  } on FormatException {
    throw const TvApiException(TvErrorKind.badData, 'The server sent something that is not JSON.');
  }
  if (json is! Map) {
    throw const TvApiException(TvErrorKind.badData, 'Unexpected response from the server.');
  }
  return TvData.fromJson(json.cast<String, dynamic>());
}

Map<String, double>? _parseRates(String body) {
  try {
    final json = jsonDecode(body);
    if (json is! Map) return null;
    final rates = json['rates'];
    if (rates is! Map) return null;
    final out = <String, double>{};
    rates.forEach((k, v) {
      if (k is String && v is num) out[k.toUpperCase()] = v.toDouble();
    });
    return out.isEmpty ? null : out;
  } catch (_) {
    return null;
  }
}
