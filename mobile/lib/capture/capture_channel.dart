import 'dart:io' show Platform;

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/services.dart';

/// Whether this build can capture notifications at all. iOS does not let an
/// app read other apps' notifications, so everything here is Android only.
bool get captureSupportedPlatform {
  if (kIsWeb) return false;
  try {
    return Platform.isAndroid;
  } catch (_) {
    return false;
  }
}

class CaptureStatus {
  /// False on iOS and when the native side is missing.
  final bool supported;

  /// Android "Notification access" is on for Kinkit.
  final bool permission;

  /// The in-app switch (Settings > Expense capture).
  final bool enabled;

  const CaptureStatus({required this.supported, required this.permission, required this.enabled});

  static const unsupported = CaptureStatus(supported: false, permission: false, enabled: false);

  /// Notifications are actually being collected.
  bool get active => supported && permission && enabled;
}

/// One payment-looking notification waiting for review.
class CapturedNotification {
  final String id;
  final String package;
  final String app;
  final String title;
  final String text;
  final DateTime postedAt;

  const CapturedNotification({
    required this.id,
    required this.package,
    required this.app,
    required this.title,
    required this.text,
    required this.postedAt,
  });

  static CapturedNotification? fromMap(Map<Object?, Object?> m) {
    final id = m['id'];
    if (id is! String || id.isEmpty) return null;
    final ms = m['posted_at'];
    return CapturedNotification(
      id: id,
      package: m['package'] is String ? m['package'] as String : '',
      app: m['app'] is String ? m['app'] as String : '',
      title: m['title'] is String ? m['title'] as String : '',
      text: m['text'] is String ? m['text'] as String : '',
      postedAt: DateTime.fromMillisecondsSinceEpoch(ms is int ? ms : 0),
    );
  }
}

/// The Android side of capture failed (not "unsupported": that is a normal
/// answer off Android). [message] says what failed and what to do next, so it
/// can be shown to the user as is.
class CaptureException implements Exception {
  final String message;
  const CaptureException(this.message);

  @override
  String toString() => message;
}

/// Bridge to the Kotlin side (ExpenseNotificationListener + CaptureStore).
/// Off Android, or when the native side is missing (tests), reads return an
/// unsupported or empty answer and writes do nothing, without touching the
/// channel. A failure reported by the native side throws [CaptureException].
class CaptureChannel {
  const CaptureChannel();

  static const MethodChannel _channel = MethodChannel('com.kinkit.mobile/capture');

  static CaptureException _failed(String what, PlatformException e) {
    final detail = (e.message != null && e.message!.isNotEmpty) ? e.message! : e.code;
    return CaptureException('Android could not $what ($detail). Close and reopen Kinkit, then try again.');
  }

  Future<CaptureStatus> status() async {
    if (!captureSupportedPlatform) return CaptureStatus.unsupported;
    try {
      final m = await _channel.invokeMapMethod<Object?, Object?>('status');
      if (m == null) return CaptureStatus.unsupported;
      return CaptureStatus(
        supported: m['supported'] == true,
        permission: m['permission'] == true,
        enabled: m['enabled'] == true,
      );
    } on MissingPluginException {
      return CaptureStatus.unsupported;
    } on PlatformException catch (e) {
      throw _failed('check whether capture is on', e);
    }
  }

  /// Opens Android's "Notification access" screen. False when it cannot open.
  Future<bool> openSettings() async {
    if (!captureSupportedPlatform) return false;
    try {
      return (await _channel.invokeMethod<bool>('openSettings')) ?? false;
    } on MissingPluginException {
      return false;
    } on PlatformException {
      return false;
    }
  }

  Future<void> setEnabled(bool enabled) => _call('setEnabled', 'switch capture ${enabled ? 'on' : 'off'}', {'enabled': enabled});

  Future<List<CapturedNotification>> pending() async {
    if (!captureSupportedPlatform) return const [];
    try {
      final raw = await _channel.invokeListMethod<Object?>('pending');
      final out = <CapturedNotification>[];
      for (final e in raw ?? const <Object?>[]) {
        if (e is Map<Object?, Object?>) {
          final c = CapturedNotification.fromMap(e);
          if (c != null) out.add(c);
        }
      }
      return out;
    } on MissingPluginException {
      return const [];
    } on PlatformException catch (e) {
      throw _failed('read the captured notifications', e);
    }
  }

  Future<void> remove(List<String> ids) {
    if (ids.isEmpty) return Future<void>.value();
    return _call('remove', 'delete the captured notification', {'ids': ids});
  }

  Future<void> clear() => _call('clear', 'delete the captured notifications');

  Future<void> _call(String method, String what, [Map<String, Object?>? args]) async {
    if (!captureSupportedPlatform) return;
    try {
      await _channel.invokeMethod<void>(method, args);
    } on MissingPluginException {
      // Native side not present (tests, other platforms): nothing to do.
    } on PlatformException catch (e) {
      throw _failed(what, e);
    }
  }
}
