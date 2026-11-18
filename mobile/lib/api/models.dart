// Typed models mirroring the kinkit backend JSON (backend-go/*.go).
// Parsing is lenient: numbers may arrive as int, double or string, and
// missing fields fall back to sensible defaults, so a newer/older server
// never crashes the app.

typedef Json = Map<String, dynamic>;

int asInt(dynamic v, [int fallback = 0]) {
  if (v is int) return v;
  if (v is num) return v.round();
  if (v is String) return int.tryParse(v) ?? double.tryParse(v)?.round() ?? fallback;
  return fallback;
}

int? asIntOrNull(dynamic v) => v == null ? null : asInt(v);

double asDouble(dynamic v, [double fallback = 0]) {
  if (v is num) return v.toDouble();
  if (v is String) return double.tryParse(v) ?? fallback;
  return fallback;
}

String asString(dynamic v, [String fallback = '']) => v == null ? fallback : v.toString();

String? asStringOrNull(dynamic v) => v?.toString();

bool asBool(dynamic v) => v == true || v == 1 || v == 'true' || v == '1';

Json asJson(dynamic v) => v is Map ? Map<String, dynamic>.from(v) : <String, dynamic>{};

List<T> asList<T>(dynamic v, T Function(Json) parse) {
  if (v is! List) return <T>[];
  return v.whereType<Map>().map((e) => parse(Map<String, dynamic>.from(e))).toList();
}

// ---------------------------------------------------------------- auth

/// GET /api/me
class Me {
  final bool authenticated;
  final int accountId;
  final int memberId;
  final String role; // owner | parent | kid | viewer
  final String displayName;
  final String email;
  final String currency;
  final String locale;
  final bool isAdmin;
  final String householdName;

  const Me({
    required this.authenticated,
    this.accountId = 0,
    this.memberId = 0,
    this.role = 'owner',
    this.displayName = '',
    this.email = '',
    this.currency = 'IDR',
    this.locale = 'id-ID',
    this.isAdmin = false,
    this.householdName = '',
  });

  factory Me.fromJson(Json j) => Me(
        authenticated: asBool(j['authenticated']),
        accountId: asInt(j['account_id']),
        memberId: asInt(j['member_id']),
        role: asString(j['role'], 'owner'),
        displayName: asString(j['display_name']),
        email: asString(j['email'] ?? j['username']),
        currency: asString(j['currency'], 'IDR'),
        locale: asString(j['locale'], 'id-ID'),
        isAdmin: asBool(j['is_admin']),
        householdName: asString(j['household_name']),
      );

  /// owner/parent run the household (budget edits, family management).
  bool get canManage => role == 'owner' || role == 'parent';

  /// Everyone but kids may see income / budget / savings (routes.jsx READERS).
  bool get canReadMoney => role != 'kid';

  /// Viewers are read-only; kids may add spending.
  bool get canAddSpending => role != 'viewer';

  bool get isKid => role == 'kid';
}

/// GET /api/fx-rates - rates relative to IDR (IDR = 1).
class FxRates {
  final String base;
  final Map<String, double> rates;

  const FxRates({this.base = 'IDR', this.rates = const {'IDR': 1}});

  factory FxRates.fromJson(Json j) {
    final raw = asJson(j['rates']);
    final rates = <String, double>{for (final e in raw.entries) e.key: asDouble(e.value)};
    if (rates.isEmpty) rates['IDR'] = 1;
    return FxRates(base: asString(j['base'], 'IDR'), rates: rates);
  }
}

// ---------------------------------------------------------------- setup

class Category {
  final int id;
  final String name;
  final String mainCategory; // Needs | Wants | Savings

  const Category({required this.id, required this.name, required this.mainCategory});

  factory Category.fromJson(Json j) => Category(
        id: asInt(j['id']),
        name: asString(j['name']),
        mainCategory: asString(j['main_category'], 'Needs'),
      );
}

class IncomeSource {
  final int id;
  final String name;
  const IncomeSource({required this.id, required this.name});
  factory IncomeSource.fromJson(Json j) => IncomeSource(id: asInt(j['id']), name: asString(j['name']));
}

class BankAccount {
  final int id;
  final String name;
  final String kind;
  const BankAccount({required this.id, required this.name, required this.kind});
  factory BankAccount.fromJson(Json j) =>
      BankAccount(id: asInt(j['id']), name: asString(j['name']), kind: asString(j['kind']));
}

/// GET /api/setup
class SetupData {
  final List<BankAccount> accounts;
  final List<Category> categories;
  final List<IncomeSource> incomeSources;

  const SetupData({this.accounts = const [], this.categories = const [], this.incomeSources = const []});

  factory SetupData.fromJson(Json j) => SetupData(
        accounts: asList(j['accounts'], BankAccount.fromJson),
        categories: asList(j['categories'], Category.fromJson),
        incomeSources: asList(j['income_sources'], IncomeSource.fromJson),
      );

  Map<String, List<Category>> get categoriesByMain {
    final out = <String, List<Category>>{'Needs': [], 'Wants': [], 'Savings': []};
    for (final c in categories) {
      out.putIfAbsent(c.mainCategory, () => []).add(c);
    }
    return out;
  }
}

// ---------------------------------------------------------------- spending

class SpendingRow {
  final int id;
  final String txDate;
  final String description;
  final int? categoryId;
  final String? categoryName;
  final int amount;
  final bool checked;
  final String source;
  final String? categoryMaster;
  final bool hasReceipt;
  final int? memberId;
  final String? memberName;

  const SpendingRow({
    required this.id,
    required this.txDate,
    required this.description,
    this.categoryId,
    this.categoryName,
    required this.amount,
    this.checked = false,
    this.source = 'web',
    this.categoryMaster,
    this.hasReceipt = false,
    this.memberId,
    this.memberName,
  });

  factory SpendingRow.fromJson(Json j) => SpendingRow(
        id: asInt(j['id']),
        txDate: asString(j['tx_date']),
        description: asString(j['description']),
        categoryId: asIntOrNull(j['category_id']),
        categoryName: asStringOrNull(j['category_name']),
        amount: asInt(j['amount']),
        checked: asBool(j['checked']),
        source: asString(j['source'], 'web'),
        categoryMaster: asStringOrNull(j['category_master']),
        hasReceipt: asBool(j['has_receipt']),
        memberId: asIntOrNull(j['member_id']),
        memberName: asStringOrNull(j['member_name']),
      );
}

/// GET /api/spending
class SpendingPage {
  final List<SpendingRow> rows;
  final int total;
  final int page;
  final int perPage;

  const SpendingPage({this.rows = const [], this.total = 0, this.page = 1, this.perPage = 20});

  factory SpendingPage.fromJson(Json j) => SpendingPage(
        rows: asList(j['rows'], SpendingRow.fromJson),
        total: asInt(j['total']),
        page: asInt(j['page'], 1),
        perPage: asInt(j['per_page'], 20),
      );

  int get sum => rows.fold(0, (a, r) => a + r.amount);
}

// ---------------------------------------------------------------- income

class IncomeEntry {
  final int id;
  final int? sourceId;
  final String? sourceName;
  final String txDate;
  final String description;
  final int amount;

  const IncomeEntry({
    required this.id,
    this.sourceId,
    this.sourceName,
    required this.txDate,
    this.description = '',
    required this.amount,
  });

  factory IncomeEntry.fromJson(Json j) => IncomeEntry(
        id: asInt(j['id']),
        sourceId: asIntOrNull(j['source_id']),
        sourceName: asStringOrNull(j['source_name']),
        txDate: asString(j['tx_date']),
        description: asString(j['description']),
        amount: asInt(j['amount']),
      );
}

class IncomeBySource {
  final int? sourceId;
  final String? sourceName;
  final int total;
  const IncomeBySource({this.sourceId, this.sourceName, required this.total});
  factory IncomeBySource.fromJson(Json j) => IncomeBySource(
        sourceId: asIntOrNull(j['source_id']),
        sourceName: asStringOrNull(j['source_name']),
        total: asInt(j['total']),
      );
}

/// GET /api/income?year=&month=
class IncomeList {
  final List<IncomeEntry> rows;
  final int total;
  final int planned;
  final List<IncomeBySource> bySource;

  const IncomeList({this.rows = const [], this.total = 0, this.planned = 0, this.bySource = const []});

  factory IncomeList.fromJson(Json j) => IncomeList(
        rows: asList(j['rows'], IncomeEntry.fromJson),
        total: asInt(j['total']),
        planned: asInt(j['planned']),
        bySource: asList(j['by_source'], IncomeBySource.fromJson),
      );
}

// ---------------------------------------------------------------- reports

class CategoryRealization {
  final String categoryName;
  final String mainCategory;
  final int allocation;
  final int realization;

  const CategoryRealization({
    required this.categoryName,
    required this.mainCategory,
    required this.allocation,
    required this.realization,
  });

  factory CategoryRealization.fromJson(Json j) => CategoryRealization(
        categoryName: asString(j['category_name']),
        mainCategory: asString(j['main_category']),
        allocation: asInt(j['allocation']),
        realization: asInt(j['realization']),
      );

  bool get over => realization > allocation;
  double get ratio => allocation > 0 ? realization / allocation : (realization > 0 ? 1.0 : 0.0);
}

/// GET /api/report/{year}/{month}
class MonthReport {
  final int totalIncome; // planned income (budget)
  final int budgetedExpenses;
  final int savings;
  final int leftForBudgeting;
  final List<CategoryRealization> expensesList;
  final int incomeActual;
  final int expenseActual;

  const MonthReport({
    this.totalIncome = 0,
    this.budgetedExpenses = 0,
    this.savings = 0,
    this.leftForBudgeting = 0,
    this.expensesList = const [],
    this.incomeActual = 0,
    this.expenseActual = 0,
  });

  factory MonthReport.fromJson(Json j) => MonthReport(
        totalIncome: asInt(j['total_income']),
        budgetedExpenses: asInt(j['budgeted_expenses']),
        savings: asInt(j['savings']),
        leftForBudgeting: asInt(j['left_for_budgeting']),
        expensesList: asList(j['expenses_list'], CategoryRealization.fromJson),
        incomeActual: asInt(j['income_actual']),
        expenseActual: asInt(j['expense_actual']),
      );

  List<CategoryRealization> get topSpending {
    final l = expensesList.where((e) => e.realization > 0).toList()
      ..sort((a, b) => b.realization.compareTo(a.realization));
    return l.take(10).toList();
  }

  int get budgetRemaining => budgetedExpenses - expenseActual;
}

/// GET /api/report/month-compare/{year}/{month}
class MonthCompare {
  final int current;
  final int previousMonth;
  final int sameMonthLastYear;

  const MonthCompare({this.current = 0, this.previousMonth = 0, this.sameMonthLastYear = 0});

  factory MonthCompare.fromJson(Json j) => MonthCompare(
        current: asInt(j['current']),
        previousMonth: asInt(j['previous_month']),
        sameMonthLastYear: asInt(j['same_month_last_year']),
      );

  /// Percentage change vs a baseline; null when the baseline is 0.
  static double? pctChange(int now, int before) => before == 0 ? null : (now - before) / before * 100;
}

// ---------------------------------------------------------------- budgeting

class BudgetIncomeRow {
  final int sourceId;
  final int month;
  final int amount;
  final String sourceName;
  const BudgetIncomeRow({required this.sourceId, required this.month, required this.amount, this.sourceName = ''});
  factory BudgetIncomeRow.fromJson(Json j) => BudgetIncomeRow(
        sourceId: asInt(j['source_id']),
        month: asInt(j['month']),
        amount: asInt(j['amount']),
        sourceName: asString(j['source_name']),
      );
}

class BudgetExpenseRow {
  final int categoryId;
  final int month;
  final int amount;
  final String categoryName;
  final String mainCategory;
  const BudgetExpenseRow({
    required this.categoryId,
    required this.month,
    required this.amount,
    this.categoryName = '',
    this.mainCategory = '',
  });
  factory BudgetExpenseRow.fromJson(Json j) => BudgetExpenseRow(
        categoryId: asInt(j['category_id']),
        month: asInt(j['month']),
        amount: asInt(j['amount']),
        categoryName: asString(j['category_name']),
        mainCategory: asString(j['main_category']),
      );
}

/// GET /api/budgeting/{year}
class Budgeting {
  final List<BudgetIncomeRow> income;
  final List<BudgetExpenseRow> expense;

  const Budgeting({this.income = const [], this.expense = const []});

  factory Budgeting.fromJson(Json j) => Budgeting(
        income: asList(j['income'], BudgetIncomeRow.fromJson),
        expense: asList(j['expense'], BudgetExpenseRow.fromJson),
      );

  int plannedExpense(int categoryId, int month) {
    for (final r in expense) {
      if (r.categoryId == categoryId && r.month == month) return r.amount;
    }
    return 0;
  }

  int plannedIncome(int sourceId, int month) {
    for (final r in income) {
      if (r.sourceId == sourceId && r.month == month) return r.amount;
    }
    return 0;
  }
}

class ActualAmount {
  final int? id; // category_id or source_id (null = no source)
  final int month;
  final int amount;
  const ActualAmount({this.id, required this.month, required this.amount});
}

/// GET /api/budgeting/{year}/actuals
class BudgetActuals {
  final List<ActualAmount> expense;
  final List<ActualAmount> income;

  const BudgetActuals({this.expense = const [], this.income = const []});

  factory BudgetActuals.fromJson(Json j) => BudgetActuals(
        expense: asList(j['expense'],
            (e) => ActualAmount(id: asIntOrNull(e['category_id']), month: asInt(e['month']), amount: asInt(e['amount']))),
        income: asList(j['income'],
            (e) => ActualAmount(id: asIntOrNull(e['source_id']), month: asInt(e['month']), amount: asInt(e['amount']))),
      );

  int expenseFor(int categoryId, int month) {
    for (final r in expense) {
      if (r.id == categoryId && r.month == month) return r.amount;
    }
    return 0;
  }

  int incomeFor(int? sourceId, int month) {
    for (final r in income) {
      if (r.id == sourceId && r.month == month) return r.amount;
    }
    return 0;
  }
}

// ---------------------------------------------------------------- goals

/// GET /api/goals (top-level array)
class Goal {
  final int id;
  final String name;
  final int targetAmount;
  final String? targetDate;
  final int savedAmount;
  final String emoji;
  final String color;
  final int linkedAmount;
  final int progress;
  final int remaining;
  final double pct;
  final int? monthsLeft;
  final int? monthlyNeeded;

  const Goal({
    required this.id,
    required this.name,
    required this.targetAmount,
    this.targetDate,
    this.savedAmount = 0,
    this.emoji = '',
    this.color = '',
    this.linkedAmount = 0,
    this.progress = 0,
    this.remaining = 0,
    this.pct = 0,
    this.monthsLeft,
    this.monthlyNeeded,
  });

  factory Goal.fromJson(Json j) => Goal(
        id: asInt(j['id']),
        name: asString(j['name']),
        targetAmount: asInt(j['target_amount']),
        targetDate: asStringOrNull(j['target_date']),
        savedAmount: asInt(j['saved_amount']),
        emoji: asString(j['emoji']),
        color: asString(j['color']),
        linkedAmount: asInt(j['linked_amount']),
        progress: asInt(j['progress']),
        remaining: asInt(j['remaining']),
        pct: asDouble(j['pct']),
        monthsLeft: asIntOrNull(j['months_left']),
        monthlyNeeded: asIntOrNull(j['monthly_needed']),
      );

  /// 0..1 for progress bars (backend pct is 0..100).
  double get fraction => (pct / 100).clamp(0.0, 1.0);
  bool get reached => targetAmount > 0 && progress >= targetAmount;
}

// ---------------------------------------------------------------- family

class FamilyMember {
  final int id;
  final String email;
  final String displayName;
  final String role;
  final double? allowanceMonthly;
  final int spentThisMonth;
  final int pointsThisMonth;
  final bool isMe;

  const FamilyMember({
    required this.id,
    required this.email,
    required this.displayName,
    required this.role,
    this.allowanceMonthly,
    this.spentThisMonth = 0,
    this.pointsThisMonth = 0,
    this.isMe = false,
  });

  factory FamilyMember.fromJson(Json j) => FamilyMember(
        id: asInt(j['id']),
        email: asString(j['email']),
        displayName: asString(j['display_name']),
        role: asString(j['role'], 'viewer'),
        allowanceMonthly: j['allowance_monthly'] == null ? null : asDouble(j['allowance_monthly']),
        spentThisMonth: asInt(j['spent_this_month']),
        pointsThisMonth: asInt(j['points_this_month']),
        isMe: asBool(j['is_me']),
      );
}

/// GET /api/family
class FamilyData {
  final String householdName;
  final String currency;
  final bool canManage;
  final String month;
  final List<FamilyMember> members;

  const FamilyData({
    this.householdName = '',
    this.currency = 'IDR',
    this.canManage = false,
    this.month = '',
    this.members = const [],
  });

  factory FamilyData.fromJson(Json j) {
    final h = asJson(j['household']);
    return FamilyData(
      householdName: asString(h['name']),
      currency: asString(h['currency'], 'IDR'),
      canManage: asBool(j['can_manage']),
      month: asString(j['month']),
      members: asList(j['members'], FamilyMember.fromJson),
    );
  }
}

// ---------------------------------------------------------------- gamification

class MemberGamification {
  final int id;
  final String displayName;
  final int points;
  final int pointsMonth;
  final int streakDays;
  final int bestStreak;
  final String level;
  final String? nextLevel;
  final int? nextLevelAt;

  const MemberGamification({
    required this.id,
    this.displayName = '',
    this.points = 0,
    this.pointsMonth = 0,
    this.streakDays = 0,
    this.bestStreak = 0,
    this.level = '',
    this.nextLevel,
    this.nextLevelAt,
  });

  factory MemberGamification.fromJson(Json j) => MemberGamification(
        id: asInt(j['id']),
        displayName: asString(j['display_name']),
        points: asInt(j['points']),
        pointsMonth: asInt(j['points_month']),
        streakDays: asInt(j['streak_days']),
        bestStreak: asInt(j['best_streak']),
        level: asString(j['level']),
        nextLevel: asStringOrNull(j['next_level']),
        nextLevelAt: asIntOrNull(j['next_level_at']),
      );
}

class BadgeView {
  final String key;
  final String name;
  final String description;
  final String icon;
  final bool earned;

  const BadgeView({required this.key, required this.name, this.description = '', this.icon = '', this.earned = false});

  factory BadgeView.fromJson(Json j) => BadgeView(
        key: asString(j['key']),
        name: asString(j['name']),
        description: asString(j['description']),
        icon: asString(j['icon']),
        earned: asBool(j['earned']),
      );
}

class LeaderRow {
  final int memberId;
  final String displayName;
  final String role;
  final int pointsMonth;
  const LeaderRow({required this.memberId, this.displayName = '', this.role = '', this.pointsMonth = 0});
  factory LeaderRow.fromJson(Json j) => LeaderRow(
        memberId: asInt(j['member_id']),
        displayName: asString(j['display_name']),
        role: asString(j['role']),
        pointsMonth: asInt(j['points_month']),
      );
}

/// GET /api/gamification
class Gamification {
  final int points; // household total
  final int streakDays;
  final String badge;
  final String? nextBadge;
  final int? nextBadgeAt;
  final MemberGamification? member;
  final List<BadgeView> badges;
  final List<LeaderRow> leaderboard;

  const Gamification({
    this.points = 0,
    this.streakDays = 0,
    this.badge = '',
    this.nextBadge,
    this.nextBadgeAt,
    this.member,
    this.badges = const [],
    this.leaderboard = const [],
  });

  factory Gamification.fromJson(Json j) => Gamification(
        points: asInt(j['points']),
        streakDays: asInt(j['streak_days']),
        badge: asString(j['badge']),
        nextBadge: asStringOrNull(j['next_badge']),
        nextBadgeAt: asIntOrNull(j['next_badge_at']),
        member: j['member'] is Map ? MemberGamification.fromJson(asJson(j['member'])) : null,
        badges: asList(j['badges'], BadgeView.fromJson),
        leaderboard: asList(j['leaderboard'], LeaderRow.fromJson),
      );

  // What the web's GamificationBadge shows: the member's own score when known.
  int get myPoints => member?.points ?? points;
  int get myStreak => member?.streakDays ?? streakDays;
  String get myLevel => member?.level ?? badge;
  String? get myNextLevel => member != null ? member!.nextLevel : nextBadge;
  int? get myNextLevelAt => member != null ? member!.nextLevelAt : nextBadgeAt;

  double get levelProgress {
    final at = myNextLevelAt;
    if (at == null || at <= 0) return 1;
    return (myPoints / at).clamp(0.0, 1.0);
  }
}

// ---------------------------------------------------------------- today

/// TodayItem / TodaySection mirror backend-go/home_contract.go.
class TodayItem {
  final String kind;
  final int id;
  final String title;
  final String detail;
  final String due;
  final String memberName;
  final bool done;
  final bool overdue;
  final String link;
  final List<String> actions;

  const TodayItem({
    required this.kind,
    this.id = 0,
    required this.title,
    this.detail = '',
    this.due = '',
    this.memberName = '',
    this.done = false,
    this.overdue = false,
    this.link = '',
    this.actions = const [],
  });

  factory TodayItem.fromJson(Json j) => TodayItem(
        kind: asString(j['kind']),
        id: asInt(j['id']),
        title: asString(j['title']),
        detail: asString(j['detail']),
        due: asString(j['due']),
        memberName: asString(j['member_name']),
        done: asBool(j['done']),
        overdue: asBool(j['overdue']),
        link: asString(j['link']),
        actions: j['actions'] is List ? (j['actions'] as List).map((e) => e.toString()).toList() : const [],
      );
}

class TodaySection {
  final String key;
  final String title;
  final List<TodayItem> items;
  final String summary;

  const TodaySection({required this.key, required this.title, this.items = const [], this.summary = ''});

  factory TodaySection.fromJson(Json j) => TodaySection(
        key: asString(j['key']),
        title: asString(j['title']),
        items: asList(j['items'], TodayItem.fromJson),
        summary: asString(j['summary']),
      );
}

/// Parses either `{"sections": [...]}` or a bare `[...]` of sections.
List<TodaySection> parseTodaySections(dynamic data) {
  if (data is List) return asList(data, TodaySection.fromJson);
  if (data is Map) return asList(data['sections'], TodaySection.fromJson);
  return const [];
}

// ---------------------------------------------------------------- chores

/// One chore occurrence (backend-go/chores.go choreInstanceView).
/// Actions (complete/approve/reject/undo) take this instance [id].
class ChoreInstance {
  final int id; // 0 = projected (not generated yet, no actions)
  final int choreId;
  final String title;
  final String emoji;
  final int points;
  final double? rewardAmount;
  final String dueDate;
  final String? dueTime;
  final int? memberId;
  final String memberName;
  final String status; // pending | submitted | approved | rejected | ...
  final bool done;
  final bool overdue;
  final String note;
  final bool requiresApproval;
  final bool requiresPhoto;
  final bool projected;

  const ChoreInstance({
    required this.id,
    this.choreId = 0,
    required this.title,
    this.emoji = '',
    this.points = 0,
    this.rewardAmount,
    this.dueDate = '',
    this.dueTime,
    this.memberId,
    this.memberName = '',
    this.status = '',
    this.done = false,
    this.overdue = false,
    this.note = '',
    this.requiresApproval = false,
    this.requiresPhoto = false,
    this.projected = false,
  });

  factory ChoreInstance.fromJson(Json j) => ChoreInstance(
        id: asInt(j['id']),
        choreId: asInt(j['chore_id']),
        title: asString(j['title']),
        emoji: asString(j['emoji']),
        points: asInt(j['points']),
        rewardAmount: j['reward_amount'] == null ? null : asDouble(j['reward_amount']),
        dueDate: asString(j['due_date']),
        dueTime: asStringOrNull(j['due_time']),
        memberId: asIntOrNull(j['member_id']),
        memberName: asString(j['member_name']),
        status: asString(j['status']),
        done: asBool(j['done']),
        overdue: asBool(j['overdue']),
        note: asString(j['note']),
        requiresApproval: asBool(j['requires_approval']),
        requiresPhoto: asBool(j['requires_photo']),
        projected: asBool(j['projected']),
      );

  bool get submitted => status == 'submitted';
  String get label => emoji.isNotEmpty ? '$emoji $title' : title;
}

class ChoreWeek {
  final int points;
  final int done;
  final int total;
  const ChoreWeek({this.points = 0, this.done = 0, this.total = 0});
  factory ChoreWeek.fromJson(Json j) =>
      ChoreWeek(points: asInt(j['points']), done: asInt(j['done']), total: asInt(j['total']));
}

/// GET /api/chores/today
class ChoresToday {
  final String date;
  final bool canManage;
  final List<ChoreInstance> items;
  final List<ChoreInstance> waiting; // submitted, awaiting approval (managers)
  final ChoreWeek week;

  const ChoresToday({
    this.date = '',
    this.canManage = false,
    this.items = const [],
    this.waiting = const [],
    this.week = const ChoreWeek(),
  });

  factory ChoresToday.fromJson(Json j) => ChoresToday(
        date: asString(j['date']),
        canManage: asBool(asJson(j['me'])['can_manage']),
        items: asList(j['items'], ChoreInstance.fromJson),
        waiting: asList(j['waiting'], ChoreInstance.fromJson),
        week: ChoreWeek.fromJson(asJson(j['week'])),
      );
}

// ---------------------------------------------------------------- reminders

/// One reminder occurrence (backend-go/reminders.go occurrenceRow).
class ReminderOccurrence {
  final int id;
  final int reminderId;
  final String title;
  final String notes;
  final String category;
  final String kind;
  final String assigneeName;
  final DateTime? dueAt;
  final String status; // pending | snoozed | done | missed
  final DateTime? snoozedUntil;
  final String doneByName;
  final bool overdue;
  final bool canAct;

  const ReminderOccurrence({
    required this.id,
    this.reminderId = 0,
    required this.title,
    this.notes = '',
    this.category = 'other',
    this.kind = '',
    this.assigneeName = '',
    this.dueAt,
    this.status = 'pending',
    this.snoozedUntil,
    this.doneByName = '',
    this.overdue = false,
    this.canAct = false,
  });

  factory ReminderOccurrence.fromJson(Json j) => ReminderOccurrence(
        id: asInt(j['id']),
        reminderId: asInt(j['reminder_id']),
        title: asString(j['title']),
        notes: asString(j['notes']),
        category: asString(j['category'], 'other'),
        kind: asString(j['kind']),
        assigneeName: asString(j['assignee_name']),
        dueAt: DateTime.tryParse(asString(j['due_at']))?.toLocal(),
        status: asString(j['status'], 'pending'),
        snoozedUntil: DateTime.tryParse(asString(j['snoozed_until']))?.toLocal(),
        doneByName: asString(j['done_by_name']),
        overdue: asBool(j['overdue']),
        canAct: asBool(j['can_act']),
      );

  bool get done => status == 'done';
  bool get open => status == 'pending' || status == 'snoozed';

  /// When it's (next) due: snoozed_until for snoozed occurrences.
  DateTime? get effective => status == 'snoozed' && snoozedUntil != null ? snoozedUntil : dueAt;
}

const reminderCategories = ['bill', 'medicine', 'school', 'birthday', 'chores', 'other'];
const reminderKinds = ['once', 'daily', 'weekly', 'monthly', 'yearly'];

// ---------------------------------------------------------------- shopping lists

class ShoppingList {
  final int id;
  final String name;
  final String emoji;
  final String store;
  final bool archived;
  final int openCount;
  final int checkedCount;

  const ShoppingList({
    required this.id,
    required this.name,
    this.emoji = '',
    this.store = '',
    this.archived = false,
    this.openCount = 0,
    this.checkedCount = 0,
  });

  factory ShoppingList.fromJson(Json j) => ShoppingList(
        id: asInt(j['id']),
        name: asString(j['name']),
        emoji: asString(j['emoji']),
        store: asString(j['store']),
        archived: asBool(j['archived']),
        openCount: asInt(j['open_count']),
        checkedCount: asInt(j['checked_count']),
      );

  String get label => emoji.isNotEmpty ? '$emoji $name' : name;
}

class ListItem {
  final int id;
  final int listId;
  final String name;
  final String qty;
  final String note;
  final String? categoryName;
  final int? estPrice;
  final bool checked;
  final String? checkedByName;
  final String? addedByName;

  const ListItem({
    required this.id,
    this.listId = 0,
    required this.name,
    this.qty = '',
    this.note = '',
    this.categoryName,
    this.estPrice,
    this.checked = false,
    this.checkedByName,
    this.addedByName,
  });

  factory ListItem.fromJson(Json j) => ListItem(
        id: asInt(j['id']),
        listId: asInt(j['list_id']),
        name: asString(j['name']),
        qty: asString(j['qty']),
        note: asString(j['note']),
        categoryName: asStringOrNull(j['category_name']),
        estPrice: asIntOrNull(j['est_price']),
        checked: asBool(j['checked']),
        checkedByName: asStringOrNull(j['checked_by_name']),
        addedByName: asStringOrNull(j['added_by_name']),
      );
}

/// GET /api/lists/{id}
class ListDetail {
  final ShoppingList list;
  final List<ListItem> items;
  const ListDetail({required this.list, this.items = const []});
  factory ListDetail.fromJson(Json j) => ListDetail(
        list: ShoppingList.fromJson(asJson(j['list'])),
        items: asList(j['items'], ListItem.fromJson),
      );
}

/// Today page payload (GET /api/today): greeting + sections.
class TodayData {
  final String date;
  final String greeting;
  final List<TodaySection> sections;
  const TodayData({this.date = '', this.greeting = '', this.sections = const []});
  factory TodayData.fromJson(dynamic data) => TodayData(
        date: data is Map ? asString(data['date']) : '',
        greeting: data is Map ? asString(data['greeting']) : '',
        sections: parseTodaySections(data),
      );
}

/// A reminder definition (GET /api/reminders -> reminders[]).
class Reminder {
  final int id;
  final String title;
  final String notes;
  final String assigneeName;
  final String kind;
  final String timeOfDay;
  final String category;
  final String description; // human-readable schedule from the server
  final bool active;
  final DateTime? nextDue;

  const Reminder({
    required this.id,
    required this.title,
    this.notes = '',
    this.assigneeName = '',
    this.kind = 'once',
    this.timeOfDay = '',
    this.category = 'other',
    this.description = '',
    this.active = true,
    this.nextDue,
  });

  factory Reminder.fromJson(Json j) => Reminder(
        id: asInt(j['id']),
        title: asString(j['title']),
        notes: asString(j['notes']),
        assigneeName: asString(j['assignee_name']),
        kind: asString(j['kind'], 'once'),
        timeOfDay: asString(j['time_of_day']),
        category: asString(j['category'], 'other'),
        description: asString(j['description']),
        active: j['active'] == null ? true : asBool(j['active']),
        nextDue: DateTime.tryParse(asString(j['next_due']))?.toLocal(),
      );
}
