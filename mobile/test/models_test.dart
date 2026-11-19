import 'package:flutter_test/flutter_test.dart';
import 'package:kinkit_mobile/api/models.dart';

void main() {
  test('Me parses /api/me and derives role permissions', () {
    final me = Me.fromJson({
      'authenticated': true,
      'account_id': 3,
      'member_id': 7,
      'role': 'kid',
      'display_name': 'Ayu',
      'username': 'ayu@example.com',
      'email': 'ayu@example.com',
      'currency': 'USD',
      'locale': 'en-US',
      'is_admin': false,
      'household_name': "Budi's Family",
    });
    expect(me.authenticated, isTrue);
    expect(me.memberId, 7);
    expect(me.currency, 'USD');
    expect(me.isKid, isTrue);
    expect(me.canReadMoney, isFalse);
    expect(me.canAddSpending, isTrue);
    expect(me.canManage, isFalse);
  });

  test('Me handles the logged-out response', () {
    final me = Me.fromJson({'authenticated': false});
    expect(me.authenticated, isFalse);
    expect(me.currency, 'IDR');
  });

  test('SpendingPage parses rows with nullable fields', () {
    final page = SpendingPage.fromJson({
      'rows': [
        {
          'id': 10,
          'account_id': 1,
          'tx_date': '2026-11-18',
          'description': 'Nasi goreng',
          'category_id': 4,
          'category_name': 'Food',
          'amount': 25000,
          'checked': false,
          'source': 'web',
          'client_id': null,
          'created_at': '2026-11-18 10:00:00',
          'category_master': 'Food',
          'has_receipt': true,
          'member_id': null,
          'member_name': null,
        },
        {'id': 11, 'tx_date': '2026-11-17', 'description': '', 'amount': '5000'},
      ],
      'total': 42,
      'page': 1,
      'per_page': 200,
    });
    expect(page.rows, hasLength(2));
    expect(page.total, 42);
    expect(page.rows.first.categoryName, 'Food');
    expect(page.rows.first.hasReceipt, isTrue);
    expect(page.rows.first.memberId, isNull);
    expect(page.rows[1].amount, 5000); // numeric string tolerated
    expect(page.sum, 30000);
  });

  test('MonthReport computes top spending and budget remaining', () {
    final r = MonthReport.fromJson({
      'total_income': 10000000,
      'budgeted_expenses': 6000000,
      'savings': 1000000,
      'left_for_budgeting': 3000000,
      'expenses_list': [
        {'category_name': 'Food', 'main_category': 'Needs', 'allocation': 2000000, 'realization': 2500000},
        {'category_name': 'Travel', 'main_category': 'Wants', 'allocation': 0, 'realization': 0},
        {'category_name': 'Rent', 'main_category': 'Needs', 'allocation': 3000000, 'realization': 3000000},
      ],
      'income_actual': 9000000,
      'expense_actual': 5500000,
    });
    expect(r.topSpending.map((e) => e.categoryName), ['Rent', 'Food']);
    expect(r.expensesList.first.over, isTrue);
    expect(r.budgetRemaining, 500000);
  });

  test('Goal parses float pct and nullable plan fields', () {
    final g = Goal.fromJson({
      'id': 1,
      'name': 'Holiday',
      'target_amount': 10000000,
      'target_date': '2027-06-01',
      'saved_amount': 2500000,
      'emoji': '',
      'color': '',
      'created_at': '2026-01-01T00:00:00Z',
      'linked_amount': 0,
      'progress': 2500000,
      'remaining': 7500000,
      'pct': 25.0,
      'months_left': 7,
      'monthly_needed': 1071429,
      'asset_ids': <int>[],
    });
    expect(g.fraction, closeTo(0.25, 1e-9));
    expect(g.reached, isFalse);
    expect(g.monthsLeft, 7);
  });

  test('Gamification prefers the member score', () {
    final g = Gamification.fromJson({
      'points': 500,
      'streak_days': 4,
      'last_log_date': '2026-11-18',
      'badge': 'Money Master',
      'member': {
        'id': 7,
        'display_name': 'Ayu',
        'role': 'kid',
        'points': 120,
        'points_month': 40,
        'streak_days': 3,
        'best_streak': 9,
        'level': 'Tracker',
        'level_index': 1,
        'next_level': 'Budget Buddy',
        'next_level_at': 200,
      },
      'badges': [
        {'key': 'first_log', 'name': 'First Log', 'description': 'x', 'icon': '', 'earned': true, 'earned_at': null},
      ],
      'challenges': [],
      'leaderboard': [
        {'member_id': 7, 'display_name': 'Ayu', 'role': 'kid', 'points_month': 40},
      ],
    });
    expect(g.myPoints, 120);
    expect(g.myStreak, 3);
    expect(g.myLevel, 'Tracker');
    expect(g.levelProgress, closeTo(0.6, 1e-9));
    expect(g.badges.single.earned, isTrue);
    expect(g.leaderboard.single.pointsMonth, 40);
  });

  test('Gamification levels: progress runs from the level floor to the next level', () {
    final g = Gamification.fromJson({
      'points': 900,
      'streak_days': 2,
      'badge': 'Penny Pincher',
      'level': 4,
      'member': {
        'id': 7,
        'points': 400,
        'level': 'Penny Pincher',
        'level_index': 3,
        'level_number': 4,
        'level_floor': 300,
        'next_level': 'Budget Boss',
        'next_level_at': 500,
      },
      'badges': [
        {
          'key': 'financial_geek',
          'name': 'Financial Geek',
          'description': 'Log 100 expenses',
          'icon': 'x',
          'category': 'money',
          'tier': 'silver',
          'target': 100,
          'progress': 37,
          'earned': false,
          'earned_at': null,
        },
        {
          'key': 'first_log',
          'name': 'First Log',
          'category': 'money',
          'tier': 'bronze',
          'target': 1,
          'progress': 1,
          'earned': true,
          'earned_at': '2026-11-02T10:00:00Z',
        },
        {'key': 'level_5', 'name': 'Level 5', 'category': 'levels', 'tier': 'gold', 'target': 5, 'progress': 4, 'earned': false},
      ],
    });
    expect(g.myLevelNumber, 4);
    expect(g.myLevelFloor, 300);
    expect(g.myPointsToNext, 100);
    expect(g.levelProgress, closeTo(0.5, 1e-9)); // (400-300)/(500-300), not 400/500
    final geek = g.badges.first;
    expect(geek.category, 'money');
    expect(geek.tier, 'silver');
    expect(geek.fraction, closeTo(0.37, 1e-9));
    expect(g.badges[1].fraction, 1);
    expect(g.badges[1].earnedAt, startsWith('2026-11-02'));
    final groups = groupBadges(g.badges);
    expect(groups.map((e) => e.$1), ['Money', 'Levels']);
    expect(groups.first.$2.length, 2);
  });

  test('Gamification tolerates an old server (no level_number / floor / badge metadata)', () {
    final g = Gamification.fromJson({
      'points': 120,
      'badge': 'Tracker',
      'badges': [
        {'key': 'first_log', 'name': 'First Log', 'earned': false},
      ],
    });
    expect(g.myLevelNumber, 0);
    expect(g.myNextLevelAt, isNull);
    expect(g.levelProgress, 1);
    expect(g.badges.single.fraction, 0);
    expect(groupBadges(g.badges).single.$1, 'Badges');
    // Household fallback derives the floor from the level number (25*n*(n-1)).
    final h = Gamification.fromJson({'points': 200, 'level': 3, 'next_badge': 'X', 'next_badge_at': 300});
    expect(levelFloorFor(3), 150);
    expect(h.myLevelFloor, 150);
    expect(h.levelProgress, closeTo(50 / 150, 1e-9));
  });

  test('Budgeting and actuals lookups', () {
    final b = Budgeting.fromJson({
      'income': [
        {'id': 1, 'account_id': 1, 'year': 2026, 'month': 11, 'source_id': 2, 'amount': 9000000, 'source_name': 'Salary'},
      ],
      'expense': [
        {'id': 1, 'account_id': 1, 'year': 2026, 'month': 11, 'category_id': 5, 'amount': 2000000, 'category_name': 'Food', 'main_category': 'Needs'},
      ],
      'savings': [],
      'income_history': {},
      'expense_history': {},
    });
    final a = BudgetActuals.fromJson({
      'year': 2026,
      'expense': [
        {'category_id': 5, 'month': 11, 'amount': 750000},
      ],
      'income': [
        {'source_id': null, 'month': 11, 'amount': 100000},
      ],
    });
    expect(b.plannedIncome(2, 11), 9000000);
    expect(b.plannedExpense(5, 11), 2000000);
    expect(b.plannedExpense(5, 10), 0);
    expect(a.expenseFor(5, 11), 750000);
    expect(a.incomeFor(null, 11), 100000);
  });

  test('FamilyData parses household and members', () {
    final f = FamilyData.fromJson({
      'household': {'id': 1, 'name': 'The Santosos', 'currency': 'IDR'},
      'me': {'id': 1, 'role': 'owner'},
      'can_manage': true,
      'month': '2026-11',
      'members': [
        {
          'id': 1,
          'email': 'budi@example.com',
          'display_name': 'Budi',
          'role': 'owner',
          'allowance_monthly': null,
          'created_at': '2026-01-01T00:00:00Z',
          'spent_this_month': 1500000,
          'points_this_month': 80,
          'is_me': true,
        },
      ],
    });
    expect(f.householdName, 'The Santosos');
    expect(f.canManage, isTrue);
    expect(f.members.single.isMe, isTrue);
    expect(f.members.single.allowanceMonthly, isNull);
  });

  test('Today sections parse from object or bare list', () {
    final fromObj = parseTodaySections({
      'sections': [
        {
          'key': 'chores',
          'title': 'Chores',
          'items': [
            {'kind': 'chore', 'id': 3, 'title': 'Take out trash', 'done': false, 'overdue': true, 'actions': ['done']},
          ],
        },
      ],
    });
    expect(fromObj.single.items.single.overdue, isTrue);
    expect(fromObj.single.items.single.actions, ['done']);
    final fromList = parseTodaySections([
      {'key': 'budget', 'title': 'Budget', 'items': [], 'summary': '2.1jt left'},
    ]);
    expect(fromList.single.summary, '2.1jt left');
  });

  test('TodayData parses greeting and sections', () {
    final t = TodayData.fromJson({
      'date': '2026-11-18',
      'greeting': 'Good morning, Budi',
      'sections': [
        {'key': 'reminders', 'title': 'Reminders', 'items': [
          {'kind': 'reminder', 'id': 5, 'title': 'Pay electricity', 'done': false, 'actions': ['done', 'snooze']},
        ]},
      ],
    });
    expect(t.greeting, 'Good morning, Budi');
    expect(t.sections.single.items.single.actions, ['done', 'snooze']);
  });

  test('ChoresToday parses items, waiting and week', () {
    final c = ChoresToday.fromJson({
      'date': '2026-11-18',
      'me': {'id': 1, 'role': 'owner', 'can_manage': true},
      'items': [
        {'id': 9, 'chore_id': 2, 'title': 'Dishes', 'emoji': 'legacy-icon', 'points': 5, 'reward_amount': null,
         'due_date': '2026-11-18', 'due_time': null, 'member_id': 3, 'member_name': 'Ayu', 'status': 'submitted',
         'done': false, 'overdue': false, 'completed_at': null, 'has_photo': false, 'note': '',
         'requires_approval': true, 'requires_photo': false, 'points_awarded': 0},
      ],
      'waiting': [],
      'week': {'week_start': '2026-11-16', 'points': 20, 'done': 4, 'total': 6, 'earned_month': 0},
      'members': [],
    });
    expect(c.canManage, isTrue);
    expect(c.items.single.submitted, isTrue);
    expect(c.items.single.label, 'Dishes');
    expect(c.week.total, 6);
  });

  test('ReminderOccurrence uses snoozed_until when snoozed', () {
    final o = ReminderOccurrence.fromJson({
      'id': 1, 'reminder_id': 2, 'title': 'Medicine', 'category': 'medicine', 'kind': 'daily',
      'due_at': '2026-11-18T01:00:00Z', 'status': 'snoozed', 'snoozed_until': '2026-11-18T02:00:00Z',
      'overdue': false, 'can_act': true,
    });
    expect(o.open, isTrue);
    expect(o.effective, DateTime.utc(2026, 11, 18, 2).toLocal());
  });

  test('ListDetail parses list and items', () {
    final d = ListDetail.fromJson({
      'list': {'id': 1, 'name': 'Groceries', 'emoji': '', 'store': '', 'sort_order': 0, 'archived': false,
               'created_at': 'x', 'open_count': 1, 'checked_count': 1},
      'items': [
        {'id': 1, 'list_id': 1, 'name': 'Milk', 'qty': '2', 'note': '', 'checked': false, 'est_price': 18000},
        {'id': 2, 'list_id': 1, 'name': 'Eggs', 'qty': '', 'note': '', 'checked': true, 'checked_by_name': 'Ayu'},
      ],
    });
    expect(d.list.name, 'Groceries');
    expect(d.items.first.estPrice, 18000);
    expect(d.items.last.checkedByName, 'Ayu');
  });

  test('Reminder parses interval, weekdays and category label', () {
    final r = Reminder.fromJson({
      'id': 9, 'title': 'Change bed sheets', 'kind': 'weekly', 'time_of_day': '10:00', 'weekdays': [5],
      'day_of_month': null, 'interval': 2, 'category': 'laundry', 'active': true,
      'description': 'Every 2 weeks on Fri at 10:00', 'assignee_id': 3,
    });
    expect(r.interval, 2);
    expect(r.weekdays, [5]);
    expect(r.assigneeId, 3);
    expect(r.categoryLabel, 'Laundry');
    // older servers without the field -> every week
    expect(Reminder.fromJson({'id': 1, 'title': 'x'}).interval, 1);
  });

  test('household categories and templates', () {
    for (final c in ['cleaning', 'laundry', 'cooking', 'groceries', 'vehicle', 'home_maintenance', 'self_care']) {
      expect(reminderCategories, contains(c));
      expect(reminderCategoryLabel(c), isNot('Other'));
    }
    expect(reminderTemplates.length, 13);
    for (final t in reminderTemplates) {
      expect(reminderCategories, contains(t.category));
      expect(reminderKinds, contains(t.kind));
      expect(t.interval, greaterThanOrEqualTo(1));
    }
    final sheets = reminderTemplates.firstWhere((t) => t.title.startsWith('Change bed sheets'));
    expect(sheets.description, 'Every 2 weeks on Fri at 10:00');
    final trash = reminderTemplates.firstWhere((t) => t.title == 'Take out the trash');
    expect(trash.description, 'Every Tue and Fri at 19:00');
    final oil = reminderTemplates.firstWhere((t) => t.title == 'Change motorbike oil');
    expect(oil.description, 'Every 3 months on day 1 at 09:00');
  });

  test('nextDayOfMonth clamps and rolls over', () {
    expect(nextDayOfMonth(1, DateTime(2026, 11, 18)), DateTime(2026, 12, 1));
    expect(nextDayOfMonth(20, DateTime(2026, 11, 18)), DateTime(2026, 11, 20));
    expect(nextDayOfMonth(18, DateTime(2026, 11, 18, 15)), DateTime(2026, 11, 18));
    expect(nextDayOfMonth(31, DateTime(2027, 2, 3)), DateTime(2027, 2, 28));
  });

  test('calendar, meal requests and widgets parse', () {
    final cal = CalendarData.fromJson({
      'timezone': 'Asia/Jakarta',
      'today': '2026-11-19',
      'activities': [
        {
          'id': 4, 'title': 'Swim', 'color': 'sky', 'all_day': false, 'start_date': '2026-11-02', 'end_date': '2026-11-02',
          'start_time': '16:00', 'end_time': '17:30', 'repeat': 'weekly', 'interval': 2, 'weekdays': [2, 4],
          'attendee_ids': [3], 'attendee_names': ['Naya'], 'occurs_on': '2026-11-17', 'occurs_until': '2026-11-17', 'repeating': true,
        },
        {'id': 5, 'title': 'Trip', 'all_day': true, 'start_date': '2026-11-28', 'end_date': '2026-11-29', 'occurs_on': '2026-11-28', 'occurs_until': '2026-11-29'},
      ],
      'reminders': [
        {'id': 9, 'title': 'Pay rent', 'status': 'pending', 'due_at': '2026-11-20T02:00:00Z'},
      ],
    });
    expect(cal.activities.length, 2);
    expect(cal.activities.first.weekdays, [2, 4]);
    expect(cal.activities.first.multiDay, isFalse);
    expect(cal.activities.last.multiDay, isTrue);
    expect(cal.activities.last.repeat, 'none'); // missing field -> default
    expect(cal.reminders.single.title, 'Pay rent');

    final p = MealProposal.fromJson({'id': 1, 'dish': 'Nasi goreng', 'status': 'approved', 'meal_date': '2026-11-19', 'can_decide': true});
    expect(p.pending, isFalse);
    expect(p.mealDate, '2026-11-19');
    expect(p.canDecide, isTrue);
    expect(MealDay.fromJson({'day': 'Monday', 'date': '2026-11-23', 'picks': [{'id': 1, 'dish': 'Soto', 'member_name': 'Rafi'}]}).picks.single.memberName, 'Rafi');

    final board = WidgetBoard.fromJson({
      'id': 2, 'name': 'TV', 'device': 'tv',
      'layout': {'widgets': [{'id': 'a', 'type': 'clock', 'w': 99, 'h': 0}, {'id': 'b', 'type': 'note', 'w': 4, 'h': 1, 'text': 'hi'}]},
    });
    expect(board.widgets.first.w, 12); // clamped like the server
    expect(board.widgets.first.h, 1);
    expect(board.widgets.last.toJson()['text'], 'hi');
    final data = WidgetData.fromJson({
      'timezone': 'Asia/Jakarta',
      'cards': {
        'budget': {'type': 'budget', 'title': 'Budget', 'big_amount': 2469000, 'rows': [{'title': 'Food', 'amount': 100, 'pct': 42.5, 'alert': true}]},
      },
    });
    expect(data.cards['budget']!.bigAmount, 2469000);
    expect(data.cards['budget']!.rows.single.pct, 42.5);
  });
}
