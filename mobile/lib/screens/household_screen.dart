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
import '../widgets/mascot.dart';
import 'calendar_screen.dart';
import 'list_detail_screen.dart';
import 'meal_plan_screen.dart';

/// Home section: Reminders / Calendar / Chores / Lists / Meals (web: /reminders,
/// /calendar, /home/chores, /home/lists, /home/meals; the phone keeps reminders as a
/// Home tab), backed by backend-go/reminders.go, chores.go, lists.go and
/// meal_plan.go.
class HouseholdScreen extends StatelessWidget {
  const HouseholdScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return DefaultTabController(
      length: 5,
      child: Scaffold(
        appBar: kAppBar(
          context,
          'Home',
          bottom: const TabBar(
            isScrollable: true,
            tabAlignment: TabAlignment.start,
            tabs: [Tab(text: 'Reminders'), Tab(text: 'Calendar'), Tab(text: 'Chores'), Tab(text: 'Lists'), Tab(text: 'Meals')],
          ),
        ),
        body: const TabBarView(children: [RemindersTab(), CalendarTab(), ChoresTab(), ListsTab(), MealsTab()]),
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
  'cleaning': Icons.cleaning_services_outlined,
  'laundry': Icons.local_laundry_service_outlined,
  'cooking': Icons.soup_kitchen_outlined,
  'groceries': Icons.shopping_basket_outlined,
  'home_maintenance': Icons.handyman_outlined,
  'vehicle': Icons.two_wheeler_outlined,
  'self_care': Icons.spa_outlined,
  'chores': Icons.checklist_outlined,
  'bill': Icons.receipt_outlined,
  'medicine': Icons.medication_outlined,
  'school': Icons.school_outlined,
  'birthday': Icons.cake_outlined,
  'other': Icons.notifications_none,
};

IconData reminderCategoryIcon(String id) => _categoryIcons[id] ?? Icons.notifications_none;

Future<void> _openReminderSheet(BuildContext context, WidgetRef ref) async {
  final ok = await showModalBottomSheet<bool>(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    showDragHandle: true,
    builder: (_) => const NewReminderSheet(),
  );
  if (ok == true) invalidateHome(ref);
}

/// Reminders: "Upcoming" occurrences (done / snooze) and "All" reminder
/// definitions grouped by category (pause / delete for family admins & parents).
class RemindersTab extends ConsumerStatefulWidget {
  const RemindersTab({super.key});

  @override
  ConsumerState<RemindersTab> createState() => _RemindersTabState();
}

class _RemindersTabState extends ConsumerState<RemindersTab> {
  int _view = 0; // 0 = next 7 days, 1 = all reminders, 2 = calendar

  @override
  Widget build(BuildContext context) {
    final canCreate = ref.watch(meProvider)?.canManage ?? false;
    return Scaffold(
      floatingActionButton: canCreate
          ? FloatingActionButton.extended(
              heroTag: 'add-reminder',
              onPressed: () => _openReminderSheet(context, ref),
              icon: const Icon(Icons.add_alert_outlined),
              label: const Text('Reminder'),
            )
          : null,
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 10, 16, 4),
            child: SegmentedButton<int>(
              segments: const [
                ButtonSegment(value: 0, label: Text('Next 7 days')),
                ButtonSegment(value: 1, label: Text('All')),
                ButtonSegment(value: 2, label: Text('Calendar'), icon: Icon(Icons.calendar_month_outlined)),
              ],
              selected: {_view},
              onSelectionChanged: (v) => setState(() => _view = v.first),
            ),
          ),
          Expanded(
            child: switch (_view) {
              1 => const _AllReminders(),
              2 => const CalendarPanel(activities: false, reminders: true),
              _ => const _UpcomingReminders(),
            },
          ),
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
    final canCreate = ref.watch(meProvider)?.canManage ?? false;
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
                children: [
                  const EmptyState(
                    mascot: MascotPose.relax,
                    title: 'Nothing due in the next 7 days',
                    message: 'Household routines, bills and birthdays show up here.',
                  ),
                  if (canCreate)
                    Center(
                      child: OutlinedButton.icon(
                        icon: const Icon(Icons.auto_awesome_outlined),
                        label: const Text('Pick a household template'),
                        onPressed: () => _openReminderSheet(context, ref),
                      ),
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
                            leading: Icon(reminderCategoryIcon(o.category),
                                color: o.done ? p.success : (o.overdue ? p.danger : p.primary)),
                            title: Text(o.title,
                                style: TextStyle(decoration: o.done ? TextDecoration.lineThrough : null)),
                            subtitle: Text([
                              if (o.effective != null) DateFormat('HH:mm').format(o.effective!),
                              reminderCategoryLabel(o.category),
                              if (o.status == 'snoozed') 'snoozed',
                              if (o.status == 'missed') 'missed',
                              if (o.assigneeName.isNotEmpty) o.assigneeName else 'Whole family',
                              if (o.done && o.doneByName.isNotEmpty) 'done by ${o.doneByName}',
                            ].join(' · ')),
                            trailing: o.open && o.canAct
                                ? Row(
                                    mainAxisSize: MainAxisSize.min,
                                    children: [
                                      PopupMenuButton<String>(
                                        tooltip: 'Snooze',
                                        icon: const Icon(Icons.snooze),
                                        onSelected: (v) => _run(
                                            context,
                                            ref,
                                            () => v == 'tomorrow' ? api.reminderSnoozeTomorrow(o.id) : api.reminderSnooze(o.id),
                                            v == 'tomorrow' ? 'Snoozed until tomorrow' : 'Snoozed for an hour'),
                                        itemBuilder: (_) => const [
                                          PopupMenuItem(value: 'hour', child: Text('Snooze 1 hour')),
                                          PopupMenuItem(value: 'tomorrow', child: Text('Snooze until tomorrow')),
                                        ],
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
          final byCat = <String, List<Reminder>>{};
          for (final r in list) {
            byCat.putIfAbsent(reminderCategories.contains(r.category) ? r.category : 'other', () => []).add(r);
          }
          return ListView(
            physics: const AlwaysScrollableScrollPhysics(),
            padding: const EdgeInsets.only(top: 8, bottom: 96),
            children: [
              for (final cat in reminderCategories)
                if (byCat[cat] != null) ...[
                  SectionTitle('${reminderCategoryLabel(cat)} (${byCat[cat]!.length})'),
                  Card(
                    child: Column(
                      children: [
                        for (final r in byCat[cat]!)
                          ListTile(
                            leading: Icon(reminderCategoryIcon(r.category), color: r.active ? p.primary : p.muted),
                            title: Text(r.title, style: TextStyle(color: r.active ? p.text : p.muted)),
                            subtitle: Text([
                              if (r.description.isNotEmpty) r.description else '${r.kind} ${r.timeOfDay}',
                              if (r.assigneeName.isNotEmpty) r.assigneeName else 'Whole family',
                              if (!r.active) 'paused',
                            ].join(' · ')),
                            trailing: canManage
                                ? PopupMenuButton<String>(
                                    tooltip: 'Reminder actions',
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
                                      PopupMenuItem(value: 'delete', child: Text('Delete', style: TextStyle(color: p.danger))),
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

const _weekdayLetters = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

/// Create a reminder. Starts with the household template gallery; picking a
/// template (or "Start from scratch") shows the form, prefilled.
class NewReminderSheet extends ConsumerStatefulWidget {
  const NewReminderSheet({super.key});

  @override
  ConsumerState<NewReminderSheet> createState() => _NewReminderSheetState();
}

class _NewReminderSheetState extends ConsumerState<NewReminderSheet> {
  final _title = TextEditingController();
  final _notes = TextEditingController();
  bool _picking = true; // template gallery first
  String _kind = 'weekly';
  String _category = 'cleaning';
  int _interval = 1;
  Set<int> _weekdays = {DateTime.now().weekday % 7};
  int _dayOfMonth = DateTime.now().day;
  int? _assigneeId;
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

  void _useTemplate(ReminderTemplate? t) {
    setState(() {
      _picking = false;
      if (t == null) return;
      _title.text = t.title;
      _category = t.category;
      _kind = t.kind;
      _interval = t.interval;
      if (t.weekdays.isNotEmpty) _weekdays = {...t.weekdays};
      final parts = t.timeOfDay.split(':');
      _time = TimeOfDay(hour: int.parse(parts[0]), minute: int.parse(parts[1]));
      final now = DateTime.now();
      if (t.kind == 'monthly' && t.dayOfMonth != null) {
        _dayOfMonth = t.dayOfMonth!;
        _date = nextDayOfMonth(t.dayOfMonth!, now); // "every 3 months on the 1st" starts on the coming 1st
      } else {
        _dayOfMonth = now.day;
        _date = now;
      }
    });
  }

  Future<void> _save() async {
    if (_title.text.trim().isEmpty) {
      setState(() => _error = 'Enter a title');
      return;
    }
    if (_kind == 'weekly' && _weekdays.isEmpty) {
      setState(() => _error = 'Pick at least one day');
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
            weekdays: _kind == 'weekly' ? (_weekdays.toList()..sort()) : null,
            dayOfMonth: _kind == 'monthly' ? _dayOfMonth : (_kind == 'yearly' ? _date.day : null),
            interval: reminderKindHasInterval(_kind) ? _interval : 1,
            assigneeId: _assigneeId,
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

  Widget _gallery(BuildContext context) {
    final p = context.palette;
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text('New reminder', style: TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: p.text)),
        const SizedBox(height: 4),
        Text('Pick a household routine or start from scratch.', style: TextStyle(color: p.muted)),
        const SizedBox(height: 12),
        OutlinedButton.icon(
          icon: const Icon(Icons.edit_outlined),
          label: const Text('Start from scratch'),
          onPressed: () => _useTemplate(null),
        ),
        const SizedBox(height: 8),
        for (final t in reminderTemplates)
          Card(
            margin: const EdgeInsets.symmetric(vertical: 4),
            child: ListTile(
              leading: Icon(reminderCategoryIcon(t.category), color: p.primary),
              title: Text(t.title),
              subtitle: Text('${t.description} · ${reminderCategoryLabel(t.category)}'),
              trailing: const Icon(Icons.add),
              onTap: () => _useTemplate(t),
            ),
          ),
      ],
    );
  }

  Widget _form(BuildContext context) {
    final p = context.palette;
    final members = ref.watch(familyProvider).valueOrNull?.members ?? const <FamilyMember>[];
    final repeats = reminderKindHasInterval(_kind);
    final preview = _kind == 'once'
        ? 'Once on ${prettyDate(isoDate(_date))} at $_hhmm'
        : describeReminderSchedule(
            kind: _kind,
            timeOfDay: _hhmm,
            weekdays: _weekdays.toList(),
            dayOfMonth: _kind == 'monthly' ? _dayOfMonth : null,
            interval: _interval);
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text('New reminder', style: TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: p.text)),
        const SizedBox(height: 12),
        TextField(
          controller: _title,
          autofocus: _title.text.isEmpty,
          textCapitalization: TextCapitalization.sentences,
          decoration: const InputDecoration(labelText: 'What to remember'),
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            Expanded(
              child: DropdownButtonFormField<String>(
                value: _kind,
                isExpanded: true,
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
                isExpanded: true,
                decoration: const InputDecoration(labelText: 'Category'),
                items: [
                  for (final c in reminderCategories)
                    DropdownMenuItem(value: c, child: Text(reminderCategoryLabel(c), overflow: TextOverflow.ellipsis)),
                ],
                onChanged: (v) => setState(() => _category = v ?? 'other'),
              ),
            ),
          ],
        ),
        if (repeats) ...[
          const SizedBox(height: 12),
          Row(
            children: [
              Text('Repeat every', style: TextStyle(color: p.text)),
              IconButton(
                tooltip: 'Fewer',
                icon: const Icon(Icons.remove_circle_outline),
                onPressed: _interval > 1 ? () => setState(() => _interval--) : null,
              ),
              Text('$_interval', key: const Key('reminder-interval'), style: TextStyle(fontWeight: FontWeight.w700, color: p.text)),
              IconButton(
                tooltip: 'More',
                icon: const Icon(Icons.add_circle_outline),
                onPressed: _interval < 52 ? () => setState(() => _interval++) : null,
              ),
              Text(_kind == 'weekly' ? (_interval == 1 ? 'week' : 'weeks') : (_interval == 1 ? 'month' : 'months'),
                  style: TextStyle(color: p.text)),
            ],
          ),
        ],
        if (_kind == 'weekly') ...[
          const SizedBox(height: 4),
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: [
              for (var i = 0; i < 7; i++)
                FilterChip(
                  label: Text(_weekdayLetters[i]),
                  selected: _weekdays.contains(i),
                  onSelected: (on) => setState(() => on ? _weekdays.add(i) : _weekdays.remove(i)),
                ),
            ],
          ),
        ],
        if (_kind == 'monthly') ...[
          const SizedBox(height: 12),
          DropdownButtonFormField<int>(
            value: _dayOfMonth,
            decoration: const InputDecoration(labelText: 'Day of month'),
            items: [for (var d = 1; d <= 31; d++) DropdownMenuItem(value: d, child: Text('$d'))],
            onChanged: (v) => setState(() => _dayOfMonth = v ?? 1),
          ),
        ],
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
        DropdownButtonFormField<int?>(
          value: _assigneeId,
          isExpanded: true,
          decoration: const InputDecoration(labelText: 'Who is it for'),
          items: [
            const DropdownMenuItem<int?>(value: null, child: Text('Whole family')),
            for (final m in members)
              DropdownMenuItem<int?>(value: m.id, child: Text(m.displayName.isNotEmpty ? m.displayName : m.email)),
          ],
          onChanged: (v) => setState(() => _assigneeId = v),
        ),
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
          decoration: BoxDecoration(color: p.primaryLight, borderRadius: BorderRadius.circular(KRadius.md)),
          child: Row(children: [
            Icon(Icons.event_repeat, size: 18, color: p.primary),
            const SizedBox(width: 8),
            Expanded(child: Text(preview, style: TextStyle(color: p.primary, fontWeight: FontWeight.w600))),
          ]),
        ),
        if (repeats && _interval > 1)
          Padding(
            padding: const EdgeInsets.only(top: 6),
            child: Text('Counted from the ${_kind == 'weekly' ? 'week' : 'month'} of the start date.',
                style: TextStyle(color: p.muted, fontSize: 12)),
          ),
        const SizedBox(height: 12),
        TextField(controller: _notes, decoration: const InputDecoration(labelText: 'Notes (optional)')),
        const SizedBox(height: 8),
        Text("Also sent to your family's Telegram/Discord chat when a bot is connected.",
            style: TextStyle(color: p.muted, fontSize: 12)),
        if (_error != null) ...[
          const SizedBox(height: 12),
          Text(_error!, style: TextStyle(color: p.danger)),
        ],
        const SizedBox(height: 20),
        Row(
          children: [
            TextButton(onPressed: () => setState(() => _picking = true), child: const Text('Templates')),
            const Spacer(),
            FilledButton(onPressed: _saving ? null : _save, child: const Text('Save reminder')),
          ],
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(20, 0, 20, 20 + MediaQuery.of(context).viewInsets.bottom),
      child: SingleChildScrollView(child: _picking ? _gallery(context) : _form(context)),
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
            icon: Icon(Icons.undo, color: p.muted),
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
                leading: Icon(Icons.cleaning_services_rounded, size: 28, color: p.primary),
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
              const EmptyState(mascot: MascotPose.relax, title: 'No chores today', message: 'Nothing to do. Nice!')
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
                        child: Icon(Icons.shopping_cart_outlined, color: p.primary),
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
