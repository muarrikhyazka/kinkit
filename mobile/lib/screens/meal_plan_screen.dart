import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../widgets/async_view.dart';

/// Home > Meals (web: /home/meals, backend-go/meal_plan.go): the weekly menu
/// (this week / next week) and the must-use ingredients. Family admins and
/// parents add / remove ingredients; the chat post schedule is set on the web.

final mealWeekProvider = StateProvider.autoDispose<bool>((ref) => false); // true = this week

final mealPlanProvider = FutureProvider.autoDispose.family<MealPlan, bool>((ref, thisWeek) {
  ref.watch(meProvider.select((m) => m?.memberId));
  return ref.watch(apiProvider).mealPlan(thisWeek: thisWeek);
});

final mealIngredientsProvider = FutureProvider.autoDispose<List<MealIngredient>>((ref) {
  ref.watch(meProvider.select((m) => m?.memberId));
  return ref.watch(apiProvider).mealIngredients();
});

String _shortDate(String iso) {
  final d = DateTime.tryParse(iso);
  if (d == null) return iso;
  const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
  return '${d.day} ${months[d.month - 1]}';
}

class MealsTab extends ConsumerWidget {
  const MealsTab({super.key});

  void _refresh(WidgetRef ref) {
    ref.invalidate(mealPlanProvider);
    ref.invalidate(mealIngredientsProvider);
    ref.invalidate(mealProposalsProvider);
    ref.invalidate(todayProvider);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final thisWeek = ref.watch(mealWeekProvider);
    final plan = ref.watch(mealPlanProvider(thisWeek));
    final canManage = ref.watch(meProvider)?.canManage ?? false;
    return RefreshIndicator(
      onRefresh: () async {
        _refresh(ref);
        await ref.read(mealPlanProvider(thisWeek).future);
      },
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(16, 10, 16, 24),
        children: [
          SegmentedButton<bool>(
            segments: const [
              ButtonSegment(value: true, label: Text('This week')),
              ButtonSegment(value: false, label: Text('Next week')),
            ],
            selected: {thisWeek},
            onSelectionChanged: (v) => ref.read(mealWeekProvider.notifier).state = v.first,
          ),
          const SizedBox(height: 12),
          plan.when(
            skipLoadingOnRefresh: true,
            data: (p) => _PlanCard(plan: p),
            loading: () => const LoadingState(),
            error: (e, _) => Padding(padding: const EdgeInsets.all(16), child: Text('$e')),
          ),
          const SizedBox(height: 12),
          _RequestsCard(days: plan.valueOrNull?.days ?? const [], onChanged: () => _refresh(ref)),
          const SizedBox(height: 12),
          _IngredientsCard(canManage: canManage, onChanged: () => _refresh(ref)),
        ],
      ),
    );
  }
}

class _PlanCard extends StatelessWidget {
  final MealPlan plan;
  const _PlanCard({required this.plan});

  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context);
    final muted = t.colorScheme.onSurfaceVariant;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Weekly menu', style: t.textTheme.titleMedium),
            Text('${_shortDate(plan.weekStart)} to ${_shortDate(plan.weekEnd)}', style: TextStyle(color: muted)),
            if (plan.promoCount > 0)
              Text('Dishes with ingredients on promo are preferred.', style: TextStyle(color: muted, fontSize: 12)),
            const SizedBox(height: 8),
            for (final d in plan.days)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 6),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    SizedBox(
                      width: 64,
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(d.day.substring(0, 3), style: const TextStyle(fontWeight: FontWeight.w600)),
                          Text(_shortDate(d.date), style: TextStyle(color: muted, fontSize: 12)),
                        ],
                      ),
                    ),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          for (final k in d.picks)
                            Text('Asked: ${k.dish}${k.memberName.isEmpty ? '' : ' · ${k.memberName}'}',
                                style: TextStyle(fontWeight: FontWeight.w700, color: context.palette.primary)),
                          Text(d.fried ? '${d.protein} (fried)' : d.protein),
                          Text(d.veggie, style: TextStyle(color: muted)),
                          if (d.promo != null)
                            Text('Promo: ${d.promo!.name} ${d.promo!.price} (${d.promo!.store})',
                                style: TextStyle(color: muted, fontSize: 12)),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            if (plan.missing.isNotEmpty)
              Text('No dish uses: ${plan.missing.join(', ')}', style: TextStyle(color: muted, fontSize: 12)),
          ],
        ),
      ),
    );
  }
}

class _IngredientsCard extends ConsumerStatefulWidget {
  final bool canManage;
  final VoidCallback onChanged;
  const _IngredientsCard({required this.canManage, required this.onChanged});

  @override
  ConsumerState<_IngredientsCard> createState() => _IngredientsCardState();
}

class _IngredientsCardState extends ConsumerState<_IngredientsCard> {
  final _ctrl = TextEditingController();
  bool _busy = false;

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  Future<void> _run(Future<void> Function() call) async {
    setState(() => _busy = true);
    try {
      await call();
      widget.onChanged();
    } catch (e) {
      if (mounted) showSnack(context, '$e', error: true);
    }
    if (mounted) setState(() => _busy = false);
  }

  @override
  Widget build(BuildContext context) {
    final items = ref.watch(mealIngredientsProvider);
    final t = Theme.of(context);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Must-use ingredients', style: t.textTheme.titleMedium),
            Text('Each one gets a dish in the menu.', style: TextStyle(color: t.colorScheme.onSurfaceVariant, fontSize: 12)),
            const SizedBox(height: 8),
            items.when(
              skipLoadingOnRefresh: true,
              data: (list) => list.isEmpty
                  ? const Text('None yet.')
                  : Wrap(
                      spacing: 6,
                      runSpacing: 6,
                      children: [
                        for (final it in list)
                          widget.canManage
                              ? InputChip(
                                  label: Text(it.name),
                                  onDeleted: _busy ? null : () => _run(() => ref.read(apiProvider).deleteMealIngredient(it.id)),
                                  deleteButtonTooltipMessage: 'Remove ${it.name}',
                                )
                              : Chip(label: Text(it.name)),
                      ],
                    ),
              loading: () => const LinearProgressIndicator(),
              error: (e, _) => Text('$e'),
            ),
            if (widget.canManage) ...[
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _ctrl,
                      maxLength: 60,
                      decoration: const InputDecoration(hintText: 'e.g. chicken, tofu', counterText: '', isDense: true),
                      textInputAction: TextInputAction.done,
                      onSubmitted: (_) => _add(),
                    ),
                  ),
                  const SizedBox(width: 8),
                  FilledButton(onPressed: _busy ? null : _add, child: const Text('Add')),
                ],
              ),
            ],
          ],
        ),
      ),
    );
  }

  void _add() {
    final name = _ctrl.text.trim();
    if (name.isEmpty) return;
    _run(() async {
      await ref.read(apiProvider).addMealIngredient(name);
      _ctrl.clear();
    });
  }
}


/// Meal requests (web: Home > Meals > Meal requests): everyone can ask for a
/// dish; family admins and parents approve it (optionally on a day) or decline.
class _RequestsCard extends ConsumerStatefulWidget {
  final List<MealDay> days;
  final VoidCallback onChanged;
  const _RequestsCard({required this.days, required this.onChanged});

  @override
  ConsumerState<_RequestsCard> createState() => _RequestsCardState();
}

class _RequestsCardState extends ConsumerState<_RequestsCard> {
  final _dish = TextEditingController();
  final _note = TextEditingController();
  bool _busy = false;

  @override
  void dispose() {
    _dish.dispose();
    _note.dispose();
    super.dispose();
  }

  Future<void> _run(Future<void> Function() call, {String? ok}) async {
    setState(() => _busy = true);
    try {
      await call();
      widget.onChanged();
      if (ok != null && mounted) showSnack(context, ok);
    } catch (e) {
      if (mounted) showSnack(context, '$e', error: true);
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _approve(MealProposal p) async {
    // Pick a day of the shown week (or none).
    final choice = await showModalBottomSheet<String>(
      context: context,
      showDragHandle: true,
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(title: Text('Approve "${p.dish}" for…', style: const TextStyle(fontWeight: FontWeight.w700))),
            ListTile(leading: const Icon(Icons.event_busy_outlined), title: const Text('No fixed day'), onTap: () => Navigator.pop(ctx, '')),
            for (final d in widget.days)
              ListTile(
                leading: const Icon(Icons.event_outlined),
                title: Text('${d.day} ${_shortDate(d.date)}'),
                selected: d.date == p.wantedDate,
                onTap: () => Navigator.pop(ctx, d.date),
              ),
          ],
        ),
      ),
    );
    if (choice == null) return;
    await _run(() => ref.read(apiProvider).approveMeal(p.id, mealDate: choice.isEmpty ? null : choice), ok: 'Added "${p.dish}"');
  }

  @override
  Widget build(BuildContext context) {
    final t = Theme.of(context);
    final p = context.palette;
    final list = ref.watch(mealProposalsProvider);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Meal requests', style: t.textTheme.titleMedium),
            Text('Want something specific? Ask for it. A parent or the family admin decides.', style: TextStyle(color: p.muted, fontSize: 12)),
            const SizedBox(height: 8),
            TextField(
              controller: _dish,
              maxLength: 100,
              decoration: const InputDecoration(hintText: 'What would you like to eat?', counterText: '', isDense: true),
            ),
            const SizedBox(height: 6),
            Row(children: [
              Expanded(
                child: TextField(
                  controller: _note,
                  maxLength: 300,
                  decoration: const InputDecoration(hintText: 'Note (optional)', counterText: '', isDense: true),
                ),
              ),
              const SizedBox(width: 8),
              FilledButton(
                onPressed: _busy
                    ? null
                    : () {
                        final dish = _dish.text.trim();
                        if (dish.isEmpty) return;
                        _run(() async {
                          await ref.read(apiProvider).proposeMeal(dish, note: _note.text);
                          _dish.clear();
                          _note.clear();
                        }, ok: 'Request sent');
                      },
                child: const Text('Ask'),
              ),
            ]),
            const SizedBox(height: 10),
            list.when(
              skipLoadingOnRefresh: true,
              loading: () => const LinearProgressIndicator(),
              error: (e, _) => Text('$e'),
              data: (items) {
                final pending = items.where((i) => i.pending).toList();
                final decided = items.where((i) => !i.pending).take(6).toList();
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Waiting for a decision', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: p.muted)),
                    if (pending.isEmpty) const Padding(padding: EdgeInsets.symmetric(vertical: 6), child: Text('No requests waiting.')),
                    for (final r in pending)
                      ListTile(
                        contentPadding: EdgeInsets.zero,
                        title: Text(r.dish, style: const TextStyle(fontWeight: FontWeight.w600)),
                        subtitle: Text([
                          r.memberName.isEmpty ? 'Someone' : r.memberName,
                          if (r.wantedDate != null) 'would like it on ${_shortDate(r.wantedDate!)}',
                          if (r.note.isNotEmpty) r.note,
                        ].join(' · ')),
                        trailing: r.canDecide
                            ? Row(mainAxisSize: MainAxisSize.min, children: [
                                IconButton(tooltip: 'Approve', icon: const Icon(Icons.check_circle_outline), onPressed: _busy ? null : () => _approve(r)),
                                IconButton(
                                    tooltip: 'Decline',
                                    icon: const Icon(Icons.cancel_outlined),
                                    onPressed: _busy ? null : () => _run(() => ref.read(apiProvider).declineMeal(r.id), ok: 'Declined')),
                              ])
                            : (r.canDelete
                                ? IconButton(
                                    tooltip: 'Withdraw',
                                    icon: const Icon(Icons.close),
                                    onPressed: _busy ? null : () => _run(() => ref.read(apiProvider).deleteMealProposal(r.id), ok: 'Request withdrawn'))
                                : null),
                      ),
                    if (decided.isNotEmpty) ...[
                      const SizedBox(height: 6),
                      Text('Recently decided', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: p.muted)),
                      for (final r in decided)
                        ListTile(
                          contentPadding: EdgeInsets.zero,
                          dense: true,
                          title: Text(r.dish),
                          subtitle: Text([
                            r.memberName.isEmpty ? 'Someone' : r.memberName,
                            r.status == 'approved' ? 'approved${r.mealDate != null ? ' for ${_shortDate(r.mealDate!)}' : ''}' : 'declined',
                            if (r.decidedByName.isNotEmpty) 'by ${r.decidedByName}',
                            if (r.decisionNote.isNotEmpty) r.decisionNote,
                          ].join(' · ')),
                          trailing: Icon(r.status == 'approved' ? Icons.check_circle : Icons.cancel, color: r.status == 'approved' ? p.success : p.muted, size: 20),
                        ),
                    ],
                  ],
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}
