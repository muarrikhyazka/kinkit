import 'package:intl/intl.dart';

/// Currency metadata - mirrors CURRENCY_META in react/src/api.js and the
/// `currencies` map in backend-go/auth.go.
class CurrencyMeta {
  final String symbol;
  final String locale; // BCP-47 as the backend sends it, e.g. "id-ID"
  const CurrencyMeta(this.symbol, this.locale);
}

const Map<String, CurrencyMeta> currencyMeta = {
  'IDR': CurrencyMeta('Rp', 'id-ID'),
  'USD': CurrencyMeta(r'$', 'en-US'),
  'EUR': CurrencyMeta('€', 'de-DE'),
  'GBP': CurrencyMeta('£', 'en-GB'),
  'SGD': CurrencyMeta(r'S$', 'en-SG'),
  'MYR': CurrencyMeta('RM', 'ms-MY'),
  'JPY': CurrencyMeta('¥', 'ja-JP'),
  'AUD': CurrencyMeta(r'A$', 'en-AU'),
  'INR': CurrencyMeta('₹', 'en-IN'),
};

/// "id-ID" -> "id_ID" (intl style).
String intlLocale(String bcp47) => bcp47.replaceAll('-', '_');

/// Groups an integer with the locale's separators ("1.250.000" for id-ID).
String groupDigits(int v, String bcp47) {
  try {
    final f = NumberFormat.decimalPattern(intlLocale(bcp47));
    f.maximumFractionDigits = 0;
    return f.format(v);
  } catch (_) {
    return NumberFormat.decimalPattern('en_US').format(v);
  }
}

/// Formats amounts the same way the web does (formatRupiah in api.js):
/// all amounts are stored as whole IDR, converted to the household currency
/// with the cached FX table, rounded, and printed as symbol + grouped digits
/// (no space: "Rp1.250.000", "$75"). When no rate is known for the chosen
/// currency it falls back to showing IDR rather than a wrong number.
class MoneyFormatter {
  final String currency;
  final Map<String, double> rates;

  const MoneyFormatter({this.currency = 'IDR', this.rates = const {'IDR': 1}});

  CurrencyMeta get _meta => currencyMeta[currency] ?? currencyMeta['IDR']!;

  double? get _rate {
    final r = rates[currency];
    if (r != null) return r;
    return currency == 'IDR' ? 1 : null;
  }

  /// Format an IDR amount in the household currency.
  String format(num idr) {
    final rate = _rate;
    final converted = rate != null ? idr * rate : idr;
    final v = converted.round();
    final meta = rate != null ? _meta : currencyMeta['IDR']!;
    final sign = v < 0 ? '-' : '';
    return '$sign${meta.symbol}${groupDigits(v.abs(), meta.locale)}';
  }

  /// Always IDR (amount inputs are entered in IDR, like the web).
  static String idr(num v) {
    final r = v.round();
    final sign = r < 0 ? '-' : '';
    return '${sign}Rp${groupDigits(r.abs(), 'id-ID')}';
  }
}

const Map<String, double> _suffix = {
  'k': 1e3, 'rb': 1e3, 'ribu': 1e3,
  'jt': 1e6, 'juta': 1e6, 'm': 1e6, 'mio': 1e6,
};

/// Parses what users type into an amount field (IDR, whole units):
///   "25000", "25.000", "25,000", "Rp 25.000" -> 25000
///   shorthand like the web's MoneyInput: "25k", "2rb", "1.5jt", "1,5jt" -> ...
/// Returns null for empty/invalid input.
int? parseAmount(String input) {
  final s = input.toLowerCase().replaceAll(RegExp(r'\s+'), '').replaceFirst(RegExp(r'^rp'), '');
  if (s.isEmpty) return null;
  final m = RegExp(r'^(-?\d+(?:[.,]\d+)?)(k|rb|ribu|jt|juta|m|mio)$').firstMatch(s);
  if (m != null) {
    final n = double.tryParse(m.group(1)!.replaceAll(',', '.'));
    if (n == null) return null;
    return (n * _suffix[m.group(2)!]!).round();
  }
  if (!RegExp(r'^-?[\d.,]+$').hasMatch(s)) return null;
  final digits = s.replaceAll(RegExp(r'[.,]'), '');
  return int.tryParse(digits);
}
