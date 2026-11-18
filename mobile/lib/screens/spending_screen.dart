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

/// Spending list for the selected month, grouped by day, with
/// swipe-to-delete and a details sheet.
class SpendingScreen extends ConsumerStatefulWidget {
  /// True when pushed as its own route (adds an app bar).
  final bool standalone;

  const SpendingScreen({super.key, this.standalone = false});

  @override
  ConsumerState<SpendingScreen> createState() => _SpendingScreenState();
}

class _SpendingScreenState extends ConsumerState<SpendingScreen> {
  String _query = '';
  // Rows deleted locally; hidden until the list refetches (keeps a
  // dismissed Dismissible out of the tree while the old data is shown).
  final Set<int> _deletedIds = {};

  bool _canModify(Me? me, SpendingRow r) {
    if (me == null) return false;
    if (me.canManage) return true;
    return me.isKid && r.memberId != null && r.memberId == me.memberId;
  }

  Future<bool> _delete(SpendingRow r) async {
    final ok = await confirmDialog(context,
        title: 'Delete spending?', message: '${r.description.isEmpty ? r.categoryName ?? '' : r.description} · ${MoneyFormatter.idr(r.amount)}');
    if (!ok) return false;
    try {
      await ref.read(apiProvider).deleteSpending(r.id);
      if (mounted) setState(() => _deletedIds.add(r.id));
      invalidateSpending(ref);
      if (mounted) showSnack(context, 'Deleted');
      return true;
    } catch (e) {
      if (mounted) showSnack(context, 'Could not delete: $e', error: true);
      return false;
    }
  }

  Future<void> _toggleChecked(SpendingRow r) async {
    try {
      await ref.read(apiProvider).updateSpending(r.id, {'checked': r.checked ? 0 : 1});
      ref.invalidate(spendingProvider);
    } catch (e) {
      if (mounted) showSnack(context, 'Could not update: $e', error: true);
    }
  }

  void _details(SpendingRow r, MoneyFormatter money, bool canModify) {
    showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      builder: (ctx) {
        final p = ctx.palette;
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 16),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(money.format(r.amount),
                    style: TextStyle(fontSize: 28, fontWeight: FontWeight.w700, color: p.text, fontFeatures: kTabular)),
                const SizedBox(height: 4),
                Text(r.description.isEmpty ? '(no description)' : r.description, style: TextStyle(color: p.text)),
                const SizedBox(height: 12),
                _kv(ctx, 'Date', prettyDate(r.txDate)),
                _kv(ctx, 'Category', [r.categoryName ?? 'Other', if (r.categoryMaster != null) '(${r.categoryMaster})'].join(' ')),
                if (r.memberName != null) _kv(ctx, 'Logged by', r.memberName!),
                _kv(ctx, 'Source', r.source),
                if (r.hasReceipt) _kv(ctx, 'Receipt', 'Attached (view on web)'),
                const SizedBox(height: 16),
                if (canModify)
                  Row(
                    children: [
                      Expanded(
                        child: OutlinedButton.icon(
                          icon: Icon(r.checked ? Icons.check_box : Icons.check_box_outline_blank),
                          label: Text(r.checked ? 'Checked' : 'Mark checked'),
                          onPressed: () {
                            Navigator.pop(ctx);
                            _toggleChecked(r);
                          },
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: FilledButton.icon(
                          style: FilledButton.styleFrom(backgroundColor: p.danger, foregroundColor: p.onPrimary),
                          icon: const Icon(Icons.delete_outline),
                          label: const Text('Delete'),
                          onPressed: () {
                            Navigator.pop(ctx);
                            _delete(r);
                          },
                        ),
                      ),
                    ],
                  ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _kv(BuildContext ctx, String k, String v) {
    final p = ctx.palette;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(width: 90, child: Text(k, style: TextStyle(color: p.muted))),
          Expanded(child: Text(v, style: TextStyle(color: p.text))),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final ym = ref.watch(selectedMonthProvider);
    final value = ref.watch(spendingProvider(ym));
    final money = ref.watch(moneyProvider);
    final me = ref.watch(meProvider);
    final p = context.palette;

    final body = Column(
      children: [
        const MonthPicker(),
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
          child: TextField(
            decoration: const InputDecoration(
              hintText: 'Search description or category',
              prefixIcon: Icon(Icons.search),
              isDense: true,
            ),
            onChanged: (v) => setState(() => _query = v.trim().toLowerCase()),
          ),
        ),
        Expanded(
          child: PullToRefresh(
            onRefresh: () => ref.refresh(spendingProvider(ym).future),
            child: AsyncView<SpendingPage>(
              value: value,
              onRetry: () => ref.invalidate(spendingProvider(ym)),
              builder: (page) {
                final rows = page.rows
                    .where((r) => !_deletedIds.contains(r.id))
                    .where((r) =>
                        _query.isEmpty ||
                        r.description.toLowerCase().contains(_query) ||
                        (r.categoryName ?? '').toLowerCase().contains(_query))
                    .toList();
                if (rows.isEmpty) {
                  return ListView(
                    physics: const AlwaysScrollableScrollPhysics(),
                    children: [
                      EmptyState(
                        icon: Icons.receipt_long_outlined,
                        title: _query.isEmpty ? 'No spending in ${ym.label}' : 'No matches',
                        action: (me?.canAddSpending ?? false) && _query.isEmpty
                            ? FilledButton.icon(
                                onPressed: () => showQuickAdd(context, allowIncome: me?.canManage ?? false),
                                icon: const Icon(Icons.add),
                                label: const Text('Add spending'),
                              )
                            : null,
                      ),
                    ],
                  );
                }
                // Group by day (rows are newest first).
                final groups = <String, List<SpendingRow>>{};
                for (final r in rows) {
                  groups.putIfAbsent(dateOnly(r.txDate), () => []).add(r);
                }
                final total = rows.fold<int>(0, (a, r) => a + r.amount);
                return ListView(
                  physics: const AlwaysScrollableScrollPhysics(),
                  padding: const EdgeInsets.only(bottom: 96),
                  children: [
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 4),
                      child: Row(
                        children: [
                          Expanded(
                            child: Text(
                              page.total > page.rows.length
                                  ? 'Showing ${page.rows.length} of ${page.total}'
                                  : '${rows.length} transactions',
                              style: TextStyle(color: p.muted),
                            ),
                          ),
                          Text(money.format(total),
                              style: TextStyle(fontWeight: FontWeight.w700, color: p.text, fontFeatures: kTabular)),
                        ],
                      ),
                    ),
                    for (final day in groups.keys) ...[
                      Padding(
                        padding: const EdgeInsets.fromLTRB(20, 12, 20, 4),
                        child: Row(
                          children: [
                            Expanded(
                              child: Text(prettyDate(day),
                                  style: TextStyle(fontWeight: FontWeight.w600, color: p.muted, fontSize: 13)),
                            ),
                            Text(money.format(groups[day]!.fold<int>(0, (a, r) => a + r.amount)),
                                style: TextStyle(color: p.muted, fontSize: 13, fontFeatures: kTabular)),
                          ],
                        ),
                      ),
                      Card(
                        child: Column(
                          children: [
                            for (final r in groups[day]!) _row(r, money, _canModify(me, r)),
                          ],
                        ),
                      ),
                    ],
                  ],
                );
              },
            ),
          ),
        ),
      ],
    );

    if (!widget.standalone) return body;
    return Scaffold(appBar: AppBar(title: const Text('Spending')), body: body);
  }

  Widget _row(SpendingRow r, MoneyFormatter money, bool canModify) {
    final p = context.palette;
    final tile = ListTile(
      onTap: () => _details(r, money, canModify),
      leading: CircleAvatar(
        backgroundColor: categoryColor(r.categoryName).withValues(alpha: 0.15),
        child: Text(
          (r.categoryName ?? '?').isEmpty ? '?' : (r.categoryName ?? '?').substring(0, 1).toUpperCase(),
          style: TextStyle(color: categoryColor(r.categoryName), fontWeight: FontWeight.w700),
        ),
      ),
      title: Text(r.description.isEmpty ? (r.categoryName ?? 'Spending') : r.description, overflow: TextOverflow.ellipsis),
      subtitle: Text([r.categoryName ?? 'Other', if (r.memberName != null) r.memberName!].join(' · '),
          overflow: TextOverflow.ellipsis),
      trailing: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          Text(money.format(r.amount), style: TextStyle(fontWeight: FontWeight.w600, color: p.text, fontFeatures: kTabular)),
          if (r.checked) Icon(Icons.check, size: 14, color: p.success),
        ],
      ),
    );
    if (!canModify) return tile;
    return Dismissible(
      key: ValueKey('sp-${r.id}'),
      direction: DismissDirection.endToStart,
      background: Container(
        color: p.danger,
        alignment: Alignment.centerRight,
        padding: const EdgeInsets.symmetric(horizontal: 20),
        child: const Icon(Icons.delete_outline, color: Colors.white),
      ),
      confirmDismiss: (_) => _delete(r),
      child: tile,
    );
  }
}
