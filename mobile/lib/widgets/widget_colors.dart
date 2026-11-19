import 'dart:math' as math;

import 'package:flutter/material.dart';

/// Colours for widget boards (web: react/src/widgetColors.js). A widget colour or
/// board background is `#rrggbb` (lower-case) or empty/null = the default theme look.

/// Preset swatches offered by the colour picker; the first (empty hex) is "Default".
const List<({String name, String hex})> kWidgetPalette = [
  (name: 'Default', hex: ''),
  (name: 'Lavender', hex: '#e0e7ff'),
  (name: 'Sky', hex: '#bae6fd'),
  (name: 'Mint', hex: '#bbf7d0'),
  (name: 'Sun', hex: '#fde68a'),
  (name: 'Peach', hex: '#fed7aa'),
  (name: 'Rose', hex: '#fecdd3'),
  (name: 'Indigo', hex: '#4f46e5'),
  (name: 'Teal', hex: '#0f766e'),
  (name: 'Slate', hex: '#1e293b'),
];

const Color _darkText = Color(0xFF1B1F2B);
const Color _lightText = Color(0xFFFFFFFF);

/// Normalises `#rgb` / `#rrggbb` (with or without `#`, any case) to `#rrggbb`
/// lower-case, or null when it is not a valid colour.
String? normalizeHexColor(String? v) {
  if (v == null) return null;
  var s = v.trim().toLowerCase();
  if (s.startsWith('#')) s = s.substring(1);
  if (RegExp(r'^[0-9a-f]{3}$').hasMatch(s)) s = s.split('').map((c) => '$c$c').join();
  if (!RegExp(r'^[0-9a-f]{6}$').hasMatch(s)) return null;
  return '#$s';
}

/// Parses `#rrggbb` into an opaque [Color]; null for empty or invalid input.
Color? parseHexColor(String? v) {
  final h = normalizeHexColor(v);
  if (h == null) return null;
  return Color(0xFF000000 | int.parse(h.substring(1), radix: 16));
}

/// Opaque colour as `#rrggbb` lower-case.
String colorToHex(Color c) {
  int ch(double v) => (v * 255).round().clamp(0, 255);
  String two(int n) => n.toRadixString(16).padLeft(2, '0');
  return '#${two(ch(c.r))}${two(ch(c.g))}${two(ch(c.b))}';
}

double _relLuminance(Color c) {
  double lin(double v) => v <= 0.03928 ? v / 12.92 : math.pow((v + 0.055) / 1.055, 2.4).toDouble();
  return 0.2126 * lin(c.r) + 0.7152 * lin(c.g) + 0.0722 * lin(c.b);
}

/// Text colour (white or near-black) with the better contrast on [bg].
Color readableOn(Color bg) {
  final l = _relLuminance(bg);
  final darkContrast = (l + 0.05) / (_relLuminance(_darkText) + 0.05);
  final lightContrast = 1.05 / (l + 0.05);
  return darkContrast >= lightContrast ? _darkText : _lightText;
}

/// Colours for content drawn on a widget card filled with [bg].
class WidgetTone {
  final Color bg, text, muted, track, accent, danger, border;
  const WidgetTone({required this.bg, required this.text, required this.muted, required this.track, required this.accent, required this.danger, required this.border});

  /// Tone for a custom fill (null/invalid hex is the caller's job to handle).
  factory WidgetTone.on(Color bg) {
    final fg = readableOn(bg);
    final light = fg == _darkText;
    return WidgetTone(
      bg: bg,
      text: fg,
      muted: Color.lerp(fg, bg, 0.32)!,
      track: Color.lerp(bg, fg, 0.16)!,
      accent: fg,
      danger: light ? const Color(0xFFB91C1C) : const Color(0xFFFECACA),
      border: Color.lerp(bg, fg, 0.14)!,
    );
  }
}
