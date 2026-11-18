import 'dart:math' as math;

import 'package:flutter/material.dart';

/// Kiki's poses (brand/README.md "Mascot: Kiki").
enum MascotPose {
  /// Waving: greetings (login, Today header).
  idle('mascot'),

  /// Arms up with a coin: level-ups, reached goals, approvals.
  celebrate('mascot-celebrate'),

  /// Eyes closed: "all done" / empty-today states.
  relax('mascot-relax'),

  /// Hand to chin: no data yet, errors.
  think('mascot-think');

  final String file;
  const MascotPose(this.file);

  String get asset => 'assets/mascot/$file-512.png';
}

/// Kiki, the Kinkit mascot, with the same gentle motion as the web
/// (react/src/mascot.css):
/// - idle "breathe": a 6/256 lift with squash and stretch, while the contact
///   shadow shrinks and fades as Kiki rises (3.4 s; 4.6 s when relaxing);
/// - [wave]: a small rotation wobble once on mount;
/// - celebrate: three hops, then it settles into the idle breathe.
///
/// The PNGs in assets/mascot are renders of brand/mascot*.svg without the
/// baked-in shadow, so the shadow can be drawn (and scaled) separately here.
/// All motion is off when [animate] is false or the platform asks for reduced
/// motion (MediaQuery.disableAnimations).
class Mascot extends StatefulWidget {
  final MascotPose pose;
  final double size;
  final bool animate;
  final bool wave;

  /// Accessible label; null (default) treats Kiki as decoration.
  final String? semanticLabel;

  const Mascot({
    super.key,
    this.pose = MascotPose.idle,
    this.size = 96,
    this.animate = true,
    this.wave = false,
    this.semanticLabel,
  });

  @override
  State<Mascot> createState() => _MascotState();
}

class _MascotState extends State<Mascot> with TickerProviderStateMixin {
  late final AnimationController _bob = AnimationController(vsync: this, duration: _bobPeriod);
  late final AnimationController _hop = AnimationController(vsync: this, duration: const Duration(milliseconds: 2250));
  late final AnimationController _wave = AnimationController(vsync: this, duration: const Duration(milliseconds: 1750));
  bool _reduced = false;
  bool _started = false;

  Duration get _bobPeriod => Duration(milliseconds: widget.pose == MascotPose.relax ? 4600 : 3400);

  bool get _motion => widget.animate && !_reduced;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _reduced = MediaQuery.maybeDisableAnimationsOf(context) ?? false;
    if (_motion && !_started) {
      _started = true;
      _start(firstRun: true);
    } else if (!_motion) {
      _stop();
    }
  }

  @override
  void didUpdateWidget(Mascot old) {
    super.didUpdateWidget(old);
    if (old.pose != widget.pose || old.animate != widget.animate) {
      _bob.duration = _bobPeriod;
      _stop();
      if (_motion) _start(firstRun: false);
    }
  }

  void _start({required bool firstRun}) {
    _started = true;
    if (widget.pose == MascotPose.celebrate) {
      _hop.forward(from: 0).whenComplete(() {
        if (mounted && _motion) _bob.repeat();
      });
    } else {
      _bob.repeat();
    }
    if (firstRun && widget.wave) _wave.forward(from: 0);
  }

  void _stop() {
    _started = false;
    _bob
      ..stop()
      ..value = 0;
    _hop
      ..stop()
      ..value = 0;
    _wave
      ..stop()
      ..value = 0;
  }

  @override
  void dispose() {
    _bob.dispose();
    _hop.dispose();
    _wave.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final s = widget.size / 256;
    final image = Image.asset(
      widget.pose.asset,
      width: widget.size,
      height: widget.size,
      filterQuality: FilterQuality.medium,
      gaplessPlayback: true,
      excludeFromSemantics: true,
    );
    final child = AnimatedBuilder(
      animation: Listenable.merge([_bob, _hop, _wave]),
      builder: (context, _) {
        final f = _frame();
        return Stack(
          clipBehavior: Clip.none,
          children: [
            Positioned.fill(
              child: CustomPaint(painter: _ShadowPainter(scale: f.shadowScale, opacity: f.shadowOpacity)),
            ),
            Transform(
              // Squash/stretch around the feet (128, 222 in the 256 viewBox).
              alignment: const Alignment(0, 222 / 128 - 1),
              transform: Matrix4.translationValues(0, f.dy * s, 0)
                ..multiply(Matrix4.rotationZ(f.angle))
                ..multiply(Matrix4.diagonal3Values(f.sx, f.sy, 1)),
              child: image,
            ),
          ],
        );
      },
    );
    return Semantics(
      label: widget.semanticLabel,
      image: widget.semanticLabel != null,
      excludeSemantics: true,
      child: SizedBox.square(dimension: widget.size, child: child),
    );
  }

  _Frame _frame() {
    var dy = 0.0, sx = 1.0, sy = 1.0, shadowScale = 1.0, shadowOpacity = 1.0, angle = 0.0;
    if (!_motion) return const _Frame();

    if (_hop.isAnimating) {
      // mk-hop: 3 hops of 0.75 s.
      final t = (_hop.value * 3) % 1.0;
      final k = _keyframes(t, const [
        (0.0, -0.0, 1.05, 0.94, 1.04, 1.0),
        (0.15, -2.0, 0.96, 1.05, 0.95, 0.9),
        (0.5, -20.0, 0.98, 1.03, 0.72, 0.5),
        (0.85, -1.0, 1.0, 1.0, 0.98, 0.95),
        (1.0, 0.0, 1.05, 0.94, 1.04, 1.0),
      ]);
      dy = k.$1;
      sx = k.$2;
      sy = k.$3;
      shadowScale = k.$4;
      shadowOpacity = k.$5;
    } else if (_bob.isAnimating || _bob.value > 0) {
      // mk-bob: ease-in-out lift, peak at 50%.
      final phase = (1 - math.cos(2 * math.pi * _bob.value)) / 2;
      dy = -6 * phase;
      sx = _lerp(1.02, 0.99, phase);
      sy = _lerp(0.98, 1.015, phase);
      shadowScale = _lerp(1, 0.86, phase);
      shadowOpacity = _lerp(1, 0.7, phase);
    }

    if (_wave.isAnimating) {
      // A small wobble (the PNG can't move one arm): starts after 0.35 s.
      final t = ((_wave.value * 1750 - 350) / 1400).clamp(0.0, 1.0);
      const deg = math.pi / 180;
      final w = _keyframes(t, const [
        (0.0, 0.0, 0.0, 0.0, 0.0, 0.0),
        (0.2, 6.0, 0.0, 0.0, 0.0, 0.0),
        (0.4, -4.0, 0.0, 0.0, 0.0, 0.0),
        (0.6, 6.0, 0.0, 0.0, 0.0, 0.0),
        (0.8, -4.0, 0.0, 0.0, 0.0, 0.0),
        (1.0, 0.0, 0.0, 0.0, 0.0, 0.0),
      ]);
      angle = w.$1 * deg;
    }
    return _Frame(dy: dy, sx: sx, sy: sy, angle: angle, shadowScale: shadowScale, shadowOpacity: shadowOpacity);
  }

  static double _lerp(double a, double b, double t) => a + (b - a) * t;

  /// Piecewise ease-in-out interpolation over (offset, v1..v5) keyframes.
  static (double, double, double, double, double) _keyframes(
      double t, List<(double, double, double, double, double, double)> frames) {
    for (var i = 0; i < frames.length - 1; i++) {
      final a = frames[i], b = frames[i + 1];
      if (t <= b.$1) {
        final span = b.$1 - a.$1;
        final u = Curves.easeInOut.transform(span <= 0 ? 1 : ((t - a.$1) / span).clamp(0.0, 1.0));
        return (
          _lerp(a.$2, b.$2, u),
          _lerp(a.$3, b.$3, u),
          _lerp(a.$4, b.$4, u),
          _lerp(a.$5, b.$5, u),
          _lerp(a.$6, b.$6, u),
        );
      }
    }
    final l = frames.last;
    return (l.$2, l.$3, l.$4, l.$5, l.$6);
  }
}

class _Frame {
  final double dy, sx, sy, angle, shadowScale, shadowOpacity;
  const _Frame({
    this.dy = 0,
    this.sx = 1,
    this.sy = 1,
    this.angle = 0,
    this.shadowScale = 1,
    this.shadowOpacity = 1,
  });
}

/// The contact shadow from the SVG: ellipse cx128 cy226 rx64 ry9, #1E1B4B at
/// 28% opacity, Gaussian blur 5 (all in the 256 viewBox).
class _ShadowPainter extends CustomPainter {
  final double scale;
  final double opacity;

  const _ShadowPainter({required this.scale, required this.opacity});

  @override
  void paint(Canvas canvas, Size size) {
    final s = size.shortestSide / 256;
    final rect = Rect.fromCenter(
      center: Offset(128 * s, 226 * s),
      width: 128 * s * scale,
      height: 18 * s * scale,
    );
    final alpha = (0.28 * opacity * 255).round().clamp(0, 255);
    canvas.drawOval(
      rect,
      Paint()
        ..color = Color.fromARGB(alpha, 0x1E, 0x1B, 0x4B)
        ..maskFilter = MaskFilter.blur(BlurStyle.normal, math.max(0.5, 5 * s)),
    );
  }

  @override
  bool shouldRepaint(_ShadowPainter old) => old.scale != scale || old.opacity != opacity;
}
