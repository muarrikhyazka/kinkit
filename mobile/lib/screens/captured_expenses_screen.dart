import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../api/models.dart';
import '../capture/capture_channel.dart';
import '../capture/capture_providers.dart';
import '../capture/notification_parser.dart';
import '../config.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../util/dates.dart';
import '../util/money.dart';
import '../widgets/amount_field.dart';
import '../widgets/async_view.dart';
import '../widgets/mascot.dart';
import 'settings_screen.dart';

String _amountLabel(ParsedCapture p) {
  if (!p.amountKnown) return 'Amount unclear';
  return p.currency == 'IDR' ? MoneyFormatter.idr(p.amount) : '${p.currency} ${p.amount}';
}

String _when(DateTime t) {
  final time = DateFormat('HH:mm').format(t);
  return isoDate(t) == todayIso() ? 'Today $time' : '${prettyDate(isoDate(t))} $time';
}

/// Payment notifications caught on this phone, waiting for a person to check
/// them. Nothing here is saved until the user reviews it and taps Save.
class CapturedExpensesScreen extends ConsumerStatefulWidget {
  const CapturedExpensesScreen({super.key});

  @override
  ConsumerState<CapturedExpensesScreen> createState() => _CapturedExpensesScreenState();
}

class _CapturedExpensesScreenState extends ConsumerState<CapturedExpensesScreen> with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // Back from Android's notification access screen, or from another app
    // that just posted a payment notification.
    if (state == AppLifecycleState.resumed) _invalidate();
  }

  void _invalidate() {
    ref.invalidate(captureStatusProvider);
    ref.invalidate(pendingCapturesProvider);
  }

  Future<void> _refresh() async {
    _invalidate();
    await ref.read(pendingCapturesProvider.future);
  }

  Future<void> _dismissAll() async {
    final ok = await confirmDialog(context,
        title: 'Dismiss all captured expenses?',
        message: 'They are deleted from this phone. Nothing is saved to Kinkit.',
        confirm: 'Dismiss all');
    if (!ok) return;
    try {
      await ref.read(captureChannelProvider).clear();
      if (mounted) showSnack(context, 'Dismissed. Nothing was saved.');
    } on CaptureException catch (e) {
      if (mounted) showSnack(context, 'Nothing was deleted, the list is unchanged. $e', error: true);
    }
    ref.invalidate(pendingCapturesProvider);
  }

  Future<void> _review(PendingCapture c) async {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      showDragHandle: true,
      builder: (_) => _ReviewSheet(capture: c),
    );
  }

  @override
  Widget build(BuildContext context) {
    final me = ref.watch(meProvider);
    final status = ref.watch(captureStatusProvider);
    final pending = ref.watch(pendingCapturesProvider);
    final p = context.palette;
    final hasItems = (pending.valueOrNull?.isNotEmpty) ?? false;

    Widget body;
    if (me != null && !me.canAddSpending) {
      body = const Center(
        child: EmptyState(
          icon: Icons.visibility_outlined,
          title: 'Viewers cannot save expenses',
          message: 'Your role is read-only. Ask a family admin to change it if you need to log spending.',
        ),
      );
    } else if (status.hasError && !status.hasValue) {
      body = ErrorState(message: status.error.toString(), onRetry: _invalidate);
    } else if (status.valueOrNull != null && !status.requireValue.supported) {
      body = const Center(
        child: EmptyState(
          icon: Icons.notifications_off_outlined,
          title: 'Capture is not available here',
          message: 'Reading payment notifications works on Android phones only. '
              'iPhone does not let an app read other apps\' notifications.',
        ),
      );
    } else {
      final st = status.valueOrNull;
      body = PullToRefresh(
        onRefresh: _refresh,
        child: AsyncView<List<PendingCapture>>(
          value: pending,
          onRetry: _invalidate,
          builder: (items) => ListView(
            physics: const AlwaysScrollableScrollPhysics(),
            padding: const EdgeInsets.only(bottom: KSpace.s6),
            children: [
              if (st != null && !st.active) _OffBanner(status: st),
              if (items.isEmpty)
                Center(
                  child: EmptyState(
                    mascot: MascotPose.relax,
                    title: 'Nothing to review',
                    message: 'Payment notifications from your bank and e-wallet apps show up here after they arrive. '
                        'One-time codes and promotions are discarded.',
                  ),
                )
              else ...[
                Padding(
                  padding: const EdgeInsets.fromLTRB(20, KSpace.s3, 20, KSpace.s1),
                  child: Text(
                    'Check each one before it is saved. Notification text stays on this phone; only an expense you save is sent to Kinkit.',
                    style: TextStyle(color: p.muted),
                  ),
                ),
                for (final c in items) _CaptureTile(capture: c, onTap: () => _review(c)),
              ],
            ],
          ),
        ),
      );
    }

    return Scaffold(
      appBar: AppBar(
        title: const Text('Captured expenses'),
        actions: [
          if (hasItems)
            IconButton(
              tooltip: 'Dismiss all',
              icon: const Icon(Icons.delete_sweep_outlined),
              onPressed: _dismissAll,
            ),
        ],
      ),
      body: body,
    );
  }
}

class _OffBanner extends StatelessWidget {
  final CaptureStatus status;
  const _OffBanner({required this.status});

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final message = !status.permission
        ? 'Notification access is off for Kinkit, so no new payments are being captured.'
        : 'Capture is switched off, so no new payments are being captured.';
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(KSpace.s4),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(Icons.notifications_off_outlined, color: p.muted),
                const SizedBox(width: KSpace.s3),
                Expanded(child: Text(message, style: TextStyle(color: p.text))),
              ],
            ),
            const SizedBox(height: KSpace.s3),
            Align(
              alignment: Alignment.centerLeft,
              child: OutlinedButton(
                onPressed: () =>
                    Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => const SettingsScreen())),
                child: const Text('Open Settings'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _CaptureTile extends StatelessWidget {
  final PendingCapture capture;
  final VoidCallback onTap;
  const _CaptureTile({required this.capture, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final item = capture.item;
    final parsed = capture.parsed;
    final title = parsed.merchant.isNotEmpty ? parsed.merchant : 'Payment notification';
    final source = item.app.isNotEmpty ? item.app : item.package;
    return Card(
      child: ListTile(
        minVerticalPadding: KSpace.s3,
        onTap: onTap,
        title: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('$source · ${_when(item.postedAt)}', maxLines: 1, overflow: TextOverflow.ellipsis),
            if (parsed.isIncome)
              Padding(
                padding: const EdgeInsets.only(top: KSpace.s1),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: KSpace.s2, vertical: 2),
                  decoration: BoxDecoration(
                    color: p.primaryLight,
                    border: Border.all(color: p.border),
                    borderRadius: BorderRadius.circular(KRadius.sm),
                  ),
                  child: Text('Money in', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: p.text)),
                ),
              ),
          ],
        ),
        trailing: Text(
          _amountLabel(parsed),
          style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: p.text, fontFeatures: kTabular),
        ),
      ),
    );
  }
}

class _ReviewSheet extends ConsumerStatefulWidget {
  final PendingCapture capture;
  const _ReviewSheet({required this.capture});

  @override
  ConsumerState<_ReviewSheet> createState() => _ReviewSheetState();
}

class _ReviewSheetState extends ConsumerState<_ReviewSheet> {
  final _form = GlobalKey<FormState>();
  late final TextEditingController _description = TextEditingController(text: widget.capture.parsed.merchant);
  // A foreign-currency or unreadable amount is never pre-filled: the field takes IDR.
  late final TextEditingController _amount = TextEditingController(
      text: widget.capture.parsed.currency == 'IDR' && widget.capture.parsed.amountKnown
          ? '${widget.capture.parsed.amount}'
          : '');
  late DateTime _date = widget.capture.item.postedAt;
  String? _category;
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _description.dispose();
    _amount.dispose();
    super.dispose();
  }

  Future<void> _pickDate() async {
    final d = await showDatePicker(
      context: context,
      initialDate: _date,
      firstDate: DateTime(2000),
      lastDate: DateTime.now().add(const Duration(days: 1)),
    );
    if (d != null) setState(() => _date = d);
  }

  Future<void> _dismiss() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(captureChannelProvider).remove([widget.capture.item.id]);
    } on CaptureException catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = 'Not dismissed, it is still in the list. $e';
      });
      return;
    }
    if (!mounted) return;
    ref.invalidate(pendingCapturesProvider);
    showSnack(context, 'Dismissed. Nothing was saved.');
    Navigator.of(context).pop();
  }

  Future<void> _save() async {
    if (!(_form.currentState?.validate() ?? false)) return;
    final amount = parseAmount(_amount.text)!;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(apiProvider).addSpending(
            txDate: isoDate(_date),
            description: _description.text.trim(),
            category: _category ?? '',
            amount: amount,
            clientId: captureClientId(widget.capture.item.id),
            source: AppConfig.notificationSource,
          );
      String? cleanupError;
      try {
        await ref.read(captureChannelProvider).remove([widget.capture.item.id]);
      } on CaptureException catch (e) {
        cleanupError = e.toString();
      }
      if (!mounted) return;
      invalidateSpending(ref);
      ref.invalidate(pendingCapturesProvider);
      showSnack(
        context,
        cleanupError == null
            ? 'Spending ${MoneyFormatter.idr(amount)} saved'
            : 'Spending ${MoneyFormatter.idr(amount)} saved, but it is still in the captured list. '
                'Dismiss it there so it is not saved twice. $cleanupError',
        error: cleanupError != null,
      );
      Navigator.of(context).pop();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = 'Could not save, so it is still in the list. $e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final item = widget.capture.item;
    final parsed = widget.capture.parsed;
    final setup = ref.watch(setupProvider);
    final bottom = MediaQuery.of(context).viewInsets.bottom;
    final original = [item.title, item.text].where((s) => s.trim().isNotEmpty).join('\n');

    return PopScope(
        canPop: !_busy,
        child: Padding(
          padding: EdgeInsets.fromLTRB(20, 0, 20, 20 + bottom),
          child: Form(
            key: _form,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(parsed.isIncome ? 'Money in' : 'Review expense',
                      style: TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: p.text)),
                  const SizedBox(height: KSpace.s3),
                  Container(
                    padding: const EdgeInsets.all(KSpace.s3),
                    decoration: BoxDecoration(
                      color: p.surface2,
                      border: Border.all(color: p.border),
                      borderRadius: BorderRadius.circular(KRadius.md),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('${item.app.isNotEmpty ? item.app : item.package} · ${_when(item.postedAt)}',
                            style: TextStyle(fontSize: 12, color: p.muted)),
                        const SizedBox(height: KSpace.s1),
                        Text(original, maxLines: 6, overflow: TextOverflow.ellipsis, style: TextStyle(color: p.text)),
                      ],
                    ),
                  ),
                  const SizedBox(height: KSpace.s4),
                  if (parsed.isIncome) ...[
                    Text(
                      'This looks like money coming in. Captured notifications are saved as spending only, '
                      'so dismiss it, or log the income yourself under Money > Income.',
                      style: TextStyle(color: p.text),
                    ),
                    const SizedBox(height: KSpace.s4),
                    if (_error != null) ...[
                      Text(_error!, style: TextStyle(color: p.danger)),
                      const SizedBox(height: KSpace.s3),
                    ],
                    FilledButton(onPressed: _busy ? null : _dismiss, child: const Text('Dismiss')),
                  ] else ...[
                    if (!parsed.amountKnown) ...[
                      Text(
                        'Kinkit could not read an amount from this notification. Check the original text above and type it in.',
                        style: TextStyle(color: p.text),
                      ),
                      const SizedBox(height: KSpace.s3),
                    ],
                    if (parsed.currency != 'IDR') ...[
                      Container(
                        padding: const EdgeInsets.all(KSpace.s3),
                        decoration: BoxDecoration(
                          color: p.surface2,
                          border: Border.all(color: p.warning),
                          borderRadius: BorderRadius.circular(KRadius.md),
                        ),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Icon(Icons.warning_amber_outlined, color: p.warning),
                            const SizedBox(width: KSpace.s3),
                            Expanded(
                              child: Text(
                                'This notification is in ${parsed.currency} (${parsed.amount}). '
                                'Kinkit takes amounts in IDR, so type the converted amount below. Nothing is converted for you.',
                                style: TextStyle(color: p.text),
                              ),
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(height: KSpace.s4),
                    ],
                    TextFormField(
                      controller: _description,
                      textCapitalization: TextCapitalization.sentences,
                      textInputAction: TextInputAction.next,
                      decoration: const InputDecoration(labelText: 'Description'),
                      validator: (v) => (v == null || v.trim().isEmpty) ? 'Add a short description' : null,
                    ),
                    const SizedBox(height: KSpace.s3),
                    AmountField(controller: _amount, textInputAction: TextInputAction.next),
                    const SizedBox(height: KSpace.s3),
                    setup.when(
                      loading: () => const LinearProgressIndicator(),
                      error: (e, _) => Text('Could not load categories: $e', style: TextStyle(color: p.danger)),
                      data: _categoryField,
                    ),
                    const SizedBox(height: KSpace.s3),
                    OutlinedButton.icon(
                      onPressed: _pickDate,
                      icon: const Icon(Icons.event),
                      label: Text(isoDate(_date) == todayIso() ? 'Today' : prettyDate(isoDate(_date))),
                    ),
                    if (_error != null) ...[
                      const SizedBox(height: KSpace.s3),
                      Text(_error!, style: TextStyle(color: p.danger)),
                    ],
                    const SizedBox(height: KSpace.s5),
                    Row(
                      children: [
                        OutlinedButton(onPressed: _busy ? null : _dismiss, child: const Text('Dismiss')),
                        const SizedBox(width: KSpace.s3),
                        Expanded(
                          child: FilledButton(
                            onPressed: _busy ? null : _save,
                            child: _busy
                                ? SizedBox(
                                    height: 20,
                                    width: 20,
                                    child: CircularProgressIndicator(
                                        strokeWidth: 2, color: context.palette.onPrimary),
                                  )
                                : const Text('Save spending'),
                          ),
                        ),
                      ],
                    ),
                  ],
                ],
              ),
            ),
          ),
        ));
  }

  Widget _categoryField(SetupData s) {
    final groups = s.categoriesByMain;
    final items = <DropdownMenuItem<String>>[];
    for (final main in groups.keys) {
      final cats = groups[main]!;
      if (cats.isEmpty) continue;
      items.add(DropdownMenuItem<String>(
        enabled: false,
        value: '__$main',
        // The group colour is a dot beside the name, never the text colour
        // (amber and green fail 4.5:1 as text on white).
        child: Row(children: [
          Container(
            width: 8,
            height: 8,
            decoration: BoxDecoration(shape: BoxShape.circle, color: mainCategoryColor(main)),
          ),
          const SizedBox(width: 8),
          Flexible(child: Text(main, style: TextStyle(fontWeight: FontWeight.w700, color: context.palette.text))),
        ]),
      ));
      for (final c in cats) {
        items.add(DropdownMenuItem<String>(
          value: c.name,
          child: Padding(padding: const EdgeInsets.only(left: 12), child: Text(c.name)),
        ));
      }
    }
    return DropdownButtonFormField<String>(
      value: _category,
      isExpanded: true,
      items: items,
      decoration: const InputDecoration(labelText: 'Category'),
      onChanged: (v) => setState(() => _category = v),
      validator: (v) => (v == null || v.isEmpty) ? 'Pick a category' : null,
    );
  }
}

/// Row for More > App: how many captured payments are waiting. Shown on
/// Android only (the caller checks [captureSupportedPlatform]).
class CapturedExpensesTile extends ConsumerStatefulWidget {
  const CapturedExpensesTile({super.key});

  @override
  ConsumerState<CapturedExpensesTile> createState() => _CapturedExpensesTileState();
}

class _CapturedExpensesTileState extends ConsumerState<CapturedExpensesTile> with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      ref.invalidate(captureStatusProvider);
      ref.invalidate(pendingCapturesProvider);
    }
  }

  @override
  Widget build(BuildContext context) {
    final status = ref.watch(captureStatusProvider);
    final st = status.valueOrNull;
    final pending = ref.watch(pendingCapturesProvider);
    if (st != null && !st.supported) return const SizedBox.shrink();

    final count = pending.valueOrNull?.length;
    final statusFailed = status.hasError && st == null;
    String subtitle;
    if (statusFailed) {
      subtitle = 'Could not check capture. Open this to retry.';
    } else if (st != null && !st.active) {
      subtitle = 'Off. Turn it on in Settings > Expense capture.';
    } else if (count == null) {
      subtitle = pending.hasError ? 'Could not read the list. Open this to retry.' : 'Checking';
    } else if (count == 0) {
      subtitle = 'Nothing waiting for review';
    } else {
      subtitle = '$count to review';
    }

    return ListTile(
      leading: const Icon(Icons.notifications_active_outlined),
      title: const Text('Captured expenses'),
      subtitle: Text(subtitle),
      trailing: const Icon(Icons.chevron_right),
      onTap: () async {
        await Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => const CapturedExpensesScreen()));
        if (mounted) ref.invalidate(pendingCapturesProvider);
      },
    );
  }
}
