import 'package:flutter/material.dart';

/// The Kinkit logo (same painter as mobile/lib/widgets/kinkit_logo.dart), drawn from the same geometry as brand/kinkit-logo.svg
/// (64x64 viewBox): a white rounded tile with a hairline #E0E7FF border and a
/// purple (#4F46E5) house outline and dot. A soft shadow keeps the tile visible
/// on white. Set [tile] to false for the bare mark (brand/kinkit-mark.svg) in
/// [markColor].
class KinkitLogo extends StatelessWidget {
  final double size;
  final bool tile;
  final Color markColor;

  const KinkitLogo({super.key, this.size = 56, this.tile = true, this.markColor = KinkitLogoPainter.purple});

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

  const KinkitLogoPainter({this.tile = true, this.markColor = purple});

  static const purple = Color(0xFF4F46E5);
  static const tileColor = Color(0xFFFFFFFF);
  static const hairline = Color(0xFFE0E7FF);

  @override
  void paint(Canvas canvas, Size size) {
    final s = size.shortestSide / 64;
    canvas.save();
    canvas.translate((size.width - 64 * s) / 2, (size.height - 64 * s) / 2);
    canvas.scale(s);

    if (tile) {
      final tileRRect = RRect.fromRectAndRadius(const Rect.fromLTWH(0, 0, 64, 64), const Radius.circular(16));
      // Soft drop shadow so the white tile still reads on white backgrounds.
      canvas.drawRRect(
        tileRRect.shift(const Offset(0, 1.5)),
        Paint()
          ..color = const Color(0x1F1E1B4B)
          ..maskFilter = const MaskFilter.blur(BlurStyle.normal, 2.5),
      );
      canvas.drawRRect(tileRRect, Paint()..color = tileColor);
      // Hairline border, inset so it stays inside the tile (as in the SVG).
      canvas.drawRRect(
        tileRRect.deflate(0.75),
        Paint()
          ..color = hairline
          ..style = PaintingStyle.stroke
          ..strokeWidth = 1.5,
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
