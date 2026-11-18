#!/usr/bin/env python3
"""Multi-tenant Telegram bot for Kinkit.

One process, many accounts: polls /api/internal/integrations for every
enabled Telegram integration, spins up one python-telegram-bot Application
per platform_token (each tenant brings their own BotFather bot), and routes
each incoming message through that tenant's X-Bot-Token to the backend.
Every /api/bot call carries X-Bot-User "telegram:<user id>" so the backend
knows which household member is talking (after /link).

Features: spending logging ("50k lunch", "makan siang 50rb", one per line;
unknown words get a category picker that teaches the bot), /recent, /fix,
/fixdate, /date, /today, /remind, /reminders, /chores, /buy, /list,
/done_shopping, /menu, /ingredients, /addingredient, /removeingredient,
/schedule, /feedback, /link, /about, /help (plus rumahbot's Indonesian
names as aliases, see kinkit_common.ALIASES), and inline-keyboard buttons
(callback_query) on reminder / chore / picker messages.

Household group chats: the integration's chat_id gates everything except
/help, /link and /about. Commands addressed to another bot in the group
("/help@OtherBot") are ignored. Free-text spending logging in a group needs
the bot's privacy mode OFF (BotFather > /setprivacy > Disable) or the bot to
be a group admin; otherwise Telegram only delivers commands to it.

Reload latency: a cheap GET /api/internal/integrations-version check runs
every VERSION_CHECK_SECONDS (default 5s); the expensive full fetch+rebuild
only runs when that version actually changed.
"""
import asyncio
import logging
import os
from urllib.error import URLError

from telegram import InlineKeyboardButton, InlineKeyboardMarkup, Update
from telegram.ext import Application, CallbackQueryHandler, ContextTypes, MessageHandler, filters

import kinkit_common as kc

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger("kinkit-telegram")

REFRESH_SECONDS = int(os.environ.get("REFRESH_SECONDS", "300"))  # fallback full-rebuild cadence even if version never changes
VERSION_CHECK_SECONDS = int(os.environ.get("VERSION_CHECK_SECONDS", "5"))


def keyboard(buttons):
    if not buttons:
        return None
    return InlineKeyboardMarkup([[InlineKeyboardButton(label, callback_data=data) for label, data in row] for row in buttons])


def make_handlers(bot_token, chat_id, bot_username=None):
    def in_household_chat(update: Update):
        return not chat_id or str(update.effective_chat.id) == str(chat_id)

    def actor(update: Update):
        u = update.effective_user
        return f"telegram:{u.id}" if u else None

    async def handle_text(update: Update, context: ContextTypes.DEFAULT_TYPE):
        msg = update.message
        if msg is None or not msg.text:
            return
        parsed = kc.split_command(msg.text, prefixes=("/",), bot_username=bot_username)
        if parsed:
            cmd, args = parsed
            if cmd not in kc.ANYWHERE and not in_household_chat(update):
                return
            text, buttons = await kc.run_command(bot_token, actor(update), cmd, args)
            await msg.reply_text(text, reply_markup=keyboard(buttons))
            return
        if msg.text.startswith("/") or not in_household_chat(update):
            return
        try:
            replies = await kc.log_spending(bot_token, actor(update), msg.text, "telegram")
        except kc.BackendError as e:
            await msg.reply_text(f"Failed to save - {e.message}")
            return
        except (URLError, OSError):
            await msg.reply_text("Backend unreachable, try again later.")
            return
        for text, buttons in replies or []:
            await msg.reply_text(text, reply_markup=keyboard(buttons))

    async def handle_callback(update: Update, context: ContextTypes.DEFAULT_TYPE):
        query = update.callback_query
        if query is None:
            return
        if query.message is not None and not in_household_chat(update):
            await query.answer()
            return
        ok, message = await kc.handle_callback(bot_token, actor(update), query.data)
        await query.answer(text=message[:190], show_alert=not ok)
        if ok and query.message is not None:
            try:
                base = query.message.text or ""
                await query.edit_message_text(f"{base}\n\n{message}", reply_markup=None)
            except Exception as e:  # message too old / unchanged: the toast is enough
                logger.info(f"could not edit message after callback: {e}")

    return handle_text, handle_callback


BOT_COMMANDS = [
    ("today", "Today's reminders, chores and budget"),
    ("remind", "Add a reminder: /remind every 2 weeks on fri 10:00 change bed sheets"),
    ("reminders", "Upcoming reminders"),
    ("chores", "Today's chores"),
    ("buy", "Add to shopping list: /buy milk, eggs"),
    ("list", "Show the shopping list"),
    ("done_shopping", "Finish the shopping trip"),
    ("recent", "Last expenses: /recent 10"),
    ("fix", "Change a category: /fix bakso Food"),
    ("fixdate", "Change a date: /fixdate bakso 2026-10-20"),
    ("date", "Log the next expenses on another day: /date 2026-10-20"),
    ("menu", "Weekly meal plan"),
    ("ingredients", "Must-use ingredients for the meal plan"),
    ("addingredient", "Add a must-use ingredient"),
    ("removeingredient", "Remove a must-use ingredient"),
    ("schedule", "When the meal plan is posted"),
    ("spending", "How to log spending"),
    ("feedback", "Request a feature"),
    ("link", "Link your chat account: /link CODE"),
    ("about", "About this bot"),
    ("help", "What can this bot do?"),
]


async def start_tenant(integ):
    application = Application.builder().token(integ["platform_token"]).build()
    await application.initialize()
    handle_text, handle_callback = make_handlers(integ["bot_token"], integ.get("chat_id"), application.bot.username)
    # Commands are routed through one text handler (also catches "/done-shopping",
    # which Telegram's CommandHandler can't express, and rumahbot's aliases).
    application.add_handler(MessageHandler(filters.TEXT, handle_text))
    application.add_handler(CallbackQueryHandler(handle_callback))
    try:
        await application.bot.set_my_commands(BOT_COMMANDS)
    except Exception as e:
        logger.info(f"set_my_commands failed: {e}")
    await application.start()
    await application.updater.start_polling(allowed_updates=["message", "callback_query"])
    return application


def integ_signature(integ):
    """Restart a tenant's poller when any of these change."""
    return (integ.get("platform_token"), integ.get("bot_token"), str(integ.get("chat_id") or ""))


async def stop_tenant(application):
    await application.updater.stop()
    await application.stop()
    await application.shutdown()


async def run():
    apps = {}  # account_id -> Application
    sigs = {}  # account_id -> integ_signature at start
    last_version = None
    last_full_rebuild = 0.0
    loop = asyncio.get_event_loop()
    while True:
        try:
            version = (await asyncio.to_thread(kc.backend_get, "/api/internal/integrations-version")).get("version")
        except Exception as e:
            logger.error(f"Could not fetch integrations version: {e}")
            version = last_version  # assume unchanged, don't force a rebuild on transient errors

        now = loop.time()
        changed = version != last_version
        if not changed and (now - last_full_rebuild) < REFRESH_SECONDS:
            await asyncio.sleep(VERSION_CHECK_SECONDS)
            continue
        last_version = version
        last_full_rebuild = now

        try:
            integrations = await asyncio.to_thread(kc.backend_get, "/api/internal/integrations", {"platform": "telegram"})
        except Exception as e:
            logger.error(f"Could not fetch integrations: {e}")
            integrations = None

        if integrations is not None:
            seen = set()
            for integ in integrations:
                aid = integ["account_id"]
                seen.add(aid)
                if not integ.get("platform_token"):
                    continue
                if aid in apps and sigs.get(aid) == integ_signature(integ):
                    continue
                if aid in apps:
                    logger.info(f"Restarting Telegram bot for account {aid} (token or chat changed)")
                    try:
                        await stop_tenant(apps.pop(aid))
                    except Exception as e:
                        logger.error(f"Failed to stop bot for account {aid}: {e}")
                try:
                    apps[aid] = await start_tenant(integ)
                    sigs[aid] = integ_signature(integ)
                    logger.info(f"Started Telegram bot for account {aid}")
                except Exception as e:
                    logger.error(f"Failed to start bot for account {aid}: {e}")
                    # Transient (e.g. Telegram timeout): retry on the next check
                    # instead of waiting for the periodic full refresh.
                    last_full_rebuild = 0.0

            for aid in list(apps.keys()):
                if aid not in seen:
                    logger.info(f"Stopping Telegram bot for account {aid} (integration removed/disabled)")
                    sigs.pop(aid, None)
                    await stop_tenant(apps.pop(aid))

        await asyncio.sleep(VERSION_CHECK_SECONDS)


if __name__ == "__main__":
    asyncio.run(run())
