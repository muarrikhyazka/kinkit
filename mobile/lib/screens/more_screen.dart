import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/providers.dart';
import '../theme.dart';
import '../widgets/async_view.dart';
import '../widgets/k_app_bar.dart';
import 'settings_screen.dart';

/// "More": profile, settings and logout (web nav "Settings").
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
          if (me != null)
            Card(
              child: ListTile(
                leading: CircleAvatar(
                  backgroundColor: p.primaryLight,
                  child: Text(me.displayName.isEmpty ? '?' : me.displayName.substring(0, 1).toUpperCase(),
                      style: TextStyle(color: p.primary, fontWeight: FontWeight.w700)),
                ),
                title: Text(me.displayName),
                subtitle: Text('${me.email}\n${me.householdName} · ${me.role}'),
                isThreeLine: true,
              ),
            ),
          const SectionTitle('App'),
          Card(
            child: Column(
              children: [
                ListTile(
                  leading: const Icon(Icons.settings_outlined),
                  title: const Text('Settings'),
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
