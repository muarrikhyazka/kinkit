import 'package:cookie_jar/cookie_jar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'app.dart';
import 'state/providers.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final prefs = await SharedPreferences.getInstance();
  // Session cookie persisted in the app's private support directory so the
  // login survives restarts (server cookie: Max-Age 30 days).
  final dir = await getApplicationSupportDirectory();
  final jar = PersistCookieJar(storage: FileStorage('${dir.path}/.cookies/'));

  runApp(ProviderScope(
    overrides: [
      sharedPrefsProvider.overrideWithValue(prefs),
      cookieJarProvider.overrideWithValue(jar),
    ],
    child: const KinkitApp(),
  ));
}
