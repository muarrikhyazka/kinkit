import 'package:cookie_jar/cookie_jar.dart';
import 'package:dio/dio.dart';
import 'package:dio_cookie_manager/dio_cookie_manager.dart';

import '../config.dart';
import 'models.dart';

/// Error from the kinkit API. [status] is null for network failures.
class ApiException implements Exception {
  final int? status;
  final String message;

  const ApiException(this.message, {this.status});

  bool get isUnauthorized => status == 401;
  bool get isNotFound => status == 404;
  bool get isForbidden => status == 403;
  bool get isNetwork => status == null;

  @override
  String toString() => message;
}

/// Typed client for the kinkit Go backend.
///
/// Auth is the same session cookie the web uses (`accountamily`, set by
/// POST /api/login). Cookies are kept in a [CookieJar] - a PersistCookieJar
/// in the app so the session survives restarts (the server issues a 30 day
/// Max-Age cookie).
class KinkitApi {
  final String baseUrl;
  final CookieJar cookieJar;
  final Dio _dio;

  /// Called when an authenticated request comes back 401 (session expired,
  /// member removed). The app uses it to drop back to the login screen.
  void Function()? onUnauthorized;

  KinkitApi({required this.baseUrl, required this.cookieJar, Dio? dio})
      : _dio = dio ?? Dio() {
    _dio.options
      ..baseUrl = '$baseUrl/api'
      ..connectTimeout = const Duration(seconds: 10)
      ..receiveTimeout = const Duration(seconds: 20)
      ..responseType = ResponseType.json
      // Handle every HTTP status ourselves (see _unwrap).
      ..validateStatus = ((s) => s != null && s < 600)
      ..headers = {
        'Accept': 'application/json',
        // Harmless for the current backend; lets a future CSRF/Origin check
        // tell native clients apart.
        'X-Requested-With': 'kinkit-mobile',
      };
    _dio.interceptors.add(CookieManager(cookieJar));
  }

  // ------------------------------------------------------------ plumbing

  Future<dynamic> _send(String method, String path, {Object? body, Map<String, dynamic>? query}) async {
    Response<dynamic> res;
    try {
      res = await _dio.request<dynamic>(
        path,
        data: body,
        queryParameters: query,
        options: Options(method: method, contentType: body != null ? Headers.jsonContentType : null),
      );
    } on DioException catch (e) {
      throw ApiException(_networkMessage(e));
    }
    return _unwrap(res, path);
  }

  dynamic _unwrap(Response<dynamic> res, String path) {
    final status = res.statusCode ?? 0;
    final data = res.data;
    if (status >= 200 && status < 300) return data;
    var msg = 'HTTP $status';
    if (data is Map && data['error'] != null) {
      msg = data['error'].toString();
    } else if (data is String && data.trim().isNotEmpty && data.length < 200) {
      msg = data.trim();
    }
    if (status == 401 && path != '/login' && path != '/signup') {
      onUnauthorized?.call();
    }
    throw ApiException(msg, status: status);
  }

  String _networkMessage(DioException e) {
    switch (e.type) {
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.receiveTimeout:
      case DioExceptionType.sendTimeout:
        return 'The server took too long to respond ($baseUrl).';
      case DioExceptionType.badCertificate:
        return 'The server certificate is not trusted ($baseUrl).';
      case DioExceptionType.connectionError:
        return "Can't reach $baseUrl. Check the server address and your connection.";
      default:
        return e.message ?? 'Network error';
    }
  }

  Future<dynamic> _get(String path, [Map<String, dynamic>? query]) => _send('GET', path, query: query);
  Future<dynamic> _post(String path, [Object? body]) => _send('POST', path, body: body ?? const <String, dynamic>{});
  Future<dynamic> _patch(String path, Object body) => _send('PATCH', path, body: body);
  Future<dynamic> _delete(String path) => _send('DELETE', path);

  // ------------------------------------------------------------ auth

  Future<bool> ping() async {
    final d = await _get('/ping');
    return d is Map && d['ok'] == true;
  }

  Future<Me> me() async => Me.fromJson(asJson(await _get('/me')));

  /// The backend accepts the member email as `username`.
  Future<void> login(String username, String password) =>
      _post('/login', {'username': username.trim(), 'password': password});

  Future<void> signup(String email, String password, String displayName) =>
      _post('/signup', {'email': email.trim(), 'password': password, 'display_name': displayName.trim()});

  Future<void> logout() async {
    try {
      await _post('/logout');
    } finally {
      await cookieJar.deleteAll();
    }
  }

  Future<FxRates> fxRates() async => FxRates.fromJson(asJson(await _get('/fx-rates')));

  Future<SetupData> setup() async => SetupData.fromJson(asJson(await _get('/setup')));

  // ------------------------------------------------------------ reports

  Future<MonthReport> report(int year, int month) async =>
      MonthReport.fromJson(asJson(await _get('/report/$year/$month')));

  Future<MonthCompare> monthCompare(int year, int month) async =>
      MonthCompare.fromJson(asJson(await _get('/report/month-compare/$year/$month')));

  // ------------------------------------------------------------ spending

  Future<SpendingPage> spending({
    int? year,
    int? month,
    int page = 1,
    int perPage = 50,
    String? search,
    String? category,
  }) async {
    final q = <String, dynamic>{'page': page, 'per_page': perPage};
    if (year != null) q['year'] = year;
    if (month != null) q['month'] = month;
    if (search != null && search.isNotEmpty) q['search'] = search;
    if (category != null && category.isNotEmpty) q['category'] = category;
    return SpendingPage.fromJson(asJson(await _get('/spending', q)));
  }

  /// POST /api/spending. [category] is the category *name* (as the web sends).
  Future<int> addSpending({
    required String txDate,
    required String description,
    required String category,
    required int amount,
    String? clientId,
  }) async {
    final d = await _post('/spending', {
      'tx_date': txDate,
      'description': description,
      'category': category,
      'amount': amount,
      'source': AppConfig.spendingSource,
      if (clientId != null) 'client_id': clientId,
    });
    return asInt(asJson(d)['id']);
  }

  Future<void> updateSpending(int id, Map<String, dynamic> fields) => _patch('/spending/$id', fields);

  Future<void> deleteSpending(int id) => _delete('/spending/$id');

  // ------------------------------------------------------------ income

  Future<IncomeList> income(int year, int month) async =>
      IncomeList.fromJson(asJson(await _get('/income', {'year': year, 'month': month})));

  Future<int> addIncome({
    required String txDate,
    required int amount,
    int? sourceId,
    String description = '',
    String? clientId,
  }) async {
    final d = await _post('/income', {
      'tx_date': txDate,
      'amount': amount,
      'source_id': sourceId,
      'description': description,
      if (clientId != null) 'client_id': clientId,
    });
    return asInt(asJson(d)['id']);
  }

  Future<void> deleteIncome(int id) => _delete('/income/$id');

  // ------------------------------------------------------------ budgeting

  Future<Budgeting> budgeting(int year) async => Budgeting.fromJson(asJson(await _get('/budgeting/$year')));

  Future<BudgetActuals> budgetActuals(int year) async =>
      BudgetActuals.fromJson(asJson(await _get('/budgeting/$year/actuals')));

  Future<void> setBudgetIncome({required int year, required int month, required int sourceId, required int amount}) =>
      _post('/budgeting/income', {'year': year, 'month': month, 'source_id': sourceId, 'amount': amount});

  Future<void> setBudgetExpense({required int year, required int month, required int categoryId, required int amount}) =>
      _post('/budgeting/expense', {'year': year, 'month': month, 'category_id': categoryId, 'amount': amount});

  // ------------------------------------------------------------ goals

  Future<List<Goal>> goals() async => asList(await _get('/goals'), Goal.fromJson);

  Future<void> addGoal({required String name, required int targetAmount, String? targetDate, String emoji = ''}) =>
      _post('/goals', {
        'name': name,
        'target_amount': targetAmount,
        'target_date': (targetDate == null || targetDate.isEmpty) ? null : targetDate,
        'saved_amount': 0,
        'emoji': emoji,
      });

  /// Positive adds, negative withdraws (never below zero server-side).
  Future<int> contributeGoal(int id, int amount) async {
    final d = await _post('/goals/$id/contribute', {'amount': amount});
    return asInt(asJson(d)['saved_amount']);
  }

  Future<void> deleteGoal(int id) => _delete('/goals/$id');

  // ------------------------------------------------------------ family / gamification

  Future<FamilyData> family() async => FamilyData.fromJson(asJson(await _get('/family')));

  Future<Gamification> gamification() async => Gamification.fromJson(asJson(await _get('/gamification')));

  // ------------------------------------------------------------ household (Today / chores / lists / reminders)

  /// GET /api/today - greeting + household sections (reminders, chores,
  /// lists, budget...). Returns null when the server doesn't have it (404).
  Future<TodayData?> today() async {
    try {
      return TodayData.fromJson(await _get('/today'));
    } on ApiException catch (e) {
      if (e.isNotFound) return null;
      rethrow;
    }
  }

  // chores - actions take the chore *instance* id.
  Future<ChoresToday> choresToday() async => ChoresToday.fromJson(asJson(await _get('/chores/today')));

  /// action: complete | approve | reject | undo. Returns the server message.
  Future<String> choreAction(int instanceId, String action, {String note = ''}) async {
    final d = await _post('/chores/$instanceId/$action', {'note': note});
    return asString(asJson(d)['message']);
  }

  // reminders
  Future<List<ReminderOccurrence>> upcomingReminders({int days = 7}) async =>
      asList(asJson(await _get('/reminders/upcoming', {'days': days}))['items'], ReminderOccurrence.fromJson);

  Future<void> reminderDone(int occurrenceId) => _post('/reminders/occurrences/$occurrenceId/done');

  Future<void> reminderSnooze(int occurrenceId, {int minutes = 60}) =>
      _post('/reminders/occurrences/$occurrenceId/snooze', {'minutes': minutes});

  /// Snooze until the same time tomorrow (household time zone).
  Future<void> reminderSnoozeTomorrow(int occurrenceId) =>
      _post('/reminders/occurrences/$occurrenceId/snooze', {'until': 'tomorrow'});

  /// POST /api/reminders. [startDate] YYYY-MM-DD, [timeOfDay] HH:MM (household time zone).
  Future<void> createReminder({
    required String title,
    required String kind,
    required String startDate,
    required String timeOfDay,
    String category = 'other',
    String notes = '',
    List<int>? weekdays,
    int? dayOfMonth,
    int interval = 1,
    int? assigneeId,
  }) =>
      _post('/reminders', {
        'title': title,
        'kind': kind,
        'start_date': startDate,
        'time_of_day': timeOfDay,
        'category': category,
        'notes': notes,
        if (weekdays != null) 'weekdays': weekdays,
        if (dayOfMonth != null) 'day_of_month': dayOfMonth,
        // every N weeks/months (weekly/monthly only; the server forces 1 otherwise)
        'interval': interval < 1 ? 1 : interval,
        if (assigneeId != null) 'assignee_id': assigneeId,
      });

  Future<List<Reminder>> reminders() async =>
      asList(asJson(await _get('/reminders'))['reminders'], Reminder.fromJson);

  Future<void> setReminderActive(int id, bool active) => _patch('/reminders/$id', {'active': active});

  Future<void> deleteReminder(int id) => _delete('/reminders/$id');

  // shopping lists
  Future<List<ShoppingList>> lists() async => asList(asJson(await _get('/lists'))['lists'], ShoppingList.fromJson);

  Future<void> createList(String name, {String emoji = ''}) => _post('/lists', {'name': name, 'emoji': emoji});

  Future<ListDetail> listDetail(int id) async => ListDetail.fromJson(asJson(await _get('/lists/$id')));

  /// Free text; the server splits "milk, 2 eggs" into items.
  Future<void> addListItems(int listId, String text) => _post('/lists/$listId/items', {'text': text});

  Future<void> setListItemChecked(int listId, int itemId, bool checked) =>
      _patch('/lists/$listId/items/$itemId', {'checked': checked});

  Future<void> deleteListItem(int listId, int itemId) => _delete('/lists/$listId/items/$itemId');

  Future<void> clearChecked(int listId) => _post('/lists/$listId/clear-checked');

  /// POST /api/lists/{id}/finish: logs the trip as one spending entry and
  /// removes the ticked items. [category] empty = server default.
  Future<int> finishShopping(int listId, {required int totalAmount, String category = '', String description = ''}) async {
    final d = await _post('/lists/$listId/finish', {
      'client_id': 'm-finish-$listId-${DateTime.now().microsecondsSinceEpoch}',
      'total_amount': totalAmount,
      if (category.isNotEmpty) 'category': category,
      if (description.isNotEmpty) 'description': description,
    });
    return asInt(asJson(d)['spending_id']);
  }

  // ------------------------------------------------------------ meal plan (backend-go/meal_plan.go)

  /// GET /api/meal-plan: next week's menu, or this week's with [thisWeek].
  Future<MealPlan> mealPlan({bool thisWeek = false}) async =>
      MealPlan.fromJson(asJson(asJson(await _get('/meal-plan', thisWeek ? {'week': 'this'} : null))['plan']));

  Future<List<MealIngredient>> mealIngredients() async =>
      asList(asJson(await _get('/meal-plan/ingredients'))['items'], MealIngredient.fromJson);

  Future<void> addMealIngredient(String name) => _post('/meal-plan/ingredients', {'name': name.trim()});

  Future<void> deleteMealIngredient(int id) => _delete('/meal-plan/ingredients/$id');

  // ------------------------------------------------------------ calendar (activities + reminders overlay)

  /// Dates are YYYY-MM-DD; [include] is "activities", "reminders" or null (both).
  Future<CalendarData> calendar(String from, String to, {String? include}) async =>
      CalendarData.fromJson(asJson(await _get('/calendar', {'from': from, 'to': to, if (include != null) 'include': include})));

  Future<void> createActivity(Map<String, dynamic> body) => _post('/activities', body);

  Future<void> updateActivity(int id, Map<String, dynamic> body) => _patch('/activities/$id', body);

  /// With [on] (YYYY-MM-DD) only that occurrence of a repeating activity is removed.
  Future<void> deleteActivity(int id, {String? on}) => _send('DELETE', '/activities/$id', query: on != null ? {'on': on} : null);

  // ------------------------------------------------------------ meal requests

  Future<List<MealProposal>> mealProposals() async => asList(asJson(await _get('/meal-plan/proposals'))['items'], MealProposal.fromJson);

  Future<void> proposeMeal(String dish, {String note = '', String? wantedDate}) =>
      _post('/meal-plan/proposals', {'dish': dish.trim(), 'note': note.trim(), 'wanted_date': wantedDate});

  Future<void> approveMeal(int id, {String? mealDate}) => _post('/meal-plan/proposals/$id/approve', {'meal_date': mealDate});

  Future<void> declineMeal(int id, {String note = ''}) => _post('/meal-plan/proposals/$id/decline', {'note': note});

  Future<void> deleteMealProposal(int id) => _delete('/meal-plan/proposals/$id');

  // ------------------------------------------------------------ widget boards

  Future<WidgetCatalog> widgetCatalog() async => WidgetCatalog.fromJson(asJson(await _get('/widgets/catalog')));

  Future<List<WidgetBoard>> widgetBoards() async => asList(asJson(await _get('/widgets/boards'))['boards'], WidgetBoard.fromJson);

  Future<void> createWidgetBoard({String? preset, String? name, List<WidgetItem> widgets = const []}) => _post('/widgets/boards', {
        if (preset != null) 'preset': preset,
        if (name != null) 'name': name,
        if (preset == null) 'layout': WidgetBoard.layoutJson(widgets),
      });

  /// Saving a layout replaces it: pass the board's `background` (null/'' clears it)
  /// and `layoutExtra` (its other top-level keys) so nothing is lost.
  Future<void> updateWidgetBoard(int id, {String? name, List<WidgetItem>? widgets, String? background, Json layoutExtra = const {}}) =>
      _patch('/widgets/boards/$id', {
        if (name != null) 'name': name,
        if (widgets != null) 'layout': WidgetBoard.layoutJson(widgets, background: background, extra: layoutExtra),
      });

  Future<void> deleteWidgetBoard(int id) => _delete('/widgets/boards/$id');

  Future<WidgetData> widgetData(List<String> types) async =>
      WidgetData.fromJson(asJson(await _get('/widgets/data', types.isEmpty ? null : {'types': types.join(',')})));
}
