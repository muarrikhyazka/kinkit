import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/providers.dart';
import '../theme.dart';
import '../widgets/quick_add_sheet.dart';
import 'family_screen.dart';
import 'household_screen.dart';
import 'money_screen.dart';
import 'more_screen.dart';
import 'today_screen.dart';

/// Bottom navigation mirroring the web nav (routes.jsx):
/// Today · Home (reminders/chores/lists) · Money · Family · More (settings).
class HomeShell extends ConsumerStatefulWidget {
  const HomeShell({super.key});

  @override
  ConsumerState<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends ConsumerState<HomeShell> {
  int _index = 0;

  static const _tabs = <_Tab>[
    _Tab('Today', Icons.wb_sunny_outlined, Icons.wb_sunny),
    _Tab('Home', Icons.home_outlined, Icons.home),
    _Tab('Money', Icons.account_balance_wallet_outlined, Icons.account_balance_wallet),
    _Tab('Family', Icons.family_restroom_outlined, Icons.family_restroom),
    _Tab('More', Icons.menu, Icons.menu),
  ];

  @override
  Widget build(BuildContext context) {
    final me = ref.watch(meProvider);
    final canAdd = me?.canAddSpending ?? false;
    // Quick add on Today and Money (Home/Family have their own actions).
    final showFab = canAdd && (_index == 0 || _index == 2);
    return Scaffold(
      body: IndexedStack(
        index: _index,
        children: const [
          TodayScreen(),
          HouseholdScreen(),
          MoneyScreen(),
          FamilyScreen(),
          MoreScreen(),
        ],
      ),
      floatingActionButton: showFab
          ? FloatingActionButton(
              heroTag: 'quick-add',
              tooltip: 'Quick add',
              onPressed: () => showQuickAdd(context, allowIncome: me?.canManage ?? false),
              child: const Icon(Icons.add),
            )
          : null,
      // White bar with a hairline and purple selection (web chrome counterpart).
      bottomNavigationBar: DecoratedBox(
        decoration: BoxDecoration(border: Border(top: BorderSide(color: context.palette.border))),
        child: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: (i) => setState(() => _index = i),
        destinations: [
          for (final t in _tabs)
            NavigationDestination(icon: Icon(t.icon), selectedIcon: Icon(t.selectedIcon), label: t.label),
        ],
        ),
      ),
    );
  }
}

class _Tab {
  final String label;
  final IconData icon;
  final IconData selectedIcon;
  const _Tab(this.label, this.icon, this.selectedIcon);
}
