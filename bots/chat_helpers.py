"""Pure helpers shared by the Slack and Matrix bots (no network, stdlib only,
so they are unit-tested in test_slack_matrix.py) plus the multi-tenant loop.

Slack: Block Kit rendering of (text, buttons), block_actions result blocks,
message-event filtering, mrkdwn escaping.
Matrix: no buttons exist in plain m.text events, so buttons become a numbered
list ("1) Done") and the user answers "!1"; also "!rem <id> done|snooze|tomorrow"
for backend-sent reminders (plain text). Plus URL/txn builders and sync
parsing for the client-server API.
"""
import asyncio
import html
import json
import logging
import os
import re
import time
import uuid
from urllib.parse import quote

import kinkit_common as kc

logger = logging.getLogger("kinkit-chat")

REFRESH_SECONDS = int(os.environ.get("REFRESH_SECONDS", "300"))  # fallback full rebuild even if the version never changes
VERSION_CHECK_SECONDS = int(os.environ.get("VERSION_CHECK_SECONDS", "5"))

# Slack and Matrix clients treat "/..." as their own client-side commands, so
# these bots document (and prefer) the "!" prefix; "/" still works when it
# arrives as plain text.
_SLASH_CMD = re.compile(r"(?<![\w/:.<])/(?=[a-z])")


def bang_prefix(text):
    """HELP/usage text written with '/cmd' -> '!cmd' for chat clients that
    swallow '/' commands. 'Telegram/Discord' and URLs are left alone."""
    return _SLASH_CMD.sub("!", text or "")


# ---------------------------------------------------------------- Slack

SLACK_TEXT_LIMIT = 3000  # a section block's text limit


def slack_escape(text):
    """Escape the three characters Slack treats specially in message text."""
    return (text or "").replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


_SLACK_LINK = re.compile(r"<(https?://[^|>]+)(?:\|([^>]*))?>")
_SLACK_USER = re.compile(r"<@([UW][A-Z0-9]+)(?:\|[^>]*)?>")
_SLACK_CHANNEL = re.compile(r"<#[CG][A-Z0-9]+\|([^>]*)>")
_SLACK_SPECIAL = re.compile(r"<!([a-z]+)(?:\|([^>]*))?>")


def slack_user_ids(text):
    """Slack user ids mentioned in a raw message text ('<@U123ABC>')."""
    return _SLACK_USER.findall(text or "")


def slack_unescape(text, names=None):
    """Raw Slack message text -> plain text the command parser understands.
    '<@U123>' -> '@name' (names maps id -> name; unknown ids stay '@U123'),
    '<http://x|label>' -> 'label', '&amp;' -> '&'."""
    names = names or {}
    text = _SLACK_USER.sub(lambda m: "@" + names.get(m.group(1), m.group(1)), text or "")
    text = _SLACK_CHANNEL.sub(lambda m: "#" + m.group(1), text)
    text = _SLACK_LINK.sub(lambda m: m.group(2) or m.group(1), text)
    text = _SLACK_SPECIAL.sub(lambda m: "@" + (m.group(2) or m.group(1)).lstrip("@"), text)
    return html.unescape(text).strip()


def slack_blocks(text, buttons):
    """(text, rows of (label, data)) -> Block Kit blocks: one section with the
    text, then one actions block per button row. action_id = value = the
    callback data. Returns None when there are no buttons (plain text post).
    Limits: label 75 chars, action_id/value 255, 25 buttons per actions block."""
    rows = [[b for b in row if b[1]] for row in (buttons or [])]
    rows = [r for r in rows if r]
    if not rows:
        return None
    blocks = [{"type": "section", "text": {"type": "mrkdwn", "text": slack_escape(text)[:SLACK_TEXT_LIMIT] or " "}}]
    for row in rows:
        blocks.append({"type": "actions", "elements": [
            {"type": "button", "text": {"type": "plain_text", "text": (label or "-")[:75]},
             "action_id": data[:255], "value": data[:255]}
            for label, data in row[:25]
        ]})
    return blocks[:50]


def slack_result_blocks(original_blocks, original_text, result):
    """After a button press: keep the message's non-button blocks (or its
    text) and append the result line, so the button row disappears."""
    kept = [b for b in (original_blocks or []) if b.get("type") != "actions"]
    if not kept:
        kept = [{"type": "section", "text": {"type": "mrkdwn", "text": slack_escape(original_text)[:SLACK_TEXT_LIMIT] or " "}}]
    kept.append({"type": "context", "elements": [
        {"type": "mrkdwn", "text": slack_escape(result)[:SLACK_TEXT_LIMIT] or " "}]})
    return kept[:50]


def slack_message_event(payload, own_user_id=None):
    """events_api payload -> (channel, user, text, is_dm, event_id) for a
    plain user message, or None (bot/own messages, edits, joins, other types)."""
    ev = (payload or {}).get("event") or {}
    if ev.get("type") != "message" or ev.get("subtype") or ev.get("bot_id"):
        return None
    user, text, channel = ev.get("user"), ev.get("text"), ev.get("channel")
    if not user or not channel or not isinstance(text, str) or not text.strip():
        return None
    if own_user_id and user == own_user_id:
        return None
    return channel, user, text, ev.get("channel_type") == "im", payload.get("event_id") or ev.get("client_msg_id") or ev.get("ts")


def slack_block_action(payload):
    """interactive payload -> (data, user_id, channel, ts, blocks, text) for a
    block_actions button click, or None."""
    if (payload or {}).get("type") != "block_actions":
        return None
    actions = payload.get("actions") or []
    if not actions:
        return None
    data = actions[0].get("value") or actions[0].get("action_id") or ""
    msg = payload.get("message") or {}
    channel = (payload.get("channel") or {}).get("id") or (payload.get("container") or {}).get("channel_id")
    user = (payload.get("user") or {}).get("id")
    if not data or not user:
        return None
    return data, user, channel, msg.get("ts"), msg.get("blocks"), msg.get("text") or ""


def slack_allowed(channel, chat_id, is_dm, cmd=None):
    """Household channel only; DMs and other channels just for ANYWHERE
    commands. An empty chat_id means any non-DM channel."""
    if is_dm:
        return cmd in kc.ANYWHERE if cmd else False
    if not chat_id:
        return True
    if channel == chat_id:
        return True
    return bool(cmd) and cmd in kc.ANYWHERE


# ---------------------------------------------------------------- Matrix

BUTTON_TTL = 24 * 3600
MATRIX_TEXT_LIMIT = 8000
# Backend-sent reminders carry no buttons on Matrix: !rem <id> <action>
REM_ACTIONS = {"done": "done", "snooze": "snooze60", "snooze60": "snooze60", "1h": "snooze60",
               "tomorrow": "snooze_tomorrow", "snooze_tomorrow": "snooze_tomorrow"}


def render_numbered(text, buttons, start=1):
    """Buttons -> '1) Label' lines appended to text. Returns
    (text, {number: callback data}, next number)."""
    mapping, lines, n = {}, [], start
    for row in buttons or []:
        for label, data in row:
            mapping[n] = data
            lines.append(f"{n}) {label}")
            n += 1
    if not mapping:
        return text, {}, start
    hint = "Reply " + (f"!{start}" if len(mapping) == 1 else f"!{start} ... !{n - 1}") + " to choose."
    return f"{text}\n\n" + "\n".join(lines) + "\n" + hint, mapping, n


def parse_choice(body, mapping):
    """'!2' / '2' (bare number only while a list is pending) -> callback data."""
    body = (body or "").strip()
    m = re.fullmatch(r"([!/]?)\s*#?(\d{1,3})", body)
    if not m or (not m.group(1) and not mapping):
        return None
    return (mapping or {}).get(int(m.group(2)))


def parse_rem(text):
    """'!rem 12 done' / '!rem 12 snooze' / '!rem 12 tomorrow' -> 'rem:12:done'
    ... ; None when it's not that command. ('!rem' with bad args raises
    ValueError carrying the usage text.)"""
    text = (text or "").strip()
    m = re.match(r"^[!/]rem(?:\s+(.*))?$", text, re.IGNORECASE | re.DOTALL)
    if not m:
        return None
    parts = (m.group(1) or "").split()
    if len(parts) != 2 or not parts[0].isdigit() or parts[1].lower() not in REM_ACTIONS:
        raise ValueError("Usage: !rem <id> done | snooze | tomorrow  (the id is in the reminder message)")
    return f"rem:{parts[0]}:{REM_ACTIONS[parts[1].lower()]}"


class ChoiceMemory:
    """Per-room numbered button lists. Numbers keep counting within one batch
    of replies and restart for the next batch; entries expire after a day."""

    def __init__(self, ttl=BUTTON_TTL, clock=time.time):
        self.ttl, self.clock = ttl, clock
        self.mapping, self.at = {}, 0.0

    def start_batch(self):
        self.mapping, self.at = {}, self.clock()

    def add(self, text, buttons):
        """Render one reply; remembers its numbers. Returns the text to send."""
        start = (max(self.mapping) + 1) if self.mapping else 1
        out, m, _ = render_numbered(text, buttons, start)
        self.mapping.update(m)
        self.at = self.clock()
        return out

    def live(self):
        if self.mapping and self.clock() - self.at > self.ttl:
            self.mapping = {}
        return self.mapping

    def pick(self, body):
        data = parse_choice(body, self.live())
        if data is not None:
            for k, v in list(self.mapping.items()):
                if v == data:
                    del self.mapping[k]
        return data


def valid_homeserver(url):
    return bool(re.fullmatch(r"https?://[^\s/?#]+(?:/[^\s?#]*)?", url or ""))


def matrix_base(homeserver):
    return (homeserver or "").strip().rstrip("/")


def matrix_url(homeserver, path):
    return f"{matrix_base(homeserver)}/_matrix/client/v3{path}"


def matrix_send_path(room, txn):
    return f"/rooms/{quote(room, safe='')}/send/m.room.message/{quote(txn, safe='')}"


def matrix_join_path(room):
    return f"/join/{quote(room, safe='')}"


def matrix_txn_id():
    return "kinkit-" + uuid.uuid4().hex


def matrix_filter(room):
    """Sync filter: only the household room, messages only, no presence etc."""
    return json.dumps({
        "presence": {"not_types": ["*"]}, "account_data": {"not_types": ["*"]},
        "room": {"rooms": [room], "ephemeral": {"not_types": ["*"]}, "account_data": {"not_types": ["*"]},
                 "state": {"types": []}, "timeline": {"types": ["m.room.message"], "limit": 50}},
    }, separators=(",", ":"))


def strip_reply_fallback(body):
    """Drop the '> <@user> quoted' lines Matrix clients prepend to replies."""
    lines = (body or "").split("\n")
    i = 0
    while i < len(lines) and lines[i].startswith(">"):
        i += 1
    if i and i < len(lines) and lines[i].strip() == "":
        i += 1
    return "\n".join(lines[i:]).strip() if i else (body or "").strip()


def matrix_events(sync, room, own_user):
    """Sync response -> [(event_id, sender, body)] of new plain-text messages
    from other users in `room` (own messages, notices, edits, encrypted and
    non-text events are skipped)."""
    out = []
    timeline = ((((sync or {}).get("rooms") or {}).get("join") or {}).get(room) or {}).get("timeline") or {}
    for ev in timeline.get("events") or []:
        if ev.get("type") != "m.room.message" or ev.get("sender") in (None, own_user):
            continue
        content = ev.get("content") or {}
        if content.get("msgtype") != "m.text" or not isinstance(content.get("body"), str):
            continue
        rel = content.get("m.relates_to") or {}
        if rel.get("rel_type") == "m.replace":
            continue  # an edit of an earlier message
        body = strip_reply_fallback(content["body"])
        if body:
            out.append((ev.get("event_id"), ev["sender"], body))
    return out


def matrix_invited(sync, room):
    return room in ((((sync or {}).get("rooms") or {}).get("invite")) or {})


# ---------------------------------------------------------------- multiplexer

def integration_signature(integ):
    return tuple(integ.get(k) for k in ("bot_token", "platform_token", "chat_id", "extra"))


async def run_tenants(platform, factory, required=("platform_token", "extra")):
    """Multi-tenant loop shared by the Slack and Matrix bots (same cadence as
    the Telegram/Discord ones): a cheap GET /api/internal/integrations-version
    every VERSION_CHECK_SECONDS; the full fetch + diff only when it changed (or
    every REFRESH_SECONDS). factory(integ) -> object with async start()/stop();
    a tenant is restarted when its credentials, channel or room changed;
    one missing a `required` field is skipped until configured."""
    tenants = {}  # account_id -> (signature, tenant)
    last_version, last_full = None, 0.0
    loop = asyncio.get_event_loop()
    while True:
        try:
            version = (await asyncio.to_thread(kc.backend_get, "/api/internal/integrations-version")).get("version")
        except Exception as e:
            logger.error(f"Could not fetch integrations version: {e}")
            version = last_version
        now = loop.time()
        if version == last_version and (now - last_full) < REFRESH_SECONDS:
            await asyncio.sleep(VERSION_CHECK_SECONDS)
            continue
        last_version, last_full = version, now
        try:
            integrations = await asyncio.to_thread(kc.backend_get, "/api/internal/integrations", {"platform": platform})
        except Exception as e:
            logger.error(f"Could not fetch integrations: {e}")
            integrations = None
        if integrations is not None:
            seen = set()
            for integ in integrations:
                aid = integ["account_id"]
                seen.add(aid)
                sig = integration_signature(integ)
                if aid in tenants and tenants[aid][0] == sig:
                    continue
                if aid in tenants:
                    logger.info(f"Restarting {platform} bot for account {aid} (settings changed)")
                    await tenants.pop(aid)[1].stop()
                if not all(integ.get(k) for k in required):
                    continue  # not fully configured yet
                try:
                    tenant = factory(integ)
                    await tenant.start()
                except Exception as e:  # never let one tenant break the loop
                    logger.error(f"Could not start {platform} bot for account {aid}: {type(e).__name__}")
                    continue
                tenants[aid] = (sig, tenant)
                logger.info(f"Started {platform} bot for account {aid}")
            for aid in list(tenants):
                if aid not in seen:
                    logger.info(f"Stopping {platform} bot for account {aid} (integration removed/disabled)")
                    await tenants.pop(aid)[1].stop()
        await asyncio.sleep(VERSION_CHECK_SECONDS)
