import 'package:flutter/material.dart';

/// The points glyph: a rising staircase (web: IconSteps in icons.jsx, path
/// "M3 20h5v-5h5v-5h5V5h3" on a 24 unit grid). Points are progress, so the
/// icon is steps up, not a star. Takes [color] or the ambient icon colour.
class StepsIcon extends StatelessWidget {
  final double size;
  final Color? color;
  const StepsIcon({super.key, this.size = 16, this.color});

  @override
  Widget build(BuildContext context) {
    final c = color ?? IconTheme.of(context).color ?? Theme.of(context).colorScheme.onSurface;
    return ExcludeSemantics(
      child: SizedBox.square(
        dimension: size,
        child: CustomPaint(painter: _StepsPainter(c)),
      ),
    );
  }
}

class _StepsPainter extends CustomPainter {
  final Color color;
  const _StepsPainter(this.color);

  @override
  void paint(Canvas canvas, Size size) {
    final k = size.width / 24;
    const pts = [
      Offset(3, 20), Offset(8, 20), Offset(8, 15), Offset(13, 15),
      Offset(13, 10), Offset(18, 10), Offset(18, 5), Offset(21, 5),
    ];
    final path = Path()..moveTo(pts.first.dx * k, pts.first.dy * k);
    for (final o in pts.skip(1)) {
      path.lineTo(o.dx * k, o.dy * k);
    }
    canvas.drawPath(
      path,
      Paint()
        ..color = color
        ..style = PaintingStyle.stroke
        ..strokeWidth = 2 * k
        ..strokeCap = StrokeCap.round
        ..strokeJoin = StrokeJoin.round,
    );
  }

  @override
  bool shouldRepaint(_StepsPainter old) => old.color != color;
}
