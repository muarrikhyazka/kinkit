import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kinkit_mobile/widgets/mascot.dart';

Widget _host(Widget child, {bool reduced = false}) => MediaQuery(
      data: MediaQueryData(disableAnimations: reduced),
      child: Directionality(textDirection: TextDirection.ltr, child: Center(child: child)),
    );

void main() {
  testWidgets('every pose renders and animates (bob, wave, celebrate hops)', (tester) async {
    for (final pose in MascotPose.values) {
      await tester.pumpWidget(_host(Mascot(key: ValueKey(pose), pose: pose, size: 96, wave: true)));
      expect(find.byType(Image), findsOneWidget);
      // Motion keeps scheduling frames.
      expect(tester.binding.hasScheduledFrame, isTrue);
      for (var i = 0; i < 10; i++) {
        await tester.pump(const Duration(milliseconds: 300));
      }
      expect(tester.takeException(), isNull);
    }
  });

  testWidgets('reduced motion keeps Kiki still', (tester) async {
    await tester.pumpWidget(_host(const Mascot(pose: MascotPose.celebrate, wave: true), reduced: true));
    await tester.pumpAndSettle();
    expect(tester.binding.hasScheduledFrame, isFalse);
    final transform = tester.widget<Transform>(find.descendant(of: find.byType(Mascot), matching: find.byType(Transform)));
    expect(transform.transform, Matrix4.identity());
  });

  testWidgets('assets resolve to the four pose PNGs', (tester) async {
    expect(MascotPose.values.map((p) => p.asset), [
      'assets/mascot/mascot-512.png',
      'assets/mascot/mascot-celebrate-512.png',
      'assets/mascot/mascot-relax-512.png',
      'assets/mascot/mascot-think-512.png',
    ]);
  });
}
