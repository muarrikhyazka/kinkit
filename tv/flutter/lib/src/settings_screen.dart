import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:intl/intl.dart';

import 'config.dart';
import 'theme.dart';
import 'widgets.dart';

enum SettingsResult { refresh, repair, forget }

/// Reached from the dashboard with MENU or a long BACK / OK press.
class SettingsScreen extends StatefulWidget {
  const SettingsScreen({
    super.key,
    required this.config,
    required this.updatedAt,
    required this.offline,
    this.offlineReason,
    this.currency,
  });

  final PairingConfig config;
  final DateTime? updatedAt;
  final bool offline;
  final String? offlineReason;
  final String? currency;

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  // Only react to a Back press that started on this screen (the long press
  // that opened us still delivers its key-up here).
  bool _backDown = false;

  // A long OK press opens this screen; swallow its auto-repeats so they do not
  // immediately activate the focused button.
  bool _selectDown = false;

  KeyEventResult _onKey(FocusNode node, KeyEvent event) {
    if (isSelectKey(event.logicalKey)) {
      if (event is KeyDownEvent) _selectDown = true;
      if (event is KeyRepeatEvent && !_selectDown) return KeyEventResult.handled;
      return KeyEventResult.ignored;
    }
    if (!isBackKey(event.logicalKey)) return KeyEventResult.ignored;
    if (event is KeyDownEvent) {
      _backDown = true;
    } else if (event is KeyUpEvent && _backDown) {
      _backDown = false;
      Navigator.of(context).pop();
    }
    return KeyEventResult.handled;
  }

  void _close([SettingsResult? r]) => Navigator.of(context).pop(r);

  @override
  Widget build(BuildContext context) {
    final updated = widget.updatedAt == null ? 'never' : DateFormat('d MMM yyyy, HH:mm').format(widget.updatedAt!);
    return Scaffold(
      body: TvCanvas(
        child: Focus(
          onKeyEvent: _onKey,
          child: FocusTraversalGroup(
            policy: ReadingOrderTraversalPolicy(),
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              const Text('Settings', style: TextStyle(fontSize: 30, fontWeight: FontWeight.w800)),
              const SizedBox(height: 18),
              _info('Server', widget.config.server),
              _info('TV token', maskToken(widget.config.token)),
              _info('Last update', updated),
              _info('Status', widget.offline ? 'Offline - ${widget.offlineReason ?? 'no connection'}' : 'Online'),
              if (widget.currency != null) _info('Currency', widget.currency!),
              const SizedBox(height: 22),
              Wrap(spacing: 14, runSpacing: 14, children: [
                TvButton(autofocus: true, primary: true, onPressed: () => _close(), child: const Text('Back to dashboard')),
                TvButton(onPressed: () => _close(SettingsResult.refresh), child: const Text('Refresh now')),
                TvButton(onPressed: () => _close(SettingsResult.repair), child: const Text('Change server / token')),
                TvButton(onPressed: () => _close(SettingsResult.forget), child: const Text('Unpair this TV')),
                TvButton(onPressed: () => SystemNavigator.pop(), child: const Text('Exit app')),
              ]),
              const Spacer(),
              const Text(
                'This app is view-only. To stop this TV from showing your data, revoke its link in the web app '
                '(Settings > Data & devices > TV / car display links).',
                style: TextStyle(fontSize: 13, color: TvColors.muted, height: 1.4),
              ),
            ]),
          ),
        ),
      ),
    );
  }

  Widget _info(String k, String v) => Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          SizedBox(width: 150, child: Text(k, style: const TextStyle(fontSize: 17, color: TvColors.muted))),
          Expanded(child: Text(v, style: const TextStyle(fontSize: 17), maxLines: 2, overflow: TextOverflow.ellipsis)),
        ]),
      );
}
