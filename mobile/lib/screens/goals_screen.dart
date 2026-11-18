import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../util/dates.dart';
import '../util/money.dart';
import '../widgets/amount_field.dart';
import '../widgets/async_view.dart';
import '../widgets/stat_card.dart';

/// Savings goals with contribute / withdraw (web: pages/Goals.jsx).
class GoalsScreen extends ConsumerWidget {
  /// True inside the Money tabs (no own app bar).
  final bool embedded;

  const GoalsScreen({super.key, this.embedded = false});

  Future<void> _contribute(BuildContext context, WidgetRef ref, Goal g) async {
    final ctrl = TextEditingController();
    final formKey = GlobalKey<FormState>();
    var withdraw = false;
    final amount = await showDialog<int>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setState) => AlertDialog(
          title: Text(g.name),
          content: Form(
            key: formKey,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                SegmentedButton<bool>(
                  segments: const [
                    ButtonSegment(value: false, label: Text('Add')),
                    ButtonSegment(value: true, label: Text('Withdraw')),
                  ],
                  selected: {withdraw},
                  onSelectionChanged: (s) => setState(() => withdraw = s.first),
                ),
                const SizedBox(height: 12),
                AmountField(controller: ctrl, autofocus: true),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
            FilledButton(
              onPressed: () {
                if (!(formKey.currentState?.validate() ?? false)) return;
                final v = parseAmount(ctrl.text)!;
                Navigator.pop(ctx, withdraw ? -v : v);
              },
              child: const Text('Save'),
            ),
          ],
        ),
      ),
    );
    if (amount == null) return;
    try {
      await ref.read(apiProvider).contributeGoal(g.id, amount);
      ref.invalidate(goalsProvider);
      if (context.mounted) {
        showSnack(context, amount > 0 ? 'Added ${MoneyFormatter.idr(amount)}' : 'Withdrew ${MoneyFormatter.idr(-amount)}');
      }
    } catch (e) {
      if (context.mounted) showSnack(context, 'Could not save: $e', error: true);
    }
  }

  Future<void> _delete(BuildContext context, WidgetRef ref, Goal g) async {
    if (!await confirmDialog(context, title: 'Delete goal "${g.name}"?')) return;
    try {
      await ref.read(apiProvider).deleteGoal(g.id);
      ref.invalidate(goalsProvider);
    } catch (e) {
      if (context.mounted) showSnack(context, 'Could not delete: $e', error: true);
    }
  }

  Future<void> _add(BuildContext context, WidgetRef ref) async {
    final created = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      showDragHandle: true,
      builder: (_) => const _NewGoalSheet(),
    );
    if (created == true) ref.invalidate(goalsProvider);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final goals = ref.watch(goalsProvider);
    final money = ref.watch(moneyProvider);
    final canManage = ref.watch(meProvider)?.canManage ?? false;
    final p = context.palette;

    return Scaffold(
      appBar: embedded ? null : AppBar(title: const Text('Goals')),
      // Embedded in Money the shell's quick-add FAB sits bottom-right.
      floatingActionButtonLocation: embedded ? FloatingActionButtonLocation.startFloat : null,
      floatingActionButton: canManage
          ? FloatingActionButton.extended(
              heroTag: 'new-goal',
              onPressed: () => _add(context, ref),
              icon: const Icon(Icons.flag_outlined),
              label: const Text('New goal'),
            )
          : null,
      body: PullToRefresh(
        onRefresh: () => ref.refresh(goalsProvider.future),
        child: AsyncView<List<Goal>>(
          value: goals,
          onRetry: () => ref.invalidate(goalsProvider),
          builder: (list) {
            if (list.isEmpty) {
              return ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                children: const [
                  EmptyState(
                    icon: Icons.flag_outlined,
                    title: 'No savings goals yet',
                    message: 'Save for a holiday, a new laptop or an emergency fund.',
                  ),
                ],
              );
            }
            final saved = list.fold<int>(0, (a, g) => a + g.progress);
            final target = list.fold<int>(0, (a, g) => a + g.targetAmount);
            return ListView(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: const EdgeInsets.only(top: 8, bottom: 96),
              children: [
                StatGrid(children: [
                  StatCard(label: 'Saved', value: money.format(saved), accent: p.success),
                  StatCard(label: 'Target', value: money.format(target)),
                ]),
                for (final g in list)
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.fromLTRB(16, 12, 8, 12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              Icon(g.reached ? Icons.emoji_events_rounded : Icons.flag_rounded, size: 26, color: g.reached ? p.success : p.primary),
                              const SizedBox(width: 10),
                              Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(g.name, style: TextStyle(fontWeight: FontWeight.w600, fontSize: 16, color: p.text)),
                                    if (g.targetDate != null)
                                      Text('by ${prettyDate(g.targetDate!)}', style: TextStyle(fontSize: 12, color: p.muted)),
                                  ],
                                ),
                              ),
                              if (canManage)
                                PopupMenuButton<String>(
                                  onSelected: (v) {
                                    if (v == 'delete') _delete(context, ref, g);
                                  },
                                  itemBuilder: (_) => const [PopupMenuItem(value: 'delete', child: Text('Delete'))],
                                ),
                            ],
                          ),
                          const SizedBox(height: 10),
                          Padding(
                            padding: const EdgeInsets.only(right: 8),
                            child: KProgressBar(value: g.fraction, color: g.reached ? p.success : p.primary),
                          ),
                          const SizedBox(height: 6),
                          Text(
                            '${money.format(g.progress)} of ${money.format(g.targetAmount)} · ${g.pct.toStringAsFixed(0)}%',
                            style: TextStyle(color: p.text),
                          ),
                          if (g.monthlyNeeded != null && !g.reached)
                            Text('${money.format(g.monthlyNeeded!)}/month needed'
                                '${g.monthsLeft != null ? ' · ${g.monthsLeft} months left' : ''}',
                                style: TextStyle(fontSize: 12, color: p.muted)),
                          if (g.reached) Text('Goal reached', style: TextStyle(color: p.success)),
                          if (canManage)
                            Align(
                              alignment: Alignment.centerRight,
                              child: TextButton.icon(
                                onPressed: () => _contribute(context, ref, g),
                                icon: const Icon(Icons.add_card),
                                label: const Text('Contribute'),
                              ),
                            ),
                        ],
                      ),
                    ),
                  ),
              ],
            );
          },
        ),
      ),
    );
  }
}

class _NewGoalSheet extends ConsumerStatefulWidget {
  const _NewGoalSheet();

  @override
  ConsumerState<_NewGoalSheet> createState() => _NewGoalSheetState();
}

class _NewGoalSheetState extends ConsumerState<_NewGoalSheet> {
  final _form = GlobalKey<FormState>();
  final _name = TextEditingController();
  final _target = TextEditingController();
  DateTime? _date;
  bool _saving = false;
  String? _error;

  @override
  void dispose() {
    _name.dispose();
    _target.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    if (!(_form.currentState?.validate() ?? false)) return;
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      await ref.read(apiProvider).addGoal(
            name: _name.text.trim(),
            targetAmount: parseAmount(_target.text)!,
            targetDate: _date == null ? null : isoDate(_date!),
          );
      if (mounted) Navigator.pop(context, true);
    } catch (e) {
      if (mounted) {
        setState(() {
          _saving = false;
          _error = e.toString();
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    return Padding(
      padding: EdgeInsets.fromLTRB(20, 0, 20, 20 + MediaQuery.of(context).viewInsets.bottom),
      child: Form(
        key: _form,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text('New goal', style: TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: p.text)),
              const SizedBox(height: 12),
              TextFormField(
                controller: _name,
                autofocus: true,
                textCapitalization: TextCapitalization.sentences,
                decoration: const InputDecoration(labelText: 'Name'),
                validator: (v) => (v ?? '').trim().isEmpty ? 'Enter a name' : null,
              ),
              const SizedBox(height: 12),
              AmountField(controller: _target, label: 'Target amount (Rp)'),
              const SizedBox(height: 12),
              OutlinedButton.icon(
                icon: const Icon(Icons.event),
                label: Text(_date == null ? 'Target date (optional)' : 'By ${prettyDate(isoDate(_date!))}'),
                onPressed: () async {
                  final d = await showDatePicker(
                    context: context,
                    initialDate: DateTime.now().add(const Duration(days: 180)),
                    firstDate: DateTime.now(),
                    lastDate: DateTime.now().add(const Duration(days: 365 * 30)),
                  );
                  if (d != null) setState(() => _date = d);
                },
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(_error!, style: TextStyle(color: p.danger)),
              ],
              const SizedBox(height: 20),
              FilledButton(onPressed: _saving ? null : _save, child: const Text('Create goal')),
            ],
          ),
        ),
      ),
    );
  }
}
