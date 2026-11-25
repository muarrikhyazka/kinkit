import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../util/dates.dart';
import '../util/money.dart';
import 'amount_field.dart';
import 'async_view.dart';

enum QuickAddKind { spending, income }

/// Opens the quick-add bottom sheet (web: QuickAddSpending.jsx).
/// Family admins/parents/viewers-with-read can switch to income when [allowIncome].
Future<void> showQuickAdd(BuildContext context, {QuickAddKind initial = QuickAddKind.spending, bool allowIncome = false}) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    showDragHandle: true,
    builder: (_) => QuickAddSheet(initial: initial, allowIncome: allowIncome),
  );
}

class QuickAddSheet extends ConsumerStatefulWidget {
  final QuickAddKind initial;
  final bool allowIncome;

  const QuickAddSheet({super.key, this.initial = QuickAddKind.spending, this.allowIncome = false});

  @override
  ConsumerState<QuickAddSheet> createState() => _QuickAddSheetState();
}

class _QuickAddSheetState extends ConsumerState<QuickAddSheet> {
  final _form = GlobalKey<FormState>();
  final _amount = TextEditingController();
  final _note = TextEditingController();
  late QuickAddKind _kind = widget.initial;
  String? _category;
  int? _sourceId;
  DateTime _date = DateTime.now();
  bool _saving = false;
  String? _error;
  // Idempotency key: a retried save after a flaky network never duplicates.
  String _clientId = _newClientId();

  static String _newClientId() => 'm-${DateTime.now().microsecondsSinceEpoch}';

  @override
  void dispose() {
    _amount.dispose();
    _note.dispose();
    super.dispose();
  }

  Future<void> _pickDate() async {
    final d = await showDatePicker(
      context: context,
      initialDate: _date,
      firstDate: DateTime(2000),
      lastDate: DateTime.now().add(const Duration(days: 366)),
    );
    if (d != null) setState(() => _date = d);
  }

  Future<void> _save() async {
    if (!(_form.currentState?.validate() ?? false)) return;
    final amount = parseAmount(_amount.text)!;
    final api = ref.read(apiProvider);
    setState(() => _saving = true);
    try {
      if (_kind == QuickAddKind.spending) {
        await api.addSpending(
          txDate: isoDate(_date),
          description: _note.text.trim(),
          category: _category ?? '',
          amount: amount,
          clientId: _clientId,
        );
        invalidateSpending(ref);
      } else {
        await api.addIncome(
          txDate: isoDate(_date),
          amount: amount,
          sourceId: _sourceId,
          description: _note.text.trim(),
          clientId: _clientId,
        );
        invalidateIncome(ref);
      }
      if (!mounted) return;
      final money = MoneyFormatter.idr(amount);
      showSnack(context, _kind == QuickAddKind.spending ? 'Spending $money saved' : 'Income $money saved');
      Navigator.of(context).pop();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _saving = false;
        _error = 'Could not save: $e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final setup = ref.watch(setupProvider);
    final p = context.palette;
    final bottom = MediaQuery.of(context).viewInsets.bottom;
    return Padding(
      padding: EdgeInsets.fromLTRB(20, 0, 20, 20 + bottom),
      child: Form(
        key: _form,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(_kind == QuickAddKind.spending ? 'Add spending' : 'Add income',
                  style: TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: p.text)),
              const SizedBox(height: 12),
              if (widget.allowIncome) ...[
                SegmentedButton<QuickAddKind>(
                  segments: const [
                    ButtonSegment(value: QuickAddKind.spending, label: Text('Spending'), icon: Icon(Icons.shopping_bag_outlined)),
                    ButtonSegment(value: QuickAddKind.income, label: Text('Income'), icon: Icon(Icons.payments_outlined)),
                  ],
                  selected: {_kind},
                  onSelectionChanged: (s) => setState(() {
                    _kind = s.first;
                    _clientId = _newClientId();
                  }),
                ),
                const SizedBox(height: 16),
              ],
              AmountField(controller: _amount, autofocus: true, textInputAction: TextInputAction.next),
              const SizedBox(height: 12),
              setup.when(
                loading: () => const LinearProgressIndicator(),
                error: (e, _) => Text('Could not load categories: $e', style: TextStyle(color: p.danger)),
                data: (s) => _kind == QuickAddKind.spending ? _categoryField(s) : _sourceField(s),
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _note,
                textCapitalization: TextCapitalization.sentences,
                decoration: InputDecoration(labelText: _kind == QuickAddKind.spending ? 'Note / description' : 'Description'),
              ),
              const SizedBox(height: 12),
              OutlinedButton.icon(
                onPressed: _pickDate,
                icon: const Icon(Icons.event),
                label: Text(isoDate(_date) == todayIso() ? 'Today' : prettyDate(isoDate(_date))),
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(_error!, style: TextStyle(color: p.danger)),
              ],
              const SizedBox(height: 20),
              FilledButton(
                onPressed: _saving ? null : _save,
                child: _saving
                    ? SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2, color: context.palette.onPrimary))
                    : const Text('Save'),
              ),
            ],
          ),
        ),
      ),
    );
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

  Widget _sourceField(SetupData s) {
    return DropdownButtonFormField<int?>(
      value: _sourceId,
      isExpanded: true,
      decoration: const InputDecoration(labelText: 'Source'),
      items: [
        const DropdownMenuItem<int?>(value: null, child: Text('No source')),
        for (final src in s.incomeSources) DropdownMenuItem<int?>(value: src.id, child: Text(src.name)),
      ],
      onChanged: (v) => setState(() => _sourceId = v),
    );
  }
}
