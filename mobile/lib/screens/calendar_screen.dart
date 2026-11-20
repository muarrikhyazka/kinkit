import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../util/dates.dart';
import '../widgets/async_view.dart';

/// Calendar (web: /calendar, backend-go/activities.go): family activities with
/// the reminders overlay, as a month grid + the selected day's list, or an
/// agenda of the next 30 days. Family admins and parents add / edit / delete
/// activities; everyone else reads. The web also has week and day hour grids.

const _colors = <String, Color>{
  'indigo': Color(0xFF4F46E5),
  'rose': Color(0xFFE11D48),
  'amber': Color(0xFFD97706),
  'emerald': Color(0xFF059669),
  'sky': Color(0xFF0284C7),
  'violet': Color(0xFF7C3AED),
  'slate': Color(0xFF64748B),
};
const _reminderColor = Color(0xFFB45309);
const _weekdayShort = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const _weekdayNums = [1, 2, 3, 4, 5, 6, 0]; // API numbers (0 = Sunday) in Monday-first order

String _iso(DateTime d) =>
    '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';
DateTime _parse(String s) => DateTime.tryParse(s) ?? DateTime.now();
DateTime _addDays(DateTime d, int n) => DateTime(d.year, d.month, d.day + n);
String _hhmm(DateTime d) => '${d.hour.toString().padLeft(2, '0')}:${d.minute.toString().padLeft(2, '0')}';
String _dayLabel(String iso) {
  final d = _parse(iso);
  const wd = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
  return '${wd[d.weekday - 1]} ${d.day} ${monthShort[d.month - 1]}';
}

/// One drawable entry: an activity occurrence or a reminder.
class _Item {
  final bool reminder;
  final String title, date, endDate, timeLabel, sub;
  final Color color;
  final int sortMin;
  final bool allDay, done;
  final ActivityOccurrence? activity;
  final ReminderOccurrence? occurrence;
  const _Item({
    required this.reminder,
    required this.title,
    required this.date,
    required this.endDate,
    required this.timeLabel,
    required this.sub,
    required this.color,
    required this.sortMin,
    this.allDay = false,
    this.done = false,
    this.activity,
    this.occurrence,
  });
}

int _minutes(String hhmm) {
  final p = hhmm.split(':');
  if (p.length < 2) return 0;
  return (int.tryParse(p[0]) ?? 0) * 60 + (int.tryParse(p[1]) ?? 0);
}

List<_Item> _items(CalendarData d, {required bool activities, required bool reminders}) {
  final out = <_Item>[];
  if (activities) {
    for (final a in d.activities) {
      final allDay = a.allDay || a.multiDay;
      out.add(_Item(
        reminder: false,
        title: a.title,
        date: a.occursOn,
        endDate: a.occursUntil,
        allDay: allDay,
        timeLabel: allDay ? 'All day' : '${a.startTime}–${a.endTime}',
        sub: [a.location, a.attendeeNames.isEmpty ? 'Whole family' : a.attendeeNames.join(', ')].where((e) => e.isNotEmpty).join(' · '),
        color: _colors[a.color] ?? _colors['indigo']!,
        sortMin: allDay ? -1 : _minutes(a.startTime),
        activity: a,
      ));
    }
  }
  if (reminders) {
    for (final o in d.reminders) {
      final at = (o.status == 'snoozed' ? o.snoozedUntil : null) ?? o.dueAt;
      if (at == null) continue;
      final day = _iso(at);
      out.add(_Item(
        reminder: true,
        title: o.title,
        date: day,
        endDate: day,
        timeLabel: _hhmm(at),
        sub: 'Reminder · ${o.assigneeName.isEmpty ? 'Whole family' : o.assigneeName}',
        color: _reminderColor,
        sortMin: at.hour * 60 + at.minute,
        done: o.status == 'done',
        occurrence: o,
      ));
    }
  }
  return out;
}

List<_Item> _onDay(List<_Item> all, String day) =>
    all.where((i) => i.date.compareTo(day) <= 0 && day.compareTo(i.endDate) <= 0).toList()..sort((a, b) => a.sortMin.compareTo(b.sortMin));

// Home > Calendar tab

class CalendarTab extends ConsumerWidget {
  const CalendarTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final canManage = ref.watch(meProvider)?.canManage ?? false;
    return Scaffold(
      floatingActionButton: canManage
          ? FloatingActionButton.extended(
              heroTag: 'add-activity',
              onPressed: () => openActivitySheet(context, ref, date: _iso(DateTime.now())),
              icon: const Icon(Icons.event_available_outlined),
              label: const Text('Activity'),
            )
          : null,
      body: const CalendarPanel(activities: true, reminders: true, layerChips: true),
    );
  }
}

/// The calendar body. [activities] / [reminders] are the layers shown; with
/// [layerChips] the user can toggle them (the Reminders tab shows reminders only).
class CalendarPanel extends ConsumerStatefulWidget {
  final bool activities, reminders, layerChips;
  const CalendarPanel({super.key, this.activities = true, this.reminders = true, this.layerChips = false});

  @override
  ConsumerState<CalendarPanel> createState() => _CalendarPanelState();
}

class _CalendarPanelState extends ConsumerState<CalendarPanel> {
  late DateTime _month; // first of the shown month
  late String _selected;
  bool _agenda = false;
  late bool _showActs, _showRems;

  @override
  void initState() {
    super.initState();
    final n = DateTime.now();
    _month = DateTime(n.year, n.month, 1);
    _selected = _iso(n);
    _showActs = widget.activities;
    _showRems = widget.reminders;
  }

  DateTime get _gridStart {
    final first = _month;
    return _addDays(first, -(first.weekday - 1));
  }

  String? get _include {
    if (_showActs && _showRems) return null;
    if (_showActs) return 'activities';
    return 'reminders';
  }

  void _refresh() {
    ref.invalidate(calendarProvider);
    ref.invalidate(todayProvider);
  }

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final today = _iso(DateTime.now());
    final from = _agenda ? today : _iso(_gridStart);
    final to = _agenda ? _iso(_addDays(DateTime.now(), 29)) : _iso(_addDays(_gridStart, 41));
    final range = CalRange(from, to, (_showActs || _showRems) ? _include : 'activities');
    final value = ref.watch(calendarProvider(range));
    final all = value.hasValue ? _items(value.requireValue, activities: _showActs, reminders: _showRems) : <_Item>[];

    return RefreshIndicator(
      onRefresh: () async {
        _refresh();
        await ref.read(calendarProvider(range).future);
      },
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(12, 10, 12, 96),
        children: [
          Row(
            children: [
              Expanded(
                child: SegmentedButton<bool>(
                  segments: const [
                    ButtonSegment(value: false, label: Text('Month'), icon: Icon(Icons.calendar_month_outlined)),
                    ButtonSegment(value: true, label: Text('Agenda'), icon: Icon(Icons.view_agenda_outlined)),
                  ],
                  selected: {_agenda},
                  onSelectionChanged: (v) => setState(() => _agenda = v.first),
                ),
              ),
            ],
          ),
          if (widget.layerChips)
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Wrap(spacing: 8, children: [
                FilterChip(
                  label: const Text('Activities'),
                  selected: _showActs,
                  onSelected: (v) => setState(() => _showActs = v || !_showRems),
                ),
                FilterChip(
                  avatar: const Icon(Icons.notifications_none, size: 16),
                  label: const Text('Reminders'),
                  selected: _showRems,
                  onSelected: (v) => setState(() => _showRems = v || !_showActs),
                ),
              ]),
            ),
          if (!_agenda) ...[
            const SizedBox(height: 4),
            Row(
              children: [
                IconButton(
                  tooltip: 'Previous month',
                  icon: const Icon(Icons.chevron_left),
                  onPressed: () => setState(() => _month = DateTime(_month.year, _month.month - 1, 1)),
                ),
                Expanded(
                  child: Text('${monthNames[_month.month - 1]} ${_month.year}',
                      textAlign: TextAlign.center, style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: p.text)),
                ),
                TextButton(
                  onPressed: () => setState(() {
                    final n = DateTime.now();
                    _month = DateTime(n.year, n.month, 1);
                    _selected = _iso(n);
                  }),
                  child: const Text('Today'),
                ),
                IconButton(
                  tooltip: 'Next month',
                  icon: const Icon(Icons.chevron_right),
                  onPressed: () => setState(() => _month = DateTime(_month.year, _month.month + 1, 1)),
                ),
              ],
            ),
            _MonthGrid(
              start: _gridStart,
              month: _month.month,
              today: today,
              selected: _selected,
              items: all,
              onSelect: (d) => setState(() => _selected = d),
            ),
            const SizedBox(height: 12),
            Text(_dayLabel(_selected), style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: p.text)),
            const SizedBox(height: 6),
            if (value.hasError && !value.hasValue)
              Text('${value.error}', style: TextStyle(color: p.danger))
            else if (!value.hasValue)
              const LinearProgressIndicator()
            else
              _DayRows(day: _selected, items: all, onTap: (i) => _openItem(context, i)),
          ] else ...[
            const SizedBox(height: 8),
            if (value.hasError && !value.hasValue)
              Text('${value.error}', style: TextStyle(color: p.danger))
            else if (!value.hasValue)
              const LinearProgressIndicator()
            else
              ..._agendaRows(context, all, today),
          ],
        ],
      ),
    );
  }

  List<Widget> _agendaRows(BuildContext context, List<_Item> all, String today) {
    final p = context.palette;
    final out = <Widget>[];
    for (var i = 0; i < 30; i++) {
      final d = _iso(_addDays(DateTime.now(), i));
      if (_onDay(all, d).isEmpty) continue;
      out.add(Padding(
        padding: const EdgeInsets.only(top: 14, bottom: 6),
        child: Text(d == today ? 'Today · ${_dayLabel(d)}' : _dayLabel(d),
            style: TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: d == today ? p.primary : p.muted)),
      ));
      out.add(_DayRows(day: d, items: all, onTap: (it) => _openItem(context, it)));
    }
    if (out.isEmpty) out.add(const Padding(padding: EdgeInsets.all(24), child: Center(child: Text('Nothing planned in the next 30 days.'))));
    return out;
  }

  void _openItem(BuildContext context, _Item item) {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      showDragHandle: true,
      builder: (_) => _ItemSheet(item: item, onChanged: _refresh),
    );
  }
}

class _MonthGrid extends StatelessWidget {
  final DateTime start;
  final int month;
  final String today, selected;
  final List<_Item> items;
  final ValueChanged<String> onSelect;
  const _MonthGrid({required this.start, required this.month, required this.today, required this.selected, required this.items, required this.onSelect});

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    return Column(
      children: [
        Row(children: [
          for (final d in _weekdayShort)
            Expanded(child: Center(child: Text(d, style: TextStyle(fontSize: 11, fontWeight: FontWeight.w600, color: p.muted)))),
        ]),
        const SizedBox(height: 4),
        for (var w = 0; w < 6; w++)
          Row(children: [
            for (var c = 0; c < 7; c++) Expanded(child: _cell(context, _addDays(start, w * 7 + c))),
          ]),
      ],
    );
  }

  Widget _cell(BuildContext context, DateTime d) {
    final p = context.palette;
    final iso = _iso(d);
    final isToday = iso == today;
    final isSel = iso == selected;
    final inMonth = d.month == month;
    final list = _onDay(items, iso);
    return Semantics(
      button: true,
      label: '${_dayLabel(iso)}, ${list.length} item${list.length == 1 ? '' : 's'}',
      child: InkWell(
        borderRadius: BorderRadius.circular(8),
        onTap: () => onSelect(iso),
        child: Container(
          height: 52,
          margin: const EdgeInsets.all(1),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: isSel ? p.primary : Colors.transparent, width: 2),
            color: inMonth ? null : p.surface2,
          ),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Container(
                width: 24,
                height: 24,
                alignment: Alignment.center,
                decoration: BoxDecoration(shape: BoxShape.circle, color: isToday ? p.primary : null),
                child: Text('${d.day}',
                    style: TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: isToday ? p.onPrimary : (inMonth ? p.text : p.muted))),
              ),
              const SizedBox(height: 3),
              SizedBox(
                height: 6,
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    for (final i in list.take(4))
                      Container(width: 6, height: 6, margin: const EdgeInsets.symmetric(horizontal: 1), decoration: BoxDecoration(shape: BoxShape.circle, color: i.color)),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _DayRows extends StatelessWidget {
  final String day;
  final List<_Item> items;
  final ValueChanged<_Item> onTap;
  const _DayRows({required this.day, required this.items, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final list = _onDay(items, day);
    if (list.isEmpty) return Padding(padding: const EdgeInsets.symmetric(vertical: 8), child: Text('Nothing planned.', style: TextStyle(color: p.muted)));
    return Column(
      children: [
        for (final i in list)
          Card(
            margin: const EdgeInsets.only(bottom: 6),
            clipBehavior: Clip.antiAlias,
            child: InkWell(
              onTap: () => onTap(i),
              child: IntrinsicHeight(
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Container(width: 5, color: i.color),
                    Padding(
                      padding: const EdgeInsets.fromLTRB(12, 10, 8, 10),
                      child: SizedBox(width: 78, child: Text(i.timeLabel, style: TextStyle(color: p.muted, fontSize: 13))),
                    ),
                    Expanded(
                      child: Padding(
                        padding: const EdgeInsets.fromLTRB(0, 10, 12, 10),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Row(children: [
                              if (i.reminder) Padding(padding: const EdgeInsets.only(right: 4), child: Icon(Icons.notifications_none, size: 15, color: p.muted)),
                              Expanded(
                                child: Text(i.title,
                                    style: TextStyle(
                                        fontWeight: FontWeight.w600,
                                        color: p.text,
                                        decoration: i.done ? TextDecoration.lineThrough : null)),
                              ),
                            ]),
                            if (i.sub.isNotEmpty) Text(i.sub, style: TextStyle(color: p.muted, fontSize: 12)),
                          ],
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
      ],
    );
  }
}

// detail sheet

String _repeatText(ActivityOccurrence a) {
  const unit = {'daily': 'day', 'weekly': 'week', 'monthly': 'month', 'yearly': 'year'};
  final u = unit[a.repeat] ?? a.repeat;
  var s = a.interval > 1 ? 'every ${a.interval} ${u}s' : 'every $u';
  if (a.repeat == 'weekly' && a.weekdays.isNotEmpty) {
    s += ' on ${_weekdayNums.where(a.weekdays.contains).map((n) => _weekdayShort[_weekdayNums.indexOf(n)]).join(', ')}';
  }
  if (a.untilDate != null) s += ', until ${a.untilDate}';
  return s;
}

class _ItemSheet extends ConsumerWidget {
  final _Item item;
  final VoidCallback onChanged;
  const _ItemSheet({required this.item, required this.onChanged});

  Future<void> _do(BuildContext context, Future<void> Function() call, String ok) async {
    try {
      await call();
      onChanged();
      if (context.mounted) {
        Navigator.pop(context);
        showSnack(context, ok);
      }
    } catch (e) {
      if (context.mounted) showSnack(context, '$e', error: true);
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final p = context.palette;
    final api = ref.read(apiProvider);
    final canManage = ref.watch(meProvider)?.canManage ?? false;
    final a = item.activity;
    final o = item.occurrence;
    final when = item.date == item.endDate ? _dayLabel(item.date) : '${_dayLabel(item.date)} → ${_dayLabel(item.endDate)}';
    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(children: [
            Container(width: 10, height: 10, decoration: BoxDecoration(shape: BoxShape.circle, color: item.color)),
            const SizedBox(width: 8),
            Expanded(child: Text(item.title, style: TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: p.text))),
          ]),
          const SizedBox(height: 12),
          _line(context, Icons.schedule, '$when · ${item.timeLabel}'),
          if (a != null) ...[
            if (a.repeating) _line(context, Icons.repeat, _repeatText(a)),
            if (a.location.isNotEmpty) _line(context, Icons.place_outlined, a.location),
            _line(context, Icons.people_outline, a.attendeeNames.isEmpty ? 'Whole family' : a.attendeeNames.join(', ')),
            if (a.notes.isNotEmpty) _line(context, Icons.notes, a.notes),
          ],
          if (o != null) ...[
            _line(context, Icons.notifications_none, 'Reminder · ${o.status}'),
            _line(context, Icons.person_outline, o.assigneeName.isEmpty ? 'Whole family' : o.assigneeName),
            if (o.notes.isNotEmpty) _line(context, Icons.notes, o.notes),
          ],
          const SizedBox(height: 16),
          Wrap(spacing: 8, runSpacing: 8, children: [
            if (a != null && canManage)
              FilledButton.icon(
                icon: const Icon(Icons.edit_outlined),
                label: const Text('Edit'),
                onPressed: () {
                  Navigator.pop(context);
                  openActivitySheet(context, ref, initial: a);
                },
              ),
            if (a != null && canManage && a.repeating)
              OutlinedButton(
                onPressed: () async {
                  final ok = await confirmDialog(context, title: 'Remove only ${a.occursOn}?', confirm: 'Remove');
                  if (ok && context.mounted) await _do(context, () => api.deleteActivity(a.id, on: a.occursOn), 'Removed');
                },
                child: const Text('Delete this one'),
              ),
            if (a != null && canManage)
              OutlinedButton(
                style: OutlinedButton.styleFrom(foregroundColor: p.danger),
                onPressed: () async {
                  final ok = await confirmDialog(context,
                      title: a.repeating ? 'Delete "${a.title}" and all repeats?' : 'Delete "${a.title}"?', confirm: 'Delete');
                  if (ok && context.mounted) await _do(context, () => api.deleteActivity(a.id), 'Deleted');
                },
                child: Text(a.repeating ? 'Delete series' : 'Delete'),
              ),
            if (o != null && o.canAct && o.status != 'done') ...[
              FilledButton.icon(
                icon: const Icon(Icons.check),
                label: const Text('Done'),
                onPressed: () => _do(context, () => api.reminderDone(o.id), 'Done'),
              ),
              OutlinedButton(
                onPressed: () => _do(context, () => api.reminderSnooze(o.id), 'Snoozed for 1 hour'),
                child: const Text('Snooze 1h'),
              ),
            ],
          ]),
        ],
      ),
    );
  }

  Widget _line(BuildContext context, IconData icon, String text) {
    final p = context.palette;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Icon(icon, size: 18, color: p.muted),
        const SizedBox(width: 10),
        Expanded(child: Text(text, style: TextStyle(color: p.text))),
      ]),
    );
  }
}

// create / edit

Future<void> openActivitySheet(BuildContext context, WidgetRef ref, {ActivityOccurrence? initial, String? date}) async {
  final ok = await showModalBottomSheet<bool>(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    showDragHandle: true,
    builder: (_) => ActivitySheet(initial: initial, date: date ?? initial?.startDate ?? _iso(DateTime.now())),
  );
  if (ok == true) {
    ref.invalidate(calendarProvider);
    ref.invalidate(todayProvider);
  }
}

class ActivitySheet extends ConsumerStatefulWidget {
  final ActivityOccurrence? initial;
  final String date;
  const ActivitySheet({super.key, this.initial, required this.date});

  @override
  ConsumerState<ActivitySheet> createState() => _ActivitySheetState();
}

class _ActivitySheetState extends ConsumerState<ActivitySheet> {
  final _title = TextEditingController();
  final _location = TextEditingController();
  final _notes = TextEditingController();
  late String _startDate, _endDate, _startTime, _endTime, _repeat, _color;
  String? _until;
  bool _allDay = false;
  int _interval = 1;
  final _weekdays = <int>{};
  final _attendees = <int>{};
  bool _busy = false;
  String _error = '';

  @override
  void initState() {
    super.initState();
    final a = widget.initial;
    _title.text = a?.title ?? '';
    _location.text = a?.location ?? '';
    _notes.text = a?.notes ?? '';
    _startDate = a?.startDate ?? widget.date;
    _endDate = a?.endDate ?? widget.date;
    _startTime = a?.startTime ?? '09:00';
    _endTime = a?.endTime ?? '10:00';
    _repeat = a?.repeat ?? 'none';
    _color = a?.color ?? 'indigo';
    _until = a?.untilDate;
    _allDay = a?.allDay ?? false;
    _interval = a?.interval ?? 1;
    _weekdays.addAll(a?.weekdays ?? const []);
    _attendees.addAll(a?.attendeeIds ?? const []);
  }

  @override
  void dispose() {
    _title.dispose();
    _location.dispose();
    _notes.dispose();
    super.dispose();
  }

  Future<String?> _pickDate(String current, {String? first}) async {
    final d = await showDatePicker(
      context: context,
      initialDate: _parse(current),
      firstDate: first != null ? _parse(first) : DateTime(2020),
      lastDate: DateTime(2100),
    );
    return d == null ? null : _iso(d);
  }

  Future<String?> _pickTime(String current) async {
    final parts = current.split(':');
    final t = await showTimePicker(
      context: context,
      initialTime: TimeOfDay(hour: int.tryParse(parts[0]) ?? 9, minute: parts.length > 1 ? int.tryParse(parts[1]) ?? 0 : 0),
    );
    if (t == null) return null;
    return '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';
  }

  Future<void> _save() async {
    final title = _title.text.trim();
    if (title.isEmpty) return;
    setState(() {
      _busy = true;
      _error = '';
    });
    final wds = _weekdays.toList()..sort();
    final body = <String, dynamic>{
      'title': title,
      'notes': _notes.text.trim(),
      'location': _location.text.trim(),
      'color': _color,
      'all_day': _allDay,
      'start_date': _startDate,
      'end_date': _endDate,
      if (!_allDay) 'start_time': _startTime,
      if (!_allDay) 'end_time': _endTime,
      'repeat': _repeat,
      'interval': _interval,
      'weekdays': _repeat == 'weekly' ? (wds.isEmpty ? [_parse(_startDate).weekday % 7] : wds) : <int>[],
      'until_date': _repeat == 'none' ? '' : (_until ?? ''),
      'attendee_ids': _attendees.toList(),
    };
    try {
      final api = ref.read(apiProvider);
      if (widget.initial != null) {
        await api.updateActivity(widget.initial!.id, body);
      } else {
        await api.createActivity(body);
      }
      if (mounted) Navigator.pop(context, true);
    } catch (e) {
      if (mounted) setState(() => _error = '$e');
    }
    if (mounted) setState(() => _busy = false);
  }

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final members = ref.watch(familyProvider).valueOrNull?.members ?? const <FamilyMember>[];
    final inset = MediaQuery.of(context).viewInsets.bottom;
    return SingleChildScrollView(
      padding: EdgeInsets.fromLTRB(20, 0, 20, 20 + inset),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(widget.initial == null ? 'New activity' : 'Edit activity', style: TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: p.text)),
          const SizedBox(height: 12),
          TextField(
            controller: _title,
            maxLength: 120,
            autofocus: widget.initial == null,
            decoration: const InputDecoration(labelText: 'Title', hintText: 'e.g. Football practice', counterText: ''),
          ),
          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('All day'), value: _allDay, onChanged: (v) => setState(() => _allDay = v)),
          Row(children: [
            Expanded(child: _pickerTile('Starts', _startDate, () async {
              final d = await _pickDate(_startDate);
              if (d != null) {
                setState(() {
                  _startDate = d;
                  if (_endDate.compareTo(d) < 0) _endDate = d;
                });
              }
            })),
            if (!_allDay)
              Expanded(child: _pickerTile('At', _startTime, () async {
                final t = await _pickTime(_startTime);
                if (t != null) setState(() => _startTime = t);
              })),
          ]),
          Row(children: [
            Expanded(child: _pickerTile('Ends', _endDate, () async {
              final d = await _pickDate(_endDate, first: _startDate);
              if (d != null) setState(() => _endDate = d);
            })),
            if (!_allDay)
              Expanded(child: _pickerTile('At', _endTime, () async {
                final t = await _pickTime(_endTime);
                if (t != null) setState(() => _endTime = t);
              })),
          ]),
          const SizedBox(height: 8),
          DropdownButtonFormField<String>(
            value: _repeat,
            decoration: const InputDecoration(labelText: 'Repeat'),
            items: const [
              DropdownMenuItem(value: 'none', child: Text('Does not repeat')),
              DropdownMenuItem(value: 'daily', child: Text('Daily')),
              DropdownMenuItem(value: 'weekly', child: Text('Weekly')),
              DropdownMenuItem(value: 'monthly', child: Text('Monthly')),
              DropdownMenuItem(value: 'yearly', child: Text('Yearly')),
            ],
            onChanged: (v) => setState(() => _repeat = v ?? 'none'),
          ),
          if (_repeat != 'none') ...[
            Row(children: [
              const Text('Every'),
              IconButton(onPressed: _interval > 1 ? () => setState(() => _interval--) : null, icon: const Icon(Icons.remove_circle_outline)),
              Text('$_interval', style: const TextStyle(fontWeight: FontWeight.w700)),
              IconButton(onPressed: _interval < 99 ? () => setState(() => _interval++) : null, icon: const Icon(Icons.add_circle_outline)),
              Text({'daily': 'day(s)', 'weekly': 'week(s)', 'monthly': 'month(s)', 'yearly': 'year(s)'}[_repeat] ?? ''),
            ]),
            if (_repeat == 'weekly')
              Wrap(spacing: 6, children: [
                for (var i = 0; i < 7; i++)
                  FilterChip(
                    label: Text(_weekdayShort[i]),
                    selected: _weekdays.contains(_weekdayNums[i]),
                    onSelected: (v) => setState(() => v ? _weekdays.add(_weekdayNums[i]) : _weekdays.remove(_weekdayNums[i])),
                  ),
              ]),
            Row(children: [
              Expanded(child: _pickerTile('Until (optional)', _until ?? 'Never', () async {
                final d = await _pickDate(_until ?? _startDate, first: _startDate);
                if (d != null) setState(() => _until = d);
              })),
              if (_until != null) TextButton(onPressed: () => setState(() => _until = null), child: const Text('Clear')),
            ]),
          ],
          const SizedBox(height: 8),
          Text('Who is it for? (none = whole family)', style: TextStyle(color: p.muted, fontSize: 13)),
          const SizedBox(height: 4),
          Wrap(spacing: 6, runSpacing: 4, children: [
            for (final m in members)
              FilterChip(
                label: Text(m.displayName.isEmpty ? m.email : m.displayName),
                selected: _attendees.contains(m.id),
                onSelected: (v) => setState(() => v ? _attendees.add(m.id) : _attendees.remove(m.id)),
              ),
          ]),
          const SizedBox(height: 8),
          TextField(controller: _location, maxLength: 200, decoration: const InputDecoration(labelText: 'Location', counterText: '')),
          TextField(controller: _notes, maxLines: 2, maxLength: 2000, decoration: const InputDecoration(labelText: 'Notes', counterText: '')),
          const SizedBox(height: 8),
          Wrap(spacing: 10, children: [
            for (final e in _colors.entries)
              Semantics(
                label: 'Color ${e.key}',
                button: true,
                selected: _color == e.key,
                child: GestureDetector(
                  onTap: () => setState(() => _color = e.key),
                  child: Container(
                    width: 30,
                    height: 30,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: e.value,
                      border: Border.all(color: _color == e.key ? p.text : Colors.transparent, width: 3),
                    ),
                  ),
                ),
              ),
          ]),
          if (_error.isNotEmpty) Padding(padding: const EdgeInsets.only(top: 10), child: Text(_error, style: TextStyle(color: p.danger))),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity,
            child: FilledButton(onPressed: _busy ? null : _save, child: Text(_busy ? 'Saving…' : 'Save')),
          ),
        ],
      ),
    );
  }

  Widget _pickerTile(String label, String value, VoidCallback onTap) => Padding(
        padding: const EdgeInsets.only(right: 8, top: 4),
        child: InkWell(
          onTap: onTap,
          child: InputDecorator(decoration: InputDecoration(labelText: label), child: Text(value)),
        ),
      );
}
