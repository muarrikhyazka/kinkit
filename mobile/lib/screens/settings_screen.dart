import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/providers.dart';
import '../theme.dart';
import '../util/money.dart';
import '../widgets/async_view.dart';

class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  Future<void> _changeServer(BuildContext context, WidgetRef ref) async {
    final ctrl = TextEditingController(text: ref.read(baseUrlProvider));
    final url = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Server address'),
        content: TextField(
          controller: ctrl,
          keyboardType: TextInputType.url,
          autocorrect: false,
          decoration: const InputDecoration(hintText: 'https://kinkit.example.com'),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
          FilledButton(onPressed: () => Navigator.pop(ctx, ctrl.text), child: const Text('Save')),
        ],
      ),
    );
    if (url == null || url.trim().isEmpty) return;
    if (!context.mounted) return;
    final ok = await confirmDialog(context,
        title: 'Switch server?', message: 'You will need to log in on the new server.', confirm: 'Switch');
    if (!ok) return;
    // The session re-check against the new server drops us to the login
    // screen unless that server already has a valid cookie.
    await ref.read(baseUrlProvider.notifier).set(url);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final me = ref.watch(meProvider);
    final server = ref.watch(baseUrlProvider);
    final mode = ref.watch(themeModeProvider);
    final p = context.palette;
    final currency = me?.currency ?? 'IDR';
    final meta = currencyMeta[currency];

    return Scaffold(
      appBar: AppBar(title: const Text('Settings')),
      body: ListView(
        children: [
          const SectionTitle('Appearance'),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: SegmentedButton<ThemeMode>(
                segments: const [
                  ButtonSegment(value: ThemeMode.system, label: Text('System'), icon: Icon(Icons.brightness_auto)),
                  ButtonSegment(value: ThemeMode.light, label: Text('Light'), icon: Icon(Icons.light_mode)),
                  ButtonSegment(value: ThemeMode.dark, label: Text('Dark'), icon: Icon(Icons.dark_mode)),
                ],
                selected: {mode},
                onSelectionChanged: (s) => ref.read(themeModeProvider.notifier).set(s.first),
              ),
            ),
          ),
          const SectionTitle('Family'),
          Card(
            child: Column(
              children: [
                ListTile(
                  leading: const Icon(Icons.family_restroom),
                  title: const Text('Family name'),
                  subtitle: Text(
                    (me?.householdName.trim().isNotEmpty ?? false) ? me!.householdName : 'Not set',
                    style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600, color: p.text),
                  ),
                  trailing: (me?.role == 'owner')
                      ? Tooltip(
                          message: 'Rename your family on the web (Settings)',
                          child: Icon(Icons.open_in_browser, color: p.muted),
                        )
                      : null,
                ),
                ListTile(
                  leading: const Icon(Icons.currency_exchange),
                  title: const Text('Currency'),
                  subtitle: Text('$currency${meta != null ? ' (${meta.symbol})' : ''} · amounts are entered in IDR '
                      'and converted for display'),
                ),
                ListTile(
                  leading: const Icon(Icons.open_in_browser),
                  title: const Text('Categories, members, bots, import/export'),
                  subtitle: Text('Manage these on the web at $server', style: TextStyle(color: p.muted)),
                ),
              ],
            ),
          ),
          const SectionTitle('Connection'),
          Card(
            child: ListTile(
              leading: const Icon(Icons.dns_outlined),
              title: const Text('Server'),
              subtitle: Text(server),
              trailing: const Icon(Icons.edit_outlined),
              onTap: () => _changeServer(context, ref),
            ),
          ),
          const SizedBox(height: 24),
          Center(child: Text('Kinkit mobile 1.0.0', style: TextStyle(color: p.muted, fontSize: 12))),
          const SizedBox(height: 24),
        ],
      ),
    );
  }
}
