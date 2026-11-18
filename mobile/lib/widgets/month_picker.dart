import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/providers.dart';
import '../theme.dart';
import '../util/dates.dart';

/// "< November 2026 >" bar bound to [selectedMonthProvider].
/// Tapping the label jumps back to the current month.
class MonthPicker extends ConsumerWidget {
  const MonthPicker({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ym = ref.watch(selectedMonthProvider);
    final ctrl = ref.read(selectedMonthProvider.notifier);
    final p = context.palette;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      child: Row(
        children: [
          IconButton(
            tooltip: 'Previous month',
            icon: const Icon(Icons.chevron_left),
            onPressed: () => ctrl.state = ym.add(-1),
          ),
          Expanded(
            child: InkWell(
              borderRadius: BorderRadius.circular(8),
              onTap: ym.isCurrent ? null : () => ctrl.state = YearMonth.now(),
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: 8),
                child: Column(
                  children: [
                    Text(ym.label, style: TextStyle(fontWeight: FontWeight.w600, fontSize: 16, color: p.text)),
                    if (!ym.isCurrent)
                      Text('Tap for this month', style: TextStyle(fontSize: 11, color: p.muted)),
                  ],
                ),
              ),
            ),
          ),
          IconButton(
            tooltip: 'Next month',
            icon: const Icon(Icons.chevron_right),
            onPressed: () => ctrl.state = ym.add(1),
          ),
        ],
      ),
    );
  }
}
