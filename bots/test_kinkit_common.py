"""Unit tests for kinkit_common (stdlib only, no network).

Run: cd bots && python3 -m unittest test_kinkit_common
"""
import asyncio
import unittest

import kinkit_common as kc

KW = {"makan": "Food", "bensin": "Transportation", "es dawet": "Jajan", "e-learning": "Learning", "bakso": "Jajan"}
CATS = [{"id": 1, "name": "Food"}, {"id": 2, "name": "Jajan"}, {"id": 3, "name": "Home Rent"}, {"id": 4, "name": "Clothes & Bags"}]


class ParseSpending(unittest.TestCase):
    def test_forms(self):
        self.assertEqual(kc.parse_spending("makan siang 50rb", KW), (50000, "makan siang", "Food"))
        self.assertEqual(kc.parse_spending("bensin rp20.000", KW), (20000, "bensin", "Transportation"))
        self.assertEqual(kc.parse_spending("Rp75.000 bensin", KW), (75000, "bensin", "Transportation"))
        self.assertEqual(kc.parse_spending("50k lunch", KW), (50000, "lunch", None))
        self.assertEqual(kc.parse_spending("25000 parking", KW), (25000, "parking", None))
        self.assertEqual(kc.parse_spending("kontrakan 1,5jt", KW), (1500000, "kontrakan", None))
        self.assertEqual(kc.parse_spending("beli 2 kg beras 60rb", KW), (60000, "beli 2 kg beras", None))
        self.assertEqual(kc.parse_spending("es dawet enak 8rb", KW), (8000, "es dawet enak", "Jajan"))
        self.assertEqual(kc.parse_spending("kursus e-learning 100k", KW), (100000, "kursus e-learning", "Learning"))

    def test_not_spending(self):
        for text in ("halo semua", "50k", "2 porsi lagi ya", "5 kg beras", "/today", "ok 5 menit lagi", "", "rp 0 makan"):
            self.assertIsNone(kc.parse_spending(text, KW), text)

    def test_lines(self):
        got = kc.parse_spending_lines("makan 20rb\nhalo\nbakso 15rb\n", KW)
        self.assertEqual([g[1] for g in got], ["makan", "bakso"])


class Commands(unittest.TestCase):
    def test_split_command(self):
        self.assertEqual(kc.split_command("/remind tomorrow 8am x"), ("remind", "tomorrow 8am x"))
        self.assertEqual(kc.split_command("/tambahbahan@KinkitBot ayam", bot_username="kinkitbot"), ("tambahbahan", "ayam"))
        self.assertIsNone(kc.split_command("/help@OtherBot", bot_username="KinkitBot"))
        self.assertEqual(kc.split_command("/help@OtherBot"), ("help", ""))
        self.assertIsNone(kc.split_command("/nope"))
        self.assertEqual(kc.split_command("/buy\nmilk\neggs"), ("buy", "milk\neggs"))
        self.assertEqual(kc.split_command("!done-shopping 250k"), ("done_shopping", "250k"))

    def test_aliases(self):
        for alias, cmd in (("bantuan", "help"), ("koreksi", "fix"), ("koreksitgl", "fixdate"), ("tanggal", "date"),
                           ("bahan", "ingredients"), ("jadwal", "schedule"), ("reminder", "schedule"), ("last", "recent"),
                           ("hariini", "today"), ("ingatkan", "remind"), ("pengingat", "reminders"), ("fitur", "feedback"),
                           ("pengeluaran", "spending"), ("menu", "menu"), ("list", "list")):
            self.assertEqual(kc.canonical(alias), cmd)
            self.assertIn(alias, kc.COMMANDS)

    def test_category_suffix(self):
        self.assertEqual(kc.split_category_suffix("bakso pak kumis home rent", CATS), ("bakso pak kumis", CATS[2]))
        self.assertEqual(kc.split_category_suffix("bakso jajan", CATS), ("bakso", CATS[1]))
        self.assertEqual(kc.split_category_suffix("baju clothes & bags", CATS), ("baju", CATS[3]))
        self.assertEqual(kc.split_category_suffix("bakso pak", CATS), ("bakso pak", None))
        self.assertEqual(kc.split_category_suffix("food", CATS), ("food", None))

    def test_date_suffix(self):
        self.assertEqual(kc.split_date_suffix("bakso 2026-10-20"), ("bakso", "2026-10-20"))
        self.assertEqual(kc.split_date_suffix("bakso 2026-13-20"), ("bakso 2026-13-20", None))

    def test_schedule_args(self):
        self.assertEqual(kc.parse_schedule_args("meal jumat 06:00")[0], {"enabled": True, "days": [5], "time": "06:00", "start_date": ""})
        self.assertEqual(kc.parse_schedule_args("meal sat,sun 7.30 2026-12-01")[0],
                         {"enabled": True, "days": [0, 6], "time": "07:30", "start_date": "2026-12-01"})
        self.assertEqual(kc.parse_schedule_args("meal setiap 20:00")[0]["days"], list(range(7)))
        self.assertEqual(kc.parse_schedule_args("menu off")[0], {"enabled": False})
        self.assertIn("Kinkit reminder", kc.parse_schedule_args("spend setiap 20:00")[1])
        self.assertIn("automatically", kc.parse_schedule_args("scrape jumat 04:00")[1])
        self.assertIsNotNone(kc.parse_schedule_args("meal funday 06:00")[1])
        self.assertIsNotNone(kc.parse_schedule_args("meal fri 25:00")[1])

    def test_date_override(self):
        self.assertIn("2026", kc.set_date_override("t1", "2026-10-20"))
        self.assertEqual(kc.DATE_OVERRIDE["t1"], "2026-10-20")
        self.assertIn("today", kc.set_date_override("t1", "reset"))
        self.assertNotIn("t1", kc.DATE_OVERRIDE)
        self.assertIn("YYYY-MM-DD", kc.set_date_override("t1", "kemarin"))

    def test_buttons_fit_platform_limits(self):
        cats = [{"id": i, "name": f"Category {i}"} for i in range(1, 30)]
        tg = kc.category_buttons("abcd1234", cats, "telegram")
        dc = kc.category_buttons("abcd1234", cats, "discord")
        self.assertTrue(all(len(d.encode()) <= 64 for row in tg for _, d in row))
        self.assertLessEqual(len(dc), 5)
        self.assertTrue(all(len(r) <= 5 for r in dc))
        self.assertEqual(dc[-1][0][1], "spx:abcd1234")


class FakeBackend:
    """Stands in for kc.bot_api; records calls."""

    def __init__(self):
        self.calls = []

    def __call__(self, bot_token, method, path, body=None, params=None, user=None):
        self.calls.append((method, path, body, params, user))
        if path == "/api/bot/config":
            return {"category_keywords": KW, "category_list": CATS, "today": "2026-11-18"}
        if path == "/api/bot/spending":
            name = body.get("category") or {c["id"]: c["name"] for c in CATS}.get(body.get("category_id"))
            return {"ok": True, "id": 7, "category": name}
        if path == "/api/bot/keywords/learn":
            return {"ok": True, "learned": ["pecel", "lele"]}
        if path == "/api/bot/spending/search":
            return [{"id": 9, "tx_date": "2026-11-17", "description": "bakso", "category_name": "Food", "amount": 15000}]
        if path.endswith("/category"):
            return {"ok": True, "category": "Jajan"}
        return {"ok": True}


class Flows(unittest.TestCase):
    def setUp(self):
        self.fake = FakeBackend()
        self.orig = kc.bot_api
        kc.bot_api = self.fake
        kc.PENDING.clear()
        kc.DATE_OVERRIDE.clear()

    def tearDown(self):
        kc.bot_api = self.orig

    def run_async(self, coro):
        return asyncio.run(coro)

    def test_known_and_unknown_lines(self):
        replies = self.run_async(kc.log_spending("tok", "telegram:1", "makan 20rb\npecel lele 25rb", "telegram"))
        self.assertEqual(len(replies), 2)
        self.assertTrue(replies[0][0].startswith("Logged: makan - Rp20.000 (Food)"))
        prompt, buttons = replies[1]
        self.assertIn("pecel lele", prompt)
        token = buttons[0][0][1].split(":")[1]
        posted = [c for c in self.fake.calls if c[1] == "/api/bot/spending"]
        self.assertEqual(len(posted), 1)  # unknown one waits for the button
        ok, msg = self.run_async(kc.handle_callback("tok", "telegram:2", f"spc:{token}:2"))
        self.assertTrue(ok)
        self.assertIn("(Jajan)", msg)
        self.assertIn("Remembered: pecel, lele", msg)
        last = [c for c in self.fake.calls if c[1] == "/api/bot/spending"][-1]
        self.assertEqual(last[2]["category_id"], 2)
        self.assertEqual(last[4], "telegram:1")  # attributed to whoever typed it
        ok, msg = self.run_async(kc.handle_callback("tok", "telegram:2", f"spc:{token}:2"))
        self.assertFalse(ok)  # used up

    def test_pending_is_per_tenant(self):
        replies = self.run_async(kc.log_spending("tok", None, "pecel 10rb", "telegram"))
        token = replies[0][1][0][0][1].split(":")[1]
        ok, _ = self.run_async(kc.handle_callback("other-tenant", None, f"spc:{token}:1"))
        self.assertFalse(ok)
        ok, msg = self.run_async(kc.handle_callback("tok", None, f"spx:{token}"))
        self.assertTrue(ok)
        self.assertIn("Skipped pecel", msg)

    def test_date_override_applies(self):
        kc.set_date_override("tok", "2026-10-01")
        replies = self.run_async(kc.log_spending("tok", None, "bakso 15rb", "telegram"))
        self.assertIn("on 2026-10-01", replies[0][0])
        self.assertEqual([c for c in self.fake.calls if c[1] == "/api/bot/spending"][0][2]["tx_date"], "2026-10-01")

    def test_not_spending_returns_none(self):
        self.assertIsNone(self.run_async(kc.log_spending("tok", None, "halo semua", "telegram")))

    def test_fix_buttons_and_callbacks(self):
        text, buttons = self.run_async(kc.run_command("tok", None, "koreksi", "bakso jajan"))
        self.assertIn("Jajan", text)
        self.assertEqual(buttons[0][0][1], "fixcat:9:2")
        ok, msg = self.run_async(kc.handle_callback("tok", None, "fixcat:9:2"))
        self.assertTrue(ok and "Jajan" in msg)
        text, buttons = self.run_async(kc.run_command("tok", None, "koreksitgl", "bakso 2026-10-20"))
        self.assertEqual(buttons[0][0][1], "fixdate:9:20261020")
        ok, msg = self.run_async(kc.handle_callback("tok", None, "fixdate:9:20261020"))
        self.assertTrue(ok)
        self.assertEqual(self.fake.calls[-1][2], {"tx_date": "2026-10-20"})

    def test_meal_commands(self):
        self.run_async(kc.run_command("tok", None, "tambahbahan", "ayam"))
        self.assertEqual(self.fake.calls[-1][1:3], ("/api/bot/ingredients", {"name": "ayam"}))
        self.run_async(kc.run_command("tok", None, "hapusbahan", "ayam"))
        self.assertEqual(self.fake.calls[-1][1], "/api/bot/ingredients/remove")
        self.run_async(kc.run_command("tok", None, "menu", "this"))
        self.assertEqual(self.fake.calls[-1][3], {"week": "this"})
        self.run_async(kc.run_command("tok", None, "jadwal", "meal jumat 06:00"))
        self.assertEqual(self.fake.calls[-1][1:3], ("/api/bot/meal-schedule", {"enabled": True, "days": [5], "time": "06:00", "start_date": ""}))


if __name__ == "__main__":
    unittest.main()
