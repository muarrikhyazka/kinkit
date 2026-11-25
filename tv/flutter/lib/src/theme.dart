import 'package:flutter/material.dart';

/// Navy surfaces mirror the web TV view (TVView.jsx). Text and focus use the
/// shared dark primary #818CF8; a focused or filled control is #4F46E5 with
/// white text, as on web and phone.
class TvColors {
  static const bg = Color(0xFF0B0E1A);
  static const card = Color(0xFF151A2E);
  static const cardHi = Color(0xFF1C2238);
  static const border = Color(0xFF262D47);
  static const text = Color(0xFFF1F3F8);
  static const muted = Color(0xFF9CA3AF);
  static const primary = Color(0xFF818CF8);

  /// Fill behind white text (focused control): 6.3:1, where white on [primary] is 2.98:1.
  static const primaryFill = Color(0xFF4F46E5);
  static const accent = Color(0xFFC4B5FD);
  static const positive = Color(0xFF4ADE80);
  static const negative = Color(0xFFF87171);
  static const warning = Color(0xFFFBBF24);
  static const focus = Color(0xFFFFFFFF);

  /// Purple chrome (web --chrome-*): header bar #4F46E5 -> #4338CA, white
  /// wordmark, lavender secondary text.
  static const chrome = Color(0xFF4F46E5);
  static const onChrome = Color(0xFFFFFFFF);
  static const chromeSoft = Color(0xFFE0E7FF);
}

/// The purple header bar with the white logo tile and white "Kinkit"
/// wordmark (web top bar / sidebar). [trailing] sits on the right.
class TvBrandBar extends StatelessWidget {
  const TvBrandBar({super.key, required this.wordmark, required this.logo, this.children = const [], this.height = 52});
  final String wordmark;
  final Widget logo;
  final List<Widget> children;
  final double height;

  @override
  Widget build(BuildContext context) {
    return Container(
      height: height,
      padding: const EdgeInsets.symmetric(horizontal: 14),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(14),
        color: TvColors.chrome,
      ),
      child: Row(crossAxisAlignment: CrossAxisAlignment.center, children: [
        logo,
        const SizedBox(width: 10),
        Text(wordmark,
            style: const TextStyle(fontSize: 26, fontWeight: FontWeight.w700, color: TvColors.onChrome, height: 1.0)),
        ...children,
      ]),
    );
  }
}

ThemeData buildTvTheme() {
  final base = ThemeData(
    useMaterial3: true,
    brightness: Brightness.dark,
    scaffoldBackgroundColor: TvColors.bg,
    colorScheme: const ColorScheme.dark(
      primary: TvColors.primary,
      secondary: TvColors.accent,
      surface: TvColors.card,
      error: TvColors.negative,
    ),
  );
  return base.copyWith(
    textTheme: base.textTheme.apply(bodyColor: TvColors.text, displayColor: TvColors.text),
  );
}

/// Android TV overscan-safe margins (48dp x 27dp on a 960x540dp canvas).
const tvSafePadding = EdgeInsets.symmetric(horizontal: 48, vertical: 27);

/// Lays [child] out on a fixed 960x540 logical canvas and scales it to the
/// screen, so the 10-foot layout is identical on 720p/1080p/4K boxes
/// regardless of the density each box reports.
class TvCanvas extends StatelessWidget {
  const TvCanvas({super.key, required this.child, this.background});
  final Widget child;
  final Color? background; // widget-board backdrop; null = default

  static const width = 960.0;
  static const height = 540.0;

  @override
  Widget build(BuildContext context) {
    return ColoredBox(
      color: background ?? TvColors.bg,
      child: SizedBox.expand(
        child: FittedBox(
          fit: BoxFit.contain,
          child: SizedBox(
            width: width,
            height: height,
            child: Padding(padding: tvSafePadding, child: child),
          ),
        ),
      ),
    );
  }
}
