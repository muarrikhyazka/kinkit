import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:intl/intl.dart';

import 'api.dart';
import 'config.dart';
import 'kinkit_logo.dart';
import 'mascot.dart';
import 'models.dart';
import 'money.dart';
import 'pages.dart';
import 'settings_screen.dart';
import 'store.dart';
import 'theme.dart';
import 'widgets.dart';

class _PrevPageIntent extends Intent {
  const _PrevPageIntent();
}

class _NextPageIntent extends Intent {
  const _NextPageIntent();
}

class _TogglePauseIntent extends Intent {
  const _TogglePauseIntent();
}

/// Full-screen, view-only dashboard: auto-refresh (60s), clock, auto-rotating
/// carousel (left/right to move), offline badge with last data, 401 -> pairing.
class ViewerScreen extends StatefulWidget {
  const ViewerScreen({
    super.key,
    required this.api,
    required this.store,
    required this.config,
    required this.onUnauthorized,
    required this.onRepair,
    required this.onForget,
  });

  final TvApi api;
  final TvStore store;
  final PairingConfig config;
  final VoidCallback onUnauthorized;
  final VoidCallback onRepair;
  final VoidCallback onForget;

  @override
  State<ViewerScreen> createState() => _ViewerScreenState();
}

class _ViewerScreenState extends State<ViewerScreen> {
  static const refreshEvery = Duration(seconds: 60);
  static const rotateEvery = Duration(seconds: 15);
  static const longPress = Duration(milliseconds: 700);
  static const fxMaxAge = Duration(hours: 6);

  final _focus = FocusNode(debugLabel: 'viewer');
  TvData? _data;
  DateTime? _updatedAt;
  bool _offline = false;
  String? _offlineReason;
  bool _loading = false;
  Map<String, double>? _rates;
  DateTime? _ratesAt;

  int _page = 0;
  bool _paused = false;
  DateTime _now = DateTime.now();

  Timer? _refreshTimer;
  Timer? _rotateTimer;
  Timer? _clockTimer;

  LogicalKeyboardKey? _heldKey;
  DateTime? _heldSince;
  bool _longFired = false;
  bool _settingsOpen = false;

  @override
  void initState() {
    super.initState();
    // Show the last known data instantly (also the offline fallback).
    final cached = widget.store.cachedJson;
    if (cached != null) {
      try {
        _data = parseTvData(cached);
        _updatedAt = widget.store.cachedAt;
      } catch (_) {
        // ignore a corrupt cache
      }
    }
    _rates = widget.store.fxRates;
    _load();
    _refreshTimer = Timer.periodic(refreshEvery, (_) => _load());
    _clockTimer = Timer.periodic(const Duration(seconds: 1), (_) {
      final n = DateTime.now();
      if (n.minute != _now.minute && mounted) setState(() => _now = n);
    });
    _restartRotation();
  }

  @override
  void dispose() {
    _refreshTimer?.cancel();
    _rotateTimer?.cancel();
    _clockTimer?.cancel();
    _focus.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    if (_loading) return;
    _loading = true;
    try {
      final (data, raw) = await widget.api.fetchData(widget.config);
      final at = DateTime.now();
      await widget.store.saveCache(raw, at);
      if (!mounted) return;
      setState(() {
        _data = data;
        _updatedAt = at;
        _offline = false;
        _offlineReason = null;
      });
      await _maybeFetchRates(data.currency);
    } on TvApiException catch (e) {
      if (!mounted) return;
      if (e.kind == TvErrorKind.unauthorized) {
        widget.onUnauthorized();
        return;
      }
      setState(() {
        _offline = true;
        _offlineReason = e.message;
      });
    } finally {
      _loading = false;
    }
  }

  Future<void> _maybeFetchRates(String currency) async {
    if (currency == 'IDR') return;
    final fresh = _ratesAt != null && DateTime.now().difference(_ratesAt!) < fxMaxAge;
    if (fresh && _rates != null) return;
    final rates = await widget.api.fetchFxRates(widget.config);
    if (rates == null || !mounted) return;
    await widget.store.saveFxRates(rates);
    setState(() {
      _rates = rates;
      _ratesAt = DateTime.now();
    });
  }

  // ---------- carousel ----------

  List<Widget> _pages(TvData d, Money money) {
    final cats = d.report.activeCategories;
    final chunks = <List<TvExpense>>[];
    for (var i = 0; i < cats.length; i += BudgetPage.perPage) {
      final end = i + BudgetPage.perPage > cats.length ? cats.length : i + BudgetPage.perPage;
      chunks.add(cats.sublist(i, end));
    }
    if (chunks.isEmpty) chunks.add(const []);
    return [
      OverviewPage(data: d, money: money),
      for (var i = 0; i < chunks.length; i++)
        BudgetPage(items: chunks[i], money: money, page: i, pages: chunks.length),
      TrendPage(data: d, money: money),
    ];
  }

  int _pageCount = 1;

  void _restartRotation() {
    _rotateTimer?.cancel();
    if (_paused) return;
    _rotateTimer = Timer.periodic(rotateEvery, (_) => _go(1, manual: false));
  }

  void _go(int delta, {bool manual = true}) {
    if (!mounted || _pageCount <= 0) return;
    setState(() => _page = (_page + delta) % _pageCount); // Dart % is never negative here
    if (manual) _restartRotation();
  }

  void _togglePause() {
    setState(() => _paused = !_paused);
    _restartRotation();
  }

  // ---------- keys ----------

  Future<void> _openSettings() async {
    if (_settingsOpen) return;
    _settingsOpen = true;
    final result = await Navigator.of(context).push<SettingsResult>(
      MaterialPageRoute(
        builder: (_) => SettingsScreen(
          config: widget.config,
          updatedAt: _updatedAt,
          offline: _offline,
          offlineReason: _offlineReason,
          currency: _data?.currency,
        ),
      ),
    );
    _settingsOpen = false;
    if (!mounted) return;
    switch (result) {
      case SettingsResult.repair:
        widget.onRepair();
        return;
      case SettingsResult.forget:
        widget.onForget();
        return;
      case SettingsResult.refresh:
        _load();
        break;
      case null:
        break;
    }
    _focus.requestFocus();
  }

  KeyEventResult _onKey(FocusNode node, KeyEvent event) {
    final k = event.logicalKey;
    if (isMenuKey(k)) {
      if (event is KeyDownEvent) _openSettings();
      return KeyEventResult.handled;
    }
    final ok = isSelectKey(k);
    final back = isBackKey(k);
    if (!ok && !back) return KeyEventResult.ignored;

    final now = DateTime.now();
    if (event is KeyDownEvent) {
      _heldKey = k;
      _heldSince = now;
      _longFired = false;
    } else if (event is KeyRepeatEvent) {
      if (_heldKey == k && !_longFired && _heldSince != null && now.difference(_heldSince!) >= longPress) {
        _longFired = true;
        _openSettings();
      }
    } else if (event is KeyUpEvent) {
      if (_heldKey == k && !_longFired && _heldSince != null) {
        if (now.difference(_heldSince!) >= longPress) {
          _openSettings();
        } else if (ok) {
          _togglePause();
        } else {
          SystemNavigator.pop(); // short Back on the dashboard leaves the app
        }
      }
      _heldKey = null;
      _heldSince = null;
    }
    return KeyEventResult.handled;
  }

  // ---------- build ----------

  @override
  Widget build(BuildContext context) {
    final d = _data;
    return Scaffold(
      body: Focus(
        onKeyEvent: _onKey,
        child: FocusableActionDetector(
          autofocus: true,
          focusNode: _focus,
          shortcuts: const <ShortcutActivator, Intent>{
            SingleActivator(LogicalKeyboardKey.arrowLeft): _PrevPageIntent(),
            SingleActivator(LogicalKeyboardKey.arrowRight): _NextPageIntent(),
            SingleActivator(LogicalKeyboardKey.channelUp): _PrevPageIntent(),
            SingleActivator(LogicalKeyboardKey.channelDown): _NextPageIntent(),
            SingleActivator(LogicalKeyboardKey.mediaRewind): _PrevPageIntent(),
            SingleActivator(LogicalKeyboardKey.mediaFastForward): _NextPageIntent(),
            SingleActivator(LogicalKeyboardKey.mediaPlayPause): _TogglePauseIntent(),
            SingleActivator(LogicalKeyboardKey.space): _TogglePauseIntent(),
          },
          actions: <Type, Action<Intent>>{
            _PrevPageIntent: CallbackAction<_PrevPageIntent>(onInvoke: (_) {
              _go(-1);
              return null;
            }),
            _NextPageIntent: CallbackAction<_NextPageIntent>(onInvoke: (_) {
              _go(1);
              return null;
            }),
            _TogglePauseIntent: CallbackAction<_TogglePauseIntent>(onInvoke: (_) {
              _togglePause();
              return null;
            }),
          },
          child: TvCanvas(child: d == null ? _emptyState() : _dashboard(d)),
        ),
      ),
    );
  }

  Widget _emptyState() {
    return Center(
      child: Column(mainAxisSize: MainAxisSize.min, children: [
        if (!_offline) const CircularProgressIndicator(color: TvColors.primary),
        if (_offline) ...[
          // Kiki "thinks" while there's no data to show.
          const Mascot(pose: MascotPose.think, size: 120),
          const SizedBox(height: 8),
          const Text('Cannot reach Kinkit', style: TextStyle(fontSize: 28, fontWeight: FontWeight.w800)),
          const SizedBox(height: 8),
          Text(_offlineReason ?? '', textAlign: TextAlign.center,
              style: const TextStyle(fontSize: 15, color: TvColors.muted)),
          const SizedBox(height: 6),
          Text('Retrying every minute. Server: ${widget.config.server}',
              style: const TextStyle(fontSize: 14, color: TvColors.muted)),
          const SizedBox(height: 18),
          const Text('Hold BACK or press MENU for settings', style: TextStyle(fontSize: 14, color: TvColors.accent)),
        ],
      ]),
    );
  }

  Widget _dashboard(TvData d) {
    final money = Money(currency: d.currency, rates: _rates);
    final pages = _pages(d, money);
    _pageCount = pages.length;
    final int page = _page < pages.length ? _page : 0;
    return Column(children: [
      _header(d),
      const SizedBox(height: 12),
      Expanded(
        child: AnimatedSwitcher(
          duration: const Duration(milliseconds: 450),
          transitionBuilder: (child, anim) => FadeTransition(
            opacity: anim,
            child: SlideTransition(
              position: Tween<Offset>(begin: const Offset(0.03, 0), end: Offset.zero).animate(anim),
              child: child,
            ),
          ),
          child: KeyedSubtree(key: ValueKey('page-$page-${pages.length}'), child: pages[page]),
        ),
      ),
      const SizedBox(height: 10),
      _footer(pages.length, page),
    ]);
  }

  Widget _header(TvData d) {
    final name = d.householdName ?? d.displayName;
    return TvBrandBar(
      wordmark: 'Kinkit',
      logo: const KinkitLogo(size: 34),
      children: [
        if (name != null) ...[
          const SizedBox(width: 14),
          Flexible(
            child: Text(name, maxLines: 1, overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 20, color: TvColors.chromeSoft, fontWeight: FontWeight.w600)),
          ),
        ],
        const SizedBox(width: 18),
        Text('${monthLong[d.month - 1]} ${d.year}', style: const TextStyle(fontSize: 20, color: TvColors.chromeSoft)),
        const Spacer(),
        if (_offline) ...[
          StatusBadge(
            label: _updatedAt == null ? 'OFFLINE' : 'OFFLINE - ${DateFormat('HH:mm').format(_updatedAt!)}',
            color: TvColors.warning,
            icon: Icons.cloud_off,
          ),
          const SizedBox(width: 12),
        ],
        if (_paused) ...[
          const StatusBadge(label: 'PAUSED', color: TvColors.onChrome, icon: Icons.pause),
          const SizedBox(width: 12),
        ],
        Column(mainAxisAlignment: MainAxisAlignment.center, crossAxisAlignment: CrossAxisAlignment.end, children: [
          Text(DateFormat('HH:mm').format(_now),
              style: const TextStyle(fontSize: 24, fontWeight: FontWeight.w700, height: 1.0, color: TvColors.onChrome)),
          Text(DateFormat('EEE d MMM').format(_now), style: const TextStyle(fontSize: 12, color: TvColors.chromeSoft)),
        ]),
      ],
    );
  }

  Widget _footer(int count, int current) {
    final updated = _updatedAt == null ? '' : 'Updated ${DateFormat('HH:mm').format(_updatedAt!)}';
    return SizedBox(
      height: 18,
      child: Row(children: [
        for (var i = 0; i < count; i++)
          AnimatedContainer(
            duration: const Duration(milliseconds: 250),
            margin: const EdgeInsets.only(right: 6),
            width: i == current ? 22 : 8,
            height: 8,
            decoration: BoxDecoration(
              color: i == current ? TvColors.primary : TvColors.border,
              borderRadius: BorderRadius.circular(4),
            ),
          ),
        const SizedBox(width: 10),
        Text(updated, style: const TextStyle(fontSize: 12, color: TvColors.muted)),
        const Spacer(),
        const Text('Left/Right: page   OK: pause   Hold BACK / MENU: settings',
            style: TextStyle(fontSize: 12, color: TvColors.muted)),
      ]),
    );
  }
}
