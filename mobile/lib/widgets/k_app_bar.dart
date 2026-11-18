import 'package:flutter/material.dart';

import '../screens/family_screen.dart';
import 'gamification_badge.dart';

/// App bar used by every top-level screen: title + points/streak chip
/// (tap opens Family & achievements, like the web sidebar badge).
AppBar kAppBar(BuildContext context, String title, {PreferredSizeWidget? bottom, List<Widget> actions = const []}) {
  return AppBar(
    title: Text(title),
    bottom: bottom,
    actions: [
      ...actions,
      GamificationBadge(
        onTap: () => Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => const FamilyScreen())),
      ),
    ],
  );
}
