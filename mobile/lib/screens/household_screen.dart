import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../util/dates.dart';
import '../widgets/async_view.dart';
import '../widgets/k_app_bar.dart';
import 'list_detail_screen.dart';

/// Home section: Reminders / Chores / Lists (web: /home/*), backed by
/// backend-go/reminders.go, chores.go and lists.go.
class HouseholdScreen extends StatelessWidget {
  const HouseholdScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return DefaultTabController(
      length: 3,
      child: Scaffold(
        appBar: kAppBar(
          context,
          'Home',
          bottom: const TabBar(tabs: [Tab(text: 'Reminders'), Tab(text: 'Chores'), Tab(text: 'Lists')]),
        ),
        body: const TabBarView(children: [RemindersTab(), ChoresTab(), ListsTab()]),
      ),
    );
  }
}

Future<void> _run(BuildContext context, WidgetRef ref, Future<void> Function() call, String ok) async {
  try {
    await call();
    invalidateHome(ref);
    if (context.mounted && ok.isNotEmpty) showSnack(context, ok);
  } catch (e) {
    if (context.mounted) showSnack(context, '$e', error: true);
  }
}

// ============================================================ reminders

const _categoryIcons = {
  'bill': Icons.receipt_outlined,
  'medicine': Icons.medication_outlined,
  'school': Icons.school_outlined,
  'birthday': Icons.cake_outlined,
  'chores': Icons.cleaning_services_outlined,
  'other': Icons.notifications_none,
};

/// Reminders: "Upcoming" occurrences (done / snooze) and "All" reminder
/// definitions (pause / delete for owners & parents).
class RemindersTab extends ConsumerStatefulWidget {
  const RemindersTab({super.key});

  @override
  ConsumerState<RemindersTab> createState() => _RemindersTabState();
}

class _RemindersTabState extends ConsumerState<RemindersTab> {
  bool _all = false;

  @override
  Widget build(BuildContext context) {
    final canCreate = ref.watch(meProvider)?.canManage ?? false;
    return Scaffold(
      floatingActionButton: canCreate
          ? FloatingActionButton.extended(
              heroTag: 'add-reminder',
              onPressed: () async {
                final ok = await showModalBottomSheet<bool>(
                  context: context,
                  isScrollControlled: true,
                  useSafeArea: true,
                  showDragHandle: true,
                  builder: (_) => const _NewReminderSheet(),
                );
                if (ok == true) invalidateHome(ref);
              },
              icon: const Icon(Icons.add_alert_outlined),
              label: const Text('Reminder'),
            )
          : null,
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 10, 16, 4),
            child: SegmentedButton<bool>(
              segments: const [
                ButtonSegment(value: false, label: Text('Next 7 days')),
                ButtonSegment(value: true, label: Text('All reminders')),
              ],
              selected: {_all},
              onSelectionChanged: (v) => setState(() => _all = v.first),
            ),
          ),
          Expanded(child: _all ? const _AllReminders() : const _UpcomingReminders()),
        ],
      ),
    );
  }
}

class _UpcomingReminders extends ConsumerWidget {
  const _UpcomingReminders();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final value = ref.watch(remindersProvider);
    final api = ref.read(apiProvider);
    final p = context.palette;
    return PullToRefresh(
        onRefresh: () async {
          ref.invalidate(remindersProvider);
          await ref.read(remindersProvider.future);
        },
        child: AsyncView<List<ReminderOccurrence>>(
          value: value,
          onRetry: () => ref.invalidate(remindersProvider),
          builder: (items) {
            if (items.isEmpty) {
              return ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                children: const [
                  EmptyState(
                    icon: Icons.notifications_none,
                    title: 'Nothing due in the next 7 days',
                    message: 'Bills, medicine, school and birthdays show up here.',
                  ),
                ],
              );
            }
            final groups = <String, List<ReminderOccurrence>>{};
            for (final o in items) {
              final d = o.effective;
              groups.putIfAbsent(d == null ? '' : isoDate(d), () => []).add(o);
            }
            final keys = groups.keys.toList()..sort();
            return ListView(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: const EdgeInsets.only(bottom: 96),
              children: [
                for (final k in keys) ...[
                  SectionTitle(k.isEmpty ? 'Unscheduled' : (k == todayIso() ? 'Today' : prettyDate(k))),
                  Card(
                    child: Column(
                      children: [
                        for (final o in groups[k]!)
                          ListTile(
                            leading: Icon(_categoryIcons[o.category] ?? Icons.notifications_none,
                                color: o.done ? p.success : (o.overdue ? p.danger : p.primary)),
                            title: Text(o.title,
                                style: TextStyle(decoration: o.done ? TextDecoration.lineThrough : null)),
                            subtitle: Text([
                              if (o.effective != null) DateFormat('HH:mm').format(o.effective!),
                              if (o.status == 'snoozed') 'snoozed',
                              if (o.status == 'missed') 'missed',
                              if (o.assigneeName.isNotEmpty) o.assigneeName,
                              if (o.done && o.doneByName.isNotEmpty) 'done by ${o.doneByName}',
                            ].join(' · ')),
                            trailing: o.open && o.canAct
                                ? Row(
                                    mainAxisSize: MainAxisSize.min,
                                    children: [
                                      IconButton(
                                        tooltip: 'Snooze 1 hour',
                                        icon: const Icon(Icons.snooze),
                                        onPressed: () => _run(context, ref, () => api.reminderSnooze(o.id), 'Snoozed for an hour'),
                                      ),
                                      IconButton(
                                        tooltip: 'Done',
                                        icon: const Icon(Icons.check),
                                        onPressed: () => _run(context, ref, () => api.reminderDone(o.id), 'Done'),
                                      ),
                                    ],
                                  )
                                : null,
                          ),
                      ],
                    ),
                  ),
                ],
              ],
            );
          },
        ),
      );
  }
}

class _AllReminders extends ConsumerWidget {
  const _AllReminders();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final value = ref.watch(reminderDefsProvider);
    final canManage = ref.watch(meProvider)?.canManage ?? false;
    final api = ref.read(apiProvider);
    final p = context.palette;
    return PullToRefresh(
      onRefresh: () async {
        ref.invalidate(reminderDefsProvider);
        await ref.read(reminderDefsProvider.future);
      },
      child: AsyncView<List<Reminder>>(
        value: value,
        onRetry: () => ref.invalidate(reminderDefsProvider),
        builder: (list) {
          if (list.isEmpty) {
            return ListView(
              physics: const AlwaysScrollableScrollPhysics(),
              children: const [EmptyState(icon: Icons.notifications_none, title: 'No reminders yet')],
            );
          }
          return ListView(
            physics: const AlwaysScrollableScrollPhysics(),
            padding: const EdgeInsets.only(top: 8, bottom: 96),
            children: [
              Card(
                child: Column(
                  children: [
                    for (final r in list)
                      ListTile(
                        leading: Icon(_categoryIcons[r.category] ?? Icons.notifications_none,
                            color: r.active ? p.primary : p.muted),
                        title: Text(r.title, style: TextStyle(color: r.active ? p.text : p.muted)),
                        subtitle: Text([
                          if (r.description.isNotEmpty) r.description else '${r.kind} ${r.timeOfDay}',
                          if (r.assigneeName.isNotEmpty) r.assigneeName,
                          if (!r.active) 'paused',
                        ].join(' · ')),
                        trailing: canManage
                            ? PopupMenuButton<String>(
                                onSelected: (v) async {
                                  if (v == 'toggle') {
                                    await _run(context, ref, () => api.setReminderActive(r.id, !r.active),
                                        r.active ? 'Paused' : 'Resumed');
                                  } else if (v == 'delete') {
                                    if (await confirmDialog(context, title: 'Delete "${r.title}"?') && context.mounted) {
                                      await _run(context, ref, () => api.deleteReminder(r.id), 'Deleted');
                                    }
                                  }
                                },
                                itemBuilder: (_) => [
                                  PopupMenuItem(value: 'toggle', child: Text(r.active ? 'Pause' : 'Resume')),
                                  const PopupMenuItem(value: 'delete', child: Text('Delete')),
                                ],
                              )
                            : null,
                      ),
                  ],
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _NewReminderSheet extends ConsumerStatefulWidget {
  const _NewReminderSheet();

  @override
  ConsumerState<_NewReminderSheet> createState() => _NewReminderSheetState();
}

class _NewReminderSheetState extends ConsumerState<_NewReminderSheet> {
  final _title = TextEditingController();
  final _notes = TextEditingController();
  String _kind = 'once';
  String _category = 'other';
  DateTime _date = DateTime.now();
  TimeOfDay _time = const TimeOfDay(hour: 9, minute: 0);
  bool _saving = false;
  String? _error;

  @override
  void dispose() {
    _title.dispose();
    _notes.dispose();
    super.dispose();
  }

  String get _hhmm => '${_time.hour.toString().padLeft(2, '0')}:${_time.minute.toString().padLeft(2, '0')}';

  Future<void> _save() async {
    if (_title.text.trim().isEmpty) {
      setState(() => _error = 'Enter a title');
      return;
    }
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      await ref.read(apiProvider).createReminder(
            title: _title.text.trim(),
            kind: _kind,
            startDate: isoDate(_date),
            timeOfDay: _hhmm,
            category: _category,
            notes: _notes.text.trim(),
            // Backend weekdays: 0 = Sunday .. 6 = Saturday (Dart: 7 = Sunday).
            weekdays: _kind == 'weekly' ? [_date.weekday % 7] : null,
            dayOfMonth: (_kind == 'monthly' || _kind == 'yearly') ? _date.day : null,
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
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text('New reminder', style: TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: p.text)),
            const SizedBox(height: 12),
            TextField(
              controller: _title,
              autofocus: true,
              textCapitalization: TextCapitalization.sentences,
              decoration: const InputDecoration(labelText: 'What to remember'),
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: DropdownButtonFormField<String>(
                    value: _kind,
                    decoration: const InputDecoration(labelText: 'Repeats'),
                    items: [
                      for (final k in reminderKinds)
                        DropdownMenuItem(value: k, child: Text(k == 'once' ? 'Once' : k[0].toUpperCase() + k.substring(1))),
                    ],
                    onChanged: (v) => setState(() => _kind = v ?? 'once'),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: DropdownButtonFormField<String>(
                    value: _category,
                    decoration: const InputDecoration(labelText: 'Category'),
                    items: [
                      for (final c in reminderCategories)
                        DropdownMenuItem(value: c, child: Text(c[0].toUpperCase() + c.substring(1))),
                    ],
                    onChanged: (v) => setState(() => _category = v ?? 'other'),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton.icon(
                    icon: const Icon(Icons.event),
                    label: Text(_kind == 'once' ? prettyDate(isoDate(_date)) : 'From ${prettyDate(isoDate(_date))}'),
                    onPressed: () async {
                      final d = await showDatePicker(
                        context: context,
                        initialDate: _date,
                        firstDate: DateTime.now().subtract(const Duration(days: 1)),
                        lastDate: DateTime.now().add(const Duration(days: 365 * 5)),
                      );
                      if (d != null) setState(() => _date = d);
                    },
                  ),
                ),
                const SizedBox(width: 12),
                OutlinedButton.icon(
                  icon: const Icon(Icons.schedule),
                  label: Text(_hhmm),
                  onPressed: () async {
                    final t = await showTimePicker(context: context, initialTime: _time);
                    if (t != null) setState(() => _time = t);
                  },
                ),
              ],
            ),
            const SizedBox(height: 12),
            TextField(controller: _notes, decoration: const InputDecoration(labelText: 'Notes (optional)')),
            if (_error != null) ...[
              const SizedBox(height: 12),
              Text(_error!, style: TextStyle(color: p.danger)),
            ],
            const SizedBox(height: 20),
            FilledButton(onPressed: _saving ? null : _save, child: const Text('Save reminder')),
          ],
        ),
      ),
    );
  }
}

// ============================================================ chores

class ChoresTab extends ConsumerWidget {
  const ChoresTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final value = ref.watch(choresTodayProvider);
    final me = ref.watch(meProvider);
    final api = ref.read(apiProvider);
    final p = context.palette;

    Widget tile(ChoreInstance c, {bool approval = false}) {
      final mine = c.memberId == null || c.memberId == me?.memberId;
      final canAct = c.id != 0 && !c.projected;
      Widget? trailing;
      if (canAct && approval) {
        trailing = Row(mainAxisSize: MainAxisSize.min, children: [
          IconButton(
            tooltip: 'Send back',
            icon: Icon(Icons.undo, color: p.danger),
            onPressed: () => _run(context, ref, () => api.choreAction(c.id, 'reject'), 'Sent back'),
          ),
          IconButton(
            tooltip: 'Approve',
            icon: Icon(Icons.thumb_up_alt_outlined, color: p.success),
            onPressed: () => _run(context, ref, () => api.choreAction(c.id, 'approve'), 'Approved'),
          ),
        ]);
      } else if (canAct && !c.done && !c.submitted && (mine || (me?.canManage ?? false))) {
        trailing = c.requiresPhoto
            ? Tooltip(message: 'Needs a photo - finish it on the web', child: Icon(Icons.photo_camera_outlined, color: p.muted))
            : FilledButton.tonal(
                onPressed: () => _run(context, ref, () => api.choreAction(c.id, 'complete'), 'Nice work!'),
                child: const Text('Done'),
              );
      } else if (canAct && (c.done || c.submitted)) {
        trailing = IconButton(
          tooltip: 'Undo',
          icon: const Icon(Icons.undo),
          onPressed: () => _run(context, ref, () => api.choreAction(c.id, 'undo'), 'Undone'),
        );
      }
      return ListTile(
        leading: Icon(
          c.done ? Icons.check_circle : (c.submitted ? Icons.hourglass_top : Icons.radio_button_unchecked),
          color: c.done ? p.success : (c.overdue ? p.danger : (c.submitted ? p.warning : p.muted)),
        ),
        title: Text(c.label, style: TextStyle(decoration: c.done ? TextDecoration.lineThrough : null)),
        subtitle: Text([
          if (c.memberName.isNotEmpty) c.memberName else 'Anyone',
          if (c.points > 0) '+${c.points} pts',
          if (c.submitted) 'waiting for approval',
          if (c.overdue) 'overdue',
          if (c.dueTime != null) 'by ${c.dueTime}',
        ].join(' · ')),
        trailing: trailing,
      );
    }

    return PullToRefresh(
      onRefresh: () async {
        ref.invalidate(choresTodayProvider);
        await ref.read(choresTodayProvider.future);
      },
      child: AsyncView<ChoresToday>(
        value: value,
        onRetry: () => ref.invalidate(choresTodayProvider),
        builder: (d) => ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.only(bottom: 32),
          children: [
            Card(
              child: ListTile(
                leading: const Text('\u{1F9F9}', style: TextStyle(fontSize: 26)),
                title: Text('This week: ${d.week.done}/${d.week.total} done'),
                subtitle: Text('+${d.week.points} chore points'),
              ),
            ),
            if (d.waiting.isNotEmpty) ...[
              SectionTitle('Waiting for approval (${d.waiting.length})'),
              Card(child: Column(children: [for (final c in d.waiting) tile(c, approval: true)])),
            ],
            const SectionTitle('Today'),
            if (d.items.isEmpty)
              const EmptyState(icon: Icons.cleaning_services_outlined, title: 'No chores today')
            else
              Card(child: Column(children: [for (final c in d.items) tile(c)])),
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 12, 20, 0),
              child: Text('Create and schedule chores on the web.', style: TextStyle(color: p.muted, fontSize: 12)),
            ),
          ],
        ),
      ),
    );
  }
}

// ============================================================ lists

class ListsTab extends ConsumerWidget {
  const ListsTab({super.key});

  Future<void> _newList(BuildContext context, WidgetRef ref) async {
    final ctrl = TextEditingController();
    final name = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('New list'),
        content: TextField(
          controller: ctrl,
          autofocus: true,
          textCapitalization: TextCapitalization.sentences,
          decoration: const InputDecoration(hintText: 'Groceries'),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
          FilledButton(onPressed: () => Navigator.pop(ctx, ctrl.text.trim()), child: const Text('Create')),
        ],
      ),
    );
    if (name == null || name.isEmpty || !context.mounted) return;
    await _run(context, ref, () => ref.read(apiProvider).createList(name), 'List created');
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final value = ref.watch(listsProvider);
    final canCreate = ref.watch(meProvider)?.canManage ?? false;
    final p = context.palette;
    return Scaffold(
      floatingActionButton: canCreate
          ? FloatingActionButton.extended(
              heroTag: 'add-list',
              onPressed: () => _newList(context, ref),
              icon: const Icon(Icons.playlist_add),
              label: const Text('List'),
            )
          : null,
      body: PullToRefresh(
        onRefresh: () async {
          ref.invalidate(listsProvider);
          await ref.read(listsProvider.future);
        },
        child: AsyncView<List<ShoppingList>>(
          value: value,
          onRetry: () => ref.invalidate(listsProvider),
          builder: (lists) {
            final active = lists.where((l) => !l.archived).toList();
            if (active.isEmpty) {
              return ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                children: const [EmptyState(icon: Icons.checklist_outlined, title: 'No lists yet')],
              );
            }
            return ListView(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: const EdgeInsets.only(top: 8, bottom: 96),
              children: [
                for (final l in active)
                  Card(
                    child: ListTile(
                      leading: CircleAvatar(
                        backgroundColor: p.primaryLight,
                        child: Text(l.emoji.isNotEmpty ? l.emoji : '\u{1F6D2}'),
                      ),
                      title: Text(l.name),
                      subtitle: Text([
                        '${l.openCount} to get',
                        if (l.checkedCount > 0) '${l.checkedCount} in cart',
                        if (l.store.isNotEmpty) l.store,
                      ].join(' · ')),
                      trailing: const Icon(Icons.chevron_right),
                      onTap: () => Navigator.of(context)
                          .push(MaterialPageRoute<void>(builder: (_) => ListDetailScreen(listId: l.id, title: l.label))),
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
