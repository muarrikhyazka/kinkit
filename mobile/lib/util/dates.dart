import 'package:intl/intl.dart';

const monthNames = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
];
const monthShort = [
  'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
  'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
];

/// A calendar month. Value type (usable as a Riverpod family key).
class YearMonth {
  final int year;
  final int month; // 1..12

  const YearMonth(this.year, this.month);

  factory YearMonth.now() {
    final n = DateTime.now();
    return YearMonth(n.year, n.month);
  }

  YearMonth add(int months) {
    final idx = year * 12 + (month - 1) + months;
    return YearMonth(idx ~/ 12, idx % 12 + 1);
  }

  bool get isCurrent {
    final n = DateTime.now();
    return n.year == year && n.month == month;
  }

  String get label => '${monthNames[month - 1]} $year';
  String get shortLabel => '${monthShort[month - 1]} $year';

  @override
  bool operator ==(Object other) =>
      other is YearMonth && other.year == year && other.month == month;

  @override
  int get hashCode => Object.hash(year, month);

  @override
  String toString() => '$year-${month.toString().padLeft(2, '0')}';
}

/// YYYY-MM-DD for a local date (what the backend expects in tx_date).
String isoDate(DateTime d) =>
    '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

String todayIso() => isoDate(DateTime.now());

/// Backend dates may come as "2026-11-18" or "2026-11-18T00:00:00Z".
String dateOnly(String s) => s.length >= 10 ? s.substring(0, 10) : s;

/// "Mon, 18 Nov" style label for a YYYY-MM-DD string; falls back to input.
String prettyDate(String s) {
  final d = DateTime.tryParse(dateOnly(s));
  if (d == null) return s;
  return DateFormat('EEE, d MMM').format(d);
}
