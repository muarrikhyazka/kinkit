import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../util/money.dart';
import '../widgets/amount_field.dart';
import '../widgets/async_view.dart';

/// One shopping list: tick items, add free text ("milk, 2 eggs"), remove,
/// clear ticked. Kids may only tick items (enforced by the server).
class ListDetailScreen extends ConsumerStatefulWidget {
  final int listId;
  final String title;

  const ListDetailScreen({super.key, required this.listId, required this.title});

  @override
  ConsumerState<ListDetailScreen> createState() => _ListDetailScreenState();
}

class _ListDetailScreenState extends ConsumerState<ListDetailScreen> {
  final _add = TextEditingController();
  bool _adding = false;

  @override
  void dispose() {
    _add.dispose();
    super.dispose();
  }

  void _refresh() {
    ref.invalidate(listDetailProvider(widget.listId));
    ref.invalidate(listsProvider);
    ref.invalidate(todayProvider);
  }

  Future<void> _call(Future<void> Function() fn) async {
    try {
      await fn();
      _refresh();
    } catch (e) {
      if (mounted) showSnack(context, '$e', error: true);
    }
  }

  Future<void> _addItems() async {
    final text = _add.text.trim();
    if (text.isEmpty) return;
    setState(() => _adding = true);
    await _call(() => ref.read(apiProvider).addListItems(widget.listId, text));
    if (mounted) {
      _add.clear();
      setState(() => _adding = false);
    }
  }

  /// "Finish shopping": log the ticked items as one spending entry.
  Future<void> _finish(int checkedCount) async {
    final ctrl = TextEditingController();
    final formKey = GlobalKey<FormState>();
    final amount = await showDialog<int>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Finish shopping'),
        content: Form(
          key: formKey,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Logs $checkedCount ticked item(s) as one spending entry and removes them from the list.'),
              const SizedBox(height: 12),
              AmountField(controller: ctrl, autofocus: true, label: 'Total paid (Rp)'),
            ],
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
          FilledButton(
            onPressed: () {
              if (formKey.currentState?.validate() ?? false) Navigator.pop(ctx, parseAmount(ctrl.text));
            },
            child: const Text('Log spending'),
          ),
        ],
      ),
    );
    if (amount == null || amount <= 0) return;
    try {
      await ref.read(apiProvider).finishShopping(widget.listId, totalAmount: amount);
      _refresh();
      invalidateSpending(ref);
      if (mounted) showSnack(context, 'Logged ${MoneyFormatter.idr(amount)} as spending');
    } catch (e) {
      if (mounted) showSnack(context, '$e', error: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final value = ref.watch(listDetailProvider(widget.listId));
    final me = ref.watch(meProvider);
    final canEdit = me?.canManage ?? false; // delete / clear
    final canAdd = me?.canAddSpending ?? false; // everyone but viewers adds & ticks
    final api = ref.read(apiProvider);
    final p = context.palette;

    final checkedCount = value.valueOrNull?.items.where((i) => i.checked).length ?? 0;
    return Scaffold(
      floatingActionButton: canEdit && checkedCount > 0
          ? FloatingActionButton.extended(
              onPressed: () => _finish(checkedCount),
              icon: const Icon(Icons.shopping_cart_checkout),
              label: const Text('Finish shopping'),
            )
          : null,
      appBar: AppBar(
        title: Text(widget.title),
        actions: [
          if (canEdit)
            IconButton(
              tooltip: 'Clear ticked items',
              icon: const Icon(Icons.cleaning_services_outlined),
              onPressed: () => _call(() => api.clearChecked(widget.listId)),
            ),
        ],
      ),
      body: Column(
        children: [
          if (canAdd)
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
              child: TextField(
                controller: _add,
                textInputAction: TextInputAction.done,
                onSubmitted: (_) => _addItems(),
                decoration: InputDecoration(
                  hintText: 'Add items, e.g. "milk, 2 eggs"',
                  prefixIcon: const Icon(Icons.add),
                  suffixIcon: _adding
                      ? const Padding(padding: EdgeInsets.all(12), child: CircularProgressIndicator(strokeWidth: 2))
                      : IconButton(icon: const Icon(Icons.send), onPressed: _addItems),
                ),
              ),
            ),
          Expanded(
            child: PullToRefresh(
              onRefresh: () async {
                ref.invalidate(listDetailProvider(widget.listId));
                await ref.read(listDetailProvider(widget.listId).future);
              },
              child: AsyncView<ListDetail>(
                value: value,
                onRetry: _refresh,
                builder: (d) {
                  if (d.items.isEmpty) {
                    return ListView(
                      physics: const AlwaysScrollableScrollPhysics(),
                      children: const [EmptyState(icon: Icons.shopping_basket_outlined, title: 'This list is empty')],
                    );
                  }
                  final open = d.items.where((i) => !i.checked).toList();
                  final done = d.items.where((i) => i.checked).toList();
                  Widget row(ListItem i) {
                    final tile = CheckboxListTile(
                      value: i.checked,
                      controlAffinity: ListTileControlAffinity.leading,
                      onChanged: canAdd ? (v) => _call(() => api.setListItemChecked(widget.listId, i.id, v ?? false)) : null,
                      title: Text(
                        [if (i.qty.isNotEmpty) i.qty, i.name].join(' '),
                        style: TextStyle(
                          decoration: i.checked ? TextDecoration.lineThrough : null,
                          color: i.checked ? p.muted : p.text,
                        ),
                      ),
                      subtitle: (i.note.isNotEmpty || i.estPrice != null || i.checkedByName != null)
                          ? Text([
                              if (i.note.isNotEmpty) i.note,
                              if (i.estPrice != null) '~${MoneyFormatter.idr(i.estPrice!)}',
                              if (i.checked && i.checkedByName != null) 'by ${i.checkedByName}',
                            ].join(' · '))
                          : null,
                    );
                    if (!canEdit) return tile;
                    return Dismissible(
                      key: ValueKey('li-${i.id}'),
                      direction: DismissDirection.endToStart,
                      background: Container(
                        color: p.danger,
                        alignment: Alignment.centerRight,
                        padding: const EdgeInsets.symmetric(horizontal: 20),
                        child: const Icon(Icons.delete_outline, color: Colors.white),
                      ),
                      // Remove after the server confirms; return false so the
                      // tile isn't dismissed while old data is still shown.
                      confirmDismiss: (_) async {
                        await _call(() => api.deleteListItem(widget.listId, i.id));
                        return false;
                      },
                      child: tile,
                    );
                  }

                  return ListView(
                    physics: const AlwaysScrollableScrollPhysics(),
                    padding: const EdgeInsets.only(bottom: 96),
                    children: [
                      if (open.isNotEmpty) Card(child: Column(children: [for (final i in open) row(i)])),
                      if (done.isNotEmpty) ...[
                        SectionTitle('In the cart (${done.length})'),
                        Card(child: Column(children: [for (final i in done) row(i)])),
                      ],
                    ],
                  );
                },
              ),
            ),
          ),
        ],
      ),
    );
  }
}
