#!/usr/bin/env python3
"""Multi-tenant Matrix bot for Kinkit.

Same multiplexing pattern as the other bots: one process, many accounts, one
/sync long-poll loop per connected Matrix integration. Plain aiohttp against
the client-server API (no matrix-nio, so no heavy dependencies and no
end-to-end encryption: use an unencrypted room). Credentials come from Kinkit
> Integrations: access token of the bot user, homeserver URL (extra), room id
(chat_id, e.g. !abc:example.org). Every /api/bot call carries
X-Bot-User "matrix:<@user:server>".

Commands are plain messages with a "!" prefix (Element swallows "/..." as its
own commands; "/" still works when it arrives as text): !today, !remind,
!chores, !buy, !list, !menu, !link, !help, ... Anything else in the household
room that looks like "50k lunch" is logged as spending (source "matrix").
Only the configured room is served; the bot auto-joins an invite to it.

Matrix messages have no buttons: replies that would carry buttons get a
numbered list ("1) Done") and the next "!1" / "!2" (or a bare "1" while a list
is pending) runs that button. Reminders posted by the backend are plain text:
answer them with "!rem <id> done | snooze | tomorrow".
"""
import asyncio
import logging
from urllib.error import URLError

import aiohttp

import chat_helpers as ch
import kinkit_common as kc

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger("kinkit-matrix")

SYNC_TIMEOUT_MS = 30000


class MatrixError(Exception):
    def __init__(self, status, errcode=""):
        super().__init__(f"HTTP {status} {errcode}".strip())
        self.status, self.errcode = status, errcode


class MatrixTenant:
    def __init__(self, integ):
        self.bot_token = integ["bot_token"]            # Kinkit's X-Bot-Token for this account
        self.access = integ["platform_token"]          # the Matrix bot user's access token
        self.homeserver = ch.matrix_base(integ["extra"])
        self.room = integ["chat_id"]
        self.user_id = None
        self.since = None
        self.session = None
        self.task = None
        self.choices = ch.ChoiceMemory()
        self.seen = []                                  # recent event ids (a short ring buffer)

    async def start(self):
        if not ch.valid_homeserver(self.homeserver):
            raise ValueError("invalid homeserver URL")
        self.task = asyncio.create_task(self.run())

    async def stop(self):
        if self.task:
            self.task.cancel()
            try:
                await self.task
            except (asyncio.CancelledError, Exception):
                pass
        if self.session is not None:
            await self.session.close()
            self.session = None

    # -- client-server API

    async def call(self, method, path, params=None, body=None, timeout=20):
        url = ch.matrix_url(self.homeserver, path)
        headers = {"Authorization": f"Bearer {self.access}"}
        async with self.session.request(method, url, params=params, json=body, headers=headers,
                                        timeout=aiohttp.ClientTimeout(total=timeout)) as resp:
            try:
                data = await resp.json(content_type=None)
            except Exception:
                data = {}
            if resp.status >= 400:
                raise MatrixError(resp.status, (data or {}).get("errcode", "") if isinstance(data, dict) else "")
            return data if isinstance(data, dict) else {}

    async def send(self, text):
        txn = ch.matrix_txn_id()
        await self.call("PUT", ch.matrix_send_path(self.room, txn),
                        body={"msgtype": "m.text", "body": text[:ch.MATRIX_TEXT_LIMIT]})

    async def join(self):
        try:
            await self.call("POST", ch.matrix_join_path(self.room), body={})
            logger.info("Matrix: joined the household room")
        except MatrixError as e:
            logger.warning(f"Matrix: could not join the room ({e}); invite the bot user to it")

    # -- main loop

    async def run(self):
        delay = 5
        self.session = aiohttp.ClientSession()
        flt = ch.matrix_filter(self.room)
        while True:
            try:
                if self.user_id is None:
                    self.user_id = (await self.call("GET", "/account/whoami")).get("user_id")
                    logger.info("Matrix: authenticated")
                    await self.join()
                if self.since is None:
                    # first sync: skip history, only learn next_batch (+ pending invites)
                    first = await self.call("GET", "/sync", {"filter": flt, "timeout": 0}, timeout=60)
                    self.since = first.get("next_batch")
                    if ch.matrix_invited(first, self.room):
                        await self.join()
                    continue
                sync = await self.call("GET", "/sync", {"filter": flt, "since": self.since, "timeout": SYNC_TIMEOUT_MS},
                                       timeout=SYNC_TIMEOUT_MS / 1000 + 30)
                self.since = sync.get("next_batch") or self.since
                delay = 5
                if ch.matrix_invited(sync, self.room):
                    await self.join()
                for event_id, sender, body in ch.matrix_events(sync, self.room, self.user_id):
                    if event_id and event_id in self.seen:
                        continue
                    if event_id:
                        self.seen = (self.seen + [event_id])[-200:]
                    try:
                        await self.on_message(sender, body)
                    except Exception as e:
                        logger.error(f"handler error: {type(e).__name__}: {str(e)[:200]}")
            except asyncio.CancelledError:
                raise
            except MatrixError as e:
                wait = 60 if e.status in (401, 403) else delay
                logger.error(f"Matrix sync failed ({e}); retrying in {wait}s")
                if e.status == 401:
                    self.user_id = None  # re-check the token next time round
                await asyncio.sleep(wait)
                delay = min(delay * 2, 60)
            except Exception as e:
                logger.error(f"Matrix connection failed ({type(e).__name__}); retrying in {delay}s")
                await asyncio.sleep(delay)
                delay = min(delay * 2, 60)

    # -- messages

    async def reply(self, text, buttons=None):
        text = ch.bang_prefix(text)
        if buttons:
            text = self.choices.add(text, buttons)
        await self.send(text)

    async def reply_all(self, replies):
        """Send a batch of (text, buttons); a batch with buttons starts a fresh
        numbering (so "!1" always refers to the latest list)."""
        if any(b for _, b in replies):
            self.choices.start_batch()
        for text, buttons in replies:
            await self.reply(text, buttons)

    async def on_message(self, sender, body):
        who = f"matrix:{sender}"
        # A numbered button choice ("!2", or "2" while a list is pending).
        data = self.choices.pick(body)
        if data is None:
            try:
                data = ch.parse_rem(body)
            except ValueError as e:
                await self.reply(str(e))
                return
        if data is not None:
            ok, message = await kc.handle_callback(self.bot_token, who, data)
            await self.reply(message)
            return
        parsed = kc.split_command(body)
        if parsed:
            cmd, args = parsed
            text, buttons = await kc.run_command(self.bot_token, who, cmd, args)
            await self.reply_all([(text, buttons)])
            return
        if body.startswith(("!", "/")):
            return
        try:
            replies = await kc.log_spending(self.bot_token, who, body, "matrix")
        except kc.BackendError as e:
            await self.reply(f"Failed to save: {e.message}")
            return
        except (URLError, OSError):
            return
        await self.reply_all(replies or [])


async def run():
    await ch.run_tenants("matrix", MatrixTenant, required=("platform_token", "extra", "chat_id"))


if __name__ == "__main__":
    asyncio.run(run())
