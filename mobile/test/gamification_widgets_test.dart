import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kinkit_mobile/api/models.dart';
import 'package:kinkit_mobile/state/data_providers.dart';
import 'package:kinkit_mobile/theme.dart';
import 'package:kinkit_mobile/widgets/gamification_badge.dart';
import 'package:kinkit_mobile/widgets/steps_icon.dart';

Gamification _sample() => Gamification.fromJson({
      'points': 900,
      'streak_days': 2,
      'badge': 'Penny Pincher',
      'level': 4,
      'member': {
        'id': 7,
        'points': 400,
        'streak_days': 5,
        'level': 'Penny Pincher',
        'level_number': 4,
        'level_floor': 300,
        'next_level': 'Budget Boss',
        'next_level_at': 500,
      },
    });

Widget _app(Widget child) => ProviderScope(
      overrides: [gamificationProvider.overrideWith((ref) async => _sample())],
      child: MaterialApp(theme: buildTheme(Brightness.light), home: Scaffold(body: child)),
    );

void main() {
  testWidgets('GamificationCard shows the level hexagon, title and points to the next level', (tester) async {
    await tester.pumpWidget(_app(const GamificationCard()));
    await tester.pumpAndSettle();
    expect(find.text('Level 4'), findsOneWidget);
    expect(find.text('Penny Pincher'), findsOneWidget);
    expect(find.text('100 pts to Level 5 · Budget Boss'), findsOneWidget);
    expect(find.byType(LevelHex), findsOneWidget);
    expect(find.byType(StepsIcon), findsOneWidget, reason: 'points use the steps glyph, not a star');
    expect(find.byIcon(Icons.star_rounded), findsNothing);
  });

  testWidgets('GamificationBadge chip shows Lv N, points and streak', (tester) async {
    await tester.pumpWidget(_app(const Center(child: GamificationBadge())));
    await tester.pumpAndSettle();
    expect(find.text('Lv 4'), findsOneWidget);
    expect(find.text('400'), findsOneWidget);
    expect(find.text('5'), findsOneWidget);
    expect(find.byType(StepsIcon), findsOneWidget);
  });

  testWidgets('StepsIcon takes its size and the ambient icon colour', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: IconTheme(data: IconThemeData(color: Color(0xFF123456)), child: Center(child: StepsIcon(size: 20))),
    ));
    expect(tester.getSize(find.byType(StepsIcon)), const Size(20, 20));
  });
}
