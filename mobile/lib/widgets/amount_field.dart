import 'package:flutter/material.dart';

import '../theme.dart';
import '../util/money.dart';

/// Amount input in IDR (amounts are stored in IDR, like the web's
/// MoneyInput). Accepts "25000", "25.000" and shorthand "25k", "2rb",
/// "1.5jt"; shows the resolved value underneath.
class AmountField extends StatefulWidget {
  final TextEditingController controller;
  final String label;
  final bool autofocus;
  final bool allowNegative;
  final TextInputAction? textInputAction;

  const AmountField({
    super.key,
    required this.controller,
    this.label = 'Amount (Rp)',
    this.autofocus = false,
    this.allowNegative = false,
    this.textInputAction,
  });

  @override
  State<AmountField> createState() => _AmountFieldState();
}

class _AmountFieldState extends State<AmountField> {
  @override
  void initState() {
    super.initState();
    widget.controller.addListener(_changed);
  }

  @override
  void dispose() {
    widget.controller.removeListener(_changed);
    super.dispose();
  }

  void _changed() => setState(() {});

  @override
  Widget build(BuildContext context) {
    final v = parseAmount(widget.controller.text);
    final text = widget.controller.text.trim();
    return TextFormField(
      controller: widget.controller,
      autofocus: widget.autofocus,
      textInputAction: widget.textInputAction,
      keyboardType: TextInputType.text,
      decoration: InputDecoration(
        labelText: widget.label,
        prefixText: 'Rp ',
        helperText: text.isEmpty ? 'Tip: 25k, 2rb, 1.5jt' : (v != null ? '= ${MoneyFormatter.idr(v)}' : null),
        helperStyle: TextStyle(color: context.palette.muted),
      ),
      validator: (s) {
        final n = parseAmount(s ?? '');
        if (n == null) return 'Enter an amount';
        if (n == 0) return 'Amount must not be zero';
        if (n < 0 && !widget.allowNegative) return 'Amount must be positive';
        return null;
      },
    );
  }
}
