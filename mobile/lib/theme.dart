import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// Brand palette - the CSS tokens from react/src/index.css (:root and
/// [data-theme="dark"]).
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
  final Color borderStrong;
  final Color surface2;
  final Color onPrimary;
  final Color dangerLight;

  /// App bar / chrome (web --chrome-*): purple bar, white text, lavender
  /// secondary text.
  final Color chrome;
  final Color onChrome;
  final Color chromeSoft;

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
    required this.borderStrong,
    required this.surface2,
    required this.onPrimary,
    required this.dangerLight,
    required this.chrome,
    this.onChrome = const Color(0xFFFFFFFF),
    this.chromeSoft = const Color(0xFFE0E7FF),
  });

  static const light = KinkitPalette(
    bg: Color(0xFFF5F6FA),
    card: Color(0xFFFFFFFF),
    border: Color(0xFFE4E6EE),
    text: Color(0xFF1B1F2B),
    muted: Color(0xFF5D6475),
    primary: Color(0xFF4F46E5),
    primaryLight: Color(0xFFEEF2FF),
    success: Color(0xFF15803D),
    danger: Color(0xFFDC2626),
    warning: Color(0xFFB45309),
    borderStrong: Color(0xFFCFD3DF),
    surface2: Color(0xFFF1F2F7),
    onPrimary: Color(0xFFFFFFFF),
    dangerLight: Color(0xFFFEF2F2),
    chrome: Color(0xFF4F46E5),
  );

  static const dark = KinkitPalette(
    bg: Color(0xFF0F1220),
    card: Color(0xFF171B2D),
    border: Color(0xFF2A3049),
    text: Color(0xFFE7E9F0),
    muted: Color(0xFFA0A7B8),
    primary: Color(0xFF818CF8),
    primaryLight: Color(0xFF262B47),
    success: Color(0xFF22C55E),
    danger: Color(0xFFF87171),
    warning: Color(0xFFFBBF24),
    borderStrong: Color(0xFF3A4161),
    surface2: Color(0xFF1F2439),
    onPrimary: Color(0xFF0F1220),
    dangerLight: Color(0xFF3A1D24),
    chrome: Color(0xFF312E81),
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
    Color? borderStrong,
    Color? surface2,
    Color? onPrimary,
    Color? dangerLight,
    Color? chrome,
    Color? onChrome,
    Color? chromeSoft,
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
        borderStrong: borderStrong ?? this.borderStrong,
        surface2: surface2 ?? this.surface2,
        onPrimary: onPrimary ?? this.onPrimary,
        dangerLight: dangerLight ?? this.dangerLight,
        chrome: chrome ?? this.chrome,
        onChrome: onChrome ?? this.onChrome,
        chromeSoft: chromeSoft ?? this.chromeSoft,
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
      borderStrong: l(borderStrong, other.borderStrong),
      surface2: l(surface2, other.surface2),
      onPrimary: l(onPrimary, other.onPrimary),
      dangerLight: l(dangerLight, other.dangerLight),
      chrome: l(chrome, other.chrome),
      onChrome: l(onChrome, other.onChrome),
      chromeSoft: l(chromeSoft, other.chromeSoft),
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
      return const Color(0xFF4F46E5);
    case 'Wants':
      return const Color(0xFFF59E0B);
    case 'Savings':
      return const Color(0xFF16A34A);
  }
  return const Color(0xFF6B7280);
}

/// Stable per-category color (same idea as react/src/categoryColor.js).
Color categoryColor(String? name) {
  const colors = [
    Color(0xFF4F46E5), Color(0xFF0EA5E9), Color(0xFF16A34A), Color(0xFFF59E0B),
    Color(0xFFDC2626), Color(0xFF8B5CF6), Color(0xFFEC4899), Color(0xFF14B8A6),
    Color(0xFFF97316), Color(0xFF64748B),
  ];
  final s = name ?? 'Other';
  var h = 0;
  for (final c in s.codeUnits) {
    h = (h * 31 + c) & 0x7fffffff;
  }
  return colors[h % colors.length];
}

/// Spacing scale (web --s-1..6).
class KSpace {
  static const s1 = 4.0;
  static const s2 = 8.0;
  static const s3 = 12.0;
  static const s4 = 16.0;
  static const s5 = 24.0;
  static const s6 = 32.0;
}

/// Radii (web --r-*): sm for tags/icon buttons, md for buttons, inputs and
/// rows, lg for cards, sheets and dialogs.
class KRadius {
  static const sm = 6.0;
  static const md = 10.0;
  static const lg = 14.0;
}

/// Tabular figures so amounts line up (web: font-variant-numeric: tabular-nums).
const kTabular = [FontFeature.tabularFigures()];

/// Status bar icons for a screen without the purple app bar.
SystemUiOverlayStyle plainOverlayStyle(Brightness b) =>
    (b == Brightness.dark ? SystemUiOverlayStyle.light : SystemUiOverlayStyle.dark)
        .copyWith(statusBarColor: Colors.transparent);

ThemeData buildTheme(Brightness brightness) {
  final p = brightness == Brightness.dark ? KinkitPalette.dark : KinkitPalette.light;
  final scheme = ColorScheme.fromSeed(seedColor: p.primary, brightness: brightness).copyWith(
    primary: p.primary,
    onPrimary: p.onPrimary,
    primaryContainer: p.primaryLight,
    onPrimaryContainer: p.text,
    surface: p.card,
    onSurface: p.text,
    onSurfaceVariant: p.muted,
    surfaceContainerHighest: p.surface2,
    outline: p.borderStrong,
    outlineVariant: p.border,
    error: p.danger,
  );
  final md = BorderRadius.circular(KRadius.md);
  final lg = BorderRadius.circular(KRadius.lg);
  final base = ThemeData(useMaterial3: true, brightness: brightness, colorScheme: scheme);
  TextStyle? tab(TextStyle? t) => t?.copyWith(fontFeatures: kTabular);
  final text = base.textTheme;
  return base.copyWith(
    scaffoldBackgroundColor: p.bg,
    extensions: [p],
    textTheme: text.copyWith(
      displayLarge: tab(text.displayLarge),
      displayMedium: tab(text.displayMedium),
      displaySmall: tab(text.displaySmall),
      headlineLarge: tab(text.headlineLarge),
      headlineMedium: tab(text.headlineMedium),
      headlineSmall: tab(text.headlineSmall),
      titleLarge: tab(text.titleLarge),
      titleMedium: tab(text.titleMedium),
      titleSmall: tab(text.titleSmall),
      bodyLarge: tab(text.bodyLarge),
      bodyMedium: tab(text.bodyMedium),
      bodySmall: tab(text.bodySmall),
      labelLarge: tab(text.labelLarge),
      labelMedium: tab(text.labelMedium),
      labelSmall: tab(text.labelSmall),
    ),
    // Purple chrome like the web top bar / sidebar; light status bar icons.
    appBarTheme: AppBarTheme(
      backgroundColor: p.chrome,
      foregroundColor: p.onChrome,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      scrolledUnderElevation: 0,
      centerTitle: false,
      iconTheme: IconThemeData(color: p.onChrome),
      actionsIconTheme: IconThemeData(color: p.onChrome),
      titleTextStyle: TextStyle(color: p.onChrome, fontSize: 20, fontWeight: FontWeight.w700),
      systemOverlayStyle: SystemUiOverlayStyle.light.copyWith(statusBarColor: Colors.transparent),
    ),
    // Tab bars only sit inside the purple app bar.
    tabBarTheme: TabBarThemeData(
      labelColor: p.onChrome,
      unselectedLabelColor: p.chromeSoft,
      indicatorColor: p.onChrome,
      indicatorSize: TabBarIndicatorSize.label,
      dividerColor: Colors.transparent,
      labelStyle: const TextStyle(fontSize: 14, fontWeight: FontWeight.w600),
      unselectedLabelStyle: const TextStyle(fontSize: 14, fontWeight: FontWeight.w500),
      overlayColor: WidgetStatePropertyAll(p.onChrome.withValues(alpha: 0.12)),
    ),
    cardTheme: CardThemeData(
      color: p.card,
      elevation: 0,
      margin: const EdgeInsets.symmetric(horizontal: KSpace.s4, vertical: 6),
      clipBehavior: Clip.antiAlias,
      shape: RoundedRectangleBorder(borderRadius: lg, side: BorderSide(color: p.border)),
    ),
    listTileTheme: ListTileThemeData(
      contentPadding: const EdgeInsets.symmetric(horizontal: KSpace.s4),
      iconColor: p.muted,
      minVerticalPadding: 10,
      shape: RoundedRectangleBorder(borderRadius: md),
    ),
    // White bar, purple selection (AA: #4F46E5 on #EEF2FF and #5D6475 on white).
    navigationBarTheme: NavigationBarThemeData(
      backgroundColor: p.card,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      height: 68,
      indicatorColor: p.primaryLight,
      iconTheme: WidgetStateProperty.resolveWith(
        (s) => IconThemeData(color: s.contains(WidgetState.selected) ? p.primary : p.muted),
      ),
      labelTextStyle: WidgetStateProperty.resolveWith((s) {
        final on = s.contains(WidgetState.selected);
        return TextStyle(fontSize: 12, fontWeight: on ? FontWeight.w600 : FontWeight.w500, color: on ? p.primary : p.muted);
      }),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: p.card,
      border: OutlineInputBorder(borderRadius: md, borderSide: BorderSide(color: p.borderStrong)),
      enabledBorder: OutlineInputBorder(borderRadius: md, borderSide: BorderSide(color: p.borderStrong)),
      focusedBorder: OutlineInputBorder(borderRadius: md, borderSide: BorderSide(color: p.primary, width: 2)),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: p.primary,
        foregroundColor: p.onPrimary,
        minimumSize: const Size(64, 44),
        shape: RoundedRectangleBorder(borderRadius: md),
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
        textStyle: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        foregroundColor: p.text,
        minimumSize: const Size(64, 44),
        side: BorderSide(color: p.borderStrong),
        shape: RoundedRectangleBorder(borderRadius: md),
        textStyle: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(
        foregroundColor: p.primary,
        minimumSize: const Size(48, 40),
        shape: RoundedRectangleBorder(borderRadius: md),
        textStyle: const TextStyle(fontSize: 14, fontWeight: FontWeight.w600),
      ),
    ),
    segmentedButtonTheme: SegmentedButtonThemeData(
      style: ButtonStyle(
        minimumSize: const WidgetStatePropertyAll(Size(48, 40)),
        side: WidgetStatePropertyAll(BorderSide(color: p.borderStrong)),
        shape: WidgetStatePropertyAll(RoundedRectangleBorder(borderRadius: md)),
      ),
    ),
    floatingActionButtonTheme: FloatingActionButtonThemeData(
      backgroundColor: p.primary,
      foregroundColor: p.onPrimary,
      elevation: 2,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
    ),
    dialogTheme: DialogThemeData(
      backgroundColor: p.card,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(borderRadius: lg),
      titleTextStyle: TextStyle(fontSize: 20, fontWeight: FontWeight.w600, color: p.text),
    ),
    bottomSheetTheme: BottomSheetThemeData(
      backgroundColor: p.card,
      surfaceTintColor: Colors.transparent,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(KRadius.lg))),
    ),
    popupMenuTheme: PopupMenuThemeData(
      color: p.card,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(borderRadius: md, side: BorderSide(color: p.border)),
    ),
    snackBarTheme: SnackBarThemeData(shape: RoundedRectangleBorder(borderRadius: md)),
    dividerTheme: DividerThemeData(color: p.border, space: 1, thickness: 1),
    chipTheme: ChipThemeData(
      side: BorderSide(color: p.border),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(999)),
    ),
    progressIndicatorTheme: ProgressIndicatorThemeData(color: p.primary),
  );
}
