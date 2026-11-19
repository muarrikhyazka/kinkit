/// Models for GET /api/tv/data (backend-go/tvshare.go handleTVData).
///
/// All money amounts are integers in IDR (the backend stores IDR and the web
/// app converts to the display currency client-side, see money.dart).
library;

int _int(Object? v) {
  if (v is int) return v;
  if (v is num) return v.round();
  if (v is String) return int.tryParse(v) ?? double.tryParse(v)?.round() ?? 0;
  return 0;
}

String _str(Object? v, [String fallback = '']) => v is String ? v : (v == null ? fallback : v.toString());

Map<String, dynamic> _map(Object? v) => v is Map ? v.cast<String, dynamic>() : const <String, dynamic>{};

List<Map<String, dynamic>> _list(Object? v) =>
    v is List ? v.whereType<Map>().map((e) => e.cast<String, dynamic>()).toList() : const [];

class TvExpense {
  const TvExpense({
    required this.categoryName,
    required this.mainCategory,
    required this.allocation,
    required this.realization,
  });

  factory TvExpense.fromJson(Map<String, dynamic> j) => TvExpense(
        categoryName: _str(j['category_name'], 'Uncategorised'),
        mainCategory: _str(j['main_category']),
        allocation: _int(j['allocation']),
        realization: _int(j['realization']),
      );

  final String categoryName;
  final String mainCategory;
  final int allocation; // budgeted this month (IDR)
  final int realization; // actually spent this month (IDR)

  /// Spent / budget as a fraction (may exceed 1). Null when nothing budgeted.
  double? get ratio => allocation > 0 ? realization / allocation : null;
  bool get isOver => allocation > 0 ? realization > allocation : realization > 0;
}

class TvReport {
  const TvReport({
    required this.totalIncome,
    required this.budgetedExpenses,
    required this.savings,
    required this.leftForBudgeting,
    required this.expenses,
  });

  factory TvReport.fromJson(Map<String, dynamic> j) => TvReport(
        totalIncome: _int(j['total_income']),
        budgetedExpenses: _int(j['budgeted_expenses']),
        savings: _int(j['savings']),
        leftForBudgeting: _int(j['left_for_budgeting']),
        expenses: _list(j['expenses_list']).map(TvExpense.fromJson).toList(),
      );

  final int totalIncome;
  final int budgetedExpenses;
  final int savings;
  final int leftForBudgeting;
  final List<TvExpense> expenses;

  /// Sum of realization across categories = actual spending this month.
  int get totalSpent => expenses.fold(0, (s, e) => s + e.realization);

  /// Categories worth showing on the budget page: budgeted or with spending,
  /// most-used-of-budget first (overspent at the top).
  List<TvExpense> get activeCategories {
    final list = expenses.where((e) => e.allocation > 0 || e.realization > 0).toList();
    double key(TvExpense e) => e.ratio ?? (e.realization > 0 ? 99 : 0);
    list.sort((a, b) {
      final c = key(b).compareTo(key(a));
      return c != 0 ? c : b.realization.compareTo(a.realization);
    });
    return list;
  }

  List<TvExpense> topExpenses([int n = 5]) {
    final list = expenses.where((e) => e.realization > 0).toList()
      ..sort((a, b) => b.realization.compareTo(a.realization));
    return list.take(n).toList();
  }
}

class TvTrendPoint {
  const TvTrendPoint({required this.year, required this.month, required this.income, required this.expense});

  factory TvTrendPoint.fromJson(Map<String, dynamic> j) {
    final m = _int(j['month']);
    return TvTrendPoint(
      year: _int(j['year']),
      month: m < 1 ? 1 : (m > 12 ? 12 : m),
      income: _int(j['income']),
      expense: _int(j['expense']),
    );
  }

  final int year;
  final int month; // 1-12
  final int income;
  final int expense;
}

/// One widget of a widget board (web: Widgets page): [w] columns of 12, [h] rows 1-4.
class TvWidget {
  const TvWidget({required this.id, required this.type, required this.w, required this.h, this.text = ''});

  factory TvWidget.fromJson(Map<String, dynamic> j) => TvWidget(
        id: _str(j['id']),
        type: _str(j['type']),
        w: _int(j['w']).clamp(1, 12),
        h: _int(j['h']).clamp(1, 4),
        text: _str(j['text']),
      );

  final String id;
  final String type;
  final int w;
  final int h;
  final String text; // note widget
}

class TvCardRow {
  const TvCardRow({required this.title, this.detail = '', this.done = false, this.alert = false, this.amount, this.pct});

  factory TvCardRow.fromJson(Map<String, dynamic> j) => TvCardRow(
        title: _str(j['title']),
        detail: _str(j['detail']),
        done: j['done'] == true,
        alert: j['alert'] == true,
        amount: j['amount'] == null ? null : _int(j['amount']),
        pct: j['pct'] is num ? (j['pct'] as num).toDouble() : null,
      );

  final String title;
  final String detail;
  final bool done;
  final bool alert;
  final int? amount; // IDR
  final double? pct;
}

/// Generic card rendered by the server for every widget type except clock/note.
class TvCard {
  const TvCard({required this.title, this.big = '', this.bigAmount, this.caption = '', this.empty = '', this.rows = const []});

  factory TvCard.fromJson(Map<String, dynamic> j) => TvCard(
        title: _str(j['title']),
        big: _str(j['big']),
        bigAmount: j['big_amount'] == null ? null : _int(j['big_amount']),
        caption: _str(j['caption']),
        empty: _str(j['empty']),
        rows: _list(j['rows']).map(TvCardRow.fromJson).toList(),
      );

  final String title;
  final String big;
  final int? bigAmount; // IDR
  final String caption;
  final String empty;
  final List<TvCardRow> rows;
}

/// The widget board a TV link shows (null board = the classic month summary).
class TvBoard {
  const TvBoard({required this.name, required this.widgets, required this.cards});

  final String name;
  final List<TvWidget> widgets;
  final Map<String, TvCard> cards;
}

class TvData {
  const TvData({
    required this.year,
    required this.month,
    required this.currency,
    required this.displayName,
    this.householdName,
    required this.report,
    required this.trend,
    this.board,
  });

  factory TvData.fromJson(Map<String, dynamic> j) {
    final now = DateTime.now().toUtc();
    final y = _int(j['year']);
    final m = _int(j['month']);
    final cur = _str(j['currency']).trim().toUpperCase();
    final name = j['display_name'];
    final family = j['household_name'];
    return TvData(
      year: y > 0 ? y : now.year,
      month: (m >= 1 && m <= 12) ? m : now.month,
      currency: cur.isEmpty ? 'IDR' : cur,
      displayName: (name is String && name.trim().isNotEmpty) ? name.trim() : null,
      householdName: (family is String && family.trim().isNotEmpty) ? family.trim() : null,
      report: TvReport.fromJson(_map(j['report'])),
      trend: _list(j['trend']).map(TvTrendPoint.fromJson).toList(),
      board: j['board'] is Map ? _parseBoard(_map(j['board']), _map(j['cards'])) : null,
    );
  }

  static TvBoard _parseBoard(Map<String, dynamic> b, Map<String, dynamic> cards) => TvBoard(
        name: _str(b['name']),
        widgets: _list(_map(b['layout'])['widgets']).map(TvWidget.fromJson).toList(),
        cards: {for (final e in cards.entries) e.key: TvCard.fromJson(_map(e.value))},
      );

  final int year;
  final int month;
  final String currency;
  final String? displayName;
  final String? householdName; // family name; the header falls back to displayName
  final TvReport report;
  final List<TvTrendPoint> trend; // oldest first
  final TvBoard? board; // widget board chosen for this link, if any
}

const monthShort = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const monthLong = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
];
