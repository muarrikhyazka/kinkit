import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../util/dates.dart';
import '../widgets/async_view.dart';
import '../widgets/gamification_badge.dart';
import '../widgets/chevron_link.dart';
import '../widgets/k_app_bar.dart';
import '../widgets/mascot.dart';
import '../widgets/stat_card.dart';
import 'spending_screen.dart';

/// Today: greeting, streak, household sections from GET /api/today (when the
/// server has it), today's spending and this month's budget at a glance.
class TodayScreen extends ConsumerWidget {
  const TodayScreen({super.key});

  String _greeting() {
    final h = DateTime.now().hour;
    if (h < 11) return 'Good morning';
    if (h < 15) return 'Good afternoon';
    if (h < 19) return 'Good evening';
    return 'Good night';
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final me = ref.watch(meProvider);
    final p = context.palette;
    final now = YearMonth.now();
    final today = ref.watch(todayProvider);
    final spending = ref.watch(spendingProvider(now));
    final report = ref.watch(reportProvider(now));
    final money = ref.watch(moneyProvider);
    final serverGreeting = today.valueOrNull?.greeting ?? '';

    Future<void> refresh() async {
      ref.invalidate(gamificationProvider);
      ref.invalidate(todayProvider);
      ref.invalidate(reportProvider(now));
      ref.invalidate(spendingProvider(now));
      await ref.read(spendingProvider(now).future);
    }

    return Scaffold(
      appBar: kAppBar(context, 'Today'),
      body: PullToRefresh(
        onRefresh: refresh,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.only(bottom: 96),
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(KSpace.s3, KSpace.s3, 20, KSpace.s1),
              child: Row(
                children: [
                  // Small idle Kiki greets, as on the web Today header.
                  const Mascot(size: 64),
                  const SizedBox(width: KSpace.s2),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(serverGreeting.isNotEmpty ? serverGreeting : '${_greeting()}, ${me?.displayName ?? ''}',
                            style: TextStyle(fontSize: 22, fontWeight: FontWeight.w700, color: p.text)),
                        Text(DateFormat('EEEE, d MMMM y').format(DateTime.now()), style: TextStyle(color: p.muted)),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: KSpace.s2),
            const GamificationCard(),
            _sectionsView(context, ref, today),
            SectionTitle('Spent today',
                trailing: ChevronLink(
                  'All spending',
                  onPressed: () => Navigator.of(context)
                      .push(MaterialPageRoute<void>(builder: (_) => const SpendingScreen(standalone: true))),
                )),
            AsyncView<SpendingPage>(
              value: spending,
              onRetry: () => ref.invalidate(spendingProvider(now)),
              builder: (page) {
                final today = todayIso();
                final rows = page.rows.where((r) => dateOnly(r.txDate) == today).toList();
                if (rows.isEmpty) {
                  return const EmptyState(
                    icon: Icons.savings_outlined,
                    title: 'Nothing logged today',
                    message: 'Tap + to add a purchase and keep your streak going.',
                  );
                }
                final total = rows.fold<int>(0, (a, r) => a + r.amount);
                return Card(
                  child: Column(
                    children: [
                      ListTile(
                        title: const Text('Total'),
                        trailing: Text(money.format(total),
                            style: TextStyle(fontWeight: FontWeight.w700, color: p.text, fontSize: 16, fontFeatures: kTabular)),
                      ),
                      const Divider(),
                      for (final r in rows)
                        ListTile(
                          dense: true,
                          leading: CircleAvatar(radius: 6, backgroundColor: categoryColor(r.categoryName)),
                          title: Text(r.description.isEmpty ? (r.categoryName ?? 'Spending') : r.description),
                          subtitle: Text([r.categoryName ?? 'Other', if (r.memberName != null) r.memberName!].join(' · ')),
                          trailing: Text(money.format(r.amount), style: const TextStyle(fontFeatures: kTabular)),
                        ),
                    ],
                  ),
                );
              },
            ),
            const SectionTitle('This month'),
            AsyncView<MonthReport>(
              value: report,
              onRetry: () => ref.invalidate(reportProvider(now)),
              builder: (r) => StatGrid(children: [
                StatCard(label: 'Spent ${now.shortLabel}', value: money.format(r.expenseActual), accent: p.warning),
                StatCard(
                  label: 'Budget left',
                  value: money.format(r.budgetRemaining),
                  accent: p.success,
                  negative: r.budgetRemaining < 0,
                  sub: r.budgetedExpenses > 0 ? 'of ${money.format(r.budgetedExpenses)} budgeted' : 'No budget set',
                ),
              ]),
            ),
          ],
        ),
      ),
    );
  }

  Widget _sectionsView(BuildContext context, WidgetRef ref, AsyncValue<TodayData?> value) {
    final p = context.palette;
    if (value.isLoading && !value.hasValue) return const SizedBox.shrink();
    if (value.hasError && !value.hasValue) {
      return Card(
        child: ListTile(
          leading: const Mascot(pose: MascotPose.think, size: 44, animate: false),
          title: const Text("Couldn't load today's list"),
          subtitle: Text('${value.error}'),
        ),
      );
    }
    final data = value.valueOrNull;
    if (data == null) {
      // Server has no /api/today yet (household features not deployed).
      return Card(
        child: ListTile(
          leading: Icon(Icons.info_outline, color: p.muted),
          title: const Text('Reminders, chores & lists'),
          subtitle: const Text('Not available on this server yet.'),
        ),
      );
    }
    final visible = data.sections.where((s) => s.items.isNotEmpty || s.summary.isNotEmpty).toList();
    if (visible.isEmpty) {
      return const Card(
        child: EmptyState(
          mascot: MascotPose.relax,
          title: 'All clear for today',
          message: 'No reminders or chores waiting. Enjoy the calm.',
        ),
      );
    }
    return Column(
      children: [
        for (final s in visible) ...[
          SectionTitle(s.title),
          Card(
            child: Column(
              children: [
                if (s.summary.isNotEmpty) ListTile(dense: true, title: Text(s.summary, style: TextStyle(color: p.muted))),
                for (final it in s.items)
                  ListTile(
                    leading: Icon(
                      it.done ? Icons.check_circle : Icons.radio_button_unchecked,
                      color: it.done ? p.success : (it.overdue ? p.danger : p.muted),
                    ),
                    title: Text(
                      it.title,
                      style: TextStyle(decoration: it.done ? TextDecoration.lineThrough : null),
                    ),
                    subtitle: _itemSubtitle(it, p),
                    trailing: _itemActions(context, ref, it),
                  ),
              ],
            ),
          ),
        ],
      ],
    );
  }

  /// Item actions advertised by the server ("done", "snooze", "approve").
  Widget? _itemActions(BuildContext context, WidgetRef ref, TodayItem it) {
    if (it.done || it.id == 0 || it.actions.isEmpty) return null;
    final api = ref.read(apiProvider);
    Future<void> run(Future<void> Function() call, String ok) async {
      try {
        await call();
        invalidateHome(ref);
        if (context.mounted) showSnack(context, ok);
      } catch (e) {
        if (context.mounted) showSnack(context, '$e', error: true);
      }
    }

    final buttons = <Widget>[];
    for (final a in it.actions) {
      if (a == 'done' && it.kind == 'reminder') {
        buttons.add(IconButton(
            tooltip: 'Done', icon: const Icon(Icons.check), onPressed: () => run(() => api.reminderDone(it.id), 'Done')));
      } else if (a == 'snooze' && it.kind == 'reminder') {
        buttons.add(IconButton(
            tooltip: 'Snooze 1 hour',
            icon: const Icon(Icons.snooze),
            onPressed: () => run(() => api.reminderSnooze(it.id), 'Snoozed for an hour')));
      } else if (a == 'done' && it.kind == 'chore') {
        buttons.add(IconButton(
            tooltip: 'Done',
            icon: const Icon(Icons.check),
            onPressed: () => run(() => api.choreAction(it.id, 'complete'), 'Nice work!')));
      } else if (a == 'approve' && it.kind == 'chore') {
        buttons.add(IconButton(
            tooltip: 'Approve',
            icon: const Icon(Icons.thumb_up_alt_outlined),
            onPressed: () => run(() => api.choreAction(it.id, 'approve'), 'Approved')));
      }
    }
    if (buttons.isEmpty) return null;
    return Row(mainAxisSize: MainAxisSize.min, children: buttons);
  }

  Widget? _itemSubtitle(TodayItem it, KinkitPalette p) {
    final parts = <String>[
      if (it.detail.isNotEmpty) it.detail,
      if (it.memberName.isNotEmpty) it.memberName,
      if (it.overdue) 'Overdue',
    ];
    if (parts.isEmpty) return null;
    return Text(parts.join(' · '), style: TextStyle(color: it.overdue ? p.danger : p.muted));
  }
}
