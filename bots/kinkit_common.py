"""Shared helpers for the Kinkit Telegram and Discord bots.

Both bots talk to the Go backend the same way:
  * /api/internal/* with X-Service-Token (which tenants/integrations exist)
  * /api/bot/*      with X-Bot-Token (the tenant) + X-Bot-User
                    "<platform>:<chat user id>" (which household member)

Commands are platform-neutral here: run_command() returns (text, buttons)
where buttons is a list of rows of (label, callback_data); each bot renders
them as a Telegram inline keyboard / Discord components. Callback data
(<= 64 bytes, Telegram's limit):
  rem:<occurrence id>:done|snooze60|snooze_tomorrow   (reminders, sent by the backend)
  chore:<chore id>:done                               (chores)
  spc:<pending token>:<category id>                   (pick a category for an unknown expense)
  spx:<pending token>                                 (not an expense, skip it)
  fixcat:<spending id>:<category id>                  (/fix, /koreksi)
  fixdate:<spending id>:<YYYYMMDD>                    (/fixdate, /koreksitgl)

Merged from the household "rumahbot": multi-line spending logging with a
category picker that learns keywords, /fix and /fixdate corrections, /date
(log under another date), the weekly meal plan and must-use ingredients,
/schedule for the meal plan post, /feedback, /about, and rumahbot's
Indonesian command names as aliases (see ALIASES).
"""
import asyncio
import json
import os
import re
import secrets
import time
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

MULTIPLIER = {"k": 1_000, "rb": 1_000, "ribu": 1_000, "jt": 1_000_000, "juta": 1_000_000}
_UNIT = r"(jt|juta|rb|ribu|k)"
# "makan siang 50rb", "bensin rp20.000", "es teh 5k"  (amount last)
TRAILING_RE = re.compile(r"^(.+?)\s+(?:rp\.?\s*)?(\d[\d.,]*)\s*" + _UNIT + r"?$", re.IGNORECASE)
# "50k lunch", "rp75.000 bensin", "25000 parking"  (amount first)
LEADING_RE = re.compile(r"^(?:(rp)\.?\s*)?(\d[\d.,]*)\s*" + _UNIT + r"?\s+(.+)$", re.IGNORECASE)
# A bare leading number below this (no unit, no "rp") is chat, not spending: "2 porsi lagi".
MIN_BARE_LEADING = 100

HELP_TEXT = (
    "Kinkit bot\n"
    "\n"
    "Spending - just type it, one per line:\n"
    "   makan siang 50rb  /  50k lunch  /  bensin rp20.000\n"
    "   Unknown words get category buttons; the bot remembers your pick.\n"
    "/recent [n] - last expenses (/last)\n"
    "/fix <words> <category> - change a category (/koreksi)\n"
    "/fixdate <words> YYYY-MM-DD - change a date (/koreksitgl)\n"
    "/date YYYY-MM-DD - log the next expenses on another day; /date reset (/tanggal)\n"
    "\n"
    "Home:\n"
    "/today - today's reminders, chores, shopping and budget (/hariini)\n"
    "/remind <when> <what> [@name] - e.g. (/ingatkan)\n"
    "   /remind every tue and fri 19:00 take out the trash @budi\n"
    "   /remind every 2 weeks on fri 10:00 change bed sheets\n"
    "   /remind tomorrow 8am take medicine\n"
    "   Indonesian works too: /remind tiap 2 minggu jumat jam 9 cuci sajadah\n"
    "/reminders - upcoming reminders (/pengingat)\n"
    "/chores - today's chores\n"
    "/buy milk, eggs, bread - add to the shopping list\n"
    "/list [name] - show a list\n"
    "/done_shopping 250k - finish the shopping trip (total spent)\n"
    "\n"
    "Meals:\n"
    "/menu [this] - weekly meal plan (next week, or this week)\n"
    "/ingredients - must-use ingredients (/bahan)\n"
    "/addingredient <name> (/tambahbahan)  /removeingredient <name> (/hapusbahan)\n"
    "/schedule - when the meal plan is posted; /schedule meal fri 06:00 (/jadwal, /reminder)\n"
    "\n"
    "/link CODE - link your chat account (code from Kinkit > Reminders)\n"
    "/feedback <idea> - request a feature (/fitur)\n"
    "/about - about this bot\n"
)

SPENDING_HELP = (
    "Log spending\n"
    "Type: <description> <amount>, one per line\n"
    "   makan siang 50rb\n"
    "   bensin rp20.000\n"
    "   50k lunch\n"
    "Units: k / rb / ribu = thousand, jt / juta = million.\n"
    "Known words are categorised automatically; otherwise pick a category from the buttons "
    "and the bot learns the words for next time.\n"
    "Wrong category? /fix <words> <category>. Wrong day? /fixdate <words> YYYY-MM-DD."
)

ABOUT_TEXT = (
    "Kinkit bot - family money and home.\n"
    "Log spending, reminders, chores, shopping lists and the weekly meal plan from this chat; "
    "everything lands in your household's Kinkit, where you can see and edit it on the web and the phone app.\n"
    "/help - all commands"
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

# rumahbot command names (Indonesian) and English synonyms -> canonical command.
ALIASES = {
    "start": "help", "bantuan": "help", "all": "help",
    "hariini": "today",
    "ingatkan": "remind",
    "pengingat": "reminders",
    "last": "recent",
    "mealplan": "menu", "meals": "menu",
    "bahan": "ingredients",
    "tambahbahan": "addingredient",
    "hapusbahan": "removeingredient",
    "koreksi": "fix",
    "koreksitgl": "fixdate",
    "tanggal": "date",
    "jadwal": "schedule", "reminder": "schedule",
    "fitur": "feedback",
    "pengeluaran": "spending",
    "add": "buy",
    "doneshopping": "done_shopping", "finish": "done_shopping",
}

COMMANDS = ("help", "recent", "today", "remind", "reminders", "link", "chores", "buy", "list", "done_shopping",
            "menu", "ingredients", "addingredient", "removeingredient", "fix", "fixdate", "date", "schedule",
            "feedback", "spending", "about") + tuple(ALIASES)
# Commands that work outside the household chat (e.g. in a private DM).
ANYWHERE = ("help", "start", "bantuan", "all", "link", "about")


def canonical(cmd):
    cmd = (cmd or "").lower().lstrip("/!").split("@")[0].replace("-", "_")
    return ALIASES.get(cmd, cmd)


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


# ---------------------------------------------------------------- spending parsing

def parse_amount(raw, unit):
    """'50.000' -> 50000, '1,5' + 'jt' -> 1500000 (dot = thousands, comma = decimal)."""
    try:
        value = float(raw.replace(".", "").replace(",", "."))
    except ValueError:
        return None
    if unit:
        value *= MULTIPLIER.get(unit.lower(), 1)
    amount = int(round(value))
    return amount if amount > 0 else None


def match_category(description, keywords):
    """First keyword hit in the description (multi-word keywords first)."""
    low = " ".join(description.lower().split())
    for kw in sorted((k for k in keywords if " " in k), key=len, reverse=True):
        if re.search(r"(?<![a-z])" + re.escape(kw) + r"(?![a-z])", low):
            return keywords[kw]
    for tok in low.split():
        if tok in keywords:
            return keywords[tok]
        for sub in re.findall(r"[a-z]+", tok):
            if sub in keywords:
                return keywords[sub]
    return None


def parse_spending(text, keywords):
    """One line -> (amount, description, category or None), or None.
    'makan siang 50rb' / 'bensin rp20.000' / '50k lunch' / '25000 parking'."""
    text = " ".join((text or "").split())
    if not text or text.startswith(("/", "!")):
        return None
    m = TRAILING_RE.match(text)
    if m:
        desc, raw, unit = m.groups()
        amount = parse_amount(raw, unit)
    else:
        m = LEADING_RE.match(text)
        if not m:
            return None
        rp, raw, unit, desc = m.groups()
        amount = parse_amount(raw, unit)
        if amount is not None and not unit and not rp and amount < MIN_BARE_LEADING:
            return None
    desc = desc.strip(" -:,")
    if amount is None or not desc or not re.search(r"[A-Za-z]", desc):
        return None
    return amount, desc, match_category(desc, keywords)


def parse_spending_lines(text, keywords):
    """Every spending line of a (possibly multi-line) message."""
    out = []
    for line in (text or "").splitlines():
        p = parse_spending(line, keywords)
        if p:
            out.append(p)
    return out


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
    return f"Rp{int(amount):,}".replace(",", ".")


# ---------------------------------------------------------------- ephemeral chat state
# Pending "which category?" prompts and /date overrides live in this process
# only (like rumahbot): a restart drops them, the user just resends.
PENDING_TTL = 24 * 3600
PENDING = {}         # token -> {bot_token, user, desc, amount, date, source, at}
DATE_OVERRIDE = {}   # bot_token -> "YYYY-MM-DD"


def _gc_pending():
    now = time.time()
    for k in [k for k, v in PENDING.items() if now - v["at"] > PENDING_TTL]:
        PENDING.pop(k, None)


def new_pending(bot_token, user, desc, amount, date, source):
    _gc_pending()
    token = secrets.token_hex(4)
    PENDING[token] = {"bot_token": bot_token, "user": user, "desc": desc, "amount": amount,
                      "date": date, "source": source, "at": time.time()}
    return token


def category_buttons(token, categories, source):
    """Rows of (name, 'spc:<token>:<id>') + a skip button. Discord allows
    5 rows of 5, so it gets 20 categories max."""
    per_row, limit = (5, 20) if source == "discord" else (3, 60)
    rows, row = [], []
    for c in categories[:limit]:
        row.append((c["name"][:30], f"spc:{token}:{c['id']}"))
        if len(row) == per_row:
            rows.append(row)
            row = []
    if row:
        rows.append(row)
    rows.append([("Not an expense", f"spx:{token}")])
    return rows


async def log_spending(bot_token, user, text, source):
    """Returns a list of replies [(text, buttons)], or None when the message
    has no spending line. Known categories are saved right away; unknown ones
    get a category picker (saved when a button is pressed)."""
    cfg = await abot_api(bot_token, "GET", "/api/bot/config", user=user)
    parsed = parse_spending_lines(text, cfg.get("category_keywords", {}))
    if not parsed:
        return None
    date = DATE_OVERRIDE.get(bot_token) or local_today(cfg)
    on = f" on {date}" if DATE_OVERRIDE.get(bot_token) else ""
    categories = cfg.get("category_list") or [{"id": i, "name": n} for i, n in enumerate(cfg.get("categories") or [])]
    replies, logged = [], []
    for amount, desc, category in parsed:
        if category is None and cfg.get("category_list"):
            token = new_pending(bot_token, user, desc, amount, date, source)
            replies.append((f"\"{desc}\" {format_rp(amount)} - which category?", category_buttons(token, categories, source)))
            continue
        res = await abot_api(bot_token, "POST", "/api/bot/spending", {
            "tx_date": date, "description": desc, "category": category, "amount": amount, "source": source,
        }, user=user)
        logged.append(f"{desc} - {format_rp(amount)} ({res.get('category') or category or 'Other'})")
    if logged:
        head = f"Logged{on}: " if len(logged) == 1 else f"Logged {len(logged)} expenses{on}:\n- "
        replies.insert(0, (head + "\n- ".join(logged), []))
    return replies


async def recent_spending(bot_token, user, n=5):
    rows = await abot_api(bot_token, "GET", "/api/bot/spending/recent", params={"n": n}, user=user)
    if not rows:
        return "No entries yet."
    lines = [f"Last {len(rows)} expenses"]
    for i, r in enumerate(rows, 1):
        lines.append(f"{i}. {r['tx_date']} - {r['description']} - {format_rp(r['amount'])} ({r.get('category_name') or 'Other'})")
    return "\n".join(lines)


# ---------------------------------------------------------------- corrections

def split_category_suffix(args, categories):
    """'bakso pak kumis home rent' -> ('bakso pak kumis', {id, name: 'Home Rent'}).
    The longest trailing run of words that names a category wins."""
    words = args.split()
    by_name = {" ".join(c["name"].lower().split()): c for c in categories}
    for k in range(len(words) - 1, 0, -1):
        cat = by_name.get(" ".join(words[k:]).lower())
        if cat:
            return " ".join(words[:k]), cat
    return args.strip(), None


def split_date_suffix(args):
    """'bakso 2026-10-20' -> ('bakso', '2026-10-20')."""
    words = args.split()
    if len(words) >= 2:
        try:
            d = datetime.strptime(words[-1], "%Y-%m-%d").strftime("%Y-%m-%d")
            return " ".join(words[:-1]), d
        except ValueError:
            pass
    return args.strip(), None


def _row_line(i, r):
    return f"{i}. {r['description']} - {r.get('category_name') or 'Other'} | {format_rp(r['amount'])} | {r['tx_date']}"


async def _find_rows(bot_token, user, query):
    """Rows matching query; a number 1-20 means 'the Nth most recent'."""
    if query.isdigit() and 1 <= int(query) <= 20:
        rows = await abot_api(bot_token, "GET", "/api/bot/spending/recent", params={"n": int(query)}, user=user)
        return rows[int(query) - 1:int(query)] if len(rows) >= int(query) else []
    return await abot_api(bot_token, "GET", "/api/bot/spending/search", params={"q": query}, user=user)


async def fix_category(bot_token, user, args):
    cfg = await abot_api(bot_token, "GET", "/api/bot/config", user=user)
    categories = cfg.get("category_list") or []
    cat_names = ", ".join(c["name"] for c in categories)
    if not args:
        text = await recent_spending(bot_token, user, 5)
        return f"{text}\n\nFix: /fix <words or number> <category>\nCategories: {cat_names}", []
    query, cat = split_category_suffix(args, categories)
    rows = await _find_rows(bot_token, user, query)
    if not rows:
        return f"No expense matches \"{query}\". /fix shows the last 5.", []
    if cat is None:
        lines = [f"Matches for \"{query}\""] + [_row_line(i, r) for i, r in enumerate(rows[:10], 1)]
        lines.append(f"\nFix: /fix {query} <category>\nCategories: {cat_names}")
        return "\n".join(lines), []
    buttons = []
    for r in rows[:10]:
        label = f"{r['description'][:30]} ({r.get('category_name') or 'Other'} > {cat['name']}) {r['tx_date']}"
        buttons.append([(label[:64], f"fixcat:{r['id']}:{cat['id']}")])
    return f"\"{query}\" -> {cat['name']}. Pick the expense to change:", buttons


async def fix_date(bot_token, user, args):
    if not args:
        text = await recent_spending(bot_token, user, 5)
        return f"{text}\n\nFix a date: /fixdate <words or number> YYYY-MM-DD", []
    query, new_date = split_date_suffix(args)
    rows = await _find_rows(bot_token, user, query)
    if not rows:
        return f"No expense matches \"{query}\". /fixdate shows the last 5.", []
    if new_date is None:
        lines = [f"Matches for \"{query}\""] + [_row_line(i, r) for i, r in enumerate(rows[:10], 1)]
        lines.append(f"\nFix: /fixdate {query} YYYY-MM-DD")
        return "\n".join(lines), []
    compact = new_date.replace("-", "")
    buttons = []
    for r in rows[:10]:
        label = f"{r['description'][:30]} ({r['tx_date']} > {new_date})"
        buttons.append([(label[:64], f"fixdate:{r['id']}:{compact}")])
    return f"\"{query}\" -> {new_date}. Pick the expense to change:", buttons


def set_date_override(bot_token, args):
    arg = (args or "").strip().lower()
    if not arg or arg in ("reset", "now", "today", "hariini"):
        had = DATE_OVERRIDE.pop(bot_token, None)
        if had or arg:
            return "Back to logging expenses on today's date."
        return "Usage: /date YYYY-MM-DD (log the next expenses on that day), /date reset to go back to today."
    try:
        d = datetime.strptime(arg, "%Y-%m-%d")
    except ValueError:
        return "Date must be YYYY-MM-DD, e.g. /date 2026-10-20. /date reset goes back to today."
    DATE_OVERRIDE[bot_token] = d.strftime("%Y-%m-%d")
    return f"Next expenses are logged on {d.strftime('%d %b %Y')} until /date reset."


# ---------------------------------------------------------------- meal plan schedule

DAY_INDEX = {
    "minggu": 0, "sun": 0, "sunday": 0, "senin": 1, "mon": 1, "monday": 1,
    "selasa": 2, "tue": 2, "tuesday": 2, "rabu": 3, "wed": 3, "wednesday": 3,
    "kamis": 4, "thu": 4, "thursday": 4, "jumat": 5, "fri": 5, "friday": 5,
    "sabtu": 6, "sat": 6, "saturday": 6,
}
EVERY_DAY = ("setiap", "harian", "daily", "every", "everyday", "all")
MEAL_JOBS = ("meal", "meal_plan", "menu")
# rumahbot jobs that are Kinkit reminders now (seeded by the importer).
REMINDER_JOBS = {
    "spend": "the daily \"log today's spending\" nudge",
    "bahan_remind": "the Thursday \"must-use ingredients for next week?\" question",
    "sarung": "Cuci sarung", "pompa_ban": "Pompa ban", "ganti_oli": "Ganti oli",
    "sprei": "Ganti sprei & handuk", "cuci_toren": "Cuci toren air", "rawat_diri": "Rawat diri Sabtu",
}
SCHEDULE_USAGE = (
    "Usage: /schedule meal <days> <HH:MM> [start YYYY-MM-DD]\n"
    "  /schedule meal fri 06:00\n"
    "  /schedule meal sabtu,minggu 07:30 2026-12-01\n"
    "  /schedule meal off  (or on)\n"
    "Days: sun..sat or minggu..sabtu, comma-separated, or \"daily\" / \"setiap\"."
)


def parse_days(arg):
    arg = arg.strip().lower()
    if arg in EVERY_DAY:
        return list(range(7))
    days = set()
    for part in arg.split(","):
        part = part.strip()
        if part not in DAY_INDEX:
            return None
        days.add(DAY_INDEX[part])
    return sorted(days) or None


def parse_schedule_args(args):
    """'meal fri 06:00 [2026-12-01]' -> (body dict, None) or (None, error text)."""
    parts = args.split()
    if not parts:
        return None, SCHEDULE_USAGE
    job = parts[0].lower()
    if job == "scrape":
        return None, "Grocery promos are now fetched automatically right before the meal plan is posted."
    if job in REMINDER_JOBS:
        return None, (f"{REMINDER_JOBS[job]} is a Kinkit reminder now. Change it on the Reminders page of the Kinkit web app, "
                      "or add one here, e.g. /remind every day 20:00 catat pengeluaran.")
    if job not in MEAL_JOBS:
        return None, f"Unknown job \"{job}\". The only bot job is \"meal\" (weekly meal plan).\n\n{SCHEDULE_USAGE}"
    rest = parts[1:]
    if len(rest) == 1 and rest[0].lower() in ("on", "off"):
        return {"enabled": rest[0].lower() == "on"}, None
    if len(rest) < 2:
        return None, SCHEDULE_USAGE
    days = parse_days(rest[0])
    if days is None:
        return None, "Unknown day name. Use sun..sat, minggu..sabtu, or \"daily\"."
    m = re.fullmatch(r"(\d{1,2})[:.](\d{2})", rest[1])
    if not m or int(m.group(1)) > 23 or int(m.group(2)) > 59:
        return None, "Time must be HH:MM, e.g. 06:00."
    body = {"enabled": True, "days": days, "time": f"{int(m.group(1)):02d}:{m.group(2)}"}
    if len(rest) > 2:
        try:
            body["start_date"] = datetime.strptime(rest[2], "%Y-%m-%d").strftime("%Y-%m-%d")
        except ValueError:
            return None, "Start date must be YYYY-MM-DD."
    else:
        body["start_date"] = ""
    return body, None


# ---------------------------------------------------------------- commands

def _not_ready(feature):
    return f"{feature} aren't available on this server yet."


async def run_command(bot_token, user, cmd, args):
    """Execute a chat command. Returns (text, buttons)."""
    cmd = canonical(cmd)
    args = (args or "").strip()
    try:
        if cmd == "help":
            return HELP_TEXT, []
        if cmd == "about":
            return ABOUT_TEXT, []
        if cmd == "spending":
            return SPENDING_HELP, []
        if cmd == "recent":
            n = 5
            if args:
                if not args.isdigit() or not 1 <= int(args) <= 20:
                    return "Usage: /recent [1-20]", []
                n = int(args)
            return await recent_spending(bot_token, user, n), []
        if cmd == "fix":
            return await fix_category(bot_token, user, args)
        if cmd == "fixdate":
            return await fix_date(bot_token, user, args)
        if cmd == "date":
            return set_date_override(bot_token, args), []
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
        if cmd == "schedule":
            if not args:
                sched = await abot_api(bot_token, "GET", "/api/bot/meal-schedule", user=user)
                rem = await abot_api(bot_token, "GET", "/api/bot/reminders", user=user)
                return (f"{sched.get('text', '')}\nChange: /schedule meal <days> <HH:MM>\n\n"
                        "Household reminders (spending nudge, ingredients question, chores...) are managed on the "
                        "Kinkit Reminders page; add one with /remind.\n\n" + format_reminders(rem)), []
            body, err = parse_schedule_args(args)
            if err:
                return err, []
            res = await abot_api(bot_token, "POST", "/api/bot/meal-schedule", body, user=user)
            return res.get("message", "OK"), []
        if cmd == "menu":
            week = "this" if args.lower() in ("this", "ini", "minggu ini", "now") else None
            res = await abot_api(bot_token, "GET", "/api/bot/meal-plan", params={"week": week}, user=user)
            return res.get("text") or "No meal plan.", []
        if cmd == "ingredients":
            res = await abot_api(bot_token, "GET", "/api/bot/ingredients", user=user)
            return res.get("text", ""), []
        if cmd in ("addingredient", "removeingredient"):
            if not args:
                return f"Usage: /{cmd} <ingredient>, e.g. /{cmd} chicken", []
            path = "/api/bot/ingredients" if cmd == "addingredient" else "/api/bot/ingredients/remove"
            res = await abot_api(bot_token, "POST", path, {"name": args}, user=user)
            return res.get("message", "OK"), []
        if cmd == "feedback":
            res = await abot_api(bot_token, "POST", "/api/bot/feedback", {"text": args}, user=user)
            return res.get("message", "OK"), []
        if cmd == "link":
            if not args:
                return "Usage: /link CODE - create the code in Kinkit > Reminders > Link my chat account.", []
            res = await abot_api(bot_token, "POST", "/api/bot/link", {"code": args.split()[0]}, user=user)
            return res.get("message", "OK"), []
        if cmd == "chores":
            return await chores_today(bot_token, user)
        if cmd == "buy":
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
                lines.append(f"- {it.get('name')}{qty}")
            return "\n".join(lines), []
        if cmd == "done_shopping":
            # "/done_shopping 250k" -> total; without it the backend asks for one
            body = {"total_amount": args.strip()} if args and args.strip() else {}
            res = await abot_api(bot_token, "POST", "/api/bot/lists/finish", body, user=user)
            return res.get("message", "Shopping done!"), []
    except BackendError as e:
        if 400 <= e.status < 500 and not e.message.startswith("HTTP "):
            return e.message, []
        if e.status == 404:
            names = {"chores": "Chores", "buy": "Lists", "list": "Lists", "done_shopping": "Lists",
                     "menu": "Meal plans", "ingredients": "Meal plans", "addingredient": "Meal plans",
                     "removeingredient": "Meal plans", "schedule": "Meal plans"}
            return _not_ready(names.get(cmd, "That command")), []
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


# ---------------------------------------------------------------- buttons

CALLBACK_PREFIXES = ("rem:", "chore:", "spc:", "spx:", "fixcat:", "fixdate:")


async def _pick_category(bot_token, user, token, cat_id):
    p = PENDING.get(token)
    if p is None or p["bot_token"] != bot_token:
        return False, "This prompt expired - send the expense again."
    res = await abot_api(bot_token, "POST", "/api/bot/spending", {
        "tx_date": p["date"], "description": p["desc"], "category_id": int(cat_id),
        "amount": p["amount"], "source": p["source"],
    }, user=p.get("user") or user)
    PENDING.pop(token, None)
    category = res.get("category") or "Other"
    learned = []
    try:
        lr = await abot_api(bot_token, "POST", "/api/bot/keywords/learn",
                            {"description": p["desc"], "category": category}, user=user)
        learned = lr.get("learned") or []
    except (BackendError, URLError, OSError):
        pass  # the expense is saved; learning is a bonus
    msg = f"Logged: {p['desc']} - {format_rp(p['amount'])} ({category}) on {p['date']}"
    if learned:
        msg += f"\nRemembered: {', '.join(learned)} -> {category}"
    return True, msg


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
        if len(parts) == 3 and parts[0] == "spc" and parts[2].isdigit():
            return await _pick_category(bot_token, user, parts[1], parts[2])
        if len(parts) == 2 and parts[0] == "spx":
            p = PENDING.pop(parts[1], None)
            desc = p["desc"] if p and p["bot_token"] == bot_token else "that"
            return True, f"Skipped {desc} (not an expense)."
        if len(parts) == 3 and parts[0] == "fixcat" and parts[1].isdigit() and parts[2].isdigit():
            res = await abot_api(bot_token, "PATCH", f"/api/bot/spending/{parts[1]}/category",
                                 {"category_id": int(parts[2])}, user=user)
            return True, f"Category changed to {res.get('category') or 'the new category'}."
        if len(parts) == 3 and parts[0] == "fixdate" and parts[1].isdigit() and re.fullmatch(r"\d{8}", parts[2]):
            d = f"{parts[2][:4]}-{parts[2][4:6]}-{parts[2][6:]}"
            await abot_api(bot_token, "PATCH", f"/api/bot/spending/{parts[1]}/date", {"tx_date": d}, user=user)
            return True, f"Date changed to {d}."
    except BackendError as e:
        return False, e.message
    except (URLError, OSError):
        return False, "Backend unreachable, try again later."
    return False, "Unknown button"


def split_command(text, prefixes=("/", "!"), bot_username=None):
    """'/remind tomorrow 8am x' -> ('remind', 'tomorrow 8am x'); None if not a
    known command. '/help@OtherBot' (a command for another bot in the same
    group) is None when bot_username is given."""
    text = (text or "").strip()
    if not text or text[0] not in prefixes:
        return None
    head, _, rest = text[1:].partition(" ")
    if "\n" in head:  # "/buy\nmilk\neggs"
        head, _, more = head.partition("\n")
        rest = (more + " " + rest).strip()
    name, _, target = head.partition("@")
    if target and bot_username and target.lower() != bot_username.lower():
        return None
    cmd = name.lower().replace("-", "_")
    if cmd not in COMMANDS:
        return None
    return cmd, rest.strip()
