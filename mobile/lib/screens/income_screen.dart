import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../util/dates.dart';
import '../util/money.dart';
import '../widgets/async_view.dart';
import '../widgets/month_picker.dart';
import '../widgets/quick_add_sheet.dart';
import '../widgets/stat_card.dart';

/// Income entries for the selected month (web: pages/Income.jsx).
class IncomeScreen extends ConsumerStatefulWidget {
  const IncomeScreen({super.key});

  @override
  ConsumerState<IncomeScreen> createState() => _IncomeScreenState();
}

class _IncomeScreenState extends ConsumerState<IncomeScreen> {
  final Set<int> _deletedIds = {};

  Future<bool> _delete(IncomeEntry e) async {
    final ok = await confirmDialog(context,
        title: 'Delete income?', message: '${e.sourceName ?? 'Income'} · ${MoneyFormatter.idr(e.amount)}');
    if (!ok) return false;
    try {
      await ref.read(apiProvider).deleteIncome(e.id);
      if (mounted) setState(() => _deletedIds.add(e.id));
      invalidateIncome(ref);
      return true;
    } catch (err) {
      if (mounted) showSnack(context, 'Could not delete: $err', error: true);
      return false;
    }
  }

  @override
  Widget build(BuildContext context) {
    final ym = ref.watch(selectedMonthProvider);
    final value = ref.watch(incomeProvider(ym));
    final money = ref.watch(moneyProvider);
    final canManage = ref.watch(meProvider)?.canManage ?? false;
    final p = context.palette;

    return Column(
      children: [
        const MonthPicker(),
        Expanded(
          child: PullToRefresh(
            onRefresh: () => ref.refresh(incomeProvider(ym).future),
            child: AsyncView<IncomeList>(
              value: value,
              onRetry: () => ref.invalidate(incomeProvider(ym)),
              builder: (data) {
                final rows = data.rows.where((r) => !_deletedIds.contains(r.id)).toList();
                return ListView(
                  physics: const AlwaysScrollableScrollPhysics(),
                  padding: const EdgeInsets.only(bottom: 96),
                  children: [
                    StatGrid(children: [
                      StatCard(label: 'Received', value: money.format(data.total), accent: p.success),
                      StatCard(
                        label: 'Planned',
                        value: money.format(data.planned),
                        sub: data.planned > 0 ? '${(data.total / data.planned * 100).round()}% received' : 'Set it in Budget',
                      ),
                    ]),
                    if (data.bySource.isNotEmpty)
                      Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 16),
                        child: Wrap(
                          spacing: 8,
                          runSpacing: 4,
                          children: [
                            for (final s in data.bySource)
                              Chip(label: Text('${s.sourceName ?? 'No source'}: ${money.format(s.total)}')),
                          ],
                        ),
                      ),
                    SectionTitle(
                      'Entries',
                      trailing: canManage
                          ? TextButton.icon(
                              onPressed: () => showQuickAdd(context, initial: QuickAddKind.income, allowIncome: true),
                              icon: const Icon(Icons.add),
                              label: const Text('Add income'),
                            )
                          : null,
                    ),
                    if (rows.isEmpty)
                      EmptyState(icon: Icons.payments_outlined, title: 'No income recorded in ${ym.label}')
                    else
                      Card(
                        child: Column(
                          children: [for (final e in rows) _row(e, money, canManage)],
                        ),
                      ),
                  ],
                );
              },
            ),
          ),
        ),
      ],
    );
  }

  Widget _row(IncomeEntry e, MoneyFormatter money, bool canManage) {
    final p = context.palette;
    final tile = ListTile(
      leading: CircleAvatar(
        backgroundColor: p.success.withValues(alpha: 0.15),
        child: Icon(Icons.south_west, color: p.success, size: 18),
      ),
      title: Text(e.description.isNotEmpty ? e.description : (e.sourceName ?? 'Income'), overflow: TextOverflow.ellipsis),
      subtitle: Text('${prettyDate(e.txDate)} · ${e.sourceName ?? 'No source'}'),
      trailing: Text(money.format(e.amount),
          style: TextStyle(fontWeight: FontWeight.w600, color: p.success, fontFeatures: kTabular)),
    );
    if (!canManage) return tile;
    return Dismissible(
      key: ValueKey('inc-${e.id}'),
      direction: DismissDirection.endToStart,
      background: Container(
        color: p.danger,
        alignment: Alignment.centerRight,
        padding: const EdgeInsets.symmetric(horizontal: 20),
        child: const Icon(Icons.delete_outline, color: Colors.white),
      ),
      confirmDismiss: (_) => _delete(e),
      child: tile,
    );
  }
}
