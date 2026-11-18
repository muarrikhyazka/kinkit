import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../util/money.dart';
import '../widgets/amount_field.dart';
import '../widgets/async_view.dart';
import '../widgets/month_picker.dart';
import '../widgets/stat_card.dart';

/// Monthly budget plan vs actuals (web: pages/Budgeting.jsx, one month at a
/// time). Family admins/parents can tap a row to change the planned amount
/// (POST /api/budgeting/income | /api/budgeting/expense).
class BudgetScreen extends ConsumerWidget {
  /// True inside the Money tabs (no own app bar).
  final bool embedded;

  const BudgetScreen({super.key, this.embedded = false});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ym = ref.watch(selectedMonthProvider);
    final setup = ref.watch(setupProvider);
    final budget = ref.watch(budgetingProvider(ym.year));
    final actuals = ref.watch(budgetActualsProvider(ym.year));
    final canEdit = ref.watch(meProvider)?.canManage ?? false;

    Future<void> refresh() async {
      ref.invalidate(setupProvider);
      ref.invalidate(budgetActualsProvider(ym.year));
      ref.invalidate(budgetingProvider(ym.year));
      await ref.read(budgetingProvider(ym.year).future);
    }

    Widget content;
    final firstError = setup.error ?? budget.error ?? actuals.error;
    if (setup.hasValue && budget.hasValue && actuals.hasValue) {
      content = _BudgetBody(
        setup: setup.requireValue,
        budget: budget.requireValue,
        actuals: actuals.requireValue,
        month: ym.month,
        year: ym.year,
        canEdit: canEdit,
      );
    } else if (firstError != null) {
      content = ErrorState(message: firstError.toString(), onRetry: refresh);
    } else {
      content = const LoadingState();
    }

    return Scaffold(
      appBar: embedded ? null : AppBar(title: const Text('Budget')),
      body: Column(
        children: [
          const MonthPicker(),
          Expanded(child: PullToRefresh(onRefresh: refresh, child: content)),
        ],
      ),
    );
  }
}

class _BudgetBody extends ConsumerWidget {
  final SetupData setup;
  final Budgeting budget;
  final BudgetActuals actuals;
  final int month;
  final int year;
  final bool canEdit;

  const _BudgetBody({
    required this.setup,
    required this.budget,
    required this.actuals,
    required this.month,
    required this.year,
    required this.canEdit,
  });

  Future<void> _edit(BuildContext context, WidgetRef ref,
      {required String title, required int current, required Future<void> Function(int amount) save}) async {
    final ctrl = TextEditingController(text: current > 0 ? current.toString() : '');
    final formKey = GlobalKey<FormState>();
    final result = await showDialog<int>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(title),
        content: Form(
          key: formKey,
          child: AmountField(controller: ctrl, autofocus: true, label: 'Planned amount (Rp)'),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
          TextButton(onPressed: () => Navigator.pop(ctx, 0), child: const Text('Clear')),
          FilledButton(
            onPressed: () {
              if (formKey.currentState?.validate() ?? false) Navigator.pop(ctx, parseAmount(ctrl.text));
            },
            child: const Text('Save'),
          ),
        ],
      ),
    );
    // ctrl isn't disposed here: the dialog's field may still detach its
    // listener during the closing animation. It holds no native resources.
    if (result == null) return;
    try {
      await save(result);
      ref.invalidate(budgetingProvider(year));
      ref.invalidate(reportProvider);
      if (context.mounted) showSnack(context, 'Budget saved');
    } catch (e) {
      if (context.mounted) showSnack(context, 'Could not save: $e', error: true);
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final money = ref.watch(moneyProvider);
    final api = ref.read(apiProvider);
    final p = context.palette;

    var plannedIncome = 0;
    var receivedIncome = 0;
    for (final s in setup.incomeSources) {
      plannedIncome += budget.plannedIncome(s.id, month);
      receivedIncome += actuals.incomeFor(s.id, month);
    }
    receivedIncome += actuals.incomeFor(null, month);
    var plannedExpense = 0;
    var actualExpense = 0;
    for (final c in setup.categories) {
      plannedExpense += budget.plannedExpense(c.id, month);
      actualExpense += actuals.expenseFor(c.id, month);
    }

    final children = <Widget>[
      StatGrid(children: [
        StatCard(label: 'Planned income', value: money.format(plannedIncome), accent: p.success,
            sub: 'Received ${money.format(receivedIncome)}'),
        StatCard(label: 'Planned spending', value: money.format(plannedExpense),
            sub: 'Spent ${money.format(actualExpense)}', accent: p.primary, negative: actualExpense > plannedExpense),
      ]),
      if (canEdit)
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 2),
          child: Text('Tap a row to change its planned amount.', style: TextStyle(color: p.muted, fontSize: 12)),
        ),
      const SectionTitle('Income'),
      Card(
        child: Column(
          children: [
            if (setup.incomeSources.isEmpty) const ListTile(title: Text('No income sources yet')),
            for (final s in setup.incomeSources)
              _PlanRow(
                name: s.name,
                planned: budget.plannedIncome(s.id, month),
                actual: actuals.incomeFor(s.id, month),
                money: money,
                color: p.success,
                higherIsBetter: true,
                onTap: canEdit
                    ? () => _edit(context, ref,
                        title: '${s.name} income',
                        current: budget.plannedIncome(s.id, month),
                        save: (a) => api.setBudgetIncome(year: year, month: month, sourceId: s.id, amount: a))
                    : null,
              ),
          ],
        ),
      ),
    ];

    setup.categoriesByMain.forEach((main, cats) {
      if (cats.isEmpty) return;
      final planned = cats.fold<int>(0, (a, c) => a + budget.plannedExpense(c.id, month));
      final actual = cats.fold<int>(0, (a, c) => a + actuals.expenseFor(c.id, month));
      children.add(SectionTitle(main,
          trailing: Text('${money.format(actual)} / ${money.format(planned)}',
              style: TextStyle(color: p.muted, fontSize: 12))));
      children.add(Card(
        child: Column(
          children: [
            for (final c in cats)
              _PlanRow(
                name: c.name,
                planned: budget.plannedExpense(c.id, month),
                actual: actuals.expenseFor(c.id, month),
                money: money,
                color: mainCategoryColor(main),
                onTap: canEdit
                    ? () => _edit(context, ref,
                        title: c.name,
                        current: budget.plannedExpense(c.id, month),
                        save: (a) => api.setBudgetExpense(year: year, month: month, categoryId: c.id, amount: a))
                    : null,
              ),
          ],
        ),
      ));
    });

    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.only(bottom: 32),
      children: children,
    );
  }
}

class _PlanRow extends StatelessWidget {
  final String name;
  final int planned;
  final int actual;
  final MoneyFormatter money;
  final Color color;
  final bool higherIsBetter;
  final VoidCallback? onTap;

  const _PlanRow({
    required this.name,
    required this.planned,
    required this.actual,
    required this.money,
    required this.color,
    this.higherIsBetter = false,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final over = !higherIsBetter && planned > 0 && actual > planned;
    final ratio = planned > 0 ? actual / planned : (actual > 0 ? 1.0 : 0.0);
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(child: Text(name, overflow: TextOverflow.ellipsis, style: TextStyle(color: p.text))),
                Text(planned > 0 ? money.format(planned) : '—',
                    style: TextStyle(fontWeight: FontWeight.w600, color: p.text)),
                if (onTap != null) Icon(Icons.edit_outlined, size: 16, color: p.muted),
              ],
            ),
            if (planned > 0 || actual > 0) ...[
              const SizedBox(height: 6),
              KProgressBar(value: ratio, color: color, over: over),
              const SizedBox(height: 2),
              Text(
                '${higherIsBetter ? 'Received' : 'Spent'} ${money.format(actual)}'
                '${planned > 0 ? ' (${(ratio * 100).round()}%)' : ''}',
                style: TextStyle(fontSize: 12, color: over ? p.danger : p.muted),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
