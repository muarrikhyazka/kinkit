package com.kinkit.mobile

import android.content.ComponentName
import android.content.Intent
import android.os.Build
import android.provider.Settings
import android.service.notification.NotificationListenerService
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, CAPTURE_CHANNEL)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "status" -> result.success(
                        mapOf(
                            "supported" to true,
                            "permission" to hasListenerAccess(),
                            "enabled" to CaptureStore.isEnabled(this)
                        )
                    )
                    "openSettings" -> {
                        try {
                            val intent = Intent(Settings.ACTION_NOTIFICATION_LISTENER_SETTINGS)
                                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                            startActivity(intent)
                            result.success(true)
                        } catch (e: Exception) {
                            result.success(false)
                        }
                    }
                    "setEnabled" -> {
                        val enabled = call.argument<Boolean>("enabled") ?: false
                        CaptureStore.setEnabled(this, enabled)
                        if (enabled && Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
                            try {
                                NotificationListenerService.requestRebind(
                                    ComponentName(this, ExpenseNotificationListener::class.java)
                                )
                            } catch (e: Exception) {
                                // Rebinding is best effort; the service binds on its own once access is granted.
                            }
                        }
                        result.success(null)
                    }
                    "pending" -> result.success(CaptureStore.list(this))
                    "remove" -> {
                        val ids = call.argument<List<String>>("ids") ?: emptyList()
                        CaptureStore.remove(this, ids)
                        result.success(null)
                    }
                    "clear" -> {
                        CaptureStore.clear(this)
                        result.success(null)
                    }
                    else -> result.notImplemented()
                }
            }
    }

    private fun hasListenerAccess(): Boolean {
        val enabledListeners = Settings.Secure.getString(contentResolver, "enabled_notification_listeners")
        val me = ComponentName(this, ExpenseNotificationListener::class.java).flattenToString()
        return enabledListeners?.contains(me) == true
    }

    companion object {
        private const val CAPTURE_CHANNEL = "com.kinkit.mobile/capture"
    }
}
