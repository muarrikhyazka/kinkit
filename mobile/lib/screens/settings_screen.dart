import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:package_info_plus/package_info_plus.dart';

import '../api/api_client.dart';
import '../capture/capture_channel.dart';
import '../capture/capture_providers.dart';
import '../game/nav_mode.dart';
import '../state/data_providers.dart';
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
          const _NavigationSection(),
          if (captureSupportedPlatform) const _ExpenseCaptureSection(),
          const SectionTitle('Family'),
          Card(
            child: Column(
              children: [
                ListTile(
                  leading: const Icon(Icons.family_restroom),
                  title: const Text('Family name'),
                  isThreeLine: me?.role == 'owner',
                  subtitle: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        (me?.householdName.trim().isNotEmpty ?? false) ? me!.householdName : 'Not set',
                        style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600, color: p.text),
                      ),
                      if (me?.role == 'owner')
                        Text('Rename it on the web under Settings', style: TextStyle(color: p.muted)),
                    ],
                  ),
                ),
                ListTile(
                  leading: const Icon(Icons.currency_exchange),
                  title: const Text('Currency'),
                  subtitle: Text('$currency${meta != null ? ' (${meta.symbol})' : ''} · amounts are entered in IDR '
                      'and converted for display'),
                ),
                ListTile(
                  leading: const Icon(Icons.laptop_outlined),
                  title: const Text('Categories, adding members, bots, import/export'),
                  subtitle: Text('Manage these on the web at $server', style: TextStyle(color: p.muted)),
                ),
              ],
            ),
          ),
          const _MembersSection(),
          const _AccountSection(),
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
          const _VersionLine(),
          const SizedBox(height: 24),
        ],
      ),
    );
  }
}

/// "Kinkit 1.3.0" read from the installed build, so it never drifts from
/// pubspec.yaml. Shows nothing while loading or when the platform plugin is
/// missing (widget tests).
class _VersionLine extends StatelessWidget {
  const _VersionLine();

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    return FutureBuilder<PackageInfo>(
      future: PackageInfo.fromPlatform(),
      builder: (context, snap) {
        final info = snap.data;
        if (info == null) return const SizedBox.shrink();
        return Center(child: Text('Kinkit ${info.version}', style: TextStyle(color: p.muted, fontSize: 12)));
      },
    );
  }
}

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
  return role.isEmpty ? role : role[0].toUpperCase() + role.substring(1);
}

/// Read-only household list (web: Settings > Members). Adding, editing and
/// removing members and allowances stay on the web.
class _MembersSection extends ConsumerWidget {
  const _MembersSection();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final family = ref.watch(familyProvider);
    final money = ref.watch(moneyProvider);
    final canSeeMoney = ref.watch(meProvider)?.canReadMoney ?? false;
    final p = context.palette;
    final count = family.valueOrNull?.members.length;

    Widget body;
    if (family.hasValue) {
      final members = family.requireValue.members;
      body = Column(
        children: [
          for (final m in members)
            ListTile(
              leading: CircleAvatar(
                backgroundColor: p.primaryLight,
                child: Text(
                  m.displayName.isEmpty ? '?' : m.displayName.substring(0, 1).toUpperCase(),
                  style: TextStyle(color: p.primary, fontWeight: FontWeight.w700),
                ),
              ),
              title: Row(
                children: [
                  Flexible(child: Text(m.displayName, overflow: TextOverflow.ellipsis)),
                  if (m.isMe) Text('  (you)', style: TextStyle(color: p.muted, fontSize: 12)),
                ],
              ),
              subtitle: Text([
                _roleLabel(m.role),
                if (canSeeMoney) 'spent ${money.format(m.spentThisMonth)}',
                if (m.allowanceMonthly != null && canSeeMoney) 'allowance ${money.format(m.allowanceMonthly!)}',
              ].join(' · ')),
            ),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 4, 16, 14),
            child: Align(
              alignment: Alignment.centerLeft,
              child: Text(
                'Add, edit or remove members, and set allowances, on the web under Settings › Members.',
                style: TextStyle(color: p.muted),
              ),
            ),
          ),
        ],
      );
    } else if (family.hasError) {
      body = Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text("Couldn't load the members. ${family.error}", style: TextStyle(color: p.text)),
            const SizedBox(height: 8),
            OutlinedButton.icon(
              style: OutlinedButton.styleFrom(minimumSize: const Size(64, 48)),
              onPressed: () => ref.invalidate(familyProvider),
              icon: const Icon(Icons.refresh),
              label: const Text('Try again'),
            ),
          ],
        ),
      );
    } else {
      body = const Padding(
        padding: EdgeInsets.all(20),
        child: Center(
          child: SizedBox(
            width: 24,
            height: 24,
            child: CircularProgressIndicator(strokeWidth: 3, semanticsLabel: 'Loading members'),
          ),
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SectionTitle(count == null ? 'Members' : 'Members ($count)'),
        Card(child: body),
      ],
    );
  }
}

/// Who you are signed in as, plus your own password (web: Settings > Your account).
class _AccountSection extends ConsumerWidget {
  const _AccountSection();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final me = ref.watch(meProvider);
    final p = context.palette;
    final who = me == null ? '' : (me.displayName.isEmpty ? me.email : me.displayName);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SectionTitle('Your account'),
        Card(
          child: Column(
            children: [
              ListTile(
                leading: const Icon(Icons.account_circle_outlined),
                title: Text('Signed in as $who'),
                subtitle: Text(
                  [if (me != null && me.email.isNotEmpty) me.email, if (me != null) _roleLabel(me.role)].join(' · '),
                  style: TextStyle(color: p.muted),
                ),
              ),
              ListTile(
                minVerticalPadding: 12,
                leading: const Icon(Icons.lock_outline),
                title: const Text('Change password'),
                trailing: const Icon(Icons.chevron_right),
                onTap: () async {
                  final changed = await showDialog<bool>(
                    context: context,
                    builder: (_) => const _ChangePasswordDialog(),
                  );
                  if (changed == true && context.mounted) showSnack(context, 'Password changed');
                },
              ),
            ],
          ),
        ),
      ],
    );
  }
}

/// Current + new password. A wrong current password is a 401 from the API;
/// [KinkitApi] does not treat that as a lost session, so the error shows here.
class _ChangePasswordDialog extends ConsumerStatefulWidget {
  const _ChangePasswordDialog();

  @override
  ConsumerState<_ChangePasswordDialog> createState() => _ChangePasswordDialogState();
}

class _ChangePasswordDialogState extends ConsumerState<_ChangePasswordDialog> {
  final _current = TextEditingController();
  final _next = TextEditingController();
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _current.dispose();
    _next.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_busy) return;
    if (_current.text.isEmpty) {
      setState(() => _error = 'Enter your current password.');
      return;
    }
    if (_next.text.length < 8) {
      setState(() => _error = 'The new password needs at least 8 characters.');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(apiProvider).changePassword(_current.text, _next.text);
      if (mounted) Navigator.pop(context, true);
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = e.isUnauthorized
            ? 'Your current password is wrong. Check it and try again.'
            : (e.isNetwork ? '${e.message} Your password was not changed.' : e.message);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    return AlertDialog(
      title: const Text('Change password'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            TextField(
              controller: _current,
              obscureText: true,
              autofocus: true,
              enabled: !_busy,
              autofillHints: const [AutofillHints.password],
              textInputAction: TextInputAction.next,
              decoration: const InputDecoration(labelText: 'Current password'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _next,
              obscureText: true,
              enabled: !_busy,
              autofillHints: const [AutofillHints.newPassword],
              textInputAction: TextInputAction.done,
              onSubmitted: (_) => _submit(),
              decoration: const InputDecoration(labelText: 'New password', helperText: 'At least 8 characters'),
            ),
            if (_error != null) ...[
              const SizedBox(height: 12),
              Semantics(
                liveRegion: true,
                child: Text(_error!, style: TextStyle(color: p.danger)),
              ),
            ],
          ],
        ),
      ),
      actions: [
        TextButton(
          style: TextButton.styleFrom(minimumSize: const Size(64, 48)),
          onPressed: _busy ? null : () => Navigator.pop(context, false),
          child: const Text('Cancel'),
        ),
        FilledButton(
          style: FilledButton.styleFrom(minimumSize: const Size(64, 48)),
          onPressed: _busy ? null : _submit,
          child: _busy
              ? SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(strokeWidth: 2, color: p.onPrimary, semanticsLabel: 'Changing password'),
                )
              : const Text('Change password'),
        ),
      ],
    );
  }
}

/// Tabs (Today first) or the Town map in place of the Today tab.
class _NavigationSection extends ConsumerWidget {
  const _NavigationSection();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final mode = ref.watch(navModeProvider);
    final p = context.palette;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SectionTitle('Navigation'),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                SegmentedButton<NavMode>(
                  style: const ButtonStyle(minimumSize: WidgetStatePropertyAll(Size(48, 44))),
                  segments: const [
                    ButtonSegment(value: NavMode.tabs, label: Text('Tabs'), icon: Icon(Icons.view_agenda_outlined)),
                    ButtonSegment(value: NavMode.townMap, label: Text('Town map'), icon: Icon(Icons.map_outlined)),
                  ],
                  selected: {mode},
                  onSelectionChanged: (s) => ref.read(navModeProvider.notifier).set(s.first),
                ),
                const SizedBox(height: 8),
                Text(
                  'Town map swaps the Today tab for a tab called Town: a small town where you walk Kiki to a door or tap a building to open a page.',
                  style: TextStyle(color: p.muted),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}

/// Android only: collect payment notifications for review (Captured expenses).
class _ExpenseCaptureSection extends ConsumerStatefulWidget {
  const _ExpenseCaptureSection();

  @override
  ConsumerState<_ExpenseCaptureSection> createState() => _ExpenseCaptureSectionState();
}

class _ExpenseCaptureSectionState extends ConsumerState<_ExpenseCaptureSection> with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // The user may have just flipped the switch in Android's own settings.
    if (state == AppLifecycleState.resumed) ref.invalidate(captureStatusProvider);
  }

  Future<void> _setEnabled(bool on) async {
    try {
      await ref.read(captureChannelProvider).setEnabled(on);
    } on CaptureException catch (e) {
      if (mounted) showSnack(context, 'Capture is still ${on ? 'off' : 'on'}. $e', error: true);
    }
    if (!mounted) return;
    ref.invalidate(captureStatusProvider);
    ref.invalidate(pendingCapturesProvider);
  }

  Future<void> _openAccess() async {
    final opened = await ref.read(captureChannelProvider).openSettings();
    if (!opened && mounted) {
      showSnack(context, 'Could not open notification access. Open Android Settings and search for "notification access".',
          error: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final status = ref.watch(captureStatusProvider);
    final st = status.valueOrNull;
    final p = context.palette;
    final enabled = st?.enabled ?? false;
    final access = st?.permission ?? false;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SectionTitle('Expense capture'),
        Card(
          child: Column(
            children: [
              SwitchListTile(
                secondary: const Icon(Icons.notifications_active_outlined),
                title: const Text('Capture expenses from notifications'),
                subtitle: const Text(
                    'Spots payment notifications from bank and e-wallet apps. You review each one before it is saved.'),
                value: enabled,
                onChanged: st == null ? null : _setEnabled,
              ),
              ListTile(
                leading: Icon(access ? Icons.check_circle_outline : Icons.error_outline, color: p.muted),
                title: Text(st == null
                    ? (status.hasError ? 'Notification access: could not check' : 'Notification access: checking')
                    : (access ? 'Notification access: on' : 'Notification access: needed')),
                subtitle: Text(status.hasError && st == null
                    ? '${status.error} Tap to try again.'
                    : (access
                        ? 'Android lets Kinkit read notifications.'
                        : 'Kinkit sees no notifications until you allow it in Android settings.')),
                trailing: status.hasError && st == null ? const Icon(Icons.refresh) : null,
                onTap: status.hasError && st == null ? () => ref.invalidate(captureStatusProvider) : null,
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: OutlinedButton.icon(
                    onPressed: _openAccess,
                    icon: const Icon(Icons.open_in_new),
                    label: const Text('Open notification access'),
                  ),
                ),
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                child: Text(
                  'Notification text stays on this phone. Only an expense you review and save is sent to Kinkit. '
                  'If Android says the setting is restricted: App info > menu > Allow restricted settings.',
                  style: TextStyle(color: p.muted),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
