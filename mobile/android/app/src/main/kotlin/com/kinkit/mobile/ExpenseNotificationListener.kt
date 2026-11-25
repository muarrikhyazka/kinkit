package com.kinkit.mobile

import android.app.Notification
import android.content.pm.ApplicationInfo
import android.os.Build
import android.os.Bundle
import android.service.notification.NotificationListenerService
import android.service.notification.StatusBarNotification

/**
 * Watches posted notifications and queues the ones that mention an amount of
 * money in [CaptureStore]. Only runs once the user has switched on "Notification
 * access" for Kinkit in Android settings and enabled capture inside the app.
 */
class ExpenseNotificationListener : NotificationListenerService() {

    override fun onNotificationPosted(sbn: StatusBarNotification?) {
        if (sbn == null) return
        try {
            handle(sbn)
        } catch (e: Exception) {
            // A malformed notification must never take the listener down.
        }
    }

    private fun handle(sbn: StatusBarNotification) {
        if (sbn.packageName == packageName) return
        if (!CaptureStore.isEnabled(this)) return
        val n = sbn.notification ?: return
        if ((n.flags and Notification.FLAG_GROUP_SUMMARY) != 0) return
        if ((n.flags and Notification.FLAG_ONGOING_EVENT) != 0) return
        val extras = n.extras ?: return

        val title = extras.getCharSequence(Notification.EXTRA_TITLE)?.toString() ?: ""
        val big = extras.getCharSequence(Notification.EXTRA_BIG_TEXT)?.toString()
        val text = if (!big.isNullOrBlank()) big else (extras.getCharSequence(Notification.EXTRA_TEXT)?.toString() ?: "")
        if (!CaptureStore.looksLikeMoney("$title\n$text")) return

        val pkg = sbn.packageName
        val app = appLabel(extras, pkg)
        CaptureStore.add(this, "${sbn.key}|${sbn.postTime}", pkg, app, title, text, sbn.postTime)
    }

    /**
     * The posting app's name. The notification carries its own ApplicationInfo,
     * which still works under Android 11+ package visibility; a direct
     * PackageManager lookup is the fallback, then the package name.
     */
    private fun appLabel(extras: Bundle, pkg: String): String {
        try {
            val info: ApplicationInfo? = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                extras.getParcelable(EXTRA_APP_INFO, ApplicationInfo::class.java)
            } else {
                @Suppress("DEPRECATION")
                extras.getParcelable<ApplicationInfo>(EXTRA_APP_INFO)
            }
            if (info != null) {
                val label = packageManager.getApplicationLabel(info).toString()
                if (label.isNotBlank()) return label
            }
        } catch (e: Exception) {
            // Fall through to the PackageManager lookup.
        }
        try {
            val label = packageManager.getApplicationLabel(packageManager.getApplicationInfo(pkg, 0)).toString()
            if (label.isNotBlank()) return label
        } catch (e: Exception) {
            // Package not visible to this app: use the package name.
        }
        return pkg
    }

    private companion object {
        const val EXTRA_APP_INFO = "android.appInfo"
    }
}
