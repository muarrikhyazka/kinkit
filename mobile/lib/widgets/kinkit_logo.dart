import 'package:flutter/material.dart';

/// The Kinkit logo, drawn from the same geometry as brand/kinkit-logo.svg
/// (64x64 viewBox): an indigo gradient tile (#6366F1 -> #4F46E5) with a white
/// house outline and dot. Set [tile] to false for the bare mark
/// (brand/kinkit-mark.svg) in [markColor].
class KinkitLogo extends StatelessWidget {
  final double size;
  final bool tile;
  final Color markColor;

  const KinkitLogo({super.key, this.size = 56, this.tile = true, this.markColor = Colors.white});

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: 'Kinkit',
      image: true,
      child: SizedBox.square(
        dimension: size,
        child: CustomPaint(painter: KinkitLogoPainter(tile: tile, markColor: markColor)),
      ),
    );
  }
}

class KinkitLogoPainter extends CustomPainter {
  final bool tile;
  final Color markColor;

  const KinkitLogoPainter({this.tile = true, this.markColor = Colors.white});

  static const gradientTop = Color(0xFF6366F1);
  static const gradientBottom = Color(0xFF4F46E5);

  @override
  void paint(Canvas canvas, Size size) {
    final s = size.shortestSide / 64;
    canvas.save();
    canvas.translate((size.width - 64 * s) / 2, (size.height - 64 * s) / 2);
    canvas.scale(s);

    if (tile) {
      const rect = Rect.fromLTWH(0, 0, 64, 64);
      canvas.drawRRect(
        RRect.fromRectAndRadius(rect, const Radius.circular(16)),
        Paint()
          ..shader = const LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [gradientTop, gradientBottom],
          ).createShader(rect),
      );
    }

    final stroke = Paint()
      ..color = markColor
      ..style = PaintingStyle.stroke
      ..strokeWidth = 5
      ..strokeJoin = StrokeJoin.round;

    // Roof: M14 31 32 16l18 15
    final roof = Path()
      ..moveTo(14, 31)
      ..lineTo(32, 16)
      ..lineTo(50, 31);
    canvas.drawPath(roof, Paint.from(stroke)..strokeCap = StrokeCap.round);

    // Walls: M19 28v18a3 3 0 0 0 3 3h20a3 3 0 0 0 3-3V28
    final walls = Path()
      ..moveTo(19, 28)
      ..lineTo(19, 46)
      ..arcToPoint(const Offset(22, 49), radius: const Radius.circular(3), clockwise: false)
      ..lineTo(42, 49)
      ..arcToPoint(const Offset(45, 46), radius: const Radius.circular(3), clockwise: false)
      ..lineTo(45, 28);
    canvas.drawPath(walls, stroke);

    canvas.drawCircle(const Offset(32, 39), 5.5, Paint()..color = markColor);
    canvas.restore();
  }

  @override
  bool shouldRepaint(KinkitLogoPainter old) => old.tile != tile || old.markColor != markColor;
}
