/// Pairing configuration (server URL + TV token) and the pure parsing logic
/// used by the pairing screen. No Flutter imports so it is unit-testable.
class PairingConfig {
  const PairingConfig({required this.server, required this.token});

  /// Normalised base URL without trailing slash, e.g. `https://kinkit.example.com`
  /// or `http://192.168.1.10:8080/kinkit`.
  final String server;

  /// Share token from `/tv?k=<token>` (created under Settings > Data & devices).
  final String token;

  /// GET <server>/api/tv/data?k=<token> - the only account-data endpoint used.
  Uri dataUri() {
    final base = Uri.parse(server);
    return base.replace(path: '${base.path}/api/tv/data', queryParameters: {'k': token});
  }

  /// GET <server>/api/fx-rates - public (no session, no account data); only used
  /// to convert IDR amounts when the account's display currency is not IDR.
  Uri fxUri() {
    final base = Uri.parse(server);
    return base.replace(path: '${base.path}/api/fx-rates');
  }

  @override
  bool operator ==(Object other) =>
      other is PairingConfig && other.server == server && other.token == token;

  @override
  int get hashCode => Object.hash(server, token);

  @override
  String toString() => 'PairingConfig($server, token=${maskToken(token)})';
}

class PairingParseResult {
  const PairingParseResult._(this.config, this.error);
  const PairingParseResult.ok(PairingConfig c) : this._(c, null);
  const PairingParseResult.fail(String e) : this._(null, e);

  final PairingConfig? config;
  final String? error;
  bool get isOk => config != null;
}

/// `abcdef...wxyz` style mask so the settings screen never shows a full token.
String maskToken(String token) {
  if (token.length <= 8) return '*' * token.length;
  return '${token.substring(0, 4)}...${token.substring(token.length - 4)}';
}

final _ipv4 = RegExp(r'^\d{1,3}(\.\d{1,3}){3}$');

bool _looksLocalHost(String host) {
  final h = host.toLowerCase();
  return h == 'localhost' ||
      _ipv4.hasMatch(h) ||
      h.endsWith('.local') ||
      h.endsWith('.lan') ||
      h.endsWith('.home') ||
      !h.contains('.');
}

/// Parses "anything a user may type" into a Uri with a scheme, or null.
/// Without a scheme, LAN-looking hosts get http://, everything else https://.
Uri? _parseLoose(String input) {
  var s = input.trim();
  if (s.isEmpty) return null;
  if (!s.contains('://')) {
    final probe = Uri.tryParse('http://$s');
    if (probe == null || probe.host.isEmpty) return null;
    s = '${_looksLocalHost(probe.host) ? 'http' : 'https'}://$s';
  }
  final uri = Uri.tryParse(s);
  if (uri == null || uri.host.isEmpty) return null;
  if (uri.scheme != 'http' && uri.scheme != 'https') return null;
  return uri;
}

/// Returns the `k` query parameter if [input] is (or contains) a share URL
/// such as `https://host/tv?k=TOKEN`, otherwise null.
String? tokenFromShareUrl(String input) {
  final s = input.trim();
  if (!s.contains('k=')) return null;
  // Full URL first, then treat whatever follows '?' (or the whole string) as a query.
  final q = s.contains('?') ? s.substring(s.indexOf('?')) : '?$s';
  for (final uri in <Uri?>[_parseLoose(s), Uri.tryParse('http://x/$q')]) {
    try {
      final k = uri?.queryParameters['k']?.trim();
      if (k != null && k.isNotEmpty) return k;
    } on FormatException {
      // malformed %-escapes: try the next interpretation
    }
  }
  return null;
}

/// Normalises a server address: adds a scheme, drops query/fragment,
/// trailing slashes and a trailing `/tv`, `/api` or `/api/tv/data`.
/// Returns null when it is not a usable http(s) URL.
String? normalizeServer(String input) {
  final uri = _parseLoose(input);
  if (uri == null) return null;
  var path = uri.path;
  var changed = true;
  while (changed) {
    changed = false;
    while (path.endsWith('/')) {
      path = path.substring(0, path.length - 1);
      changed = true;
    }
    for (final suffix in const ['/api/tv/data', '/api', '/tv']) {
      if (path.toLowerCase().endsWith(suffix)) {
        path = path.substring(0, path.length - suffix.length);
        changed = true;
      }
    }
  }
  final out = Uri(
    scheme: uri.scheme,
    host: uri.host,
    port: uri.hasPort ? uri.port : null,
    path: path,
  );
  return out.toString();
}

String _decode(String s) {
  try {
    return Uri.decodeQueryComponent(s);
  } catch (_) {
    return s;
  }
}

final _tokenChars = RegExp(r'^[A-Za-z0-9_\-.~]+$');

/// Combines the two pairing fields. Either field may contain the full share
/// link (`https://host/tv?k=TOKEN`); a separate non-empty token field wins.
PairingParseResult parsePairing(String serverInput, String tokenInput) {
  final serverRaw = serverInput.trim();
  final tokenRaw = tokenInput.trim();

  String? token;
  if (tokenRaw.isNotEmpty) {
    token = tokenFromShareUrl(tokenRaw) ?? _decode(tokenRaw);
  } else {
    token = tokenFromShareUrl(serverRaw);
  }

  String? server;
  if (serverRaw.isNotEmpty) {
    server = normalizeServer(serverRaw);
    if (server == null) return const PairingParseResult.fail('Server address is not a valid http(s) URL.');
  } else if (tokenRaw.contains('://') || tokenRaw.contains('/tv?')) {
    server = normalizeServer(tokenRaw);
  }

  if (server == null || server.isEmpty) {
    return const PairingParseResult.fail('Enter the server address (or paste the full TV link).');
  }
  if (token == null || token.isEmpty) {
    return const PairingParseResult.fail('Enter the TV token (or paste the full TV link).');
  }
  if (token.length > 200 || !_tokenChars.hasMatch(token)) {
    return const PairingParseResult.fail('That token does not look right - check for typos.');
  }
  return PairingParseResult.ok(PairingConfig(server: server, token: token));
}
