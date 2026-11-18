/// Build-time configuration.
///
/// The default server can be baked in at build time:
///   flutter run --dart-define=KINKIT_BASE_URL=https://kinkit.example.com
/// Users can always override it on the login screen (persisted).
class AppConfig {
  AppConfig._();

  static const String defaultBaseUrl = String.fromEnvironment(
    'KINKIT_BASE_URL',
    defaultValue: 'http://10.0.2.2:5000', // Android emulator -> host machine
  );

  /// Value stored in Spending.source for rows created from this app.
  static const String spendingSource = 'mobile';

  /// Normalises what a user typed into a usable origin:
  /// trims, adds https:// when no scheme, strips trailing slashes and a
  /// trailing "/api" (the client appends /api itself).
  static String normalizeBaseUrl(String input) {
    var s = input.trim();
    if (s.isEmpty) return defaultBaseUrl;
    if (!s.contains('://')) s = 'https://$s';
    while (s.endsWith('/')) {
      s = s.substring(0, s.length - 1);
    }
    if (s.endsWith('/api')) s = s.substring(0, s.length - 4);
    return s;
  }
}
