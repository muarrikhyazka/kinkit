#!/usr/bin/env python3
"""Multi-tenant Discord bot for Kinkit.

Same multiplexing pattern as bots/telegram_bot.py: one process, many
accounts, one discord.py Client per connected integration's bot token.
Every /api/bot call carries X-Bot-User "discord:<user id>".

Commands are plain messages with a "!" (or "/") prefix: !today, !remind,
!reminders, !chores, !buy, !list, !done-shopping, !link, !recent, !help.
Anything else that looks like "50k lunch" is logged as spending.
Buttons: reminder messages are posted by the backend with message
components (custom_id "rem:<id>:..."); !chores replies carry
"chore:<id>:done" buttons. Both are handled in on_interaction.

Reload latency: a cheap GET /api/internal/integrations-version check runs
every VERSION_CHECK_SECONDS (default 5s); the expensive full fetch+rebuild
only runs when that version actually changed.
"""
import asyncio
import logging
import os
from urllib.error import URLError

import discord

import kinkit_common as kc

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger("kinkit-discord")

REFRESH_SECONDS = int(os.environ.get("REFRESH_SECONDS", "300"))  # fallback full-rebuild cadence even if version never changes
VERSION_CHECK_SECONDS = int(os.environ.get("VERSION_CHECK_SECONDS", "5"))


class RawButton(discord.ui.Button):
    """A button whose clicks are handled in TenantClient.on_interaction
    (so they keep working after a bot restart, like backend-sent buttons)."""


def make_view(buttons):
    if not buttons:
        return None
    # timeout=None + not stored: discord.py never auto-responds for us;
    # on_interaction handles every click.
    view = discord.ui.View(timeout=None)
    for r, row in enumerate(buttons[:5]):
        for label, data in row[:5]:
            view.add_item(RawButton(label=label[:80], custom_id=data, style=discord.ButtonStyle.success, row=r))
    view.stop()  # don't track it in the view store
    return view


class TenantClient(discord.Client):
    def __init__(self, bot_token, chat_id, **kwargs):
        super().__init__(**kwargs)
        self.bot_token = bot_token
        self.chat_id = chat_id

    async def on_ready(self):
        logger.info(f"Discord client ready: {self.user}")

    def in_household_channel(self, channel):
        if isinstance(channel, discord.DMChannel):
            return False
        return not self.chat_id or str(channel.id) == str(self.chat_id)

    async def on_message(self, message):
        if message.author.bot:
            return
        content = message.content.strip()
        user = f"discord:{message.author.id}"
        parsed = kc.split_command(content)
        if parsed:
            cmd, args = parsed
            if cmd not in kc.ANYWHERE and not self.in_household_channel(message.channel):
                return
            text, buttons = await kc.run_command(self.bot_token, user, cmd, args)
            view = make_view(buttons)
            if view is not None:
                await message.channel.send(text[:2000], view=view)
            else:
                await message.channel.send(text[:2000])
            return
        if not self.in_household_channel(message.channel) or content.startswith(("!", "/")):
            return
        try:
            reply = await kc.log_spending(self.bot_token, user, content, "discord")
        except kc.BackendError as e:
            await message.channel.send(f"Failed to save - {e.message}")
            return
        except (URLError, OSError):
            return
        if reply:
            await message.channel.send(reply)

    async def on_interaction(self, interaction: discord.Interaction):
        if interaction.type != discord.InteractionType.component:
            return
        data = (interaction.data or {}).get("custom_id", "")
        if not (data.startswith("rem:") or data.startswith("chore:")):
            return
        if interaction.channel is not None and not self.in_household_channel(interaction.channel):
            return
        # backend calls are quick, but defer-free responses must happen in 3s
        ok, message = await kc.handle_callback(self.bot_token, f"discord:{interaction.user.id}", data)
        try:
            if ok and interaction.message is not None:
                base = interaction.message.content or ""
                await interaction.response.edit_message(content=f"{base}\n{message}"[:2000], view=None)
            else:
                await interaction.response.send_message(message[:2000], ephemeral=True)
        except discord.HTTPException as e:
            logger.info(f"interaction response failed: {e}")


async def run():
    clients = {}  # account_id -> (client, task)
    intents = discord.Intents.default()
    intents.message_content = True
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
            integrations = await asyncio.to_thread(kc.backend_get, "/api/internal/integrations", {"platform": "discord"})
        except Exception as e:
            logger.error(f"Could not fetch integrations: {e}")
            integrations = None

        if integrations is not None:
            seen = set()
            for integ in integrations:
                aid = integ["account_id"]
                seen.add(aid)
                if aid in clients or not integ.get("platform_token"):
                    continue
                client = TenantClient(integ["bot_token"], integ.get("chat_id"), intents=intents)
                task = asyncio.create_task(client.start(integ["platform_token"]))
                clients[aid] = (client, task)
                logger.info(f"Started Discord client for account {aid}")

            for aid in list(clients.keys()):
                if aid not in seen:
                    logger.info(f"Stopping Discord client for account {aid} (integration removed/disabled)")
                    client, task = clients.pop(aid)
                    await client.close()
                    task.cancel()

        await asyncio.sleep(VERSION_CHECK_SECONDS)


if __name__ == "__main__":
    asyncio.run(run())
