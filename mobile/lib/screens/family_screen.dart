import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../widgets/async_view.dart';
import '../widgets/gamification_badge.dart';

/// Household members + achievements (web: pages/Family.jsx). Read-only;
/// member management stays on the web.
class FamilyScreen extends ConsumerWidget {
  const FamilyScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final family = ref.watch(familyProvider);
    final gami = ref.watch(gamificationProvider);
    final money = ref.watch(moneyProvider);
    final canSeeMoney = ref.watch(meProvider)?.canReadMoney ?? false;
    final p = context.palette;

    Future<void> refresh() async {
      ref.invalidate(gamificationProvider);
      ref.invalidate(familyProvider);
      await ref.read(familyProvider.future);
    }

    return Scaffold(
      appBar: AppBar(title: const Text('Family')),
      body: PullToRefresh(
        onRefresh: refresh,
        child: AsyncView<FamilyData>(
          value: family,
          onRetry: () => ref.invalidate(familyProvider),
          builder: (f) {
            final g = gami.valueOrNull;
            return ListView(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: const EdgeInsets.only(bottom: 32),
              children: [
                Padding(
                  padding: const EdgeInsets.fromLTRB(20, 8, 20, 4),
                  child: Text(f.householdName,
                      style: TextStyle(fontSize: 22, fontWeight: FontWeight.w700, color: p.text)),
                ),
                const GamificationCard(),
                SectionTitle('Members (${f.members.length})'),
                Card(
                  child: Column(
                    children: [
                      for (final m in f.members)
                        ListTile(
                          leading: CircleAvatar(
                            backgroundColor: _roleColor(m.role).withValues(alpha: 0.18),
                            child: Text(
                              m.displayName.isEmpty ? '?' : m.displayName.substring(0, 1).toUpperCase(),
                              style: TextStyle(color: _roleColor(m.role), fontWeight: FontWeight.w700),
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
                            if (m.allowanceMonthly != null && canSeeMoney)
                              'allowance ${money.format(m.allowanceMonthly!)}',
                          ].join(' · ')),
                          trailing: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Icon(Icons.star_rounded, size: 16, color: p.warning),
                              const SizedBox(width: 3),
                              Text('${m.pointsThisMonth}', style: TextStyle(color: p.text)),
                            ],
                          ),
                        ),
                    ],
                  ),
                ),
                if (g != null && g.leaderboard.isNotEmpty) ...[
                  const SectionTitle('Leaderboard this month'),
                  Card(
                    child: Column(
                      children: [
                        for (var i = 0; i < g.leaderboard.length; i++)
                          ListTile(
                            dense: true,
                            leading: i < 3
                                ? Icon(Icons.emoji_events_rounded,
                                    color: const [Color(0xFFD4A017), Color(0xFF9CA3AF), Color(0xFFB87333)][i])
                                : SizedBox(
                                    width: 24,
                                    child: Text('${i + 1}.', textAlign: TextAlign.center, style: const TextStyle(fontSize: 16)),
                                  ),
                            title: Text(g.leaderboard[i].displayName),
                            trailing: Text('${g.leaderboard[i].pointsMonth} pts'),
                          ),
                      ],
                    ),
                  ),
                ],
                if (g != null && g.badges.isNotEmpty) ...[
                  SectionTitle('Badges (${g.badges.where((b) => b.earned).length}/${g.badges.length})'),
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 16),
                    child: Wrap(
                      spacing: 8,
                      runSpacing: 8,
                      children: [for (final b in g.badges) _BadgeChip(b: b)],
                    ),
                  ),
                ],
              ],
            );
          },
        ),
      ),
    );
  }

  static String _roleLabel(String role) {
    if (role == 'owner') return 'Family Admin';
    return role.isEmpty ? role : role[0].toUpperCase() + role.substring(1);
  }

  static Color _roleColor(String role) {
    switch (role) {
      case 'owner':
        return const Color(0xFF7C3AED);
      case 'parent':
        return const Color(0xFF2563EB);
      case 'kid':
        return const Color(0xFFD97706);
    }
    return const Color(0xFF6B7280);
  }
}

class _BadgeChip extends StatelessWidget {
  final BadgeView b;
  const _BadgeChip({required this.b});

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    return Tooltip(
      message: b.description,
      child: Opacity(
        opacity: b.earned ? 1 : 0.4,
        child: Container(
          width: 104,
          padding: const EdgeInsets.all(10),
          decoration: BoxDecoration(
            color: b.earned ? p.primaryLight : p.card,
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: p.border),
          ),
          child: Column(
            children: [
              Icon(_badgeIcon(b.key), size: 28, color: b.earned ? p.primary : p.muted),
              const SizedBox(height: 4),
              Text(b.name,
                  textAlign: TextAlign.center,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 12, color: p.text)),
            ],
          ),
        ),
      ),
    );
  }
}

/// Material icon for a gamification badge key. Server `icon` strings are never rendered.
IconData _badgeIcon(String key) {
  switch (key) {
    case 'first_log':
      return Icons.edit_note_rounded;
    case 'first_budget':
      return Icons.account_balance_wallet_rounded;
    case 'streak_7':
      return Icons.local_fire_department;
    case 'streak_30':
      return Icons.whatshot_rounded;
    case 'under_budget':
      return Icons.verified_rounded;
    case 'saver':
      return Icons.savings_rounded;
    case 'chores_10':
      return Icons.cleaning_services_rounded;
    case 'chores_50':
      return Icons.workspace_premium_rounded;
  }
  return Icons.military_tech_rounded;
}
