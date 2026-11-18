import 'package:flutter/material.dart';

import '../theme.dart';

/// Small KPI tile (like the web's StatCard): an uppercase 12px label with a
/// small tone square, then a 24px tabular value in the text colour. Only a
/// [negative] value turns red, so colour carries meaning.
class StatCard extends StatelessWidget {
  final String label;
  final String value;
  final String? sub;

  /// Tone of the small square next to the label.
  final Color? accent;
  final IconData? icon;
  final bool negative;

  const StatCard({
    super.key,
    required this.label,
    required this.value,
    this.sub,
    this.accent,
    this.icon,
    this.negative = false,
  });

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final tone = accent ?? p.primary;
    return Container(
      padding: const EdgeInsets.all(KSpace.s4),
      decoration: BoxDecoration(
        color: p.card,
        borderRadius: BorderRadius.circular(KRadius.lg),
        border: Border.all(color: p.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              if (icon != null)
                Icon(icon, size: 14, color: tone)
              else
                Container(
                  width: 8,
                  height: 8,
                  decoration: BoxDecoration(color: tone, borderRadius: BorderRadius.circular(2)),
                ),
              const SizedBox(width: 6),
              Expanded(
                child: Text(label.toUpperCase(),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, letterSpacing: 0.4, color: p.muted)),
              ),
            ],
          ),
          const SizedBox(height: KSpace.s2),
          FittedBox(
            fit: BoxFit.scaleDown,
            alignment: Alignment.centerLeft,
            child: Text(value,
                style: TextStyle(
                  fontSize: 24,
                  fontWeight: FontWeight.w700,
                  color: negative ? p.danger : p.text,
                  fontFeatures: kTabular,
                )),
          ),
          if (sub != null) ...[
            const SizedBox(height: KSpace.s1),
            Text(sub!, maxLines: 2, overflow: TextOverflow.ellipsis, style: TextStyle(fontSize: 12, color: p.muted)),
          ],
        ],
      ),
    );
  }
}

/// Two-column grid of StatCards.
class StatGrid extends StatelessWidget {
  final List<Widget> children;
  const StatGrid({super.key, required this.children});

  @override
  Widget build(BuildContext context) {
    final rows = <Widget>[];
    for (var i = 0; i < children.length; i += 2) {
      rows.add(Padding(
        padding: const EdgeInsets.only(bottom: KSpace.s3),
        child: IntrinsicHeight(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Expanded(child: children[i]),
              const SizedBox(width: KSpace.s3),
              Expanded(child: i + 1 < children.length ? children[i + 1] : const SizedBox()),
            ],
          ),
        ),
      ));
    }
    return Padding(padding: const EdgeInsets.symmetric(horizontal: 16), child: Column(children: rows));
  }
}

/// Rounded progress bar; turns red when [over].
class KProgressBar extends StatelessWidget {
  final double value; // 0..1
  final Color? color;
  final bool over;

  const KProgressBar({super.key, required this.value, this.color, this.over = false});

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    return ClipRRect(
      borderRadius: BorderRadius.circular(99),
      child: LinearProgressIndicator(
        value: value.clamp(0.0, 1.0),
        minHeight: 8,
        backgroundColor: p.surface2,
        color: over ? p.danger : (color ?? p.primary),
      ),
    );
  }
}
