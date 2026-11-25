import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../capture/capture_channel.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../widgets/async_view.dart';
import '../widgets/k_app_bar.dart';
import 'calendar_page.dart';
import 'captured_expenses_screen.dart';
import 'chores_screen.dart';
import 'family_screen.dart';
import 'lists_screen.dart';
import 'meals_screen.dart';
import 'settings_screen.dart';
import 'widgets_screen.dart';

String _roleLabel(String role) {
  switch (role) {
    case 'owner':
      return 'Family admin';
    case 'parent':
      return 'Parent';
    case 'kid':
      return 'Kid';
    case 'viewer':
      return 'Viewer';
  }
  return role;
}

/// "More": profile, the Family group (Leaderboard, Chores, Lists, Meals,
/// Widgets, Calendar), settings and logout (web sidebar "Family" group /
/// "Settings"). On Android it also lists captured expenses waiting for review.
class MoreScreen extends ConsumerWidget {
  const MoreScreen({super.key});

  void _open(BuildContext context, Widget screen) =>
      Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => screen));

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final me = ref.watch(meProvider);
    final p = context.palette;

    return Scaffold(
      appBar: kAppBar(context, 'More'),
      body: ListView(
        children: [
          if (me != null && me.householdName.trim().isNotEmpty)
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 12, 20, 4),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Family name', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: p.muted)),
                  const SizedBox(height: 2),
                  Text(me.householdName, style: TextStyle(fontSize: 24, fontWeight: FontWeight.w700, color: p.text)),
                ],
              ),
            ),
          if (me != null)
            Card(
              child: ListTile(
                leading: CircleAvatar(
                  backgroundColor: p.primaryLight,
                  child: Text(me.displayName.isEmpty ? '?' : me.displayName.substring(0, 1).toUpperCase(),
                      style: TextStyle(color: p.primary, fontWeight: FontWeight.w700)),
                ),
                title: Text(me.displayName),
                subtitle: Text('${me.email}\n${_roleLabel(me.role)}'),
                isThreeLine: true,
              ),
            ),
          const SectionTitle('Family'),
          Card(
            child: Column(
              children: [
                ListTile(
                  leading: const Icon(Icons.family_restroom_outlined),
                  title: const Text('Leaderboard'),
                  subtitle: const Text('Points, levels and badges'),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => _open(context, const FamilyScreen()),
                ),
                ListTile(
                  leading: const Icon(Icons.checklist_outlined),
                  title: const Text('Chores'),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => _open(context, const ChoresScreen()),
                ),
                ListTile(
                  leading: const Icon(Icons.list_alt_outlined),
                  title: const Text('Lists'),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => _open(context, const ListsScreen()),
                ),
                ListTile(
                  leading: const Icon(Icons.restaurant_outlined),
                  title: const Text('Meals'),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => _open(context, const MealsScreen()),
                ),
                ListTile(
                  leading: const Icon(Icons.dashboard_customize_outlined),
                  title: const Text('Widgets'),
                  subtitle: const Text('Your own screens: clock, agenda, reminders…'),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => _open(context, const WidgetsScreen()),
                ),
                ListTile(
                  leading: const Icon(Icons.calendar_month_outlined),
                  title: const Text('Calendar'),
                  subtitle: const Text('Activities and reminders'),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => _open(context, const CalendarScreen()),
                ),
              ],
            ),
          ),
          const SectionTitle('App'),
          Card(
            child: Column(
              children: [
                if (captureSupportedPlatform) const CapturedExpensesTile(),
                ListTile(
                  leading: const Icon(Icons.settings_outlined),
                  title: const Text('Settings'),
                  subtitle: const Text('Members, your password, appearance, server'),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => _open(context, const SettingsScreen()),
                ),
                ListTile(
                  leading: Icon(Icons.logout, color: p.danger),
                  title: Text('Log out', style: TextStyle(color: p.danger)),
                  onTap: () async {
                    final ok = await confirmDialog(context, title: 'Log out?', confirm: 'Log out');
                    if (ok) await ref.read(authProvider.notifier).logout();
                  },
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
