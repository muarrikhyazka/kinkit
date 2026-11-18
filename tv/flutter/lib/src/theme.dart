import 'package:flutter/material.dart';

/// Kinkit brand on a 10-foot dark canvas: deep navy background, Persian Blue
/// (#16284C) cards, cream (#EFE8DA) text, amber (#FFB92E) accents, progress
/// and focus rings. Kept in sync with tv/web/styles.css.
class TvColors {
  static const bg = Color(0xFF0D1830);
  static const card = Color(0xFF16284C);
  static const cardHi = Color(0xFF1F3560);
  static const border = Color(0xFF2A3D66);
  static const track = Color(0xFF0F1D3A);
  static const text = Color(0xFFEFE8DA);
  static const muted = Color(0xFFA9B1C2);
  static const primary = Color(0xFFFFB92E); // amber
  static const onPrimary = Color(0xFF16284C);
  static const accent = Color(0xFF9DB4E3); // light navy tint (panel titles, savings)
  static const brandEnd = Color(0xFFFFD98A);
  static const button = Color(0xFF2A4A8A);
  static const positive = Color(0xFF6FCF97);
  static const negative = Color(0xFFF2877E);
  static const warning = Color(0xFFFF9D5C);
  static const focus = Color(0xFFFFB92E);
  // Trend chart: navy tint for income, amber for spending.
  static const chartIncome = Color(0xFF8FA8DB);
  static const chartSpent = Color(0xFFFFB92E);
}

/// Serif for the wordmark and headlines (platform serif, nothing bundled).
const tvSerif = 'serif';
const tvSerifFallback = <String>['Noto Serif', 'Georgia', 'Times New Roman'];

ThemeData buildTvTheme() {
  final base = ThemeData(
    useMaterial3: true,
    brightness: Brightness.dark,
    scaffoldBackgroundColor: TvColors.bg,
    colorScheme: const ColorScheme.dark(
      primary: TvColors.primary,
      onPrimary: TvColors.onPrimary,
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
  const TvCanvas({super.key, required this.child});
  final Widget child;

  static const width = 960.0;
  static const height = 540.0;

  @override
  Widget build(BuildContext context) {
    return ColoredBox(
      color: TvColors.bg,
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
