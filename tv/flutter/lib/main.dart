import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'src/api.dart';
import 'src/config.dart';
import 'src/pairing_screen.dart';
import 'src/store.dart';
import 'src/theme.dart';
import 'src/viewer_screen.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  // TVs have no touch: always draw focus highlights.
  FocusManager.instance.highlightStrategy = FocusHighlightStrategy.alwaysTraditional;
  SystemChrome.setEnabledSystemUIMode(SystemUiMode.immersiveSticky);
  runApp(const KinkitTvApp());
}

class KinkitTvApp extends StatelessWidget {
  const KinkitTvApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Kinkit TV',
      debugShowCheckedModeBanner: false,
      theme: buildTvTheme(),
      // Remote "OK"/centre buttons activate the focused control.
      shortcuts: <ShortcutActivator, Intent>{
        ...WidgetsApp.defaultShortcuts,
        const SingleActivator(LogicalKeyboardKey.select): const ActivateIntent(),
        const SingleActivator(LogicalKeyboardKey.gameButtonA): const ActivateIntent(),
        const SingleActivator(LogicalKeyboardKey.numpadEnter): const ActivateIntent(),
      },
      home: const RootScreen(),
    );
  }
}

/// Decides between pairing and the viewer based on the stored config.
class RootScreen extends StatefulWidget {
  const RootScreen({super.key});

  @override
  State<RootScreen> createState() => _RootScreenState();
}

class _RootScreenState extends State<RootScreen> {
  final _api = TvApi();
  TvStore? _store;
  PairingConfig? _config;
  bool _repairing = false;
  String? _message;

  @override
  void initState() {
    super.initState();
    TvStore.open().then((s) {
      if (!mounted) return;
      setState(() {
        _store = s;
        _config = s.config;
      });
    });
  }

  @override
  void dispose() {
    _api.close();
    super.dispose();
  }

  Future<void> _onPaired(PairingConfig c, String raw) async {
    final store = _store!;
    await store.saveConfig(c);
    await store.saveCache(raw, DateTime.now());
    if (!mounted) return;
    setState(() {
      _config = c;
      _repairing = false;
      _message = null;
    });
  }

  Future<void> _unpair(String? message) async {
    await _store!.clearToken();
    if (!mounted) return;
    setState(() {
      _config = null;
      _repairing = false;
      _message = message;
    });
  }

  @override
  Widget build(BuildContext context) {
    final store = _store;
    if (store == null) {
      return const Scaffold(body: Center(child: CircularProgressIndicator(color: TvColors.primary)));
    }
    final config = _config;
    if (config == null || _repairing) {
      return PairingScreen(
        key: ValueKey('pair-${config?.token}'),
        api: _api,
        initialServer: config?.server ?? store.lastServer,
        initialToken: config?.token ?? '',
        message: _message,
        onPaired: _onPaired,
        onCancel: _repairing ? () => setState(() => _repairing = false) : null,
      );
    }
    return ViewerScreen(
      key: ValueKey(config),
      api: _api,
      store: store,
      config: config,
      onUnauthorized: () => _unpair(
          'This TV link is invalid or was revoked. Create a new one in the web app under Settings > Data & devices.'),
      onRepair: () => setState(() => _repairing = true),
      onForget: () => _unpair(null),
    );
  }
}
