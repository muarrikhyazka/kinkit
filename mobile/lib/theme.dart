import 'package:flutter/material.dart';

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
  });

  static const light = KinkitPalette(
    bg: Color(0xFFF1F3F8),
    card: Color(0xFFFFFFFF),
    border: Color(0xFFE5E7EB),
    text: Color(0xFF1E2330),
    muted: Color(0xFF6B7280),
    primary: Color(0xFF4F46E5),
    primaryLight: Color(0xFFEEF2FF),
    success: Color(0xFF16A34A),
    danger: Color(0xFFDC2626),
    warning: Color(0xFFD97706),
  );

  static const dark = KinkitPalette(
    bg: Color(0xFF0F1220),
    card: Color(0xFF1A1F33),
    border: Color(0xFF2C3350),
    text: Color(0xFFE5E7EB),
    muted: Color(0xFF9CA3AF),
    primary: Color(0xFF818CF8),
    primaryLight: Color(0xFF2A2F4A),
    success: Color(0xFF22C55E),
    danger: Color(0xFFF87171),
    warning: Color(0xFFFBBF24),
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

ThemeData buildTheme(Brightness brightness) {
  final p = brightness == Brightness.dark ? KinkitPalette.dark : KinkitPalette.light;
  final scheme = ColorScheme.fromSeed(seedColor: p.primary, brightness: brightness).copyWith(
    primary: p.primary,
    onPrimary: Colors.white,
    primaryContainer: p.primaryLight,
    onPrimaryContainer: p.text,
    surface: p.card,
    onSurface: p.text,
    onSurfaceVariant: p.muted,
    outline: p.border,
    outlineVariant: p.border,
    error: p.danger,
  );
  final radius = BorderRadius.circular(12);
  return ThemeData(
    useMaterial3: true,
    brightness: brightness,
    colorScheme: scheme,
    scaffoldBackgroundColor: p.bg,
    extensions: [p],
    appBarTheme: AppBarTheme(
      backgroundColor: p.bg,
      foregroundColor: p.text,
      elevation: 0,
      scrolledUnderElevation: 0.5,
      centerTitle: false,
      titleTextStyle: TextStyle(color: p.text, fontSize: 20, fontWeight: FontWeight.w700),
    ),
    cardTheme: CardThemeData(
      color: p.card,
      elevation: 0,
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
      shape: RoundedRectangleBorder(borderRadius: radius, side: BorderSide(color: p.border)),
    ),
    navigationBarTheme: NavigationBarThemeData(
      backgroundColor: p.card,
      indicatorColor: p.primaryLight,
      labelTextStyle: WidgetStatePropertyAll(TextStyle(fontSize: 12, color: p.text)),
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
        foregroundColor: Colors.white,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
      ),
    ),
    floatingActionButtonTheme: FloatingActionButtonThemeData(
      backgroundColor: p.primary,
      foregroundColor: Colors.white,
    ),
    dividerTheme: DividerThemeData(color: p.border, space: 1),
    chipTheme: ChipThemeData(side: BorderSide(color: p.border)),
  );
}
