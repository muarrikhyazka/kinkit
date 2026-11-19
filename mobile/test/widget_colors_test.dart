import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kinkit_mobile/api/models.dart';
import 'package:kinkit_mobile/widgets/widget_colors.dart';

void main() {
  test('hex parsing accepts #rrggbb / #rgb / no hash and rejects junk', () {
    expect(normalizeHexColor('#4F46E5'), '#4f46e5');
    expect(normalizeHexColor('4f46e5'), '#4f46e5');
    expect(normalizeHexColor('#abc'), '#aabbcc');
    expect(normalizeHexColor(''), isNull);
    expect(normalizeHexColor('red'), isNull);
    expect(normalizeHexColor('#12345'), isNull);
    expect(normalizeHexColor(null), isNull);
    expect(parseHexColor('#4f46e5'), const Color(0xFF4F46E5));
    expect(parseHexColor('zz'), isNull);
    expect(colorToHex(const Color(0xFF4F46E5)), '#4f46e5');
  });

  test('readableOn picks white on dark fills and dark text on light fills', () {
    expect(readableOn(const Color(0xFF1E293B)), const Color(0xFFFFFFFF)); // slate
    expect(readableOn(const Color(0xFF4F46E5)), const Color(0xFFFFFFFF)); // indigo
    expect(readableOn(const Color(0xFF000000)), const Color(0xFFFFFFFF));
    expect(readableOn(const Color(0xFFFFFFFF)), const Color(0xFF1B1F2B));
    expect(readableOn(const Color(0xFFFDE68A)), const Color(0xFF1B1F2B)); // sun
    for (final sw in kWidgetPalette.where((s) => s.hex.isNotEmpty)) {
      final bg = parseHexColor(sw.hex)!;
      final tone = WidgetTone.on(bg);
      expect(tone.text, readableOn(bg), reason: sw.name);
    }
  });

  test('widget colour and board background round-trip, unknown keys are kept', () {
    final b = WidgetBoard.fromJson({
      'id': 1,
      'name': 'TV',
      'layout': {
        'background': '#1e293b',
        'future': {'a': 1},
        'widgets': [
          {'id': 'w1', 'type': 'clock', 'w': 4, 'h': 1, 'color': '#fde68a', 'z': 9},
          {'id': 'w2', 'type': 'note', 'w': 4, 'h': 1, 'text': 'hi'},
        ],
      },
    });
    expect(b.background, '#1e293b');
    expect(b.widgets[0].color, '#fde68a');
    expect(b.widgets[1].color, isNull);
    final json = WidgetBoard.layoutJson(b.widgets, background: b.background, extra: b.layoutExtra);
    expect(json['background'], '#1e293b');
    expect(json['future'], {'a': 1});
    final ws = json['widgets'] as List;
    expect((ws[0] as Map)['color'], '#fde68a');
    expect((ws[0] as Map)['z'], 9);
    expect((ws[1] as Map).containsKey('color'), isFalse);

    final cleared = b.widgets[0].copyWith(clearColor: true);
    expect(cleared.toJson().containsKey('color'), isFalse);
    expect(WidgetBoard.layoutJson(b.widgets, background: '').containsKey('background'), isFalse);
  });
}
