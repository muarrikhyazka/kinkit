import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../util/dates.dart';
import 'providers.dart';

// Feature data. All autoDispose: data is dropped when no screen shows it,
// so logging out / switching servers never leaks the previous household.
// Every provider watches meProvider's member id so a different login
// refetches.

int? _member(Ref ref) => ref.watch(meProvider.select((m) => m?.memberId));

final setupProvider = FutureProvider.autoDispose<SetupData>((ref) {
  _member(ref);
  return ref.watch(apiProvider).setup();
});

final reportProvider = FutureProvider.autoDispose.family<MonthReport, YearMonth>((ref, ym) {
  _member(ref);
  return ref.watch(apiProvider).report(ym.year, ym.month);
});

final compareProvider = FutureProvider.autoDispose.family<MonthCompare, YearMonth>((ref, ym) {
  _member(ref);
  return ref.watch(apiProvider).monthCompare(ym.year, ym.month);
});

/// Spending for a month (newest first). 200 rows per month covers typical
/// households; the screen shows a "showing N of total" note beyond that.
final spendingProvider = FutureProvider.autoDispose.family<SpendingPage, YearMonth>((ref, ym) {
  _member(ref);
  return ref.watch(apiProvider).spending(year: ym.year, month: ym.month, perPage: 200);
});

final incomeProvider = FutureProvider.autoDispose.family<IncomeList, YearMonth>((ref, ym) {
  _member(ref);
  return ref.watch(apiProvider).income(ym.year, ym.month);
});

final budgetingProvider = FutureProvider.autoDispose.family<Budgeting, int>((ref, year) {
  _member(ref);
  return ref.watch(apiProvider).budgeting(year);
});

final budgetActualsProvider = FutureProvider.autoDispose.family<BudgetActuals, int>((ref, year) {
  _member(ref);
  return ref.watch(apiProvider).budgetActuals(year);
});

final goalsProvider = FutureProvider.autoDispose<List<Goal>>((ref) {
  _member(ref);
  return ref.watch(apiProvider).goals();
});

final familyProvider = FutureProvider.autoDispose<FamilyData>((ref) {
  _member(ref);
  return ref.watch(apiProvider).family();
});

final gamificationProvider = FutureProvider.autoDispose<Gamification>((ref) {
  _member(ref);
  return ref.watch(apiProvider).gamification();
});

/// Today payload from GET /api/today; null when the server doesn't have it.
final todayProvider = FutureProvider.autoDispose<TodayData?>((ref) {
  _member(ref);
  return ref.watch(apiProvider).today();
});

final choresTodayProvider = FutureProvider.autoDispose<ChoresToday>((ref) {
  _member(ref);
  return ref.watch(apiProvider).choresToday();
});

final remindersProvider = FutureProvider.autoDispose<List<ReminderOccurrence>>((ref) {
  _member(ref);
  return ref.watch(apiProvider).upcomingReminders(days: 7);
});

final reminderDefsProvider = FutureProvider.autoDispose<List<Reminder>>((ref) {
  _member(ref);
  return ref.watch(apiProvider).reminders();
});

final listsProvider = FutureProvider.autoDispose<List<ShoppingList>>((ref) {
  _member(ref);
  return ref.watch(apiProvider).lists();
});

final listDetailProvider = FutureProvider.autoDispose.family<ListDetail, int>((ref, id) {
  _member(ref);
  return ref.watch(apiProvider).listDetail(id);
});

/// After a chore / reminder / list change (they all feed Today).
void invalidateHome(WidgetRef ref) {
  ref.invalidate(todayProvider);
  ref.invalidate(choresTodayProvider);
  ref.invalidate(remindersProvider);
  ref.invalidate(reminderDefsProvider);
  ref.invalidate(listsProvider);
  ref.invalidate(listDetailProvider);
  ref.invalidate(gamificationProvider);
}

/// Call after any spending write so every view showing it refreshes
/// (mirrors notifySpendingChanged() on the web).
void invalidateSpending(WidgetRef ref) {
  ref.invalidate(spendingProvider);
  ref.invalidate(reportProvider);
  ref.invalidate(compareProvider);
  ref.invalidate(budgetActualsProvider);
  ref.invalidate(gamificationProvider);
  ref.invalidate(todayProvider);
  ref.invalidate(familyProvider);
}

void invalidateIncome(WidgetRef ref) {
  ref.invalidate(incomeProvider);
  ref.invalidate(reportProvider);
  ref.invalidate(budgetActualsProvider);
}
