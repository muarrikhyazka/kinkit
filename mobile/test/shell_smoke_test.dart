import 'dart:convert';
import 'dart:typed_data';

import 'package:cookie_jar/cookie_jar.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kinkit_mobile/api/api_client.dart';
import 'package:kinkit_mobile/app.dart';
import 'package:kinkit_mobile/state/providers.dart';
import 'package:shared_preferences/shared_preferences.dart';

final String _today = DateTime.now().toIso8601String().substring(0, 10);

/// Serves canned JSON for each endpoint so the whole signed-in UI renders.
class _FakeAdapter implements HttpClientAdapter {
  static final Map<String, Object> routes = {
    '/api/me': {
      'authenticated': true, 'account_id': 1, 'member_id': 1, 'role': 'owner', 'display_name': 'Budi',
      'email': 'budi@example.com', 'currency': 'IDR', 'locale': 'id-ID', 'household_name': 'Santoso',
    },
    '/api/fx-rates': {'base': 'IDR', 'rates': {'IDR': 1}},
    '/api/gamification': {'points': 120, 'streak_days': 3, 'badge': 'Tracker', 'badges': [], 'leaderboard': [], 'challenges': []},
    '/api/today': {
      'date': '2026-11-18', 'greeting': 'Good morning, Budi',
      'sections': [
        {'key': 'reminders', 'title': 'Reminders', 'items': [
          {'kind': 'reminder', 'id': 4, 'title': 'Pay electricity', 'done': false, 'actions': ['done', 'snooze']},
        ]},
        {'key': 'budget', 'title': 'Budget', 'items': [], 'summary': 'Rp2.100.000 left'},
      ],
    },
    '/api/spending': {
      'rows': [
        {'id': 1, 'tx_date': '2026-11-18', 'description': 'Lunch', 'category_name': 'Food', 'amount': 25000, 'member_id': 1, 'member_name': 'Budi'},
      ],
      'total': 1, 'page': 1, 'per_page': 200,
    },
    '/api/setup': {
      'accounts': [], 'income_sources': [{'id': 1, 'name': 'Salary'}],
      'categories': [{'id': 5, 'name': 'Food', 'main_category': 'Needs'}],
    },
    '/api/income': {'rows': [], 'total': 0, 'planned': 0, 'by_source': []},
    '/api/goals': [
      {'id': 1, 'name': 'Holiday', 'target_amount': 1000000, 'progress': 250000, 'pct': 25.0, 'emoji': ''},
    ],
    '/api/family': {
      'household': {'name': 'Santoso', 'currency': 'IDR'}, 'can_manage': true,
      'members': [{'id': 1, 'email': 'budi@example.com', 'display_name': 'Budi', 'role': 'owner', 'is_me': true}],
    },
    '/api/chores/today': {
      'date': '2026-11-18', 'me': {'can_manage': true},
      'items': [{'id': 9, 'title': 'Dishes', 'points': 5, 'status': 'pending', 'member_name': 'Ayu'}],
      'waiting': [], 'week': {'points': 5, 'done': 1, 'total': 3},
    },
    '/api/reminders/upcoming': {
      'items': [{'id': 4, 'title': 'Pay electricity', 'category': 'bill', 'due_at': '2026-11-18T02:00:00Z', 'status': 'pending', 'can_act': true}],
    },
    '/api/reminders': {'reminders': [{'id': 2, 'title': 'Pay electricity', 'kind': 'monthly', 'category': 'bill', 'active': true}]},
    '/api/calendar': {
      'timezone': 'UTC', 'today': '2026-11-18', 'reminders': [],
      'activities': [
        {'id': 3, 'title': 'Swim class', 'color': 'sky', 'start_date': _today, 'end_date': _today, 'start_time': '16:00', 'end_time': '17:00', 'occurs_on': _today, 'occurs_until': _today},
      ],
    },
    '/api/meal-plan/proposals': {
      'items': [{'id': 1, 'dish': 'Soto ayam', 'member_name': 'Ayu', 'status': 'pending', 'can_decide': true}],
    },
    '/api/widgets/boards': {'boards': []},
    '/api/widgets/catalog': {'widgets': [], 'presets': [{'id': 'phone', 'name': 'Phone', 'widgets': []}]},
    '/api/lists': {'lists': [{'id': 1, 'name': 'Groceries', 'open_count': 2, 'checked_count': 0}]},
  };

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    final path = options.uri.path;
    Object body = const <String, dynamic>{};
    if (routes.containsKey(path)) {
      body = routes[path]!;
    } else if (path.startsWith('/api/report/month-compare/')) {
      body = {'current': 25000, 'previous_month': 20000, 'same_month_last_year': 0};
    } else if (path.startsWith('/api/report/')) {
      body = {
        'total_income': 10000000, 'budgeted_expenses': 2000000, 'expense_actual': 25000,
        'expenses_list': [{'category_name': 'Food', 'main_category': 'Needs', 'allocation': 2000000, 'realization': 25000}],
      };
    } else if (path.endsWith('/actuals')) {
      body = {'expense': [], 'income': []};
    } else if (path.startsWith('/api/budgeting/')) {
      body = {'income': [], 'expense': []};
    }
    return ResponseBody.fromString(jsonEncode(body), 200, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  testWidgets('signed-in shell renders every tab', (tester) async {
    // Reduced motion keeps Kiki still so pumpAndSettle can settle.
    tester.platformDispatcher.accessibilityFeaturesTestValue = const FakeAccessibilityFeatures(disableAnimations: true);
    addTearDown(tester.platformDispatcher.clearAccessibilityFeaturesTestValue);
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final dio = Dio()..httpClientAdapter = _FakeAdapter();
    final api = KinkitApi(baseUrl: 'http://fake', cookieJar: CookieJar(), dio: dio);

    await tester.pumpWidget(ProviderScope(
      overrides: [
        sharedPrefsProvider.overrideWithValue(prefs),
        apiProvider.overrideWithValue(api),
      ],
      child: const KinkitApp(),
    ));
    Future<void> settle() async {
      await tester.runAsync(() => Future<void>.delayed(const Duration(milliseconds: 100)));
      await tester.pumpAndSettle();
    }

    await settle();
    expect(find.text('Good morning, Budi'), findsOneWidget);
    expect(find.text('Pay electricity'), findsOneWidget);

    // Home: reminders, chores, lists
    await tester.tap(find.text('Home').last);
    await settle();
    expect(find.text('Pay electricity'), findsWidgets);
    await tester.tap(find.text('All'));
    await settle();
    await tester.tap(find.widgetWithText(Tab, 'Calendar'));
    await settle();
    await tester.tap(find.text('Agenda'));
    await settle();
    expect(find.text('Swim class'), findsWidgets);
    await tester.tap(find.text('Chores'));
    await settle();
    expect(find.text('Dishes'), findsOneWidget);
    await tester.tap(find.text('Lists'));
    await settle();
    expect(find.text('Groceries'), findsOneWidget);
    await tester.tap(find.text('Meals'));
    await settle();
    expect(find.text('Soto ayam'), findsOneWidget);

    // Money: every sub-tab
    await tester.tap(find.text('Money').last);
    await settle();
    expect(find.text('Top expenses', skipOffstage: false), findsOneWidget);
    for (final tab in ['Spending', 'Income', 'Budget', 'Goals']) {
      await tester.tap(find.widgetWithText(Tab, tab));
      await settle();
    }
    expect(find.text('Holiday'), findsOneWidget);

    // Family and More
    await tester.tap(find.text('Family').last);
    await settle();
    expect(find.text('Santoso'), findsWidgets);
    await tester.tap(find.text('More').last);
    await settle();
    expect(find.text('Log out'), findsOneWidget);
    await tester.tap(find.text('Widgets'));
    await settle();
    expect(find.text('Start from a template'), findsOneWidget);
    await tester.pageBack();
    await settle();

    // Quick add opens from Today
    await tester.tap(find.text('Today').last);
    await settle();
    await tester.tap(find.byTooltip('Quick add'));
    await settle();
    expect(find.text('Add spending'), findsOneWidget);
  });
}
