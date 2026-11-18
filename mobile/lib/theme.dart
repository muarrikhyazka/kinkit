import 'package:flutter/material.dart';

/// Brand palette: Persian Blue #16284C, Cream #EFE8DA, Amber #FFB92E.
///
/// Light: cream background, warm white cards, navy text/primary, amber accent
/// (FAB / primary call to action, navy foreground). Dark: deep navy
/// background, Persian Blue surfaces, cream text, amber primary.
const kPersianBlue = Color(0xFF16284C);
const kCream = Color(0xFFEFE8DA);
const kAmber = Color(0xFFFFB92E);

/// Serif used for headlines and the brand wordmark. No bundled font: Georgia
/// on iOS, the platform serif (Noto Serif) on Android.
const kSerifFamily = 'Georgia';
const kSerifFallback = <String>['serif', 'Noto Serif', 'Times New Roman'];

/// Headline / brand text style in the serif face.
TextStyle serifStyle({double? fontSize, FontWeight fontWeight = FontWeight.w700, Color? color}) => TextStyle(
      fontFamily: kSerifFamily,
      fontFamilyFallback: kSerifFallback,
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
    );
class KinkitPalette extends ThemeExtension<KinkitPalette> {
  final Color bg;
  final Color card;
  final Color border;
  final Color text;
  final Color muted;
  final Color primary;
  final Color primaryLight;
  final Color success;
  final Color danger;
  final Color warning;
  final Color accent;
  final Color onAccent;

  const KinkitPalette({
    required this.bg,
    required this.card,
    required this.border,
    required this.text,
    required this.muted,
    required this.primary,
    required this.primaryLight,
    required this.success,
    required this.danger,
    required this.warning,
    required this.accent,
    required this.onAccent,
  });

  static const light = KinkitPalette(
    bg: kCream,
    card: Color(0xFFFBF8F1),
    border: Color(0xFFDDD3C0),
    text: kPersianBlue,
    muted: Color(0xFF5A6377),
    primary: kPersianBlue,
    primaryLight: Color(0xFFE2E5EC),
    success: Color(0xFF2E7D4F),
    danger: Color(0xFFB3261E),
    warning: Color(0xFF8F5A00),
    accent: kAmber,
    onAccent: kPersianBlue,
  );

  static const dark = KinkitPalette(
    bg: Color(0xFF0D1830),
    card: kPersianBlue,
    border: Color(0xFF2A3D66),
    text: kCream,
    muted: Color(0xFFA9B1C2),
    primary: kAmber,
    primaryLight: Color(0xFF233760),
    success: Color(0xFF6FCF97),
    danger: Color(0xFFF2877E),
    warning: kAmber,
    accent: kAmber,
    onAccent: kPersianBlue,
  );

  @override
  KinkitPalette copyWith({
    Color? bg,
    Color? card,
    Color? border,
    Color? text,
    Color? muted,
    Color? primary,
    Color? primaryLight,
    Color? success,
    Color? danger,
    Color? warning,
    Color? accent,
    Color? onAccent,
  }) =>
      KinkitPalette(
        bg: bg ?? this.bg,
        card: card ?? this.card,
        border: border ?? this.border,
        text: text ?? this.text,
        muted: muted ?? this.muted,
        primary: primary ?? this.primary,
        primaryLight: primaryLight ?? this.primaryLight,
        success: success ?? this.success,
        danger: danger ?? this.danger,
        warning: warning ?? this.warning,
        accent: accent ?? this.accent,
        onAccent: onAccent ?? this.onAccent,
      );

  @override
  KinkitPalette lerp(ThemeExtension<KinkitPalette>? other, double t) {
    if (other is! KinkitPalette) return this;
    Color l(Color a, Color b) => Color.lerp(a, b, t)!;
    return KinkitPalette(
      bg: l(bg, other.bg),
      card: l(card, other.card),
      border: l(border, other.border),
      text: l(text, other.text),
      muted: l(muted, other.muted),
      primary: l(primary, other.primary),
      primaryLight: l(primaryLight, other.primaryLight),
      success: l(success, other.success),
      danger: l(danger, other.danger),
      warning: l(warning, other.warning),
      accent: l(accent, other.accent),
      onAccent: l(onAccent, other.onAccent),
    );
  }
}

extension KinkitThemeX on BuildContext {
  KinkitPalette get palette =>
      Theme.of(this).extension<KinkitPalette>() ?? KinkitPalette.light;
}

/// Colors for the Needs / Wants / Savings groups (as on the web dashboard).
Color mainCategoryColor(String main) {
  switch (main) {
    case 'Needs':
      return const Color(0xFF3D5A99);
    case 'Wants':
      return const Color(0xFFE0A21F);
    case 'Savings':
      return const Color(0xFF2E7D4F);
  }
  return const Color(0xFF7C8598);
}

/// Stable per-category color (same idea as react/src/categoryColor.js).
Color categoryColor(String? name) {
  const colors = [
    Color(0xFF3D5A99), Color(0xFFE0A21F), Color(0xFF2E7D4F), Color(0xFF6F86B8),
    Color(0xFFB3261E), Color(0xFF8B6F47), Color(0xFFC2410C), Color(0xFF0F766E),
    Color(0xFF7A5FA8), Color(0xFF7C8598),
  ];
  final s = name ?? 'Other';
  var h = 0;
  for (final c in s.codeUnits) {
    h = (h * 31 + c) & 0x7fffffff;
  }
  return colors[h % colors.length];
}

ThemeData buildTheme(Brightness brightness) {
  final dark = brightness == Brightness.dark;
  final p = dark ? KinkitPalette.dark : KinkitPalette.light;
  // Light: navy primary on cream. Dark: amber primary with navy on-primary.
  final onPrimary = dark ? kPersianBlue : kCream;
  final scheme = ColorScheme.fromSeed(seedColor: kPersianBlue, brightness: brightness).copyWith(
    primary: p.primary,
    onPrimary: onPrimary,
    primaryContainer: p.primaryLight,
    onPrimaryContainer: p.text,
    secondary: p.accent,
    onSecondary: p.onAccent,
    secondaryContainer: dark ? const Color(0xFF3A3420) : const Color(0xFFFFE7B3),
    onSecondaryContainer: dark ? kCream : kPersianBlue,
    tertiary: p.accent,
    onTertiary: p.onAccent,
    surface: p.card,
    onSurface: p.text,
    onSurfaceVariant: p.muted,
    surfaceContainerLowest: p.card,
    surfaceContainerLow: p.card,
    surfaceContainer: p.card,
    surfaceContainerHigh: p.card,
    surfaceContainerHighest: p.primaryLight,
    outline: p.border,
    outlineVariant: p.border,
    error: p.danger,
  );
  final radius = BorderRadius.circular(12);
  final base = ThemeData(useMaterial3: true, brightness: brightness).textTheme;
  TextStyle? serif(TextStyle? s) =>
      s?.copyWith(fontFamily: kSerifFamily, fontFamilyFallback: kSerifFallback, color: p.text);
  final textTheme = base
      .apply(bodyColor: p.text, displayColor: p.text)
      .copyWith(
        displayLarge: serif(base.displayLarge),
        displayMedium: serif(base.displayMedium),
        displaySmall: serif(base.displaySmall),
        headlineLarge: serif(base.headlineLarge),
        headlineMedium: serif(base.headlineMedium),
        headlineSmall: serif(base.headlineSmall),
      );
  // Bottom nav: navy bar (both modes), cream labels, amber pill behind a navy
  // selected icon. Cream on navy and navy on amber are both well above AA.
  final navBg = dark ? const Color(0xFF0A1428) : kPersianBlue;
  return ThemeData(
    useMaterial3: true,
    brightness: brightness,
    colorScheme: scheme,
    scaffoldBackgroundColor: p.bg,
    canvasColor: p.bg,
    textTheme: textTheme,
    extensions: [p],
    appBarTheme: AppBarTheme(
      backgroundColor: p.bg,
      surfaceTintColor: Colors.transparent,
      foregroundColor: p.text,
      elevation: 0,
      scrolledUnderElevation: 0.5,
      centerTitle: false,
      titleTextStyle: serifStyle(fontSize: 22, color: p.text),
    ),
    cardTheme: CardThemeData(
      color: p.card,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
      shape: RoundedRectangleBorder(borderRadius: radius, side: BorderSide(color: p.border)),
    ),
    dialogTheme: DialogThemeData(
      backgroundColor: p.card,
      surfaceTintColor: Colors.transparent,
      titleTextStyle: serifStyle(fontSize: 22, color: p.text),
    ),
    bottomSheetTheme: BottomSheetThemeData(backgroundColor: p.card, surfaceTintColor: Colors.transparent),
    navigationBarTheme: NavigationBarThemeData(
      backgroundColor: navBg,
      surfaceTintColor: Colors.transparent,
      indicatorColor: kAmber,
      iconTheme: WidgetStateProperty.resolveWith((states) => IconThemeData(
            color: states.contains(WidgetState.selected) ? kPersianBlue : kCream.withValues(alpha: 0.85),
          )),
      labelTextStyle: WidgetStateProperty.resolveWith((states) => TextStyle(
            fontSize: 12,
            color: states.contains(WidgetState.selected) ? kAmber : kCream,
            fontWeight: states.contains(WidgetState.selected) ? FontWeight.w700 : FontWeight.w500,
          )),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: p.card,
      border: OutlineInputBorder(borderRadius: BorderRadius.circular(8), borderSide: BorderSide(color: p.border)),
      enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(8), borderSide: BorderSide(color: p.border)),
      focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(8), borderSide: BorderSide(color: p.primary, width: 2)),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: p.primary,
        foregroundColor: onPrimary,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
      ),
    ),
    textButtonTheme: TextButtonThemeData(style: TextButton.styleFrom(foregroundColor: p.primary)),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(foregroundColor: p.primary, side: BorderSide(color: p.border)),
    ),
    segmentedButtonTheme: SegmentedButtonThemeData(
      style: ButtonStyle(
        backgroundColor: WidgetStateProperty.resolveWith(
            (s) => s.contains(WidgetState.selected) ? p.accent : Colors.transparent),
        foregroundColor: WidgetStateProperty.resolveWith(
            (s) => s.contains(WidgetState.selected) ? p.onAccent : p.text),
        side: WidgetStatePropertyAll(BorderSide(color: p.border)),
      ),
    ),
    floatingActionButtonTheme: FloatingActionButtonThemeData(
      backgroundColor: p.accent,
      foregroundColor: p.onAccent,
    ),
    progressIndicatorTheme: ProgressIndicatorThemeData(color: p.accent, linearTrackColor: p.primaryLight),
    switchTheme: SwitchThemeData(
      thumbColor: WidgetStateProperty.resolveWith((s) => s.contains(WidgetState.selected) ? p.onAccent : null),
      trackColor: WidgetStateProperty.resolveWith((s) => s.contains(WidgetState.selected) ? p.accent : null),
    ),
    dividerTheme: DividerThemeData(color: p.border, space: 1),
    chipTheme: ChipThemeData(side: BorderSide(color: p.border)),
  );
}
