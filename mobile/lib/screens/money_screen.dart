import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/providers.dart';
import '../widgets/k_app_bar.dart';
import 'budget_screen.dart';
import 'dashboard_screen.dart';
import 'goals_screen.dart';
import 'income_screen.dart';
import 'spending_screen.dart';

/// Money section (web nav "Money": Dashboard, Transactions, Budget, Savings).
/// Kids see Dashboard + Spending only (routes.jsx READERS rule).
class MoneyScreen extends ConsumerWidget {
  const MoneyScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final readMoney = ref.watch(meProvider)?.canReadMoney ?? false;
    final tabs = <(String, Widget)>[
      ('Dashboard', const DashboardScreen(embedded: true)),
      ('Spending', const SpendingScreen()),
      if (readMoney) ('Income', const IncomeScreen()),
      if (readMoney) ('Budget', const BudgetScreen(embedded: true)),
      if (readMoney) ('Goals', const GoalsScreen(embedded: true)),
    ];
    return DefaultTabController(
      key: ValueKey(tabs.length),
      length: tabs.length,
      child: Scaffold(
        appBar: kAppBar(
          context,
          'Money',
          bottom: TabBar(
            isScrollable: true,
            tabAlignment: TabAlignment.start,
            tabs: [for (final t in tabs) Tab(text: t.$1)],
          ),
        ),
        body: TabBarView(children: [for (final t in tabs) t.$2]),
      ),
    );
  }
}
