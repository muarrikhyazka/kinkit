import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/providers.dart';

/// How the phone app is navigated. [tabs] is the usual bottom bar with Today
/// first; [townMap] swaps the Today tab for a walkable town map (web twin:
/// react/src/game/navMode.js, 'game' mode).
enum NavMode { tabs, townMap }

const _kNavMode = 'kinkit.navMode';

/// Per-device preference, persisted like the theme mode.
class NavModeNotifier extends Notifier<NavMode> {
  @override
  NavMode build() {
    final v = ref.watch(sharedPrefsProvider).getString(_kNavMode);
    return NavMode.values.firstWhere((m) => m.name == v, orElse: () => NavMode.tabs);
  }

  Future<void> set(NavMode mode) async {
    await ref.read(sharedPrefsProvider).setString(_kNavMode, mode.name);
    state = mode;
  }
}

final navModeProvider = NotifierProvider<NavModeNotifier, NavMode>(NavModeNotifier.new);
