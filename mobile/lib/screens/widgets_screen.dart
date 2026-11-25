import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/api_client.dart';
import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../widgets/async_view.dart';
import '../widgets/color_picker_sheet.dart';
import '../widgets/widget_colors.dart';

/// Widgets (web: /widgets, backend-go/widgets.go): boards of widgets (clock,
/// agenda, reminders, chores, lists, meals, budget, goals, family points, a
/// note) for a phone, TV, car screen or monitor. Family admins and parents
/// edit (reorder, resize, add, remove); everyone can view, also full screen.
/// The web arranges by drag and drop on a grid; here the list is reordered by
/// long-press dragging. Each widget and the board background can be coloured
/// (preset swatches or a hex code); text colour follows the fill's brightness.

class WidgetsScreen extends ConsumerStatefulWidget {
  const WidgetsScreen({super.key});

  @override
  ConsumerState<WidgetsScreen> createState() => _WidgetsScreenState();
}

class _WidgetsScreenState extends ConsumerState<WidgetsScreen> {
  int? _activeId;
  List<WidgetItem>? _draft; // non-null while editing
  String? _draftBg; // board background while editing (null = default)
  bool _busy = false;
  Timer? _refresh;

  @override
  void initState() {
    super.initState();
    _refresh = Timer.periodic(const Duration(minutes: 1), (_) {
      if (mounted && _draft == null) ref.invalidate(widgetDataProvider);
    });
  }

  @override
  void dispose() {
    _refresh?.cancel();
    super.dispose();
  }

  Future<void> _run(Future<void> Function() call, {String? ok}) async {
    setState(() => _busy = true);
    try {
      await call();
      ref.invalidate(widgetBoardsProvider);
      if (ok != null && mounted) showSnack(context, ok);
    } catch (e) {
      if (mounted) showSnack(context, '$e', error: true);
    }
    if (mounted) setState(() => _busy = false);
  }

  @override
  Widget build(BuildContext context) {
    final boards = ref.watch(widgetBoardsProvider);
    final catalog = ref.watch(widgetCatalogProvider).valueOrNull;
    final canManage = ref.watch(meProvider)?.canManage ?? false;
    final api = ref.read(apiProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Widgets')),
      body: boards.when(
        loading: () => const LoadingState(),
        error: (e, _) => ErrorState(message: '$e', onRetry: () => ref.invalidate(widgetBoardsProvider)),
        data: (list) {
          if (list.isEmpty) return _empty(context, catalog, canManage, api);
          final board = list.firstWhere((b) => b.id == _activeId, orElse: () => list.first);
          final widgets = _draft ?? board.widgets;
          final types = (widgets.map((w) => w.type).toSet().toList()..sort()).join(',');
          final data = ref.watch(widgetDataProvider(types)).valueOrNull;
          return RefreshIndicator(
            onRefresh: () async {
              ref.invalidate(widgetBoardsProvider);
              ref.invalidate(widgetDataProvider);
            },
            child: ListView(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: const EdgeInsets.fromLTRB(12, 10, 12, 32),
              children: [
                Wrap(spacing: 8, children: [
                  for (final b in list)
                    ChoiceChip(
                      label: Text(b.name),
                      selected: b.id == board.id,
                      onSelected: (_) => setState(() {
                        _activeId = b.id;
                        _draft = null;
                        _draftBg = null;
                      }),
                    ),
                ]),
                const SizedBox(height: 8),
                Wrap(spacing: 8, runSpacing: 4, children: [
                  if (_draft == null) ...[
                    FilledButton.icon(
                      icon: const Icon(Icons.fullscreen),
                      label: const Text('Full screen'),
                      onPressed: () => Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => WidgetsFullScreen(board: board))),
                    ),
                    if (canManage)
                      OutlinedButton.icon(
                        icon: const Icon(Icons.edit_outlined),
                        label: const Text('Edit'),
                        onPressed: () => setState(() {
                          _draft = List.of(board.widgets);
                          _draftBg = board.background;
                        }),
                      ),
                    if (canManage)
                      OutlinedButton.icon(
                        icon: const Icon(Icons.add),
                        label: const Text('New board'),
                        onPressed: _busy ? null : () => _newBoard(context, catalog, api),
                      ),
                  ] else ...[
                    FilledButton(
                      onPressed: _busy
                          ? null
                          : () async {
                              final d = _draft!;
                              await _run(() => api.updateWidgetBoard(board.id, widgets: d, background: _draftBg, layoutExtra: board.layoutExtra), ok: 'Layout saved');
                              if (mounted) setState(() => _draft = null);
                            },
                      child: const Text('Save layout'),
                    ),
                    OutlinedButton(onPressed: () => setState(() => _draft = null), child: const Text('Cancel')),
                  ],
                ]),
                const SizedBox(height: 12),
                if (_draft != null)
                  _editor(context, catalog)
                else
                  BoardBackdrop(color: board.background, child: WidgetBoardView(widgets: widgets, data: data, catalog: catalog)),
                if (canManage && _draft == null)
                  Padding(
                    padding: const EdgeInsets.only(top: 16),
                    child: TextButton.icon(
                      icon: Icon(Icons.delete_outline, color: context.palette.danger),
                      label: Text('Delete "${board.name}"', style: TextStyle(color: context.palette.danger)),
                      onPressed: () async {
                        final ok = await confirmDialog(context, title: 'Delete "${board.name}"?', message: 'TV links using it go back to the month summary.');
                        if (ok) {
                          await _run(() => api.deleteWidgetBoard(board.id), ok: 'Board deleted');
                          if (mounted) setState(() => _activeId = null);
                        }
                      },
                    ),
                  ),
                Padding(
                  padding: const EdgeInsets.only(top: 12),
                  child: Text('TV or car screens use a board through a link under Settings › Data & devices on the web.',
                      style: TextStyle(color: context.palette.muted, fontSize: 12)),
                ),
              ],
            ),
          );
        },
      ),
    );
  }

  Widget _empty(BuildContext context, WidgetCatalog? catalog, bool canManage, KinkitApi api) {
    final p = context.palette;
    if (!canManage) {
      return const EmptyState(icon: Icons.dashboard_customize_outlined, title: 'No widget boards yet', message: 'Ask a parent or the family admin to create one.');
    }
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Text('Start from a template', style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: p.text)),
        Text('You can add, remove and reorder widgets afterwards.', style: TextStyle(color: p.muted)),
        const SizedBox(height: 12),
        for (final pr in catalog?.presets ?? const <WidgetPreset>[])
          Card(
            child: ListTile(
              title: Text(pr.name),
              subtitle: Text('${pr.count} widgets'),
              trailing: const Icon(Icons.chevron_right),
              onTap: _busy ? null : () => _run(() => api.createWidgetBoard(preset: pr.id), ok: 'Board created'),
            ),
          ),
        Card(
          child: ListTile(
            title: const Text('Blank board'),
            subtitle: const Text('Pick every widget yourself'),
            trailing: const Icon(Icons.chevron_right),
            onTap: _busy ? null : () => _run(() => api.createWidgetBoard(name: 'My board'), ok: 'Board created'),
          ),
        ),
      ],
    );
  }

  Future<void> _newBoard(BuildContext context, WidgetCatalog? catalog, KinkitApi api) async {
    final choice = await showModalBottomSheet<String>(
      context: context,
      showDragHandle: true,
      builder: (ctx) => SafeArea(
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          const ListTile(title: Text('New board', style: TextStyle(fontWeight: FontWeight.w700))),
          for (final pr in catalog?.presets ?? const <WidgetPreset>[])
            ListTile(title: Text(pr.name), subtitle: Text('${pr.count} widgets'), onTap: () => Navigator.pop(ctx, pr.id)),
          ListTile(title: const Text('Blank board'), onTap: () => Navigator.pop(ctx, '')),
        ]),
      ),
    );
    if (choice == null) return;
    await _run(() => choice.isEmpty ? api.createWidgetBoard(name: 'New board') : api.createWidgetBoard(preset: choice), ok: 'Board created');
  }

  Widget _editor(BuildContext context, WidgetCatalog? catalog) {
    final p = context.palette;
    final draft = _draft!;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        PopupMenuButton<WidgetTypeInfo>(
          tooltip: 'Add a widget',
          onSelected: (t) => setState(() => draft.add(WidgetItem(id: 'w${DateTime.now().microsecondsSinceEpoch.toRadixString(36)}', type: t.type, w: t.w, h: t.h))),
          itemBuilder: (_) => [for (final t in catalog?.widgets ?? const <WidgetTypeInfo>[]) PopupMenuItem(value: t, child: Text(t.label))],
          child: const Chip(avatar: Icon(Icons.add, size: 18), label: Text('Add a widget')),
        ),
        const SizedBox(height: 8),
        ActionChip(
          avatar: _swatch(_draftBg, 18, p),
          label: const Text('Board background'),
          tooltip: 'Board background colour',
          onPressed: () async {
            final c = await showColorPickerSheet(context, title: 'Board background', current: _draftBg);
            if (c != null) setState(() => _draftBg = c.isEmpty ? null : c);
          },
        ),
        const SizedBox(height: 8),
        Text('Long-press and drag a row to reorder. Width is in columns of 12. Tap the dot to colour a widget.', style: TextStyle(color: p.muted, fontSize: 12)),
        ReorderableListView(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          onReorder: (a, b) => setState(() {
            if (b > a) b--;
            draft.insert(b, draft.removeAt(a));
          }),
          children: [
            for (var i = 0; i < draft.length; i++)
              Card(
                key: ValueKey(draft[i].id),
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(12, 8, 4, 8),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(children: [
                        Expanded(child: Text(catalog?.labelOf(draft[i].type) ?? draft[i].type, style: const TextStyle(fontWeight: FontWeight.w700))),
                        IconButton(
                          tooltip: '${catalog?.labelOf(draft[i].type) ?? draft[i].type} colour',
                          icon: _swatch(draft[i].color, 22, p),
                          onPressed: () async {
                            final c = await showColorPickerSheet(context, title: '${catalog?.labelOf(draft[i].type) ?? draft[i].type} colour', current: draft[i].color);
                            if (c != null) setState(() => draft[i] = c.isEmpty ? draft[i].copyWith(clearColor: true) : draft[i].copyWith(color: c));
                          },
                        ),
                        IconButton(tooltip: 'Remove', icon: Icon(Icons.close, color: p.danger), onPressed: () => setState(() => draft.removeAt(i))),
                      ]),
                      Wrap(spacing: 16, children: [
                        _stepper('Width', draft[i].w, 2, 12, (v) => setState(() => draft[i] = draft[i].copyWith(w: v))),
                        _stepper('Height', draft[i].h, 1, 4, (v) => setState(() => draft[i] = draft[i].copyWith(h: v))),
                      ]),
                      if (draft[i].type == 'note')
                        Padding(
                          padding: const EdgeInsets.only(right: 8, top: 4),
                          child: TextFormField(
                            initialValue: draft[i].text,
                            maxLength: 400,
                            maxLines: 2,
                            decoration: const InputDecoration(labelText: 'Note text', counterText: ''),
                            onChanged: (v) => draft[i] = draft[i].copyWith(text: v),
                          ),
                        ),
                    ],
                  ),
                ),
              ),
          ],
        ),
      ],
    );
  }

  Widget _swatch(String? hex, double size, KinkitPalette p) {
    final c = parseHexColor(hex);
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(shape: BoxShape.circle, color: c ?? p.card, border: Border.all(color: p.borderStrong)),
      child: c == null ? Icon(Icons.block, size: size * 0.7, color: p.muted) : null,
    );
  }

  Widget _stepper(String label, int v, int min, int max, ValueChanged<int> on) => Row(mainAxisSize: MainAxisSize.min, children: [
        Text(label),
        IconButton(visualDensity: VisualDensity.compact, onPressed: v > min ? () => on(v - 1) : null, icon: const Icon(Icons.remove_circle_outline)),
        Text('$v', style: const TextStyle(fontWeight: FontWeight.w700)),
        IconButton(visualDensity: VisualDensity.compact, onPressed: v < max ? () => on(v + 1) : null, icon: const Icon(Icons.add_circle_outline)),
      ]);
}

// full screen

/// Always-on display of one board: immersive, refreshes every minute.
class WidgetsFullScreen extends ConsumerStatefulWidget {
  final WidgetBoard board;
  const WidgetsFullScreen({super.key, required this.board});

  @override
  ConsumerState<WidgetsFullScreen> createState() => _WidgetsFullScreenState();
}

class _WidgetsFullScreenState extends ConsumerState<WidgetsFullScreen> {
  Timer? _t;

  @override
  void initState() {
    super.initState();
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.immersiveSticky);
    _t = Timer.periodic(const Duration(minutes: 1), (_) => ref.invalidate(widgetDataProvider));
  }

  @override
  void dispose() {
    _t?.cancel();
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final b = widget.board;
    final types = (b.widgets.map((w) => w.type).toSet().toList()..sort()).join(',');
    final data = ref.watch(widgetDataProvider(types)).valueOrNull;
    final catalog = ref.watch(widgetCatalogProvider).valueOrNull;
    final p = context.palette;
    final bgColor = parseHexColor(b.background);
    final barText = bgColor == null ? p.muted : Color.lerp(readableOn(bgColor), bgColor, 0.32)!;
    return Scaffold(
      backgroundColor: bgColor ?? p.bg,
      body: SafeArea(
        child: Column(children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 8, 0),
            child: Row(children: [
              Expanded(child: Text('${data?.householdName ?? ''}  ·  ${b.name}', style: TextStyle(color: barText))),
              IconButton(tooltip: 'Exit full screen', color: bgColor == null ? null : readableOn(bgColor), icon: const Icon(Icons.fullscreen_exit), onPressed: () => Navigator.pop(context)),
            ]),
          ),
          Expanded(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(12),
              child: WidgetBoardView(widgets: b.widgets, data: data, catalog: catalog, display: true),
            ),
          ),
        ]),
      ),
    );
  }
}

// board renderer

/// 12-column flow grid (same layout rules as the web WidgetBoard.jsx).
class WidgetBoardView extends ConsumerWidget {
  final List<WidgetItem> widgets;
  final WidgetData? data;
  final WidgetCatalog? catalog;
  final bool display;
  const WidgetBoardView({super.key, required this.widgets, required this.data, required this.catalog, this.display = false});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (widgets.isEmpty) return const Padding(padding: EdgeInsets.all(24), child: Center(child: Text('This board is empty.')));
    final money = ref.watch(moneyProvider);
    return LayoutBuilder(builder: (context, c) {
      const gap = 12.0;
      final narrow = c.maxWidth < 600; // phones: one widget per row
      final row = display ? (c.maxWidth > 900 ? 150.0 : 120.0) : 112.0;
      return Wrap(
        spacing: gap,
        runSpacing: gap,
        children: [
          for (final w in widgets)
            SizedBox(
              width: narrow ? c.maxWidth : (c.maxWidth * w.w / 12 - gap * (12 - w.w) / 12).floorToDouble(),
              height: w.h * row + (w.h - 1) * gap,
              child: _WidgetTile(item: w, card: data?.cards[w.type], money: money.format, display: display),
            ),
        ],
      );
    });
  }
}

/// Colours for a widget filled with [hex]; null/invalid keeps the theme card look.
WidgetTone _toneFor(BuildContext context, String? hex) {
  final c = parseHexColor(hex);
  if (c != null) return WidgetTone.on(c);
  final p = context.palette;
  return WidgetTone(bg: p.card, text: p.text, muted: p.muted, track: p.surface2, accent: p.primary, danger: p.danger, border: p.border);
}

/// Whole-board backdrop behind the widgets (padded only when a colour is set).
class BoardBackdrop extends StatelessWidget {
  final String? color;
  final Widget child;
  const BoardBackdrop({super.key, required this.color, required this.child});

  @override
  Widget build(BuildContext context) {
    final c = parseHexColor(color);
    if (c == null) return child;
    return Container(
      key: const ValueKey('board-backdrop'),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(color: c, borderRadius: BorderRadius.circular(16)),
      child: child,
    );
  }
}

class _WidgetTile extends StatelessWidget {
  final WidgetItem item;
  final WidgetCard? card;
  final String Function(num) money;
  final bool display;
  const _WidgetTile({required this.item, required this.card, required this.money, required this.display});

  @override
  Widget build(BuildContext context) {
    final tone = _toneFor(context, item.color);
    return Container(
      decoration: BoxDecoration(color: tone.bg, borderRadius: BorderRadius.circular(14), border: Border.all(color: tone.border)),
      padding: const EdgeInsets.all(12),
      clipBehavior: Clip.antiAlias,
      child: switch (item.type) {
        'clock' => _Clock(display: display, tone: tone),
        'note' => Text(item.text.isEmpty ? 'Empty note' : item.text, style: TextStyle(fontSize: display ? 24 : 18, color: tone.text)),
        _ => card == null ? Text('No data', style: TextStyle(color: tone.muted)) : _CardBody(card: card!, money: money, display: display, tone: tone),
      },
    );
  }
}

class _Clock extends StatefulWidget {
  final bool display;
  final WidgetTone tone;
  const _Clock({required this.display, required this.tone});

  @override
  State<_Clock> createState() => _ClockState();
}

class _ClockState extends State<_Clock> {
  late final Timer _t;
  DateTime _now = DateTime.now();

  @override
  void initState() {
    super.initState();
    _t = Timer.periodic(const Duration(seconds: 1), (_) => setState(() => _now = DateTime.now()));
  }

  @override
  void dispose() {
    _t.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final tone = widget.tone;
    const wd = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
    const mo = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
    final hh = _now.hour.toString().padLeft(2, '0');
    final mm = _now.minute.toString().padLeft(2, '0');
    return Column(
      mainAxisAlignment: MainAxisAlignment.center,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        FittedBox(
          fit: BoxFit.scaleDown,
          child: Text('$hh:$mm', style: TextStyle(fontSize: widget.display ? 64 : 44, fontWeight: FontWeight.w700, color: tone.text, fontFeatures: const [FontFeature.tabularFigures()])),
        ),
        Text('${wd[_now.weekday - 1]}, ${_now.day} ${mo[_now.month - 1]}', style: TextStyle(color: tone.muted, fontSize: widget.display ? 20 : 14)),
      ],
    );
  }
}

class _CardBody extends StatelessWidget {
  final WidgetCard card;
  final String Function(num) money;
  final bool display;
  final WidgetTone tone;
  const _CardBody({required this.card, required this.money, required this.display, required this.tone});

  @override
  Widget build(BuildContext context) {
    final p = tone;
    final big = card.bigAmount != null ? money(card.bigAmount!) : card.big;
    final scale = display ? 1.25 : 1.0;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(card.title, style: TextStyle(fontSize: 12 * scale, fontWeight: FontWeight.w600, color: p.muted)),
        if (big.isNotEmpty)
          FittedBox(
            fit: BoxFit.scaleDown,
            child: Text(big,
                style: TextStyle(fontSize: 28 * scale, fontWeight: FontWeight.w700, color: (card.bigAmount ?? 0) < 0 ? p.danger : p.text)),
          ),
        if (card.caption.isNotEmpty) Text(card.caption, style: TextStyle(color: p.muted, fontSize: 12 * scale)),
        const SizedBox(height: 4),
        Expanded(
          child: card.rows.isEmpty
              ? (big.isEmpty ? Text(card.empty.isEmpty ? 'Nothing to show' : card.empty, style: TextStyle(color: p.muted)) : const SizedBox.shrink())
              : ClipRect(
                  child: SingleChildScrollView(
                    physics: const NeverScrollableScrollPhysics(),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        for (final r in card.rows)
                          Padding(
                            padding: const EdgeInsets.only(bottom: 6),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Row(children: [
                                  Expanded(
                                    child: Text(r.title,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: TextStyle(
                                          fontWeight: FontWeight.w600,
                                          fontSize: 14 * scale,
                                          color: r.alert ? p.danger : p.text,
                                          decoration: r.done ? TextDecoration.lineThrough : null,
                                        )),
                                  ),
                                  if (r.amount != null) Text(money(r.amount!), style: TextStyle(color: p.muted, fontSize: 12 * scale)),
                                ]),
                                if (r.detail.isNotEmpty) Text(r.detail, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: p.muted, fontSize: 12 * scale)),
                                if (r.pct != null)
                                  Padding(
                                    padding: const EdgeInsets.only(top: 3),
                                    child: ClipRRect(
                                      borderRadius: BorderRadius.circular(3),
                                      child: LinearProgressIndicator(
                                        minHeight: 5,
                                        value: (r.pct!.clamp(0, 100)) / 100,
                                        color: r.alert ? p.danger : p.accent,
                                        backgroundColor: p.track,
                                      ),
                                    ),
                                  ),
                              ],
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
