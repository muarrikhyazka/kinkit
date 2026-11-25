import 'package:intl/intl.dart';

/// Mirrors react/src/api.js formatRupiah: amounts arrive in IDR and are
/// converted with the public /api/fx-rates table (IDR-based, e.g. USD: 0.000061).
/// Without a rate for the account currency we show IDR rather than a wrong number.
class Money {
  Money({required String currency, Map<String, double>? rates})
      : _currency = currency.toUpperCase(),
        _rates = rates ?? const {'IDR': 1};

  final String _currency;
  final Map<String, double> _rates;

  static const _meta = <String, (String, String)>{
    'IDR': ('Rp', 'id'),
    'USD': (r'$', 'en_US'),
    'EUR': ('€', 'de'),
    'GBP': ('£', 'en_GB'),
    'SGD': (r'S$', 'en_SG'),
    'MYR': ('RM', 'ms'),
    'JPY': ('¥', 'ja'),
    'AUD': (r'A$', 'en_AU'),
    'INR': ('₹', 'en_IN'),
  };

  double? get _rate {
    if (_currency == 'IDR') return 1;
    final r = _rates[_currency];
    return (r != null && r > 0) ? r : null;
  }

  /// Currency actually used for display (falls back to IDR without a rate).
  String get effectiveCurrency => (_rate != null && _meta.containsKey(_currency)) ? _currency : 'IDR';

  static NumberFormat _fmt(String locale, {bool compact = false}) {
    try {
      return compact ? NumberFormat.compact(locale: 'en_US') : NumberFormat.decimalPattern(locale);
    } catch (_) {
      return compact ? NumberFormat.compact(locale: 'en_US') : NumberFormat.decimalPattern('en_US');
    }
  }

  num _convert(int idr) => (idr * (_rate ?? 1)).round();

  /// e.g. "Rp12.500.000" or "$1,234".
  String format(int idr) {
    final meta = _meta[effectiveCurrency]!;
    final v = effectiveCurrency == 'IDR' ? idr : _convert(idr);
    final neg = v < 0;
    final s = meta.$1 + _fmt(meta.$2).format(v.abs());
    return neg ? '-$s' : s;
  }

  /// Short form for chart axes, always English units, e.g. "Rp12M" / "$1.2K", whatever the currency locale.
  String compact(int idr) {
    final meta = _meta[effectiveCurrency]!;
    final v = effectiveCurrency == 'IDR' ? idr : _convert(idr);
    return meta.$1 + _fmt(meta.$2, compact: true).format(v);
  }
}
