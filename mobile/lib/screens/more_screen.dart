import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/providers.dart';
import '../theme.dart';
import '../widgets/async_view.dart';
import '../widgets/k_app_bar.dart';
import 'settings_screen.dart';

String _roleLabel(String role) {
  switch (role) {
    case 'owner':
      return 'Family Admin';
    case 'parent':
      return 'Parent';
    case 'kid':
      return 'Kid';
    case 'viewer':
      return 'Viewer';
  }
  return role;
}

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
          if (me != null && me.householdName.trim().isNotEmpty)
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 12, 20, 4),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('FAMILY NAME',
                      style: TextStyle(fontSize: 11, letterSpacing: 1.2, fontWeight: FontWeight.w700, color: p.muted)),
                  const SizedBox(height: 2),
                  Text(me.householdName, style: serifStyle(fontSize: 26, color: p.text)),
                ],
              ),
            ),
          if (me != null)
            Card(
              child: ListTile(
                leading: CircleAvatar(
                  backgroundColor: p.accent,
                  child: Text(me.displayName.isEmpty ? '?' : me.displayName.substring(0, 1).toUpperCase(),
                      style: serifStyle(color: p.onAccent)),
                ),
                title: Text(me.displayName),
                subtitle: Text('${me.email}\n${_roleLabel(me.role)}'),
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
