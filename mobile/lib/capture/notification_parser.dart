/// Turns the text of a payment notification into a draft expense.
///
/// Pure Dart (no Flutter imports) so it can be unit-tested. The result is only
/// ever a suggestion: the user reviews and edits it before anything is saved.
library;

class ParsedCapture {
  /// Whole units of [currency] (rounded). Greater than 0 unless [amountKnown] is false.
  final int amount;

  /// ISO code: IDR for Rp / IDR / rupiah, otherwise from the symbol.
  final String currency;

  /// Best guess at who was paid; may be empty.
  final String merchant;

  /// True for money coming in (refund, transfer received). Those are never
  /// saved as spending.
  final bool isIncome;

  /// False when the text had a currency marker but no readable amount; the
  /// review sheet then opens with an empty amount field.
  bool get amountKnown => amount > 0;

  const ParsedCapture({
    required this.amount,
    required this.currency,
    required this.merchant,
    required this.isIncome,
  });

  @override
  String toString() => 'ParsedCapture($amount $currency, "$merchant", income: $isIncome)';
}

// Matching vocabulary for bank notification text, not UI copy.
// One-time codes: never a payment, even when the same text also mentions an amount.
final RegExp _otpWords = RegExp(
  r'\botp\b|\bkode\s+(?:otp|verifikasi|verification)\b|\bverification\s+code\b|\bone[\s-]?time\b|\bjangan\s+berikan\b',
  caseSensitive: false,
);

// Advertising words. These only discard a message that has no payment wording
// and no "amount at/to merchant" phrase, so a real payment that mentions a
// promo or voucher still parses.
final RegExp _promoWords = RegExp(r'\b(?:promo|voucher)\b', caseSensitive: false);

// Wording that says money left the account.
final RegExp _paymentWords = RegExp(
  r'\b(?:pembayaran|payment|paid|debit|bayar|dibayar|purchase|belanja|charged)\b',
  caseSensitive: false,
);

// "Rp 25.000 at Indomaret", "Rp 54.000 to NETFLIX", "Rp 50.000 di ALFAMART".
final RegExp _amountThenMerchant = RegExp(r'[0-9][0-9.,]*\s+(?:at|to|di|ke)\s+\S', caseSensitive: false);

// Matching vocabulary for bank notification text, not UI copy.
// Wording that marks money arriving instead of leaving. Whole words only, so
// "termasuk" (including) is not "masuk" (in).
final RegExp _incomeWords = RegExp(
  r'\b(?:received|credited|incoming|refund(?:ed|s)?|transfer\s+in|masuk|diterima)\b',
  caseSensitive: false,
);

// "received" and "diterima" also end payment confirmations ("Pembayaran
// diterima"), so they only mean income when no payment wording is present.
final RegExp _weakIncomeWords = RegExp(r'\b(?:received|diterima)\b', caseSensitive: false);

// Matching vocabulary for bank notification text, not UI copy.
// Words that end a merchant name inside a sentence ("... to NETFLIX successful").
const String _merchantStops =
    'on|for|Rp|IDR|successful|successfully|success|completed|approved|using|via|with|from|dengan|berhasil|pada|tanggal';

// Titles that name the kind of message rather than who was paid.
const Set<String> _genericTitles = {
  'payment',
  'payment successful',
  'transaction',
  'transaction successful',
  'notification',
  'alert',
  'info',
  'transfer',
  'successful',
  'success',
  'transaksi',
  'transaksi berhasil',
  'pembayaran',
  'pembayaran berhasil',
  'berhasil',
};

// Currency code or Rp before the number, with an optional magnitude word:
// "Rp 25.000", "IDR1,250", "SGD 12.50", "Rp1.5 juta", "Rp 50 rb".
final RegExp _codeAmount = RegExp(
  r'(?<![a-z])(rp\.?|idr|usd|sgd|myr|eur|gbp)\s*([0-9][0-9.,]*)(?:\s*(juta|jt|ribu|rb|k)\b)?',
  caseSensitive: false,
);
// Number before the code: "150.000 IDR", "12.50 SGD", "75.000 rupiah".
final RegExp _suffixAmount = RegExp(r'([0-9][0-9.,]*)\s*(idr|rupiah|usd|sgd|myr|eur|gbp)\b', caseSensitive: false);
final RegExp _symbolAmount = RegExp(r'([$€£])\s*([0-9][0-9.,]*)');

/// Whether the text has any currency marker followed by a digit. The Kotlin
/// listener (CaptureStore.moneyPattern) queues exactly the notifications this
/// matches; keep the two in step.
final RegExp moneyMarker = RegExp(
  r'(?<![a-z])(?:rp\.?|idr|usd|sgd|myr|eur|gbp|[$€£])\s*[0-9]|[0-9]\s*(?:idr|rupiah)\b',
  caseSensitive: false,
);

final RegExp _merchantRegex = RegExp(
  r'\b(?:at|to|merchant|di|ke)\b\s+(.+?)(?=[\n,.;]|\s+(?:' + _merchantStops + r')\b|$)',
  caseSensitive: false,
);

/// True for one-time codes and advertising: messages that are never payments
/// and are safe to delete from the phone. A message that mentions a promo or
/// voucher but also has payment wording, or an "amount at merchant" phrase,
/// is not noise.
bool isCaptureNoise({required String title, required String text}) {
  final all = '$title\n$text';
  if (_otpWords.hasMatch(all)) return true;
  if (_promoWords.hasMatch(all)) {
    return !_paymentWords.hasMatch(all) && !_amountThenMerchant.hasMatch(all);
  }
  return false;
}

bool _looksLikeIncome(String all) {
  final m = _incomeWords.firstMatch(all);
  if (m == null) return false;
  if (_weakIncomeWords.hasMatch(m.group(0)!) && _paymentWords.hasMatch(all)) return false;
  return true;
}

/// Parses a notification. Returns null when it is a one-time code, an
/// advertisement, or has no readable amount.
ParsedCapture? parseCapture({required String title, required String text, String app = ''}) {
  if (isCaptureNoise(title: title, text: text)) return null;
  final all = '$title\n$text';
  final found = _firstAmount(all);
  if (found == null) return null;
  return ParsedCapture(
    amount: found.amount,
    currency: found.currency,
    merchant: _merchant(title: title, text: text, app: app),
    isIncome: _looksLikeIncome(all),
  );
}

/// What the review list shows for a notification: the parse, or, when the
/// text has a currency marker but no readable amount, a draft with amount 0
/// ([ParsedCapture.amountKnown] is false) so the person can read the original
/// and type the amount. Null only for noise and text with no money in it,
/// which are safe to delete.
ParsedCapture? draftForReview({required String title, required String text, String app = ''}) {
  final parsed = parseCapture(title: title, text: text, app: app);
  if (parsed != null) return parsed;
  if (isCaptureNoise(title: title, text: text)) return null;
  final all = '$title\n$text';
  if (!moneyMarker.hasMatch(all)) return null;
  return ParsedCapture(
    amount: 0,
    currency: 'IDR',
    merchant: _merchant(title: title, text: text, app: app),
    isIncome: _looksLikeIncome(all),
  );
}

class _Found {
  final int start;
  final int amount;
  final String currency;
  const _Found(this.start, this.amount, this.currency);
}

/// The amount that appears first in the text, so "Rp 54.000 ... balance
/// Rp 1.000.000" yields 54000.
_Found? _firstAmount(String s) {
  _Found? best;
  // True when the match holds a usable amount (then it is the first of its kind).
  bool consider(int start, String rawNumber, String currency, [int multiplier = 1]) {
    final v = parseLocaleNumber(rawNumber);
    if (v == null || !v.isFinite) return false;
    final scaled = v * multiplier;
    if (scaled > 1e12) return false;
    final n = scaled.round();
    if (n <= 0) return false;
    if (best == null || start < best!.start) best = _Found(start, n, currency);
    return true;
  }

  String codeOf(String raw) {
    final c = raw.toLowerCase();
    return (c.startsWith('rp') || c == 'rupiah') ? 'IDR' : c.toUpperCase();
  }

  int magnitude(String? word) {
    switch (word?.toLowerCase()) {
      case 'juta':
      case 'jt':
        return 1000000;
      case 'ribu':
      case 'rb':
      case 'k':
        return 1000;
      default:
        return 1;
    }
  }

  // The first readable amount of each kind: "Rp 0 ... Rp 5.000" still finds 5000.
  for (final m in _codeAmount.allMatches(s)) {
    if (consider(m.start, m.group(2)!, codeOf(m.group(1)!), magnitude(m.group(3)))) break;
  }
  for (final m in _suffixAmount.allMatches(s)) {
    if (consider(m.start, m.group(1)!, codeOf(m.group(2)!))) break;
  }
  for (final m in _symbolAmount.allMatches(s)) {
    final sym = m.group(1)!;
    if (consider(m.start, m.group(2)!, sym == r'$' ? 'USD' : (sym == '€' ? 'EUR' : 'GBP'))) break;
  }
  return best;
}

/// Reads "1.250.000", "1,250,000.00", "25.000,00" or "12.50" as a number.
///
/// When both separators appear, the last one is the decimal separator. A
/// single separator that repeats is a thousands separator. A single separator
/// followed by exactly three digits is a thousands separator, otherwise it is
/// a decimal point. Returns null when [raw] is not a number.
double? parseLocaleNumber(String raw) {
  var s = raw.trim().replaceAll(RegExp(r'[.,]+$'), '');
  if (!RegExp(r'^[0-9][0-9.,]*$').hasMatch(s)) return null;
  final dots = '.'.allMatches(s).length;
  final commas = ','.allMatches(s).length;
  if (dots == 0 && commas == 0) return double.tryParse(s);
  if (dots > 0 && commas > 0) {
    final decimal = s.lastIndexOf('.') > s.lastIndexOf(',') ? '.' : ',';
    final thousands = decimal == '.' ? ',' : '.';
    s = s.replaceAll(thousands, '').replaceAll(decimal, '.');
    return double.tryParse(s);
  }
  final sep = dots > 0 ? '.' : ',';
  final count = dots > 0 ? dots : commas;
  if (count > 1) return double.tryParse(s.replaceAll(sep, ''));
  final digitsAfter = s.length - s.indexOf(sep) - 1;
  if (digitsAfter == 3) return double.tryParse(s.replaceAll(sep, ''));
  return double.tryParse(s.replaceAll(sep, '.'));
}

String _merchant({required String title, required String text, required String app}) {
  for (final m in _merchantRegex.allMatches(text)) {
    final c = _clean(m.group(1) ?? '');
    if (c.isNotEmpty && !RegExp(r'^(?:\d|rp\b|idr\b)', caseSensitive: false).hasMatch(c)) return c;
  }
  final t = _clean(title);
  if (t.isNotEmpty && !_isGenericTitle(t)) return t;
  final a = app.trim();
  // An unresolved app label is the package name ("com.bank.app"): not a merchant.
  if (a.isNotEmpty && !(a.contains('.') && !a.contains(' '))) return _clean(a);
  return '';
}

bool _isGenericTitle(String t) {
  if (t.length < 2) return true;
  if (RegExp(r'\d').hasMatch(t)) return true;
  return _genericTitles.contains(t.toLowerCase());
}

String _clean(String s) {
  var c = s.replaceAll(RegExp(r'\s+'), ' ').trim();
  c = c.replaceAll(RegExp(r'[\s.,;:!\-]+$'), '');
  if (c.length > 40) c = c.substring(0, 40).trim();
  return c;
}

/// Idempotency key for POST /api/spending: the same notification saved twice
/// (a retry after a flaky network) is stored once. Two 32-bit FNV-1a hashes
/// with different seeds, so unrelated notifications do not collide.
String captureClientId(String notificationId) {
  String hex(int seed) {
    var h = seed;
    for (final b in notificationId.codeUnits) {
      h = ((h ^ b) * 16777619) & 0xFFFFFFFF;
    }
    return h.toRadixString(16).padLeft(8, '0');
  }

  return 'notif-${hex(0x811C9DC5)}${hex(0x9747B28C)}';
}
