import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'api.dart';
import 'config.dart';
import 'theme.dart';
import 'widgets.dart';

enum _Field { server, token }

/// First-run / re-pair screen: server URL + TV token, entered with a built-in
/// D-pad keyboard, the system keyboard, a hardware keyboard, or by pasting.
/// Either field accepts the full share link (https://host/tv?k=TOKEN).
class PairingScreen extends StatefulWidget {
  const PairingScreen({
    super.key,
    required this.api,
    required this.initialServer,
    required this.onPaired,
    this.initialToken = '',
    this.message,
    this.onCancel,
  });

  final TvApi api;
  final String initialServer;
  final String initialToken;
  final String? message;
  final Future<void> Function(PairingConfig config, String rawJson) onPaired;

  /// Set when re-pairing from settings: Back returns to the dashboard.
  final VoidCallback? onCancel;

  @override
  State<PairingScreen> createState() => _PairingScreenState();
}

class _PairingScreenState extends State<PairingScreen> {
  late String _server = widget.initialServer;
  late String _token = widget.initialToken;
  _Field _active = _Field.server;
  bool _upper = false;
  bool _busy = false;
  String? _error;
  late String? _info = widget.message;

  String get _value => _active == _Field.server ? _server : _token;
  set _value(String v) => setState(() {
        if (_active == _Field.server) {
          _server = v;
        } else {
          _token = v;
        }
        _error = null;
      });

  void _type(String s) => _value = _value + s;
  void _backspace() {
    final v = _value;
    if (v.isNotEmpty) _value = v.substring(0, v.length - 1);
  }

  Future<void> _paste() async {
    final data = await Clipboard.getData(Clipboard.kTextPlain);
    final text = data?.text?.trim();
    if (text != null && text.isNotEmpty) _type(text);
  }

  Future<void> _systemKeyboard() async {
    final controller = TextEditingController(text: _value);
    final result = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: TvColors.card,
        title: Text(_active == _Field.server ? 'Server address' : 'TV token or link'),
        content: SizedBox(
          width: 560,
          child: TextField(
            controller: controller,
            autofocus: true,
            autocorrect: false,
            enableSuggestions: false,
            keyboardType: TextInputType.url,
            textInputAction: TextInputAction.done,
            style: const TextStyle(fontSize: 22),
            onSubmitted: (v) => Navigator.of(ctx).pop(v),
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Cancel')),
          TextButton(onPressed: () => Navigator.of(ctx).pop(controller.text), child: const Text('OK')),
        ],
      ),
    );
    controller.dispose();
    if (result != null) _value = result.trim();
  }

  Future<void> _connect() async {
    if (_busy) return;
    final parsed = parsePairing(_server, _token);
    if (!parsed.isOk) {
      setState(() => _error = parsed.error);
      return;
    }
    final config = parsed.config!;
    setState(() {
      _busy = true;
      _error = null;
      _info = null;
      _server = config.server;
      _token = config.token;
    });
    try {
      final (_, raw) = await widget.api.fetchData(config);
      await widget.onPaired(config, raw);
    } on TvApiException catch (e) {
      if (!mounted) return;
      setState(() => _error = e.kind == TvErrorKind.unauthorized
          ? 'Token rejected: it is wrong or was revoked. Create a new TV link in the web app.'
          : '${e.message} Check the server address and that this TV is on the same network.');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Hardware keyboards / remotes with keyboards: type straight into the active field.
  bool _backDown = false;

  KeyEventResult _onKey(FocusNode node, KeyEvent event) {
    final key = event.logicalKey;
    if (isBackKey(key)) {
      // Back: cancel re-pairing, or leave the app on first run.
      if (event is KeyDownEvent) {
        _backDown = true;
      } else if (event is KeyUpEvent && _backDown) {
        _backDown = false;
        if (widget.onCancel != null) {
          widget.onCancel!();
        } else {
          SystemNavigator.pop();
        }
      }
      return KeyEventResult.handled;
    }
    if (event is KeyUpEvent) return KeyEventResult.ignored;
    if (key == LogicalKeyboardKey.backspace) {
      _backspace();
      return KeyEventResult.handled;
    }
    final ch = event.character;
    if (ch != null && ch.length == 1 && ch.codeUnitAt(0) > 32 && ch.codeUnitAt(0) != 127) {
      _type(ch);
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: TvCanvas(
        child: Focus(
          onKeyEvent: _onKey,
          child: FocusTraversalGroup(
            policy: ReadingOrderTraversalPolicy(),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(flex: 5, child: _form()),
                const SizedBox(width: 24),
                Expanded(flex: 6, child: _keyboard()),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _form() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ShaderMask(
          shaderCallback: (r) => const LinearGradient(colors: [TvColors.primary, TvColors.brandEnd]).createShader(r),
          child: const Text('Kinkit TV',
              style: TextStyle(
                  fontFamily: tvSerif,
                  fontFamilyFallback: tvSerifFallback,
                  fontSize: 30,
                  fontWeight: FontWeight.w700,
                  color: Colors.white)),
        ),
        const SizedBox(height: 6),
        const Text(
          'In the Kinkit web app open Settings > Data & devices > TV / car display links, '
          'create a link, then enter the server and token here - or paste the whole link into either field.',
          style: TextStyle(fontSize: 13, color: TvColors.muted, height: 1.35),
        ),
        const SizedBox(height: 14),
        _fieldTile(_Field.server, 'Server address', 'e.g. https://kinkit.example.com', _server, autofocus: true),
        const SizedBox(height: 10),
        _fieldTile(_Field.token, 'TV token (or full TV link)', 'the part after ?k=', _token),
        const SizedBox(height: 14),
        Row(children: [
          TvButton(
            primary: true,
            onPressed: _connect,
            child: Text(_busy ? 'Connecting...' : 'Connect'),
          ),
          const SizedBox(width: 10),
          TvButton(onPressed: _systemKeyboard, child: const Text('System keyboard')),
        ]),
        const SizedBox(height: 12),
        if (_error != null)
          Text(_error!, style: const TextStyle(color: TvColors.negative, fontSize: 14, fontWeight: FontWeight.w600)),
        if (_error == null && _info != null)
          Text(_info!, style: const TextStyle(color: TvColors.warning, fontSize: 14, fontWeight: FontWeight.w600)),
        const Spacer(),
        const Text('View-only: this TV can never change your data.',
            style: TextStyle(fontSize: 12, color: TvColors.muted)),
      ],
    );
  }

  Widget _fieldTile(_Field f, String label, String hint, String value, {bool autofocus = false}) {
    final active = _active == f;
    return _FieldTile(
      autofocus: autofocus,
      label: label,
      hint: hint,
      value: value,
      active: active,
      onActivate: () => setState(() => _active = f),
      // Moving focus onto a field also makes it the keyboard target.
      onFocus: () => setState(() => _active = f),
    );
  }

  Widget _keyboard() {
    const rows = ['1234567890', 'qwertyuiop', 'asdfghjkl-', 'zxcvbnm_.:', '/?=&@~'];
    Widget key(String label, VoidCallback onTap, {double width = 34}) => Padding(
          padding: const EdgeInsets.all(2.5),
          child: TvButton(
            width: width,
            height: 34,
            padding: EdgeInsets.zero,
            onPressed: onTap,
            child: Text(label, style: const TextStyle(fontSize: 15)),
          ),
        );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Typing into: ${_active == _Field.server ? 'Server address' : 'TV token'}',
          style: const TextStyle(fontSize: 13, color: TvColors.accent, fontWeight: FontWeight.w700),
        ),
        const SizedBox(height: 6),
        for (final row in rows)
          Row(children: [
            for (final c in row.split(''))
              key(_upper ? c.toUpperCase() : c, () => _type(_upper ? c.toUpperCase() : c)),
          ]),
        Row(children: [
          key(_upper ? 'abc' : 'ABC', () => setState(() => _upper = !_upper), width: 56),
          key('Del', _backspace, width: 52),
          key('Clear', () => _value = '', width: 60),
          key('Paste', _paste, width: 60),
        ]),
        Row(children: [
          key('https://', () => _type('https://'), width: 82),
          key('http://', () => _type('http://'), width: 74),
          key('.com', () => _type('.com'), width: 56),
          key('/tv?k=', () => _type('/tv?k='), width: 70),
        ]),
      ],
    );
  }
}

class _FieldTile extends StatefulWidget {
  const _FieldTile({
    required this.label,
    required this.hint,
    required this.value,
    required this.active,
    required this.onActivate,
    required this.onFocus,
    this.autofocus = false,
  });

  final String label;
  final String hint;
  final String value;
  final bool active;
  final bool autofocus;
  final VoidCallback onActivate;
  final VoidCallback onFocus;

  @override
  State<_FieldTile> createState() => _FieldTileState();
}

class _FieldTileState extends State<_FieldTile> {
  bool _focused = false;

  @override
  Widget build(BuildContext context) {
    final borderColor = _focused ? TvColors.focus : (widget.active ? TvColors.primary : TvColors.border);
    return FocusableActionDetector(
      autofocus: widget.autofocus,
      onFocusChange: (f) {
        setState(() => _focused = f);
        if (f) widget.onFocus();
      },
      actions: <Type, Action<Intent>>{
        ActivateIntent: CallbackAction<ActivateIntent>(onInvoke: (_) {
          widget.onActivate();
          return null;
        }),
      },
      child: GestureDetector(
        onTap: widget.onActivate,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 120),
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          decoration: BoxDecoration(
            color: _focused ? TvColors.cardHi : TvColors.card,
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: borderColor, width: _focused ? 2.5 : 1.5),
          ),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text(widget.label, style: const TextStyle(fontSize: 12, color: TvColors.muted)),
            const SizedBox(height: 2),
            Text(
              widget.value.isEmpty ? widget.hint : '${widget.value}${widget.active ? '|' : ''}',
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 16,
                fontFamily: 'monospace',
                color: widget.value.isEmpty ? TvColors.border : TvColors.text,
              ),
            ),
          ]),
        ),
      ),
    );
  }
}
