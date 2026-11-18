import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/data_providers.dart';
import '../theme.dart';

/// Compact "[star] 120  [flame] 3" chip for the purple app bar (web:
/// GamificationBadge.jsx): translucent white on the chrome, 40px tap target.
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
    const amber = Color(0xFFFBBF24); // reads on the purple chrome in both themes
    final label = TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: p.onChrome, fontFeatures: kTabular);
    return Tooltip(
      message: tooltip,
      child: Material(
        type: MaterialType.transparency,
        child: InkWell(
          borderRadius: BorderRadius.circular(99),
          onTap: onTap,
          child: Container(
            constraints: const BoxConstraints(minHeight: 40),
            padding: const EdgeInsets.symmetric(horizontal: 12),
            decoration: BoxDecoration(
              color: p.onChrome.withValues(alpha: 0.14),
              borderRadius: BorderRadius.circular(99),
              border: Border.all(color: p.onChrome.withValues(alpha: 0.18)),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.star_rounded, size: 16, color: amber),
                const SizedBox(width: 3),
                Text('${g.myPoints}', style: label),
                const SizedBox(width: 8),
                const Icon(Icons.local_fire_department, size: 16, color: amber),
                const SizedBox(width: 3),
                Text('${g.myStreak}', style: label),
              ],
            ),
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
                Text('${g.myPoints} pts',
                    style: TextStyle(fontWeight: FontWeight.w700, fontSize: 16, color: p.text, fontFeatures: kTabular)),
                const Spacer(),
                Icon(Icons.local_fire_department, size: 18, color: p.warning),
                const SizedBox(width: 4),
                Flexible(
                  child: Text('${g.myStreak} day streak',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: p.warning, fontWeight: FontWeight.w600)),
                ),
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
