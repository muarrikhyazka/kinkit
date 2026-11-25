import 'dart:convert';
import 'dart:typed_data';

import 'package:cookie_jar/cookie_jar.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kinkit_mobile/api/api_client.dart';
import 'package:kinkit_mobile/api/models.dart';
import 'package:kinkit_mobile/capture/capture_channel.dart';
import 'package:kinkit_mobile/capture/capture_providers.dart';
import 'package:kinkit_mobile/screens/captured_expenses_screen.dart';
import 'package:kinkit_mobile/state/providers.dart';
import 'package:kinkit_mobile/theme.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _FakeChannel extends CaptureChannel {
  _FakeChannel(this.items);

  final List<CapturedNotification> items;
  final List<String> removed = [];

  /// Set to make the matching call throw like a failed native call.
  bool failStatus = false;
  bool failPending = false;
  bool failRemove = false;
  bool failClear = false;

  @override
  Future<CaptureStatus> status() async {
    if (failStatus) throw const CaptureException('Android could not check whether capture is on (boom). Close and reopen Kinkit, then try again.');
    return const CaptureStatus(supported: true, permission: true, enabled: true);
  }

  @override
  Future<List<CapturedNotification>> pending() async {
    if (failPending) throw const CaptureException('Android could not read the captured notifications (boom). Close and reopen Kinkit, then try again.');
    return List.of(items);
  }

  @override
  Future<void> remove(List<String> ids) async {
    if (failRemove) throw const CaptureException('Android could not delete the captured notification (boom).');
    removed.addAll(ids);
    items.removeWhere((c) => ids.contains(c.id));
  }

  @override
  Future<void> clear() async {
    if (failClear) throw const CaptureException('Android could not delete the captured notifications (boom).');
    items.clear();
  }
}

/// Answers /api/setup and records every POST /api/spending body.
class _RecordingAdapter implements HttpClientAdapter {
  final List<Map<String, dynamic>> spendingPosts = [];

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    Object body = const <String, dynamic>{};
    if (options.uri.path == '/api/setup') {
      body = {
        'accounts': [],
        'income_sources': [],
        'categories': [
          {'id': 5, 'name': 'Food', 'main_category': 'Needs'},
        ],
      };
    } else if (options.uri.path == '/api/spending' && options.method == 'POST') {
      spendingPosts.add(Map<String, dynamic>.from(options.data as Map));
      body = {'id': 7};
    }
    return ResponseBody.fromString(jsonEncode(body), 200, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });
  }

  @override
  void close({bool force = false}) {}
}

CapturedNotification _note(String id, String title, String text) => CapturedNotification(
      id: id,
      package: 'com.bank.demo',
      app: 'Bank Demo',
      title: title,
      text: text,
      postedAt: DateTime.now(),
    );

void main() {
  late _RecordingAdapter adapter;
  late _FakeChannel channel;

  Future<void> pumpScreen(WidgetTester tester, {String role = 'owner'}) async {
    tester.view.physicalSize = const Size(800, 1600);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    tester.platformDispatcher.accessibilityFeaturesTestValue = const FakeAccessibilityFeatures(disableAnimations: true);
    addTearDown(tester.platformDispatcher.clearAccessibilityFeaturesTestValue);
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    adapter = _RecordingAdapter();
    final api = KinkitApi(baseUrl: 'http://fake', cookieJar: CookieJar(), dio: Dio()..httpClientAdapter = adapter);
    await tester.pumpWidget(ProviderScope(
      overrides: [
        sharedPrefsProvider.overrideWithValue(prefs),
        apiProvider.overrideWithValue(api),
        meProvider.overrideWithValue(Me(authenticated: true, memberId: 1, role: role, displayName: 'Budi')),
        captureChannelProvider.overrideWithValue(channel),
      ],
      child: MaterialApp(theme: buildTheme(Brightness.light), home: const CapturedExpensesScreen()),
    ));
    await tester.pumpAndSettle();
  }

  testWidgets('lists payments, discards one-time codes, saves only after review', (tester) async {
    channel = _FakeChannel([
      _note('n1', 'Bank Demo', 'Payment of Rp 54.000 to NETFLIX successful'),
      _note('n2', 'Bank Demo', 'Your OTP is 482913. Rp 10.000 purchase'),
      _note('n3', 'Bank Demo', 'You received Rp 500.000 from Budi'),
    ]);
    await pumpScreen(tester);

    expect(find.text('NETFLIX'), findsOneWidget);
    expect(find.text('Rp54.000'), findsOneWidget);
    expect(find.text('Money in'), findsOneWidget);
    // The OTP message is dropped from the phone instead of listed.
    expect(channel.removed, ['n2']);
    expect(adapter.spendingPosts, isEmpty);

    await tester.tap(find.text('NETFLIX'));
    await tester.pumpAndSettle();
    expect(find.text('Review expense'), findsOneWidget);
    // Nothing is sent just by opening the review.
    expect(adapter.spendingPosts, isEmpty);

    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Food').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Save spending'));
    await tester.pumpAndSettle();

    expect(adapter.spendingPosts, hasLength(1));
    final post = adapter.spendingPosts.single;
    expect(post['source'], 'notification');
    expect(post['amount'], 54000);
    expect(post['description'], 'NETFLIX');
    expect(post['category'], 'Food');
    expect(post['client_id'], startsWith('notif-'));
    expect(channel.removed, containsAll(['n2', 'n1']));
  });

  testWidgets('a payment with an unreadable amount is kept, not deleted', (tester) async {
    channel = _FakeChannel([
      _note('n1', 'Kopi Kenangan', 'Debit Rp 0 at Kopi Kenangan'),
      _note('n2', 'Bank Demo', 'Promo! Get Rp 10.000 off your next order'),
    ]);
    await pumpScreen(tester);

    expect(find.text('Amount unclear'), findsOneWidget);
    // Only the bare promotion is deleted.
    expect(channel.removed, ['n2']);

    await tester.tap(find.text('Kopi Kenangan').first);
    await tester.pumpAndSettle();
    expect(find.textContaining('could not read an amount'), findsOneWidget);
    final amount = tester.widget<TextField>(find.widgetWithText(TextField, 'Amount (Rp)'));
    expect(amount.controller!.text, isEmpty);
  });

  testWidgets('a failed dismiss says so and keeps the item', (tester) async {
    channel = _FakeChannel([_note('n3', 'Bank Demo', 'You received Rp 500.000 from Budi')])..failRemove = true;
    await pumpScreen(tester);

    await tester.tap(find.text('Bank Demo'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Dismiss'));
    await tester.pumpAndSettle();

    expect(find.textContaining('Not dismissed'), findsOneWidget);
    expect(find.text('Dismissed. Nothing was saved.'), findsNothing);
    expect(channel.items, hasLength(1));
  });

  testWidgets('a failed Dismiss all does not claim success', (tester) async {
    channel = _FakeChannel([_note('n1', 'Bank Demo', 'Payment of Rp 54.000 to NETFLIX successful')])..failClear = true;
    await pumpScreen(tester);

    await tester.tap(find.byTooltip('Dismiss all'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Dismiss all').last);
    await tester.pumpAndSettle();

    expect(find.textContaining('Nothing was deleted'), findsOneWidget);
    expect(find.text('Dismissed. Nothing was saved.'), findsNothing);
    expect(find.text('NETFLIX'), findsOneWidget);
  });

  testWidgets('a native failure reading the list shows an error with Retry', (tester) async {
    channel = _FakeChannel([])..failPending = true;
    await pumpScreen(tester);

    expect(find.textContaining('could not read the captured notifications'), findsOneWidget);
    expect(find.text('Retry'), findsOneWidget);
    expect(find.text('Nothing to review'), findsNothing);

    channel.failPending = false;
    await tester.tap(find.text('Retry'));
    await tester.pumpAndSettle();
    expect(find.text('Nothing to review'), findsOneWidget);
  });

  testWidgets('a native failure reading the status is not shown as "not available"', (tester) async {
    channel = _FakeChannel([])..failStatus = true;
    await pumpScreen(tester);

    expect(find.text('Capture is not available here'), findsNothing);
    expect(find.textContaining('could not check whether capture is on'), findsOneWidget);
    expect(find.text('Retry'), findsOneWidget);
  });

  testWidgets('money-in items can only be dismissed', (tester) async {
    channel = _FakeChannel([_note('n3', 'Bank Demo', 'You received Rp 500.000 from Budi')]);
    await pumpScreen(tester);

    await tester.tap(find.text('Bank Demo'));
    await tester.pumpAndSettle();
    expect(find.text('Save spending'), findsNothing);
    await tester.tap(find.text('Dismiss'));
    await tester.pumpAndSettle();

    expect(adapter.spendingPosts, isEmpty);
    expect(channel.removed, ['n3']);
    expect(find.text('Nothing to review'), findsOneWidget);
  });

  testWidgets('empty list says why and how it fills', (tester) async {
    channel = _FakeChannel([]);
    await pumpScreen(tester);
    expect(find.text('Nothing to review'), findsOneWidget);
    expect(find.textContaining('One-time codes and promotions are discarded'), findsOneWidget);
  });

  testWidgets('viewers get a read-only message', (tester) async {
    channel = _FakeChannel([_note('n1', 'Bank Demo', 'Payment of Rp 54.000 to NETFLIX successful')]);
    await pumpScreen(tester, role: 'viewer');
    expect(find.text('Viewers cannot save expenses'), findsOneWidget);
    expect(find.text('NETFLIX'), findsNothing);
  });
}
