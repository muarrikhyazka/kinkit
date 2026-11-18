import 'package:flutter_test/flutter_test.dart';
import 'package:kinkit_mobile/config.dart';
import 'package:kinkit_mobile/util/dates.dart';
import 'package:kinkit_mobile/util/money.dart';

void main() {
  group('MoneyFormatter', () {
    test('formats IDR like the web (symbol, no space, dot grouping)', () {
      const f = MoneyFormatter();
      expect(f.format(1250000), 'Rp1.250.000');
      expect(f.format(0), 'Rp0');
      expect(f.format(999), 'Rp999');
    });

    test('negative amounts keep the sign in front of the symbol', () {
      const f = MoneyFormatter();
      expect(f.format(-50000), '-Rp50.000');
    });

    test('converts IDR to the household currency using FX rates', () {
      const f = MoneyFormatter(currency: 'USD', rates: {'IDR': 1, 'USD': 0.0000625});
      // 1,600,000 IDR * 0.0000625 = 100 USD
      expect(f.format(1600000), r'$100');
      expect(f.format(160000000), r'$10,000');
    });

    test('uses locale grouping for EUR', () {
      const f = MoneyFormatter(currency: 'EUR', rates: {'IDR': 1, 'EUR': 0.0001});
      expect(f.format(12345670000), '€1.234.567');
    });

    test('falls back to IDR when no rate is known', () {
      const f = MoneyFormatter(currency: 'USD', rates: {'IDR': 1});
      expect(f.format(25000), 'Rp25.000');
    });

    test('unknown currency code falls back to IDR metadata', () {
      const f = MoneyFormatter(currency: 'XYZ', rates: {'IDR': 1, 'XYZ': 1});
      expect(f.format(1000), 'Rp1.000');
    });

    test('static idr helper', () {
      expect(MoneyFormatter.idr(25000), 'Rp25.000');
    });
  });

  group('parseAmount', () {
    test('plain and grouped numbers', () {
      expect(parseAmount('25000'), 25000);
      expect(parseAmount('25.000'), 25000);
      expect(parseAmount('25,000'), 25000);
      expect(parseAmount('Rp 1.250.000'), 1250000);
    });

    test('shorthand like the web MoneyInput', () {
      expect(parseAmount('25k'), 25000);
      expect(parseAmount('2rb'), 2000);
      expect(parseAmount('1.5jt'), 1500000);
      expect(parseAmount('1,5jt'), 1500000);
      expect(parseAmount('1.5m'), 1500000);
      expect(parseAmount('3 juta'), 3000000);
    });

    test('invalid input', () {
      expect(parseAmount(''), isNull);
      expect(parseAmount('abc'), isNull);
      expect(parseAmount('12x'), isNull);
    });
  });

  group('YearMonth', () {
    test('add wraps years', () {
      expect(const YearMonth(2026, 12).add(1), const YearMonth(2027, 1));
      expect(const YearMonth(2026, 1).add(-1), const YearMonth(2025, 12));
      expect(const YearMonth(2026, 5).add(-17), const YearMonth(2024, 12));
    });

    test('value equality', () {
      expect(const YearMonth(2026, 11) == const YearMonth(2026, 11), isTrue);
      expect(const YearMonth(2026, 11).hashCode, const YearMonth(2026, 11).hashCode);
    });

    test('isoDate pads', () {
      expect(isoDate(DateTime(2026, 3, 7)), '2026-03-07');
      expect(dateOnly('2026-03-07T00:00:00Z'), '2026-03-07');
    });
  });

  group('AppConfig.normalizeBaseUrl', () {
    test('adds scheme and strips slashes and /api', () {
      expect(AppConfig.normalizeBaseUrl('kinkit.example.com/'), 'https://kinkit.example.com');
      expect(AppConfig.normalizeBaseUrl(' http://192.168.1.5:5000/api/ '), 'http://192.168.1.5:5000');
    });
  });
}
