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
                  for (final (title, list) in groupBadges(g.badges)) ...[
                    Padding(
                      padding: const EdgeInsets.fromLTRB(20, 8, 20, 6),
                      child: Text('$title  ${list.where((b) => b.earned).length}/${list.length}',
                          style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700, color: p.text)),
                    ),
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 16),
                      child: LayoutBuilder(
                        builder: (context, c) {
                          const gap = 8.0;
                          final w = (c.maxWidth - gap) / 2;
                          return Wrap(
                            spacing: gap,
                            runSpacing: gap,
                            children: [for (final b in list) _BadgeTile(b: b, width: w)],
                          );
                        },
                      ),
                    ),
                  ],
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

class _BadgeTile extends StatelessWidget {
  final BadgeView b;
  final double width;
  const _BadgeTile({required this.b, required this.width});

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final tier = tierColor(b.tier);
    final earned = b.earned;
    final ring = earned ? tier : p.borderStrong;
    return Semantics(
      label: '${b.name}, ${b.tier.isEmpty ? '' : '${b.tier} '}badge, ${earned ? 'earned' : 'not earned yet, ${b.progress} of ${b.target}'}',
      child: Container(
        width: width,
        padding: const EdgeInsets.fromLTRB(10, 12, 10, 10),
        decoration: BoxDecoration(
          color: earned ? Color.alphaBlend(tier.withValues(alpha: 0.10), p.card) : p.card,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: earned ? tier.withValues(alpha: 0.6) : p.border),
        ),
        child: Column(
          children: [
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: earned ? tier.withValues(alpha: 0.16) : p.surface2,
                border: Border.all(color: ring, width: 3),
              ),
              child: Icon(_badgeIcon(b.key), size: 24, color: earned ? tier : p.muted.withValues(alpha: 0.7)),
            ),
            const SizedBox(height: 6),
            Text(b.name,
                textAlign: TextAlign.center,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: earned ? p.text : p.muted)),
            if (b.tier.isNotEmpty)
              Text(b.tier[0].toUpperCase() + b.tier.substring(1),
                  style: TextStyle(
                      fontSize: 11, fontWeight: FontWeight.w700, color: earned ? tier : p.muted)),
            const SizedBox(height: 2),
            Text(b.description,
                textAlign: TextAlign.center,
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(fontSize: 11, color: p.muted)),
            const SizedBox(height: 8),
            if (earned)
              Text(
                b.earnedAt == null || b.earnedAt!.length < 10 ? 'Earned' : 'Earned ${b.earnedAt!.substring(0, 10)}',
                style: TextStyle(fontSize: 11, fontWeight: FontWeight.w600, color: p.success),
              )
            else if (b.target > 0) ...[
              ClipRRect(
                borderRadius: BorderRadius.circular(99),
                child: LinearProgressIndicator(value: b.fraction, minHeight: 6, backgroundColor: p.border, color: tier),
              ),
              const SizedBox(height: 3),
              Text('${b.progress.clamp(0, b.target)}/${b.target}',
                  style: TextStyle(fontSize: 11, color: p.muted, fontFeatures: kTabular)),
            ],
          ],
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
    case 'financial_geek':
      return Icons.analytics_rounded;
    case 'number_cruncher':
      return Icons.calculate_rounded;
    case 'first_budget':
      return Icons.account_balance_wallet_rounded;
    case 'under_budget':
      return Icons.verified_rounded;
    case 'saver':
      return Icons.savings_rounded;
    case 'goal_setter':
      return Icons.flag_rounded;
    case 'goal_crusher':
      return Icons.bolt_rounded;
    case 'asset_tracker':
      return Icons.layers_rounded;
    case 'income_insider':
      return Icons.trending_up_rounded;
    case 'streak_3':
      return Icons.eco_rounded;
    case 'streak_7':
      return Icons.local_fire_department;
    case 'streak_30':
      return Icons.whatshot_rounded;
    case 'never_forget':
      return Icons.notifications_active_rounded;
    case 'planner':
      return Icons.event_available_rounded;
    case 'chores_10':
      return Icons.cleaning_services_rounded;
    case 'chores_50':
      return Icons.emoji_events_rounded;
    case 'menu_maker':
      return Icons.restaurant_menu_rounded;
    case 'chefs_choice':
      return Icons.soup_kitchen_rounded;
    case 'level_5':
      return Icons.shield_rounded;
    case 'level_10':
      return Icons.workspace_premium_rounded;
  }
  return Icons.military_tech_rounded;
}
