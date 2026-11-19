import 'dart:math' as math;

import 'package:flutter/material.dart';

/// Widget colours (web: react/src/widgetColors.js). A widget `color` or board
/// `background` is `#rrggbb`; empty/invalid = the default TV look.

const Color _darkText = Color(0xFF1B1F2B);
const Color _lightText = Color(0xFFFFFFFF);

/// Parses `#rrggbb` (or `#rgb`, no `#` accepted) into an opaque colour; null for empty/invalid.
Color? parseHexColor(String? v) {
  if (v == null) return null;
  var s = v.trim().toLowerCase();
  if (s.startsWith('#')) s = s.substring(1);
  if (RegExp(r'^[0-9a-f]{3}$').hasMatch(s)) s = s.split('').map((c) => '$c$c').join();
  if (!RegExp(r'^[0-9a-f]{6}$').hasMatch(s)) return null;
  return Color(0xFF000000 | int.parse(s, radix: 16));
}

double _lum(Color c) {
  double lin(double v) => v <= 0.03928 ? v / 12.92 : math.pow((v + 0.055) / 1.055, 2.4).toDouble();
  return 0.2126 * lin(c.r) + 0.7152 * lin(c.g) + 0.0722 * lin(c.b);
}

/// Text colour (white or near-black) with the better contrast on [bg].
Color readableOn(Color bg) {
  final l = _lum(bg);
  final dark = (l + 0.05) / (_lum(_darkText) + 0.05);
  final light = 1.05 / (l + 0.05);
  return dark >= light ? _darkText : _lightText;
}

/// Colours for content on a widget card filled with [bg].
class WidgetTone {
  const WidgetTone({required this.bg, required this.text, required this.muted, required this.track, required this.accent, required this.danger, required this.border});

  factory WidgetTone.on(Color bg) {
    final fg = readableOn(bg);
    return WidgetTone(
      bg: bg,
      text: fg,
      muted: Color.lerp(fg, bg, 0.32)!,
      track: Color.lerp(bg, fg, 0.16)!,
      accent: fg,
      danger: fg == _darkText ? const Color(0xFFB91C1C) : const Color(0xFFFECACA),
      border: Color.lerp(bg, fg, 0.14)!,
    );
  }

  final Color bg, text, muted, track, accent, danger, border;
}
