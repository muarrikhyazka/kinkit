import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../screens/family_screen.dart';
import '../state/providers.dart';
import '../theme.dart';
import 'gamification_badge.dart';

/// App bar used by every top-level screen: title with the family name
/// (`household_name` from /api/me) underneath, plus the points/streak chip
/// (tap opens Family & achievements, like the web sidebar badge).
AppBar kAppBar(BuildContext context, String title, {PreferredSizeWidget? bottom, List<Widget> actions = const []}) {
  return AppBar(
    title: KAppBarTitle(title),
    bottom: bottom,
    actions: [
      ...actions,
      GamificationBadge(
        onTap: () => Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => const FamilyScreen())),
      ),
    ],
  );
}

/// Title + family-name subtitle. The subtitle is hidden when the family has
/// no name yet.
class KAppBarTitle extends ConsumerWidget {
  final String title;
  const KAppBarTitle(this.title, {super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final family = ref.watch(meProvider)?.householdName.trim() ?? '';
    final p = context.palette;
    if (family.isEmpty) return Text(title);
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(title, maxLines: 1, overflow: TextOverflow.ellipsis),
        Text(
          family,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: TextStyle(fontSize: 12, fontWeight: FontWeight.w500, color: p.muted, letterSpacing: 0.2),
        ),
      ],
    );
  }
}
