#!/usr/bin/env python3
"""Multi-tenant Slack bot for Kinkit.

Same multiplexing pattern as the Telegram/Discord bots: one process, many
accounts, one Socket Mode connection per connected Slack integration (bot
token xoxb-..., app-level token xapp-... with connections:write, both entered
in Kinkit > Integrations; no public URL needed). Every /api/bot call carries
X-Bot-User "slack:<user id>".

Messages: commands are plain messages with a "!" prefix ("/" also works when
it arrives as text, but Slack swallows "/..." typed in the composer as slash
commands): !today, !remind, !chores, !buy, !list, !menu, !link, !help, ...
Anything else in the household channel that looks like "50k lunch" is logged as
spending (source "slack"). Outside the household channel (and in DMs) only the
kc.ANYWHERE commands (help, link, about) answer.
Buttons: replies and backend-sent reminders carry Block Kit buttons whose
action_id/value is the callback data ("rem:<id>:done"...); a click arrives as a
block_actions payload, runs kc.handle_callback and replaces the button row
with the result line (chat.update).

Slack app setup (Kinkit > Integrations shows it too): enable Socket Mode,
create an App-Level token (connections:write), bot scopes chat:write,
channels:history, groups:history, im:history, users:read, event subscriptions
message.channels / message.groups / message.im, install to the workspace and
invite the bot to the household channel.
"""
import asyncio
import logging
import os
from collections import OrderedDict
from urllib.error import URLError

from slack_sdk.socket_mode.aiohttp import SocketModeClient
from slack_sdk.socket_mode.request import SocketModeRequest
from slack_sdk.socket_mode.response import SocketModeResponse
from slack_sdk.web.async_client import AsyncWebClient

import chat_helpers as ch
import kinkit_common as kc

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger("kinkit-slack")

SLACK_TEXT_MAX = 3900  # chat.postMessage truncates around 4000 characters


class SlackTenant:
    def __init__(self, integ):
        self.bot_token = integ["bot_token"]          # Kinkit's X-Bot-Token for this account
        self.slack_token = integ["platform_token"]   # xoxb-...
        self.app_token = integ["extra"]              # xapp-...
        self.chat_id = integ.get("chat_id") or ""
        self.user_id = None                          # the bot's own Slack user id
        self.web = None
        self.sm = None
        self.task = None
        self.jobs = set()
        self.seen = OrderedDict()                    # event ids already handled (Slack redelivers un-acked events)
        self.names = {}                              # slack user id -> display name

    async def start(self):
        self.task = asyncio.create_task(self.run())

    async def stop(self):
        if self.task:
            self.task.cancel()
            try:
                await self.task
            except (asyncio.CancelledError, Exception):
                pass
        for j in list(self.jobs):
            j.cancel()
        await self.close()

    async def close(self):
        if self.sm is not None:
            try:
                await self.sm.close()
            except Exception:
                pass
            self.sm = None

    async def run(self):
        delay = 5
        while True:
            try:
                self.web = AsyncWebClient(token=self.slack_token)
                auth = await self.web.auth_test()
                self.user_id = auth.get("user_id")
                self.sm = SocketModeClient(app_token=self.app_token, web_client=self.web)
                self.sm.socket_mode_request_listeners.append(self.on_request)
                await self.sm.connect()
                logger.info(f"Slack Socket Mode connected (bot user {self.user_id})")
                await asyncio.Event().wait()  # the client reconnects by itself
            except asyncio.CancelledError:
                await self.close()
                raise
            except Exception as e:
                logger.error(f"Slack connection failed ({type(e).__name__}: {str(e)[:200]}); retrying in {delay}s")
                await self.close()
                await asyncio.sleep(delay)
                delay = min(delay * 2, 120)

    async def on_request(self, client, req: SocketModeRequest):
        # Ack first: Slack retries anything not acknowledged within 3 seconds.
        try:
            await client.send_socket_mode_response(SocketModeResponse(envelope_id=req.envelope_id))
        except Exception as e:
            logger.warning(f"ack failed: {type(e).__name__}")
        if req.type == "events_api":
            coro = self.on_message(req.payload or {})
        elif req.type == "interactive":
            coro = self.on_action(req.payload or {})
        else:
            return
        job = asyncio.create_task(self.guarded(coro))
        self.jobs.add(job)
        job.add_done_callback(self.jobs.discard)

    async def guarded(self, coro):
        try:
            await coro
        except Exception as e:
            logger.error(f"handler error: {type(e).__name__}: {str(e)[:200]}")

    def first_time(self, key):
        if not key:
            return True
        if key in self.seen:
            return False
        self.seen[key] = True
        while len(self.seen) > 500:
            self.seen.popitem(last=False)
        return True

    async def resolve(self, raw):
        """Message text with <@U123> mentions replaced by '@display name'."""
        for uid in set(ch.slack_user_ids(raw)):
            if uid in self.names:
                continue
            try:
                info = await self.web.users_info(user=uid)
                u = info.get("user") or {}
                prof = u.get("profile") or {}
                self.names[uid] = prof.get("display_name") or u.get("real_name") or u.get("name") or uid
            except Exception:
                self.names[uid] = uid
        return ch.slack_unescape(raw, self.names)

    async def post(self, channel, text, buttons=None):
        text = ch.bang_prefix(text)[:SLACK_TEXT_MAX]
        kwargs = {"channel": channel, "text": text}
        blocks = ch.slack_blocks(text, buttons)
        if blocks:
            kwargs["blocks"] = blocks
        try:
            await self.web.chat_postMessage(**kwargs)
        except Exception as e:
            logger.error(f"chat.postMessage failed: {type(e).__name__}: {str(e)[:200]}")

    async def on_message(self, payload):
        info = ch.slack_message_event(payload, self.user_id)
        if info is None:
            return
        channel, user, raw, is_dm, event_id = info
        if not self.first_time(event_id):
            return
        text = await self.resolve(raw)
        who = f"slack:{user}"
        parsed = kc.split_command(text)
        if parsed:
            cmd, args = parsed
            if not ch.slack_allowed(channel, self.chat_id, is_dm, cmd):
                return
            reply, buttons = await kc.run_command(self.bot_token, who, cmd, args)
            await self.post(channel, reply, buttons)
            return
        if is_dm or not ch.slack_allowed(channel, self.chat_id, False) or text.startswith(("!", "/")):
            return
        try:
            replies = await kc.log_spending(self.bot_token, who, text, "slack")
        except kc.BackendError as e:
            await self.post(channel, f"Failed to save - {e.message}")
            return
        except (URLError, OSError):
            return
        for reply, buttons in replies or []:
            await self.post(channel, reply, buttons)

    async def on_action(self, payload):
        act = ch.slack_block_action(payload)
        if act is None:
            return
        data, user, channel, ts, blocks, orig = act
        if not data.startswith(kc.CALLBACK_PREFIXES):
            return
        if self.chat_id and channel and channel != self.chat_id:
            return
        ok, message = await kc.handle_callback(self.bot_token, f"slack:{user}", data)
        message = ch.bang_prefix(message)
        if ok and channel and ts:
            try:
                await self.web.chat_update(channel=channel, ts=ts, text=orig or message,
                                           blocks=ch.slack_result_blocks(blocks, orig, message))
                return
            except Exception as e:
                logger.info(f"chat.update failed: {type(e).__name__}: {str(e)[:200]}")
        if channel:
            try:
                await self.web.chat_postEphemeral(channel=channel, user=user, text=message[:SLACK_TEXT_MAX])
            except Exception as e:
                logger.info(f"chat.postEphemeral failed: {type(e).__name__}: {str(e)[:200]}")


async def run():
    await ch.run_tenants("slack", SlackTenant, required=("platform_token", "extra"))


if __name__ == "__main__":
    asyncio.run(run())
