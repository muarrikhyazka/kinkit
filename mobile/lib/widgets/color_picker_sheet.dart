import 'package:flutter/material.dart';

import '../theme.dart';
import 'widget_colors.dart';

/// Bottom sheet to choose a widget / board-background colour: preset swatches
/// plus a hex field for a custom one. Pops a `#rrggbb` string, or '' for
/// "Default"; dismissing the sheet pops null (no change).
Future<String?> showColorPickerSheet(BuildContext context, {required String title, String? current}) =>
    showModalBottomSheet<String>(
      context: context,
      showDragHandle: true,
      isScrollControlled: true,
      builder: (_) => ColorPickerSheet(title: title, current: current),
    );

class ColorPickerSheet extends StatefulWidget {
  final String title;
  final String? current;
  const ColorPickerSheet({super.key, required this.title, this.current});

  @override
  State<ColorPickerSheet> createState() => _ColorPickerSheetState();
}

class _ColorPickerSheetState extends State<ColorPickerSheet> {
  late final TextEditingController _hex = TextEditingController(text: normalizeHexColor(widget.current) ?? '');
  String? _error;

  @override
  void dispose() {
    _hex.dispose();
    super.dispose();
  }

  void _applyCustom() {
    final h = normalizeHexColor(_hex.text);
    if (h == null) {
      setState(() => _error = 'Use a hex colour like #4f46e5');
      return;
    }
    Navigator.pop(context, h);
  }

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final cur = normalizeHexColor(widget.current) ?? '';
    final preview = parseHexColor(_hex.text);
    return SafeArea(
      child: Padding(
        padding: EdgeInsets.fromLTRB(16, 0, 16, 16 + MediaQuery.of(context).viewInsets.bottom),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(widget.title, style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700)),
              const SizedBox(height: 12),
              Wrap(spacing: 12, runSpacing: 12, children: [
                for (final sw in kWidgetPalette)
                  Semantics(
                    button: true,
                    selected: sw.hex == cur,
                    label: sw.name,
                    child: InkWell(
                      key: ValueKey('swatch-${sw.name}'),
                      customBorder: const CircleBorder(),
                      onTap: () => Navigator.pop(context, sw.hex),
                      child: Container(
                        width: 44,
                        height: 44,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          color: parseHexColor(sw.hex) ?? p.card,
                          border: Border.all(color: sw.hex == cur ? p.primary : p.borderStrong, width: sw.hex == cur ? 3 : 1),
                        ),
                        child: sw.hex.isEmpty
                            ? Icon(Icons.block, size: 20, color: p.muted)
                            : (sw.hex == cur ? Icon(Icons.check, size: 22, color: readableOn(parseHexColor(sw.hex)!)) : null),
                      ),
                    ),
                  ),
              ]),
              const SizedBox(height: 16),
              Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Container(
                  width: 40,
                  height: 40,
                  margin: const EdgeInsets.only(top: 4, right: 12),
                  decoration: BoxDecoration(shape: BoxShape.circle, color: preview ?? p.card, border: Border.all(color: p.borderStrong)),
                ),
                Expanded(
                  child: TextField(
                    controller: _hex,
                    autocorrect: false,
                    textCapitalization: TextCapitalization.none,
                    decoration: InputDecoration(labelText: 'Custom colour (hex)', hintText: '#4f46e5', errorText: _error),
                    onChanged: (_) => setState(() => _error = null),
                    onSubmitted: (_) => _applyCustom(),
                  ),
                ),
                const SizedBox(width: 8),
                Padding(padding: const EdgeInsets.only(top: 6), child: FilledButton(onPressed: _applyCustom, child: const Text('Use'))),
              ]),
            ],
          ),
        ),
      ),
    );
  }
}
