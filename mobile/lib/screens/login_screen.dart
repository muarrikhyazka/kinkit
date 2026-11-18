import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../config.dart';
import '../state/providers.dart';
import '../theme.dart';
import '../widgets/kinkit_logo.dart';

class LoginScreen extends ConsumerStatefulWidget {
  /// Error from the initial session check (e.g. server unreachable).
  final String? startupError;

  const LoginScreen({super.key, this.startupError});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _form = GlobalKey<FormState>();
  final _email = TextEditingController();
  final _password = TextEditingController();
  final _name = TextEditingController();
  late final TextEditingController _server;
  bool _signup = false;
  bool _busy = false;
  bool _showServer = false;
  bool _obscure = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _server = TextEditingController(text: ref.read(baseUrlProvider));
    _error = widget.startupError;
    // Make the server field obvious when we couldn't even reach it.
    _showServer = widget.startupError != null;
  }

  @override
  void dispose() {
    _email.dispose();
    _password.dispose();
    _name.dispose();
    _server.dispose();
    super.dispose();
  }

  Future<void> _applyServer() async {
    final wanted = AppConfig.normalizeBaseUrl(_server.text);
    if (wanted == ref.read(baseUrlProvider)) return;
    await ref.read(baseUrlProvider.notifier).set(wanted);
    _server.text = wanted;
    // Wait for the session re-check against the new server so it can't
    // race with (and overwrite) the login below.
    try {
      await ref.read(authProvider.future);
    } catch (_) {}
  }

  Future<void> _testServer() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await _applyServer();
      await ref.read(apiProvider).ping();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Connected to ${ref.read(baseUrlProvider)}'), behavior: SnackBarBehavior.floating),
      );
    } catch (e) {
      if (mounted) setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _submit() async {
    if (!(_form.currentState?.validate() ?? false)) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await _applyServer();
      final auth = ref.read(authProvider.notifier);
      if (_signup) {
        await auth.signup(_email.text, _password.text, _name.text);
      } else {
        await auth.login(_email.text, _password.text);
      }
      // AuthGate swaps to the app when the session is set.
    } catch (e) {
      if (mounted) setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final server = ref.watch(baseUrlProvider);
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: Form(
                key: _form,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    const Center(child: KinkitLogo(size: 64)),
                    const SizedBox(height: 8),
                    Text('kinkit',
                        textAlign: TextAlign.center,
                        style: TextStyle(fontSize: 32, fontWeight: FontWeight.w800, color: p.text)),
                    Text('Family money & home, together',
                        textAlign: TextAlign.center, style: TextStyle(color: p.muted)),
                    const SizedBox(height: 28),
                    SegmentedButton<bool>(
                      segments: const [
                        ButtonSegment(value: false, label: Text('Log in')),
                        ButtonSegment(value: true, label: Text('Sign up')),
                      ],
                      selected: {_signup},
                      onSelectionChanged: (s) => setState(() {
                        _signup = s.first;
                        _error = null;
                      }),
                    ),
                    const SizedBox(height: 20),
                    if (_signup) ...[
                      TextFormField(
                        controller: _name,
                        textCapitalization: TextCapitalization.words,
                        textInputAction: TextInputAction.next,
                        decoration: const InputDecoration(labelText: 'Your name', prefixIcon: Icon(Icons.person_outline)),
                      ),
                      const SizedBox(height: 12),
                    ],
                    TextFormField(
                      controller: _email,
                      keyboardType: TextInputType.emailAddress,
                      autofillHints: const [AutofillHints.email, AutofillHints.username],
                      autocorrect: false,
                      textInputAction: TextInputAction.next,
                      decoration: const InputDecoration(labelText: 'Email', prefixIcon: Icon(Icons.alternate_email)),
                      validator: (v) {
                        final s = (v ?? '').trim();
                        if (s.isEmpty) return 'Enter your email';
                        if (_signup && !s.contains('@')) return 'Enter a valid email';
                        return null;
                      },
                    ),
                    const SizedBox(height: 12),
                    TextFormField(
                      controller: _password,
                      obscureText: _obscure,
                      autofillHints: const [AutofillHints.password],
                      textInputAction: TextInputAction.done,
                      onFieldSubmitted: (_) => _busy ? null : _submit(),
                      decoration: InputDecoration(
                        labelText: 'Password',
                        prefixIcon: const Icon(Icons.lock_outline),
                        suffixIcon: IconButton(
                          icon: Icon(_obscure ? Icons.visibility_outlined : Icons.visibility_off_outlined),
                          onPressed: () => setState(() => _obscure = !_obscure),
                        ),
                      ),
                      validator: (v) {
                        final s = v ?? '';
                        if (s.isEmpty) return 'Enter your password';
                        if (_signup && s.length < 8) return 'At least 8 characters';
                        return null;
                      },
                    ),
                    if (_error != null) ...[
                      const SizedBox(height: 14),
                      Container(
                        padding: const EdgeInsets.all(12),
                        decoration: BoxDecoration(
                          color: p.danger.withValues(alpha: 0.1),
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: Row(
                          children: [
                            Expanded(child: Text(_error!, style: TextStyle(color: p.danger))),
                            if (widget.startupError != null && _error == widget.startupError)
                              TextButton(
                                onPressed: () => ref.invalidate(authProvider),
                                child: const Text('Retry'),
                              ),
                          ],
                        ),
                      ),
                    ],
                    const SizedBox(height: 20),
                    FilledButton(
                      onPressed: _busy ? null : _submit,
                      child: _busy
                          ? SizedBox(
                              height: 20,
                              width: 20,
                              child: CircularProgressIndicator(
                                  strokeWidth: 2, color: Theme.of(context).colorScheme.onPrimary))
                          : Text(_signup ? 'Create household' : 'Log in'),
                    ),
                    const SizedBox(height: 24),
                    InkWell(
                      onTap: () => setState(() => _showServer = !_showServer),
                      child: Padding(
                        padding: const EdgeInsets.symmetric(vertical: 8),
                        child: Row(
                          children: [
                            Icon(Icons.dns_outlined, size: 18, color: p.muted),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Text('Server: $server',
                                  overflow: TextOverflow.ellipsis, style: TextStyle(color: p.muted, fontSize: 13)),
                            ),
                            Icon(_showServer ? Icons.expand_less : Icons.expand_more, color: p.muted),
                          ],
                        ),
                      ),
                    ),
                    if (_showServer) ...[
                      const SizedBox(height: 8),
                      TextFormField(
                        controller: _server,
                        keyboardType: TextInputType.url,
                        autocorrect: false,
                        decoration: const InputDecoration(
                          labelText: 'Server address',
                          hintText: 'https://kinkit.example.com',
                          helperText: 'The address you open kinkit at in a browser',
                        ),
                      ),
                      const SizedBox(height: 8),
                      Align(
                        alignment: Alignment.centerRight,
                        child: TextButton.icon(
                          onPressed: _busy ? null : _testServer,
                          icon: const Icon(Icons.wifi_tethering),
                          label: const Text('Save & test connection'),
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
