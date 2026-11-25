import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/data_providers.dart';
import '../theme.dart';
import 'steps_icon.dart';

/// Medal colours for badge tiers (same as the web's --tier-* tokens).
const kTierBronze = Color(0xFFB4753C);
const kTierSilver = Color(0xFF8A94A6);
const kTierGold = Color(0xFFE0A100);

Color tierColor(String tier) {
  switch (tier) {
    case 'gold':
      return kTierGold;
    case 'silver':
      return kTierSilver;
  }
  return kTierBronze;
}

/// Level emblem rim: bronze below level 10, silver below 25, gold from 25.
String levelTier(int n) => n >= 25 ? 'gold' : (n >= 10 ? 'silver' : 'bronze');

/// Hexagon with the level number inside (web: LevelBadge in icons.jsx).
class LevelHex extends StatelessWidget {
  final int level;
  final double size;
  final Color? fill;
  final Color? numberColor;
  const LevelHex({super.key, required this.level, this.size = 40, this.fill, this.numberColor});

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final digits = '$level'.length;
    return Semantics(
      label: 'Level $level',
      child: SizedBox(
        width: size,
        height: size,
        child: CustomPaint(
          painter: _HexPainter(fill: fill ?? p.primaryFill, rim: tierColor(levelTier(level))),
          child: Center(
            child: ExcludeSemantics(
              child: Text(
                '$level',
                style: TextStyle(
                  color: numberColor ?? p.onPrimary,
                  fontWeight: FontWeight.w800,
                  fontSize: size * (digits > 1 ? 0.36 : 0.44),
                  height: 1,
                  fontFeatures: kTabular,
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _HexPainter extends CustomPainter {
  final Color fill;
  final Color rim;
  const _HexPainter({required this.fill, required this.rim});

  @override
  void paint(Canvas canvas, Size size) {
    final stroke = size.width * 0.06;
    final r = size.width / 2 - stroke;
    final c = size.center(Offset.zero);
    final path = Path();
    for (var i = 0; i < 6; i++) {
      final a = -math.pi / 2 + i * math.pi / 3;
      final pt = Offset(c.dx + r * math.cos(a), c.dy + r * math.sin(a));
      i == 0 ? path.moveTo(pt.dx, pt.dy) : path.lineTo(pt.dx, pt.dy);
    }
    path.close();
    canvas.drawPath(path, Paint()..color = fill);
    canvas.drawPath(
      path,
      Paint()
        ..color = rim
        ..style = PaintingStyle.stroke
        ..strokeWidth = stroke
        ..strokeJoin = StrokeJoin.round,
    );
  }

  @override
  bool shouldRepaint(_HexPainter old) => old.fill != fill || old.rim != rim;
}

/// Compact "Lv 3  [steps] 120  [flame] 3" chip for the purple app bar (web:
/// GamificationBadge.jsx): translucent white on the chrome, 40px tap target,
/// with a thin progress bar toward the next level along the bottom.
class GamificationBadge extends ConsumerWidget {
  final VoidCallback? onTap;
  const GamificationBadge({super.key, this.onTap});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final g = ref.watch(gamificationProvider).valueOrNull;
    if (g == null) return const SizedBox.shrink();
    final p = context.palette;
    final lv = g.myLevelNumber;
    final tooltip = '${lv > 0 ? 'Level $lv · ' : ''}${g.myLevel} · ${g.myPoints} pts · ${g.myStreak} day streak'
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
            decoration: BoxDecoration(
              color: p.onChrome.withValues(alpha: 0.14),
              borderRadius: BorderRadius.circular(99),
              border: Border.all(color: p.onChrome.withValues(alpha: 0.18)),
            ),
            clipBehavior: Clip.antiAlias,
            child: Stack(
              alignment: Alignment.bottomCenter,
              children: [
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (lv > 0) ...[
                        Text('Lv $lv', style: label.copyWith(color: p.chromeSoft)),
                        const SizedBox(width: 8),
                      ],
                      const StepsIcon(size: 16, color: amber),
                      const SizedBox(width: 3),
                      Text('${g.myPoints}', style: label),
                      const SizedBox(width: 8),
                      const Icon(Icons.local_fire_department, size: 16, color: amber),
                      const SizedBox(width: 3),
                      Text('${g.myStreak}', style: label),
                    ],
                  ),
                ),
                if (lv > 0 && g.myNextLevelAt != null)
                  LinearProgressIndicator(
                    value: g.levelProgress,
                    minHeight: 3,
                    backgroundColor: Colors.transparent,
                    color: p.onChrome.withValues(alpha: 0.85),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// Level card used on Today and Family: big level hexagon, title, XP bar from the
/// level's floor to the next level, "N pts to Level X", plus points and streak.
class GamificationCard extends ConsumerWidget {
  const GamificationCard({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final g = ref.watch(gamificationProvider).valueOrNull;
    if (g == null) return const SizedBox.shrink();
    final p = context.palette;
    final lv = g.myLevelNumber;
    final next = g.myNextLevel;
    final nextAt = g.myNextLevelAt;
    final hasNext = next != null && nextAt != null && nextAt > 0;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                if (lv > 0) ...[
                  LevelHex(level: lv, size: 64),
                  const SizedBox(width: 14),
                ],
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (lv > 0)
                        Text('Level $lv',
                            style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: p.muted)),
                      Text(g.myLevel,
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(fontWeight: FontWeight.w700, fontSize: 20, color: p.text)),
                      const SizedBox(height: 4),
                      Row(
                        children: [
                          StepsIcon(size: 18, color: p.warning),
                          const SizedBox(width: 3),
                          Text('${g.myPoints} pts',
                              style: TextStyle(fontWeight: FontWeight.w600, color: p.text, fontFeatures: kTabular)),
                          const SizedBox(width: 12),
                          Icon(Icons.local_fire_department, size: 18, color: p.warning),
                          const SizedBox(width: 3),
                          Flexible(
                            child: Text('${g.myStreak} day streak',
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: TextStyle(color: p.warning, fontWeight: FontWeight.w600)),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ],
            ),
            if (hasNext) ...[
              const SizedBox(height: 12),
              ClipRRect(
                borderRadius: BorderRadius.circular(99),
                child: LinearProgressIndicator(
                  value: g.levelProgress,
                  minHeight: 10,
                  backgroundColor: p.border,
                  color: p.primary,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                '${g.myPointsToNext} pts to ${lv > 0 ? 'Level ${lv + 1} · ' : ''}$next',
                style: TextStyle(fontSize: 12, color: p.muted, fontFeatures: kTabular),
              ),
            ] else if (lv > 0) ...[
              const SizedBox(height: 8),
              Text('Max level reached', style: TextStyle(fontSize: 12, color: p.muted)),
            ],
          ],
        ),
      ),
    );
  }
}
