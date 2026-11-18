import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../state/providers.dart';
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
          _IngredientsCard(canManage: canManage, onChanged: () => _refresh(ref)),
          const SizedBox(height: 12),
          plan.when(
            skipLoadingOnRefresh: true,
            data: (p) => _PlanCard(plan: p),
            loading: () => const LoadingState(),
            error: (e, _) => Padding(padding: const EdgeInsets.all(16), child: Text('$e')),
          ),
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
            Text('${_shortDate(plan.weekStart)} - ${_shortDate(plan.weekEnd)}', style: TextStyle(color: muted)),
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
                      decoration: const InputDecoration(hintText: 'e.g. chicken, tempe', counterText: '', isDense: true),
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
