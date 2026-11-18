import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../screens/family_screen.dart';
import '../state/providers.dart';
import '../theme.dart';
import 'gamification_badge.dart';
import 'kinkit_logo.dart';

/// App bar used by every top-level screen: the purple chrome (web top bar)
/// with the white logo tile, a white title and the family name
/// (`household_name` from /api/me) underneath in lavender, plus the
/// points/streak chip (tap opens Family & achievements, like the web sidebar
/// badge).
AppBar kAppBar(BuildContext context, String title, {PreferredSizeWidget? bottom, List<Widget> actions = const []}) {
  return AppBar(
    titleSpacing: KSpace.s4,
    title: KAppBarTitle(title),
    bottom: bottom,
    actions: [
      ...actions,
      GamificationBadge(
        onTap: () => Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => const FamilyScreen())),
      ),
      const SizedBox(width: KSpace.s2),
    ],
  );
}

/// Logo tile + title + family-name subtitle. The subtitle is hidden when the
/// family has no name yet.
class KAppBarTitle extends ConsumerWidget {
  final String title;
  const KAppBarTitle(this.title, {super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final family = ref.watch(meProvider)?.householdName.trim() ?? '';
    final p = context.palette;
    return Row(
      children: [
        const KinkitLogo(size: 32),
        const SizedBox(width: KSpace.s3),
        Expanded(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(title, maxLines: 1, overflow: TextOverflow.ellipsis),
              if (family.isNotEmpty)
                Text(
                  family,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 12, fontWeight: FontWeight.w500, color: p.chromeSoft, letterSpacing: 0.2),
                ),
            ],
          ),
        ),
      ],
    );
  }
}
