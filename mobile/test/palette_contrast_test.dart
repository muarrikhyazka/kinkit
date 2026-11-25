import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kinkit_mobile/theme.dart';

double _lin(double c) => c <= 0.03928 ? c / 12.92 : math.pow((c + 0.055) / 1.055, 2.4).toDouble();
double _lum(Color c) => 0.2126 * _lin(c.r) + 0.7152 * _lin(c.g) + 0.0722 * _lin(c.b);
double ratio(Color a, Color b) {
  final la = _lum(a), lb = _lum(b);
  return (math.max(la, lb) + 0.05) / (math.min(la, lb) + 0.05);
}

void main() {
  for (final entry in {'light': KinkitPalette.light, 'dark': KinkitPalette.dark}.entries) {
    final p = entry.value;
    final name = entry.key;

    test('$name: brand and status colours read as text on card and bg (4.5:1)', () {
      for (final bg in [p.card, p.bg]) {
        for (final fg in {'primary': p.primary, 'success': p.success, 'danger': p.danger, 'warning': p.warning}.entries) {
          expect(ratio(fg.value, bg), greaterThanOrEqualTo(4.5), reason: '$name ${fg.key} on $bg');
        }
      }
      expect(ratio(p.muted, p.card), greaterThanOrEqualTo(4.5), reason: '$name muted on card');
      expect(ratio(p.danger, p.dangerLight), greaterThanOrEqualTo(4.5), reason: '$name danger on its tint');
      // Primary text also sits on the selected-tab tint and on grey surfaces.
      expect(ratio(p.primary, p.primaryLight), greaterThanOrEqualTo(4.5), reason: '$name primary on tint');
      expect(ratio(p.primary, p.surface2), greaterThanOrEqualTo(4.5), reason: '$name primary on surface2');
    });

    test('$name: filled controls carry white text at 4.5:1', () {
      expect(ratio(p.onPrimary, p.primaryFill), greaterThanOrEqualTo(4.5));
      expect(ratio(p.onPrimary, p.dangerFill), greaterThanOrEqualTo(4.5));
    });

    test('$name: focus border and Material primary stand out from the card (3:1)', () {
      expect(ratio(p.primary, p.card), greaterThanOrEqualTo(3));
    });
  }

  test('theme: filled buttons use the fill colour, the scheme primary stays the text colour', () {
    for (final b in Brightness.values) {
      final t = buildTheme(b);
      final p = t.extension<KinkitPalette>()!;
      expect(t.filledButtonTheme.style!.backgroundColor!.resolve({}), p.primaryFill);
      expect(t.floatingActionButtonTheme.backgroundColor, p.primaryFill);
      expect(t.colorScheme.primary, p.primary);
      // Material paints onPrimary on the scheme primary (switch thumb, checkmark).
      expect(ratio(t.colorScheme.onPrimary, t.colorScheme.primary), greaterThanOrEqualTo(4.5));
    }
  });
}
