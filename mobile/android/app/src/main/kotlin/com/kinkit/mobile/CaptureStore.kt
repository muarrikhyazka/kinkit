package com.kinkit.mobile

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject

/**
 * Phone-local queue of notifications that look like payments.
 *
 * The listener service can run while the Flutter engine is not, so matches
 * are kept in this app's private SharedPreferences until the Dart side pulls,
 * parses and shows them. Nothing here is sent anywhere: only an expense the
 * user reviews and saves in the app is posted to the Kinkit server.
 */
object CaptureStore {
    private const val PREFS = "kinkit_capture"
    private const val KEY_ENABLED = "enabled"
    private const val KEY_ITEMS = "items"
    private const val MAX_ITEMS = 100
    private const val DUPLICATE_WINDOW_MS = 120_000L

    private val lock = Any()

    // Same markers as moneyMarker in lib/capture/notification_parser.dart; keep the two in step.
    // A currency prefix then a digit ("Rp 25.000", "IDR1,250", "SGD 12.50", "$12.50"),
    // or a number then IDR / rupiah ("150.000 IDR").
    private val moneyPattern = Regex(
        "(?i)(?<![a-z])(?:rp\\.?|idr|usd|sgd|myr|eur|gbp|[\\u0024€£])\\s*\\d|\\d\\s*(?:idr|rupiah)\\b"
    )

    fun looksLikeMoney(s: String): Boolean = moneyPattern.containsMatchIn(s)

    fun isEnabled(ctx: Context): Boolean =
        ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getBoolean(KEY_ENABLED, false)

    fun setEnabled(ctx: Context, enabled: Boolean) {
        ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit().putBoolean(KEY_ENABLED, enabled).apply()
    }

    /** Adds a capture, newest first. Skips repeats and keeps at most [MAX_ITEMS]. */
    fun add(ctx: Context, id: String, pkg: String, app: String, title: String, text: String, postedAt: Long) {
        synchronized(lock) {
            val items = read(ctx)
            for (i in 0 until items.length()) {
                val o = items.getJSONObject(i)
                if (o.optString("id") == id) return
                if (o.optString("package") == pkg &&
                    o.optString("text") == text &&
                    Math.abs(o.optLong("posted_at") - postedAt) < DUPLICATE_WINDOW_MS
                ) return
            }
            val next = JSONArray()
            next.put(
                JSONObject()
                    .put("id", id)
                    .put("package", pkg)
                    .put("app", app)
                    .put("title", title)
                    .put("text", text)
                    .put("posted_at", postedAt)
            )
            var i = 0
            while (i < items.length() && next.length() < MAX_ITEMS) {
                next.put(items.getJSONObject(i))
                i++
            }
            write(ctx, next)
        }
    }

    fun list(ctx: Context): List<Map<String, Any>> = synchronized(lock) {
        val items = read(ctx)
        val out = ArrayList<Map<String, Any>>()
        for (i in 0 until items.length()) {
            val o = items.getJSONObject(i)
            out.add(
                mapOf(
                    "id" to o.optString("id"),
                    "package" to o.optString("package"),
                    "app" to o.optString("app"),
                    "title" to o.optString("title"),
                    "text" to o.optString("text"),
                    "posted_at" to o.optLong("posted_at")
                )
            )
        }
        out
    }

    fun remove(ctx: Context, ids: Collection<String>) {
        if (ids.isEmpty()) return
        synchronized(lock) {
            val items = read(ctx)
            val next = JSONArray()
            for (i in 0 until items.length()) {
                val o = items.getJSONObject(i)
                if (!ids.contains(o.optString("id"))) next.put(o)
            }
            write(ctx, next)
        }
    }

    fun clear(ctx: Context) {
        synchronized(lock) { write(ctx, JSONArray()) }
    }

    private fun read(ctx: Context): JSONArray {
        val raw = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY_ITEMS, null)
            ?: return JSONArray()
        return try {
            JSONArray(raw)
        } catch (e: Exception) {
            JSONArray()
        }
    }

    private fun write(ctx: Context, items: JSONArray) {
        ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit().putString(KEY_ITEMS, items.toString()).apply()
    }
}
