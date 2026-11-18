import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/data_providers.dart';
import '../theme.dart';

/// Compact "[star] 120  [flame] 3" chip for the app bar (web: GamificationBadge.jsx).
class GamificationBadge extends ConsumerWidget {
  final VoidCallback? onTap;
  const GamificationBadge({super.key, this.onTap});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final g = ref.watch(gamificationProvider).valueOrNull;
    if (g == null) return const SizedBox.shrink();
    final p = context.palette;
    final tooltip = '${g.myPoints} pts · ${g.myStreak} day streak · ${g.myLevel}'
        '${g.member != null ? ' · household ${g.points} pts' : ''}';
    return Tooltip(
      message: tooltip,
      child: InkWell(
        borderRadius: BorderRadius.circular(99),
        onTap: onTap,
        child: Container(
          margin: const EdgeInsets.symmetric(horizontal: 8),
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
          decoration: BoxDecoration(color: p.primaryLight, borderRadius: BorderRadius.circular(99)),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.star_rounded, size: 16, color: p.warning),
              const SizedBox(width: 3),
              Text('${g.myPoints}', style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: p.text)),
              const SizedBox(width: 8),
              Icon(Icons.local_fire_department, size: 16, color: p.warning),
              const SizedBox(width: 3),
              Text('${g.myStreak}', style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: p.text)),
            ],
          ),
        ),
      ),
    );
  }
}

/// Larger card with level progress, used on Today and Family.
class GamificationCard extends ConsumerWidget {
  const GamificationCard({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final g = ref.watch(gamificationProvider).valueOrNull;
    if (g == null) return const SizedBox.shrink();
    final p = context.palette;
    final next = g.myNextLevel;
    final nextAt = g.myNextLevelAt;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.star_rounded, size: 20, color: p.warning),
                const SizedBox(width: 4),
                Text('${g.myPoints} pts', style: TextStyle(fontWeight: FontWeight.w700, fontSize: 16, color: p.text)),
                const Spacer(),
                Icon(Icons.local_fire_department, size: 18, color: p.warning),
                const SizedBox(width: 4),
                Text('${g.myStreak} day streak', style: TextStyle(color: p.warning, fontWeight: FontWeight.w600)),
              ],
            ),
            const SizedBox(height: 6),
            Text(g.myLevel, style: TextStyle(color: p.muted)),
            if (next != null && nextAt != null && nextAt > 0) ...[
              const SizedBox(height: 10),
              ClipRRect(
                borderRadius: BorderRadius.circular(99),
                child: LinearProgressIndicator(
                  value: g.levelProgress,
                  minHeight: 8,
                  backgroundColor: p.border,
                  color: p.primary,
                ),
              ),
              const SizedBox(height: 4),
              Text('${nextAt - g.myPoints} pts to $next', style: TextStyle(fontSize: 12, color: p.muted)),
            ],
          ],
        ),
      ),
    );
  }
}
