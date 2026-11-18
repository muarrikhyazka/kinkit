import 'package:flutter/material.dart';

import '../theme.dart';

/// "Go somewhere" text button with a trailing chevron (web ChevronLink.jsx).
/// Never underlined; at least 40px tall.
class ChevronLink extends StatelessWidget {
  final String label;
  final VoidCallback? onPressed;

  const ChevronLink(this.label, {super.key, this.onPressed});

  @override
  Widget build(BuildContext context) {
    return TextButton(
      onPressed: onPressed,
      style: TextButton.styleFrom(
        padding: const EdgeInsets.only(left: KSpace.s3, right: KSpace.s1),
        minimumSize: const Size(48, 40),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(label),
          const SizedBox(width: 2),
          const Icon(Icons.chevron_right, size: 20),
        ],
      ),
    );
  }
}
