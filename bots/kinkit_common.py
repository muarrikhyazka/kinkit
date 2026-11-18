"""Shared helpers for the Kinkit Telegram and Discord bots.

Both bots talk to the Go backend the same way:
  * /api/internal/* with X-Service-Token (which tenants/integrations exist)
  * /api/bot/*      with X-Bot-Token (the tenant) + X-Bot-User
                    "<platform>:<chat user id>" (which household member)

Commands are platform-neutral here: run_command() returns (text, buttons)
where buttons is a list of rows of (label, callback_data); each bot renders
them as a Telegram inline keyboard / Discord components. Callback data:
  rem:<occurrence id>:done|snooze60|snooze_tomorrow   (reminders)
  chore:<chore id>:done                               (chores)
"""
import asyncio
import json
import os
import re
from datetime import datetime, timezone, timedelta
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen

try:
    from zoneinfo import ZoneInfo
except ImportError:  # pragma: no cover
    ZoneInfo = None

BACKEND_URL = os.environ.get("BACKEND_URL", "http://localhost:5000")
SERVICE_TOKEN = os.environ.get("SERVICE_TOKEN", "")
FALLBACK_TZ = timezone(timedelta(hours=7))

AMOUNT_RE = re.compile(r"(\d[\d.,]*)\s*(k|rb|jt|ribu|juta)?", re.IGNORECASE)
MULTIPLIER = {"k": 1_000, "rb": 1_000, "ribu": 1_000, "jt": 1_000_000, "juta": 1_000_000}

HELP_TEXT = (
    "Kinkit bot\n"
    "\n"
    "Log spending: just type it, e.g. \"50k lunch\" or \"25000 parking\"\n"
    "/recent - last 5 expenses\n"
    "\n"
    "/today - today's reminders, chores, shopping and budget\n"
    "/remind <when> <what> [@name] - e.g.\n"
    "   /remind every tue and fri 19:00 take out the trash @budi\n"
    "   /remind every 2 weeks on fri 10:00 change bed sheets\n"
    "   /remind every other week on fri 9am wash prayer mats\n"
    "   /remind every 3 months on the 1st change motorbike oil\n"
    "   /remind every day 7am water the plants\n"
    "   /remind tomorrow 8am take medicine\n"
    "   /remind in 2 hours call grandma\n"
    "   Indonesian works too: /remind tiap 2 minggu jumat jam 9 cuci sajadah\n"
    "/reminders - upcoming reminders (with category)\n"
    "/chores - today's chores\n"
    "/buy milk, eggs, bread - add to the shopping list\n"
    "/list [name] - show a list\n"
    "/done_shopping 250k - finish the shopping trip (total spent)\n"
    "/link CODE - link your chat account (code from Kinkit › Reminders)\n"
)

REMIND_USAGE = (
    "Usage: /remind <when> <what> [@name]\n"
    "Household examples:\n"
    "  /remind every 2 weeks on fri 10:00 change bed sheets\n"
    "  /remind every tue and fri 19:00 take out the trash\n"
    "  /remind every sat 9am clean the bathroom @budi\n"
    "  /remind every 3 months on the 1st change motorbike oil\n"
    "  /remind every 6 months service the AC\n"
    "Repeat every N weeks/months: \"every 2 weeks\", \"every other week\", \"every 3 months\" "
    "(also \"tiap 2 minggu\", \"setiap 3 bulan\")."
)


def format_reminders(res):
    """Text for /reminders: the backend's text, or built from items (with category labels)."""
    if res.get("text"):
        return res["text"]
    items = res.get("items") or []
    if not items:
        return "No reminders in the next 7 days."
    lines = ["Upcoming reminders (7 days)"]
    for it in items:
        label = it.get("category_label") or "Other"
        who = f" ({it['who']})" if it.get("who") else ""
        lines.append(f"- {it.get('title', '')} [{label}]{who}")
    return "\n".join(lines)


class BackendError(Exception):
    def __init__(self, status, message):
        super().__init__(message)
        self.status = status
        self.message = message


def backend_get(path, params=None):
    url = f"{BACKEND_URL}{path}"
    if params:
        url += "?" + urlencode(params)
    req = Request(url, headers={"X-Service-Token": SERVICE_TOKEN})
    with urlopen(req, timeout=10) as resp:
        return json.loads(resp.read().decode())


def bot_api(bot_token, method, path, body=None, params=None, user=None):
    """Call /api/bot/*. Raises BackendError (with the backend's error text)
    on HTTP errors and URLError when unreachable."""
    url = f"{BACKEND_URL}{path}"
    if params:
        url += "?" + urlencode({k: v for k, v in params.items() if v is not None})
    data = json.dumps(body).encode() if body is not None else (b"{}" if method == "POST" else None)
    headers = {"Content-Type": "application/json", "X-Bot-Token": bot_token}
    if user:
        headers["X-Bot-User"] = user
    req = Request(url, data=data, method=method, headers=headers)
    try:
        with urlopen(req, timeout=10) as resp:
            raw = resp.read().decode()
            return json.loads(raw) if raw else {}
    except HTTPError as e:
        msg = f"HTTP {e.code}"
        try:
            body = json.loads(e.read().decode())
            # feature endpoints put a friendly text in "message", errors in "error"
            msg = body.get("message") or body.get("error") or msg
        except Exception:
            pass
        raise BackendError(e.code, msg)


async def abot_api(*args, **kwargs):
    """bot_api in a worker thread so the event loop keeps serving updates."""
    return await asyncio.to_thread(bot_api, *args, **kwargs)


def parse_spending(text, keywords):
    """'50000 makan siang' or '50k makan' -> (amount, description, category)."""
    m = AMOUNT_RE.search(text)
    if not m:
        return None
    raw, unit = m.groups()
    amount = int(float(raw.replace(".", "").replace(",", "."))) if "," in raw else int(raw.replace(".", ""))
    if unit:
        amount *= MULTIPLIER.get(unit.lower(), 1)
    description = (text[:m.start()] + text[m.end():]).strip() or "expense"
    category = None
    for word in re.findall(r"[a-z]+", description.lower()):
        if word in keywords:
            category = keywords[word]
            break
    return amount, description, category


def local_today(cfg):
    """Household-local date for logging spending (backend sends it)."""
    if cfg.get("today"):
        return cfg["today"]
    tz = FALLBACK_TZ
    if ZoneInfo and cfg.get("timezone"):
        try:
            tz = ZoneInfo(cfg["timezone"])
        except Exception:
            pass
    return datetime.now(tz).strftime("%Y-%m-%d")


def format_rp(amount):
    return f"Rp{amount:,}".replace(",", ".")


async def log_spending(bot_token, user, text, source):
    """Returns a reply string, or None when the text isn't a spending entry."""
    cfg = await abot_api(bot_token, "GET", "/api/bot/config", user=user)
    parsed = parse_spending(text, cfg.get("category_keywords", {}))
    if not parsed:
        return None
    amount, description, category = parsed
    await abot_api(bot_token, "POST", "/api/bot/spending", {
        "tx_date": local_today(cfg), "description": description, "category": category,
        "amount": amount, "source": source,
    }, user=user)
    return f"Logged: {description} - {format_rp(amount)} ({category or 'Other'})"


async def recent_spending(bot_token, user):
    rows = await abot_api(bot_token, "GET", "/api/bot/spending/recent", params={"n": 5}, user=user)
    if not rows:
        return "No entries yet."
    return "\n".join(
        f"{r['tx_date']} - {r['description']} - {format_rp(r['amount'])} ({r.get('category_name') or 'Other'})" for r in rows
    )


def _not_ready(feature):
    return f"{feature} aren't available on this server yet."


async def run_command(bot_token, user, cmd, args):
    """Execute a chat command. Returns (text, buttons)."""
    cmd = cmd.lower().lstrip("/!").split("@")[0].replace("-", "_")
    args = (args or "").strip()
    try:
        if cmd in ("help", "start"):
            return HELP_TEXT, []
        if cmd == "recent":
            return await recent_spending(bot_token, user), []
        if cmd == "today":
            res = await abot_api(bot_token, "GET", "/api/bot/today", user=user)
            return res.get("text") or "Nothing today.", []
        if cmd == "remind":
            if not args:
                return REMIND_USAGE, []
            res = await abot_api(bot_token, "POST", "/api/bot/reminders", {"text": args}, user=user)
            return res.get("message", "OK"), []
        if cmd == "reminders":
            res = await abot_api(bot_token, "GET", "/api/bot/reminders", user=user)
            return format_reminders(res), []
        if cmd == "link":
            if not args:
                return "Usage: /link CODE - create the code in Kinkit › Reminders › Link my chat account.", []
            res = await abot_api(bot_token, "POST", "/api/bot/link", {"code": args.split()[0]}, user=user)
            return res.get("message", "OK"), []
        if cmd == "chores":
            return await chores_today(bot_token, user)
        if cmd in ("buy", "add"):
            if not args:
                return "Usage: /buy milk, eggs, bread", []
            res = await abot_api(bot_token, "POST", "/api/bot/lists/items", {"text": args}, user=user)
            added = res.get("added") or []
            if not added:
                return res.get("message") or "Nothing added.", []
            return f"Added to {res.get('list') or 'the list'}: {', '.join(added)}", []
        if cmd == "list":
            res = await abot_api(bot_token, "GET", "/api/bot/lists", params={"list": args or None}, user=user)
            items = res.get("items") or []
            name = res.get("list") or "Shopping"
            if not items:
                return f"{name} is empty.", []
            lines = [f"{name}"]
            for it in items:
                qty = f" ({it['qty']})" if it.get("qty") else ""
                lines.append(f"• {it.get('name')}{qty}")
            return "\n".join(lines), []
        if cmd in ("done_shopping", "doneshopping", "finish"):
            # "/done_shopping 250k" -> total; without it the backend asks for one
            body = {"total_amount": args.strip()} if args and args.strip() else {}
            res = await abot_api(bot_token, "POST", "/api/bot/lists/finish", body, user=user)
            return res.get("message", "Shopping done!"), []
    except BackendError as e:
        if 400 <= e.status < 500 and not e.message.startswith("HTTP "):
            return e.message, []
        if e.status == 404:
            return _not_ready({"chores": "Chores"}.get(cmd, "Lists" if cmd in ("buy", "add", "list", "done_shopping", "doneshopping", "finish") else "That command")), []
        if e.status == 403:
            return e.message, []
        return f"Backend error: {e.message}", []
    except (URLError, OSError):
        return "Backend unreachable, try again later.", []
    return f"Unknown command /{cmd}. Try /help", []


async def chores_today(bot_token, user):
    res = await abot_api(bot_token, "GET", "/api/bot/chores/today", user=user)
    items = res.get("items") or []
    if not items:
        return "No chores today.", []
    lines, buttons = ["Today's chores"], []
    for it in items:
        mark = "[x]" if it.get("done") else "[ ]"
        who = f" ({it['assignee_name']})" if it.get("assignee_name") else ""
        pts = f" +{it['points']}pt" if it.get("points") else ""
        lines.append(f"{mark} {it.get('title')}{who}{pts}")
        if not it.get("done") and len(buttons) < 5:
            buttons.append([(f"Done: {it.get('title')}"[:40], f"chore:{it['id']}:done")])
    return "\n".join(lines), buttons


async def handle_callback(bot_token, user, data):
    """Button press -> (ok, short message)."""
    parts = (data or "").split(":")
    try:
        if len(parts) == 3 and parts[0] == "rem" and parts[1].isdigit():
            oid, action = parts[1], parts[2]
            if action == "done":
                res = await abot_api(bot_token, "POST", f"/api/bot/reminders/occurrences/{oid}/done", {}, user=user)
            elif action == "snooze60":
                res = await abot_api(bot_token, "POST", f"/api/bot/reminders/occurrences/{oid}/snooze", {"minutes": 60}, user=user)
            elif action == "snooze_tomorrow":
                res = await abot_api(bot_token, "POST", f"/api/bot/reminders/occurrences/{oid}/snooze", {"until": "tomorrow"}, user=user)
            else:
                return False, "Unknown action"
            return bool(res.get("ok")), res.get("message", "OK")
        if len(parts) == 3 and parts[0] == "chore" and parts[1].isdigit() and parts[2] == "done":
            res = await abot_api(bot_token, "POST", f"/api/bot/chores/{parts[1]}/done", {}, user=user)
            return bool(res.get("ok", True)), res.get("message", "Chore done")
    except BackendError as e:
        return False, e.message
    except (URLError, OSError):
        return False, "Backend unreachable, try again later."
    return False, "Unknown button"


COMMANDS = ("help", "start", "recent", "today", "remind", "reminders", "link", "chores", "buy", "add", "list",
            "done_shopping", "doneshopping", "finish")
# Commands that work outside the household chat (e.g. in a private DM).
ANYWHERE = ("help", "start", "link")


def split_command(text, prefixes=("/", "!")):
    """'/remind tomorrow 8am x' -> ('remind', 'tomorrow 8am x'); None if not a known command."""
    text = (text or "").strip()
    if not text or text[0] not in prefixes:
        return None
    head, _, rest = text[1:].partition(" ")
    cmd = head.lower().split("@")[0].replace("-", "_")
    if cmd not in COMMANDS:
        return None
    return cmd, rest.strip()
