import 'package:flutter_test/flutter_test.dart';
import 'package:kinkit_tv/src/api.dart';
import 'package:kinkit_tv/src/models.dart';
import 'package:kinkit_tv/src/money.dart';

const sample = '''
{
  "year": 2026, "month": 11, "currency": "IDR", "display_name": "John Smith",
  "household_name": "The Smiths",
  "report": {
    "total_income": 20000000, "budgeted_expenses": 12000000, "savings": 3000000,
    "left_for_budgeting": 5000000,
    "expenses_list": [
      {"category_name": "Food", "main_category": "Needs", "allocation": 4000000, "realization": 4500000},
      {"category_name": "Fuel", "main_category": "Needs", "allocation": 1000000, "realization": 850000},
      {"category_name": "Fun", "main_category": "Wants", "allocation": 0, "realization": 200000},
      {"category_name": "Unused", "main_category": "Wants", "allocation": 0, "realization": 0}
    ]
  },
  "trend": [
    {"year": 2026, "month": 6, "income": 18000000, "expense": 11000000},
    {"year": 2026, "month": 11, "income": 20000000, "expense": 5550000}
  ]
}
''';

void main() {
  test('parses the /api/tv/data payload', () {
    final d = parseTvData(sample);
    expect(d.year, 2026);
    expect(d.month, 11);
    expect(d.currency, 'IDR');
    expect(d.displayName, 'John Smith');
    expect(d.householdName, 'The Smiths');
    expect(d.report.totalIncome, 20000000);
    expect(d.report.budgetedExpenses, 12000000);
    expect(d.report.savings, 3000000);
    expect(d.report.leftForBudgeting, 5000000);
    expect(d.report.expenses, hasLength(4));
    expect(d.report.totalSpent, 5550000);
    expect(d.trend.map((t) => t.month), [6, 11]);
  });

  test('active categories: overspent first, unused dropped', () {
    final cats = parseTvData(sample).report.activeCategories.map((e) => e.categoryName).toList();
    expect(cats, ['Fun', 'Food', 'Fuel']);
    final top = parseTvData(sample).report.topExpenses(2).map((e) => e.categoryName).toList();
    expect(top, ['Food', 'Fuel']);
  });

  test('tolerates nulls, missing fields and numeric strings', () {
    final d = TvData.fromJson({
      'year': 2026,
      'month': '3',
      'currency': null,
      'display_name': null,
      'report': {'total_income': 1.6, 'expenses_list': null},
      'trend': null,
    });
    expect(d.month, 3);
    expect(d.currency, 'IDR');
    expect(d.displayName, isNull);
    expect(d.householdName, isNull);
    expect(d.report.totalIncome, 2);
    expect(d.report.expenses, isEmpty);
    expect(d.trend, isEmpty);
  });

  test('bad JSON raises a badData error', () {
    expect(() => parseTvData('<html>'), throwsA(isA<TvApiException>().having((e) => e.kind, 'kind', TvErrorKind.badData)));
    expect(() => parseTvData('[1,2]'), throwsA(isA<TvApiException>()));
  });

  group('Money', () {
    test('formats IDR like the web app', () {
      expect(Money(currency: 'IDR').format(12500000), 'Rp12.500.000');
      expect(Money(currency: 'IDR').format(-5000), '-Rp5.000');
    });
    test('converts with fx rates', () {
      expect(Money(currency: 'USD', rates: {'USD': 0.0001}).format(12500000), r'$1,250');
    });
    test('falls back to IDR without a rate', () {
      final m = Money(currency: 'USD');
      expect(m.effectiveCurrency, 'IDR');
      expect(m.format(1000), 'Rp1.000');
    });
  });

  test('TvData parses a widget board and its cards', () {
    final d = TvData.fromJson({
      'year': 2026, 'month': 11, 'currency': 'IDR', 'report': {}, 'trend': [],
      'board': {'id': 1, 'name': 'Living-room TV', 'layout': {'widgets': [
        {'id': 'a', 'type': 'clock', 'w': 4, 'h': 1},
        {'id': 'b', 'type': 'agenda', 'w': 99, 'h': 3},
      ]}},
      'cards': {'agenda': {'type': 'agenda', 'title': 'Next 7 days', 'rows': [{'title': 'Swim', 'detail': 'Today', 'pct': 50}], 'big_amount': 1000}},
    });
    expect(d.board, isNotNull);
    expect(d.board!.name, 'Living-room TV');
    expect(d.board!.widgets.length, 2);
    expect(d.board!.widgets.last.w, 12); // clamped
    expect(d.board!.cards['agenda']!.rows.single.pct, 50);
    expect(d.board!.cards['agenda']!.bigAmount, 1000);
    expect(TvData.fromJson({'report': {}, 'trend': []}).board, isNull); // classic month summary
  });
}
