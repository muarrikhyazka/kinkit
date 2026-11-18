import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../theme.dart';

/// Renders an [AsyncValue] with consistent loading / error / data states.
/// Keeps showing previous data while refreshing.
class AsyncView<T> extends StatelessWidget {
  final AsyncValue<T> value;
  final Widget Function(T data) builder;
  final VoidCallback? onRetry;

  const AsyncView({super.key, required this.value, required this.builder, this.onRetry});

  @override
  Widget build(BuildContext context) {
    if (value.hasValue) return builder(value.requireValue);
    if (value.hasError) {
      return ErrorState(message: value.error.toString(), onRetry: onRetry);
    }
    return const LoadingState();
  }
}

class LoadingState extends StatelessWidget {
  const LoadingState({super.key});

  @override
  Widget build(BuildContext context) =>
      const Center(child: Padding(padding: EdgeInsets.all(32), child: CircularProgressIndicator()));
}

/// Scrollable so it still works inside a RefreshIndicator.
class ErrorState extends StatelessWidget {
  final String message;
  final VoidCallback? onRetry;

  const ErrorState({super.key, required this.message, this.onRetry});

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.all(32),
      children: [
        Icon(Icons.cloud_off_outlined, size: 48, color: p.danger),
        const SizedBox(height: 12),
        Text(message, textAlign: TextAlign.center, style: TextStyle(color: p.text)),
        if (onRetry != null) ...[
          const SizedBox(height: 16),
          Center(child: OutlinedButton.icon(onPressed: onRetry, icon: const Icon(Icons.refresh), label: const Text('Retry'))),
        ],
      ],
    );
  }
}

class EmptyState extends StatelessWidget {
  final IconData icon;
  final String title;
  final String? message;
  final Widget? action;

  const EmptyState({super.key, required this.icon, required this.title, this.message, this.action});

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 40),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 48, color: p.muted),
          const SizedBox(height: 12),
          Text(title, textAlign: TextAlign.center, style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600, color: p.text)),
          if (message != null) ...[
            const SizedBox(height: 6),
            Text(message!, textAlign: TextAlign.center, style: TextStyle(color: p.muted)),
          ],
          if (action != null) ...[const SizedBox(height: 16), action!],
        ],
      ),
    );
  }
}

/// Pull-to-refresh wrapper that swallows refresh errors (the error state
/// is rendered by the provider's AsyncValue instead).
class PullToRefresh extends StatelessWidget {
  final Future<void> Function() onRefresh;
  final Widget child;

  const PullToRefresh({super.key, required this.onRefresh, required this.child});

  @override
  Widget build(BuildContext context) {
    return RefreshIndicator(
      onRefresh: () async {
        try {
          await onRefresh();
        } catch (_) {}
      },
      child: child,
    );
  }
}

/// Section heading used inside list screens.
class SectionTitle extends StatelessWidget {
  final String text;
  final Widget? trailing;

  const SectionTitle(this.text, {super.key, this.trailing});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 18, 16, 6),
      child: Row(
        children: [
          Expanded(
            child: Text(text, style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: context.palette.text)),
          ),
          if (trailing != null) trailing!,
        ],
      ),
    );
  }
}

void showSnack(BuildContext context, String message, {bool error = false}) {
  final p = context.palette;
  ScaffoldMessenger.of(context)
    ..hideCurrentSnackBar()
    ..showSnackBar(SnackBar(
      content: Text(message),
      backgroundColor: error ? p.danger : null,
      behavior: SnackBarBehavior.floating,
    ));
}

Future<bool> confirmDialog(BuildContext context, {required String title, String? message, String confirm = 'Delete'}) async {
  final ok = await showDialog<bool>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: Text(title),
      content: message != null ? Text(message) : null,
      actions: [
        TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
        FilledButton(
          style: FilledButton.styleFrom(backgroundColor: ctx.palette.danger),
          onPressed: () => Navigator.pop(ctx, true),
          child: Text(confirm),
        ),
      ],
    ),
  );
  return ok ?? false;
}
