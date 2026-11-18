import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';

import 'models.dart';
import 'money.dart';
import 'theme.dart';

/// The carousel sections of the viewer, mirroring react/src/pages/TVView.jsx.

class _Panel extends StatelessWidget {
  const _Panel({required this.title, required this.child, this.trailing});
  final String title;
  final Widget child;
  final Widget? trailing;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(20, 14, 20, 14),
      decoration: BoxDecoration(color: TvColors.card, borderRadius: BorderRadius.circular(14)),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Row(children: [
          Expanded(
            child: Text(title,
                style: const TextStyle(fontSize: 18, color: TvColors.accent, fontWeight: FontWeight.w700)),
          ),
          if (trailing != null) trailing!,
        ]),
        const SizedBox(height: 10),
        Expanded(child: child),
      ]),
    );
  }
}

class _StatCard extends StatelessWidget {
  const _StatCard({required this.label, required this.value, required this.color, this.sub});
  final String label;
  final String value;
  final Color color;
  final String? sub;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      decoration: BoxDecoration(
        color: TvColors.card,
        borderRadius: BorderRadius.circular(14),
        border: Border(top: BorderSide(color: color, width: 3)),
      ),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisAlignment: MainAxisAlignment.center, children: [
        Text(label, style: const TextStyle(fontSize: 15, color: TvColors.muted)),
        const SizedBox(height: 4),
        FittedBox(
          fit: BoxFit.scaleDown,
          alignment: Alignment.centerLeft,
          child: Text(value, style: TextStyle(fontSize: 28, fontWeight: FontWeight.w800, color: color)),
        ),
        if (sub != null) ...[
          const SizedBox(height: 2),
          Text(sub!, maxLines: 1, overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 12, color: TvColors.muted)),
        ],
      ]),
    );
  }
}

Color ratioColor(double? ratio, int realization) {
  if (ratio == null) return realization > 0 ? TvColors.negative : TvColors.muted;
  if (ratio > 1) return TvColors.negative;
  if (ratio >= 0.8) return TvColors.warning;
  return TvColors.positive;
}

class OverviewPage extends StatelessWidget {
  const OverviewPage({super.key, required this.data, required this.money});
  final TvData data;
  final Money money;

  @override
  Widget build(BuildContext context) {
    final r = data.report;
    final spent = r.totalSpent;
    final budgetLeft = r.budgetedExpenses - spent;
    final top = r.topExpenses(5);
    final pctOfBudget = r.budgetedExpenses > 0 ? (spent * 100 / r.budgetedExpenses).round() : null;
    return Column(children: [
      SizedBox(
        height: 112,
        child: Row(children: [
          Expanded(child: _StatCard(label: 'Income', value: money.format(r.totalIncome), color: TvColors.positive)),
          const SizedBox(width: 14),
          Expanded(child: _StatCard(label: 'Budget', value: money.format(r.budgetedExpenses), color: TvColors.primary)),
          const SizedBox(width: 14),
          Expanded(
            child: _StatCard(
              label: 'Spent',
              value: money.format(spent),
              color: spent > r.budgetedExpenses && r.budgetedExpenses > 0 ? TvColors.negative : TvColors.warning,
              sub: pctOfBudget == null ? null : '$pctOfBudget% of budget',
            ),
          ),
          const SizedBox(width: 14),
          Expanded(child: _StatCard(label: 'Savings', value: money.format(r.savings), color: TvColors.accent)),
        ]),
      ),
      const SizedBox(height: 14),
      Expanded(
        child: Row(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          Expanded(
            flex: 4,
            child: _Panel(
              title: 'This month',
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                _kv('Left to budget', money.format(r.leftForBudgeting),
                    r.leftForBudgeting >= 0 ? TvColors.positive : TvColors.negative),
                const SizedBox(height: 12),
                _kv(budgetLeft >= 0 ? 'Budget remaining' : 'Over budget by', money.format(budgetLeft.abs()),
                    budgetLeft >= 0 ? TvColors.positive : TvColors.negative),
                const Spacer(),
                if (r.budgetedExpenses > 0) ...[
                  _Bar(ratio: spent / r.budgetedExpenses, color: ratioColor(spent / r.budgetedExpenses, spent), height: 14),
                  const SizedBox(height: 4),
                  Text('Spent ${money.format(spent)} of ${money.format(r.budgetedExpenses)}',
                      style: const TextStyle(fontSize: 13, color: TvColors.muted)),
                ],
              ]),
            ),
          ),
          const SizedBox(width: 14),
          Expanded(
            flex: 5,
            child: _Panel(
              title: 'Top expenses',
              child: top.isEmpty
                  ? const Center(child: Text('No spending yet this month', style: TextStyle(fontSize: 18, color: TvColors.muted)))
                  : Column(children: [
                      for (final e in top)
                        Expanded(
                          child: Container(
                            decoration: const BoxDecoration(border: Border(bottom: BorderSide(color: TvColors.border))),
                            child: Row(children: [
                              Expanded(
                                child: Text(e.categoryName, maxLines: 1, overflow: TextOverflow.ellipsis,
                                    style: const TextStyle(fontSize: 18)),
                              ),
                              Text(money.format(e.realization),
                                  style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700)),
                            ]),
                          ),
                        ),
                    ]),
            ),
          ),
        ]),
      ),
    ]);
  }

  Widget _kv(String k, String v, Color c) => Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Text(k, style: const TextStyle(fontSize: 14, color: TvColors.muted)),
        FittedBox(
          fit: BoxFit.scaleDown,
          alignment: Alignment.centerLeft,
          child: Text(v, style: TextStyle(fontSize: 30, fontWeight: FontWeight.w800, color: c)),
        ),
      ]);
}

class _Bar extends StatelessWidget {
  const _Bar({required this.ratio, required this.color, this.height = 10});
  final double ratio;
  final Color color;
  final double height;

  @override
  Widget build(BuildContext context) {
    final r = ratio.isNaN ? 0.0 : (ratio < 0 ? 0.0 : (ratio > 1 ? 1.0 : ratio));
    return ClipRRect(
      borderRadius: BorderRadius.circular(height),
      child: SizedBox(
        width: double.infinity,
        height: height,
        child: Stack(children: [
          const Positioned.fill(child: ColoredBox(color: Color(0xFF232842))),
          FractionallySizedBox(widthFactor: r, heightFactor: 1, child: ColoredBox(color: color)),
        ]),
      ),
    );
  }
}

/// One page of the "budget vs actual" list; long category lists are split
/// into several carousel pages by the viewer.
class BudgetPage extends StatelessWidget {
  const BudgetPage({super.key, required this.items, required this.money, required this.page, required this.pages});
  final List<TvExpense> items;
  final Money money;
  final int page;
  final int pages;

  static const perPage = 6;

  @override
  Widget build(BuildContext context) {
    return _Panel(
      title: 'Budget vs actual',
      trailing: pages > 1
          ? Text('${page + 1} / $pages', style: const TextStyle(fontSize: 15, color: TvColors.muted))
          : null,
      child: items.isEmpty
          ? const Center(
              child: Text('No budget set for this month', style: TextStyle(fontSize: 20, color: TvColors.muted)))
          : Column(children: [
              for (var i = 0; i < perPage; i++)
                Expanded(child: i < items.length ? _row(items[i]) : const SizedBox.shrink()),
            ]),
    );
  }

  Widget _row(TvExpense e) {
    final color = ratioColor(e.ratio, e.realization);
    final pct = e.ratio == null ? 'no budget' : '${(e.ratio! * 100).round()}%';
    return Column(mainAxisAlignment: MainAxisAlignment.center, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      Row(crossAxisAlignment: CrossAxisAlignment.baseline, textBaseline: TextBaseline.alphabetic, children: [
        Expanded(
          child: Text(e.categoryName, maxLines: 1, overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w600)),
        ),
        Text('${money.format(e.realization)} / ${money.format(e.allocation)}',
            style: const TextStyle(fontSize: 16, color: TvColors.text)),
        const SizedBox(width: 12),
        SizedBox(
          width: 78,
          child: Text(pct,
              textAlign: TextAlign.right,
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: color)),
        ),
      ]),
      const SizedBox(height: 6),
      _Bar(ratio: e.ratio ?? (e.realization > 0 ? 1 : 0), color: color),
    ]);
  }
}

class TrendPage extends StatelessWidget {
  const TrendPage({super.key, required this.data, required this.money});
  final TvData data;
  final Money money;

  @override
  Widget build(BuildContext context) {
    final t = data.trend;
    if (t.isEmpty) {
      return const _Panel(
        title: '6-month trend',
        child: Center(child: Text('No history yet', style: TextStyle(fontSize: 20, color: TvColors.muted))),
      );
    }
    var maxY = 0;
    for (final p in t) {
      if (p.income > maxY) maxY = p.income;
      if (p.expense > maxY) maxY = p.expense;
    }
    final top = maxY <= 0 ? 1.0 : maxY * 1.15;
    return _Panel(
      title: '6-month trend',
      trailing: const Row(mainAxisSize: MainAxisSize.min, children: [
        _Legend(color: TvColors.positive, label: 'Income'),
        SizedBox(width: 16),
        _Legend(color: TvColors.warning, label: 'Spent'),
      ]),
      child: Padding(
        padding: const EdgeInsets.only(top: 8),
        child: BarChart(
          BarChartData(
            maxY: top,
            minY: 0,
            alignment: BarChartAlignment.spaceAround,
            borderData: FlBorderData(show: false),
            barTouchData: BarTouchData(enabled: false),
            gridData: FlGridData(
              show: true,
              drawVerticalLine: false,
              horizontalInterval: top / 4,
              getDrawingHorizontalLine: (_) => const FlLine(color: TvColors.border, strokeWidth: 1),
            ),
            titlesData: FlTitlesData(
              topTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
              rightTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
              leftTitles: AxisTitles(
                sideTitles: SideTitles(
                  showTitles: true,
                  reservedSize: 72,
                  interval: top / 4,
                  getTitlesWidget: (value, meta) => Padding(
                    padding: const EdgeInsets.only(right: 6),
                    child: Text(money.compact(value.round()),
                        textAlign: TextAlign.right, style: const TextStyle(fontSize: 12, color: TvColors.muted)),
                  ),
                ),
              ),
              bottomTitles: AxisTitles(
                sideTitles: SideTitles(
                  showTitles: true,
                  reservedSize: 28,
                  getTitlesWidget: (value, meta) {
                    final i = value.round();
                    if (i < 0 || i >= t.length) return const SizedBox.shrink();
                    final p = t[i];
                    final current = p.year == data.year && p.month == data.month;
                    return Padding(
                      padding: const EdgeInsets.only(top: 6),
                      child: Text(monthShort[p.month - 1],
                          style: TextStyle(
                            fontSize: 15,
                            color: current ? TvColors.text : TvColors.muted,
                            fontWeight: current ? FontWeight.w800 : FontWeight.w500,
                          )),
                    );
                  },
                ),
              ),
            ),
            barGroups: [
              for (var i = 0; i < t.length; i++)
                BarChartGroupData(x: i, barsSpace: 6, barRods: [
                  BarChartRodData(
                    toY: t[i].income.toDouble(),
                    color: TvColors.positive,
                    width: 26,
                    borderRadius: const BorderRadius.vertical(top: Radius.circular(5)),
                  ),
                  BarChartRodData(
                    toY: t[i].expense.toDouble(),
                    color: TvColors.warning,
                    width: 26,
                    borderRadius: const BorderRadius.vertical(top: Radius.circular(5)),
                  ),
                ]),
            ],
          ),
        ),
      ),
    );
  }
}

class _Legend extends StatelessWidget {
  const _Legend({required this.color, required this.label});
  final Color color;
  final String label;

  @override
  Widget build(BuildContext context) => Row(mainAxisSize: MainAxisSize.min, children: [
        Container(width: 12, height: 12, decoration: BoxDecoration(color: color, borderRadius: BorderRadius.circular(3))),
        const SizedBox(width: 6),
        Text(label, style: const TextStyle(fontSize: 14, color: TvColors.muted)),
      ]);
}
