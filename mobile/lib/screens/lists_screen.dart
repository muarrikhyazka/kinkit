import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../state/data_providers.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../widgets/async_view.dart';
import '../widgets/k_app_bar.dart';
import 'home_actions.dart';
import 'list_detail_screen.dart';

/// Lists destination (web: /lists), backed by backend-go/lists.go.
class ListsScreen extends StatelessWidget {
  const ListsScreen({super.key});

  @override
  Widget build(BuildContext context) => Scaffold(appBar: kAppBar(context, 'Lists'), body: const ListsTab());
}

// ============================================================ lists

class ListsTab extends ConsumerWidget {
  const ListsTab({super.key});

  Future<void> _newList(BuildContext context, WidgetRef ref) async {
    final ctrl = TextEditingController();
    final name = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('New list'),
        content: TextField(
          controller: ctrl,
          autofocus: true,
          textCapitalization: TextCapitalization.sentences,
          decoration: const InputDecoration(hintText: 'Groceries'),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
          FilledButton(onPressed: () => Navigator.pop(ctx, ctrl.text.trim()), child: const Text('Create')),
        ],
      ),
    );
    if (name == null || name.isEmpty || !context.mounted) return;
    await runHomeAction(context, ref, () => ref.read(apiProvider).createList(name), 'List created');
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final value = ref.watch(listsProvider);
    final canCreate = ref.watch(meProvider)?.canManage ?? false;
    final p = context.palette;
    return Scaffold(
      floatingActionButton: canCreate
          ? FloatingActionButton.extended(
              heroTag: 'add-list',
              onPressed: () => _newList(context, ref),
              icon: const Icon(Icons.playlist_add),
              label: const Text('List'),
            )
          : null,
      body: PullToRefresh(
        onRefresh: () async {
          ref.invalidate(listsProvider);
          await ref.read(listsProvider.future);
        },
        child: AsyncView<List<ShoppingList>>(
          value: value,
          onRetry: () => ref.invalidate(listsProvider),
          builder: (lists) {
            final active = lists.where((l) => !l.archived).toList();
            if (active.isEmpty) {
              return ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                children: const [EmptyState(icon: Icons.checklist_outlined, title: 'No lists yet')],
              );
            }
            return ListView(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: const EdgeInsets.only(top: 8, bottom: 96),
              children: [
                for (final l in active)
                  Card(
                    child: ListTile(
                      leading: CircleAvatar(
                        backgroundColor: p.primaryLight,
                        child: Icon(Icons.shopping_cart_outlined, color: p.primary),
                      ),
                      title: Text(l.name),
                      subtitle: Text([
                        '${l.openCount} to get',
                        if (l.checkedCount > 0) '${l.checkedCount} in cart',
                        if (l.store.isNotEmpty) l.store,
                      ].join(' · ')),
                      trailing: const Icon(Icons.chevron_right),
                      onTap: () => Navigator.of(context)
                          .push(MaterialPageRoute<void>(builder: (_) => ListDetailScreen(listId: l.id, title: l.label))),
                    ),
                  ),
              ],
            );
          },
        ),
      ),
    );
  }
}
