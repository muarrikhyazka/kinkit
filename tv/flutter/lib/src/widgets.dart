import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'theme.dart';

/// D-pad friendly button: big focus ring + slight zoom when focused,
/// activates on OK/Enter/Select (ActivateIntent) and on tap.
class TvButton extends StatefulWidget {
  const TvButton({
    super.key,
    required this.onPressed,
    required this.child,
    this.autofocus = false,
    this.focusNode,
    this.width,
    this.height = 40,
    this.primary = false,
    this.padding = const EdgeInsets.symmetric(horizontal: 16),
  });

  final VoidCallback onPressed;
  final Widget child;
  final bool autofocus;
  final FocusNode? focusNode;
  final double? width;
  final double height;
  final bool primary;
  final EdgeInsetsGeometry padding;

  @override
  State<TvButton> createState() => _TvButtonState();
}

class _TvButtonState extends State<TvButton> {
  bool _focused = false;

  @override
  Widget build(BuildContext context) {
    final bg = _focused
        ? TvColors.primary
        : (widget.primary ? TvColors.button : TvColors.cardHi);
    return FocusableActionDetector(
      autofocus: widget.autofocus,
      focusNode: widget.focusNode,
      onFocusChange: (f) => setState(() => _focused = f),
      actions: <Type, Action<Intent>>{
        ActivateIntent: CallbackAction<ActivateIntent>(onInvoke: (_) {
          widget.onPressed();
          return null;
        }),
      },
      child: GestureDetector(
        onTap: widget.onPressed,
        child: AnimatedScale(
          scale: _focused ? 1.08 : 1.0,
          duration: const Duration(milliseconds: 120),
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 120),
            width: widget.width,
            height: widget.height,
            padding: widget.padding,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: bg,
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: _focused ? TvColors.focus : TvColors.border, width: _focused ? 2.5 : 1),
              boxShadow: _focused
                  ? const [BoxShadow(color: Color(0x88FFB92E), blurRadius: 12, spreadRadius: 1)]
                  : const [],
            ),
            child: DefaultTextStyle.merge(
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w600,
                color: _focused ? TvColors.onPrimary : TvColors.text,
              ),
              child: widget.child,
            ),
          ),
        ),
      ),
    );
  }
}

/// Keys that mean "OK" on the various remotes.
bool isSelectKey(LogicalKeyboardKey k) =>
    k == LogicalKeyboardKey.select ||
    k == LogicalKeyboardKey.enter ||
    k == LogicalKeyboardKey.numpadEnter ||
    k == LogicalKeyboardKey.gameButtonA;

/// Keys that mean "Back".
bool isBackKey(LogicalKeyboardKey k) =>
    k == LogicalKeyboardKey.goBack ||
    k == LogicalKeyboardKey.escape ||
    k == LogicalKeyboardKey.browserBack ||
    k == LogicalKeyboardKey.gameButtonB;

/// Keys that open settings directly.
bool isMenuKey(LogicalKeyboardKey k) =>
    k == LogicalKeyboardKey.contextMenu ||
    k == LogicalKeyboardKey.info ||
    k == LogicalKeyboardKey.colorF0Red ||
    k == LogicalKeyboardKey.keyM ||
    k == LogicalKeyboardKey.keyS;

class StatusBadge extends StatelessWidget {
  const StatusBadge({super.key, required this.label, required this.color, this.icon});
  final String label;
  final Color color;
  final IconData? icon;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: color.withAlpha(40),
        borderRadius: BorderRadius.circular(20),
        border: Border.all(color: color, width: 1.5),
      ),
      child: Row(mainAxisSize: MainAxisSize.min, children: [
        if (icon != null) ...[Icon(icon, size: 14, color: color), const SizedBox(width: 5)],
        Text(label, style: TextStyle(color: color, fontSize: 13, fontWeight: FontWeight.w700)),
      ]),
    );
  }
}
