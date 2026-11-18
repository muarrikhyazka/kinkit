import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../util/dates.dart';
import '../util/money.dart';
import '../widgets/async_view.dart';
import '../widgets/k_app_bar.dart';
import '../widgets/mascot.dart';
import '../widgets/month_picker.dart';
import '../widgets/stat_card.dart';

/// Monthly summary - the web Dashboard's "Month" tab.
class DashboardScreen extends ConsumerWidget {
  /// True inside the Money tabs (no own app bar).
  final bool embedded;

  const DashboardScreen({super.key, this.embedded = false});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ym = ref.watch(selectedMonthProvider);
    final report = ref.watch(reportProvider(ym));
    final compare = ref.watch(compareProvider(ym));
    final recent = ref.watch(spendingProvider(ym));

    Future<void> refresh() async {
      ref.invalidate(compareProvider(ym));
      ref.invalidate(spendingProvider(ym));
      ref.invalidate(reportProvider(ym));
      await ref.read(reportProvider(ym).future);
    }

    return Scaffold(
      appBar: embedded ? null : kAppBar(context, 'Dashboard'),
      body: Column(
        children: [
          const MonthPicker(),
          Expanded(
            child: PullToRefresh(
              onRefresh: refresh,
              child: AsyncView<MonthReport>(
                value: report,
                onRetry: () => ref.invalidate(reportProvider(ym)),
                builder: (r) => _DashboardBody(
                  ym: ym,
                  report: r,
                  compare: compare.valueOrNull,
                  recent: recent.valueOrNull?.rows.take(6).toList() ?? const [],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _DashboardBody extends ConsumerWidget {
  final YearMonth ym;
  final MonthReport report;
  final MonthCompare? compare;
  final List<SpendingRow> recent;

  const _DashboardBody({required this.ym, required this.report, required this.compare, required this.recent});

  String _pct(double? v, String label) {
    if (v == null) return '';
    return '${v >= 0 ? '+' : ''}${v.toStringAsFixed(1)}% $label';
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final money = ref.watch(moneyProvider);
    final p = context.palette;
    final spent = compare?.current ?? report.expenseActual;
    final vsPrev = compare == null ? null : MonthCompare.pctChange(compare!.current, compare!.previousMonth);
    final vsYear = compare == null ? null : MonthCompare.pctChange(compare!.current, compare!.sameMonthLastYear);
    final subParts = [_pct(vsPrev, 'vs last month'), _pct(vsYear, 'vs last year')].where((s) => s.isNotEmpty);
    final incomePct = report.totalIncome > 0 ? ' (${(report.incomeActual / report.totalIncome * 100).round()}%)' : '';
    final netFlow = report.totalIncome - report.budgetedExpenses;

    final byRatio = [...report.expensesList]
      ..retainWhere((e) => e.allocation > 0 || e.realization > 0)
      ..sort((a, b) => b.ratio.compareTo(a.ratio));
    final top = report.topSpending;
    final topMax = top.isEmpty ? 1 : top.first.realization;

    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.only(bottom: 96),
      children: [
        StatGrid(children: [
          StatCard(
            label: 'Spending ${monthShort[ym.month - 1]}',
            value: money.format(spent),
            accent: p.danger,
            icon: Icons.shopping_bag_outlined,
            sub: subParts.isEmpty ? null : subParts.join(' · '),
          ),
          StatCard(
            label: 'Income received',
            value: money.format(report.incomeActual),
            accent: p.success,
            icon: Icons.payments_outlined,
            sub: 'Planned: ${money.format(report.totalIncome)}$incomePct',
          ),
          StatCard(
            label: 'Savings',
            value: money.format(report.savings),
            accent: mainCategoryColor('Savings'),
            icon: Icons.savings_outlined,
          ),
          StatCard(
            label: 'Left for budgeting',
            value: money.format(report.leftForBudgeting),
            accent: p.primary,
            negative: netFlow < 0,
            icon: Icons.account_balance_wallet_outlined,
            sub: 'Budgeted expenses: ${money.format(report.budgetedExpenses)}',
          ),
        ]),
        if (report.budgetedExpenses > 0)
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Budget used', style: TextStyle(fontWeight: FontWeight.w600, color: p.text)),
                  const SizedBox(height: 10),
                  KProgressBar(
                    value: report.expenseActual / report.budgetedExpenses,
                    over: report.expenseActual > report.budgetedExpenses,
                  ),
                  const SizedBox(height: 6),
                  Text(
                    '${money.format(report.expenseActual)} of ${money.format(report.budgetedExpenses)}'
                    ' (${(report.expenseActual / report.budgetedExpenses * 100).round()}%)',
                    style: TextStyle(fontSize: 12, color: p.muted),
                  ),
                ],
              ),
            ),
          ),
        const SectionTitle('Top expenses'),
        if (top.isEmpty)
          const EmptyState(
            mascot: MascotPose.think,
            title: 'No expenses this month yet',
            message: 'Spending you log this month shows up here.',
          )
        else
          Card(
            child: Padding(
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: Column(
                children: [
                  for (final e in top)
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              Expanded(child: Text(e.categoryName, overflow: TextOverflow.ellipsis)),
                              Text(money.format(e.realization), style: const TextStyle(fontWeight: FontWeight.w600, fontFeatures: kTabular)),
                            ],
                          ),
                          const SizedBox(height: 4),
                          KProgressBar(value: e.realization / topMax, color: categoryColor(e.categoryName)),
                        ],
                      ),
                    ),
                ],
              ),
            ),
          ),
        if (recent.isNotEmpty) ...[
          const SectionTitle('Recent transactions'),
          Card(
            child: Column(
              children: [
                for (final s in recent)
                  ListTile(
                    dense: true,
                    leading: CircleAvatar(radius: 6, backgroundColor: categoryColor(s.categoryName)),
                    title: Text(s.description.isEmpty ? (s.categoryName ?? 'Spending') : s.description,
                        overflow: TextOverflow.ellipsis),
                    subtitle: Text('${prettyDate(s.txDate)} · ${s.categoryName ?? 'Other'}'),
                    trailing: Text(money.format(s.amount), style: const TextStyle(fontFeatures: kTabular)),
                  ),
              ],
            ),
          ),
        ],
        if (byRatio.isNotEmpty) ...[
          const SectionTitle('Budget vs actual'),
          Card(
            child: Column(
              children: [
                for (final e in byRatio) _BudgetRow(e: e, money: money),
              ],
            ),
          ),
        ],
      ],
    );
  }
}

class _BudgetRow extends StatelessWidget {
  final CategoryRealization e;
  final MoneyFormatter money;

  const _BudgetRow({required this.e, required this.money});

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final pct = e.allocation > 0 ? (e.realization / e.allocation * 100).round() : 0;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(child: Text(e.categoryName, overflow: TextOverflow.ellipsis)),
              Text('$pct%', style: TextStyle(color: e.over ? p.danger : p.muted, fontSize: 12)),
            ],
          ),
          const SizedBox(height: 4),
          KProgressBar(value: e.ratio, color: categoryColor(e.categoryName), over: e.over),
          const SizedBox(height: 2),
          Text('${money.format(e.realization)} of ${money.format(e.allocation)}',
              style: TextStyle(fontSize: 12, color: p.muted)),
        ],
      ),
    );
  }
}
