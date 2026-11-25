import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'capture_channel.dart';
import 'notification_parser.dart';

/// Overridable in tests.
final captureChannelProvider = Provider<CaptureChannel>((ref) => const CaptureChannel());

/// Permission and switch state for Settings > Expense capture.
final captureStatusProvider = FutureProvider.autoDispose<CaptureStatus>((ref) {
  return ref.watch(captureChannelProvider).status();
});

/// A queued notification plus what the parser made of it.
class PendingCapture {
  final CapturedNotification item;
  final ParsedCapture parsed;

  const PendingCapture(this.item, this.parsed);
}

/// Notifications waiting for review, newest first.
///
/// Only clear noise is deleted from the phone here: one-time codes, and
/// promotions with no payment wording (see [isCaptureNoise]), plus text with no
/// money in it. A notification that has a currency amount but could not be
/// read is kept and listed with an empty amount, so a real payment is never
/// dropped silently.
final pendingCapturesProvider = FutureProvider.autoDispose<List<PendingCapture>>((ref) async {
  final channel = ref.watch(captureChannelProvider);
  final items = await channel.pending();
  final out = <PendingCapture>[];
  final noise = <String>[];
  for (final c in items) {
    final draft = draftForReview(title: c.title, text: c.text, app: c.app);
    if (draft == null) {
      noise.add(c.id);
    } else {
      out.add(PendingCapture(c, draft));
    }
  }
  if (noise.isNotEmpty) {
    try {
      await channel.remove(noise);
    } on CaptureException {
      // Noise is never listed, so a failed delete costs nothing: it is retried on the next load.
    }
  }
  return out;
});
