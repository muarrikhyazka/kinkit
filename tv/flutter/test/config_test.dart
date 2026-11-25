import 'package:flutter_test/flutter_test.dart';
import 'package:kinkit_tv/src/config.dart';

void main() {
  group('normalizeServer', () {
    test('adds https for public hosts and strips trailing slash', () {
      expect(normalizeServer('kinkit.example.com/'), 'https://kinkit.example.com');
    });
    test('adds http for LAN hosts and keeps the port', () {
      expect(normalizeServer('192.168.1.20:8080'), 'http://192.168.1.20:8080');
      expect(normalizeServer('nas.local:5000'), 'http://nas.local:5000');
    });
    test('keeps explicit scheme and sub-path, drops /tv, /api, query', () {
      expect(normalizeServer('http://host.example.com/kinkit/tv?k=abc'), 'http://host.example.com/kinkit');
      expect(normalizeServer('https://host.example.com/api/'), 'https://host.example.com');
      expect(normalizeServer('https://host.example.com/api/tv/data?k=x'), 'https://host.example.com');
    });
    test('rejects junk and non-http schemes', () {
      expect(normalizeServer(''), isNull);
      expect(normalizeServer('ftp://host.example.com'), isNull);
    });
  });

  group('tokenFromShareUrl', () {
    test('extracts k from a full share link', () {
      expect(tokenFromShareUrl('https://kinkit.example.com/tv?k=AbC-12_x'), 'AbC-12_x');
    });
    test('handles a bare query and extra params', () {
      expect(tokenFromShareUrl('?k=tok123'), 'tok123');
      expect(tokenFromShareUrl('k=tok123'), 'tok123');
      expect(tokenFromShareUrl('host.example.com/tv?x=1&k=tok%2D9'), 'tok-9');
    });
    test('returns null without k', () {
      expect(tokenFromShareUrl('https://kinkit.example.com/tv'), isNull);
      expect(tokenFromShareUrl('justatoken'), isNull);
    });
  });

  group('parsePairing', () {
    test('full link pasted into the server field', () {
      final r = parsePairing('https://kinkit.example.com/tv?k=Tok_123', '');
      expect(r.isOk, isTrue);
      expect(r.config!.server, 'https://kinkit.example.com');
      expect(r.config!.token, 'Tok_123');
    });
    test('full link pasted into the token field', () {
      final r = parsePairing('', 'http://192.168.0.5:8080/tv?k=Tok_123');
      expect(r.config, const PairingConfig(server: 'http://192.168.0.5:8080', token: 'Tok_123'));
    });
    test('separate server and token; token field wins over link token', () {
      final r = parsePairing('https://a.example.com/tv?k=old', ' newTok ');
      expect(r.config, const PairingConfig(server: 'https://a.example.com', token: 'newTok'));
    });
    test('errors', () {
      expect(parsePairing('', '').isOk, isFalse);
      expect(parsePairing('https://a.example.com', '').error, contains('token'));
      expect(parsePairing('', 'tokenOnly').error, contains('server'));
      expect(parsePairing('https://a.example.com', 'bad token!').isOk, isFalse);
      expect(parsePairing('https://a.example.com', 'bad token!').error, 'That token does not look right. Check for typos.');
    });
  });

  test('dataUri encodes the token and keeps a sub-path', () {
    const c = PairingConfig(server: 'https://h.example.com/kinkit', token: 'a+b');
    expect(c.dataUri().toString(), 'https://h.example.com/kinkit/api/tv/data?k=a%2Bb');
    expect(c.fxUri().toString(), 'https://h.example.com/kinkit/api/fx-rates');
  });

  test('maskToken never shows the full token', () {
    expect(maskToken('abcdefghijklmnop'), 'abcd...mnop');
    expect(maskToken('short'), '*****');
  });
}
