import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/data_providers.dart';
import '../widgets/async_view.dart';

/// Runs an API call, refreshes the household providers and shows a snackbar
/// (shared by the reminders, chores and lists screens).
Future<void> runHomeAction(BuildContext context, WidgetRef ref, Future<void> Function() call, String ok) async {
  try {
    await call();
    invalidateHome(ref);
    if (context.mounted && ok.isNotEmpty) showSnack(context, ok);
  } catch (e) {
    if (context.mounted) showSnack(context, '$e', error: true);
  }
}
