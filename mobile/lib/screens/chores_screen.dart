import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../widgets/async_view.dart';
import '../widgets/k_app_bar.dart';
import '../widgets/mascot.dart';
import 'home_actions.dart';

/// Chores destination (web: /chores), backed by backend-go/chores.go.
class ChoresScreen extends StatelessWidget {
  const ChoresScreen({super.key});

  @override
  Widget build(BuildContext context) => Scaffold(appBar: kAppBar(context, 'Chores'), body: const ChoresTab());
}

// ============================================================ chores

class ChoresTab extends ConsumerWidget {
  const ChoresTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final value = ref.watch(choresTodayProvider);
    final me = ref.watch(meProvider);
    final api = ref.read(apiProvider);
    final p = context.palette;

    Widget tile(ChoreInstance c, {bool approval = false}) {
      final mine = c.memberId == null || c.memberId == me?.memberId;
      final canAct = c.id != 0 && !c.projected;
      Widget? trailing;
      if (canAct && approval) {
        trailing = Row(mainAxisSize: MainAxisSize.min, children: [
          IconButton(
            tooltip: 'Send back',
            icon: Icon(Icons.undo, color: p.muted),
            onPressed: () => runHomeAction(context, ref, () => api.choreAction(c.id, 'reject'), 'Sent back'),
          ),
          IconButton(
            tooltip: 'Approve',
            icon: Icon(Icons.thumb_up_alt_outlined, color: p.success),
            onPressed: () => runHomeAction(context, ref, () => api.choreAction(c.id, 'approve'), 'Approved'),
          ),
        ]);
      } else if (canAct && !c.done && !c.submitted && (mine || (me?.canManage ?? false))) {
        trailing = c.requiresPhoto
            ? Tooltip(message: 'Needs a photo - finish it on the web', child: Icon(Icons.photo_camera_outlined, color: p.muted))
            : FilledButton.tonal(
                onPressed: () => runHomeAction(context, ref, () => api.choreAction(c.id, 'complete'), 'Nice work!'),
                child: const Text('Done'),
              );
      } else if (canAct && (c.done || c.submitted)) {
        trailing = IconButton(
          tooltip: 'Undo',
          icon: const Icon(Icons.undo),
          onPressed: () => runHomeAction(context, ref, () => api.choreAction(c.id, 'undo'), 'Undone'),
        );
      }
      return ListTile(
        leading: Icon(
          c.done ? Icons.check_circle : (c.submitted ? Icons.hourglass_top : Icons.radio_button_unchecked),
          color: c.done ? p.success : (c.overdue ? p.danger : (c.submitted ? p.warning : p.muted)),
        ),
        title: Text(c.label, style: TextStyle(decoration: c.done ? TextDecoration.lineThrough : null)),
        subtitle: Text([
          if (c.memberName.isNotEmpty) c.memberName else 'Anyone',
          if (c.points > 0) '+${c.points} pts',
          if (c.submitted) 'waiting for approval',
          if (c.overdue) 'overdue',
          if (c.dueTime != null) 'by ${c.dueTime}',
        ].join(' · ')),
        trailing: trailing,
      );
    }

    return PullToRefresh(
      onRefresh: () async {
        ref.invalidate(choresTodayProvider);
        await ref.read(choresTodayProvider.future);
      },
      child: AsyncView<ChoresToday>(
        value: value,
        onRetry: () => ref.invalidate(choresTodayProvider),
        builder: (d) => ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.only(bottom: 32),
          children: [
            Card(
              child: ListTile(
                leading: Icon(Icons.cleaning_services_rounded, size: 28, color: p.primary),
                title: Text('This week: ${d.week.done}/${d.week.total} done'),
                subtitle: Text('+${d.week.points} chore points'),
              ),
            ),
            if (d.waiting.isNotEmpty) ...[
              SectionTitle('Waiting for approval (${d.waiting.length})'),
              Card(child: Column(children: [for (final c in d.waiting) tile(c, approval: true)])),
            ],
            const SectionTitle('Today'),
            if (d.items.isEmpty)
              const EmptyState(mascot: MascotPose.relax, title: 'No chores today', message: 'Nothing to do. Nice!')
            else
              Card(child: Column(children: [for (final c in d.items) tile(c)])),
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 12, 20, 0),
              child: Text('Create and schedule chores on the web.', style: TextStyle(color: p.muted, fontSize: 12)),
            ),
          ],
        ),
      ),
    );
  }
}
