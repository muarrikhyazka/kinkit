"""Unit tests for chat_helpers (Slack Block Kit + Matrix numbered buttons).
Stdlib only, no network.

Run: cd bots && python3 -m unittest
"""
import unittest

import chat_helpers as ch
import kinkit_common as kc

REM_BUTTONS = [[("Done", "rem:12:done"), ("Snooze 1h", "rem:12:snooze60"), ("Tomorrow", "rem:12:snooze_tomorrow")]]


class Prefix(unittest.TestCase):
    def test_bang_prefix(self):
        self.assertEqual(ch.bang_prefix("/today - now\n/remind <when>"), "!today - now\n!remind <when>")
        self.assertEqual(ch.bang_prefix("Telegram/Discord at https://x.y/z and/or 1/2"), "Telegram/Discord at https://x.y/z and/or 1/2")
        self.assertEqual(ch.bang_prefix("makan siang 50rb  /  50k lunch"), "makan siang 50rb  /  50k lunch")

    def test_commands_with_bang(self):
        self.assertEqual(kc.split_command("!today"), ("today", ""))
        self.assertEqual(kc.split_command("/today"), ("today", ""))


class SlackBlocks(unittest.TestCase):
    def test_no_buttons(self):
        self.assertIsNone(ch.slack_blocks("hi", []))
        self.assertIsNone(ch.slack_blocks("hi", None))

    def test_blocks(self):
        b = ch.slack_blocks("Take <out> & go", REM_BUTTONS + [[("Skip", "spx:ab12")]])
        self.assertEqual(b[0]["type"], "section")
        self.assertEqual(b[0]["text"]["text"], "Take &lt;out&gt; &amp; go")
        self.assertEqual([x["type"] for x in b], ["section", "actions", "actions"])
        els = b[1]["elements"]
        self.assertEqual([e["action_id"] for e in els], ["rem:12:done", "rem:12:snooze60", "rem:12:snooze_tomorrow"])
        self.assertEqual([e["value"] for e in els], [e["action_id"] for e in els])
        self.assertEqual(els[0]["text"], {"type": "plain_text", "text": "Done"})

    def test_limits(self):
        b = ch.slack_blocks("x" * 5000, [[("L" * 200, "spc:ab:1")]])
        self.assertEqual(len(b[0]["text"]["text"]), ch.SLACK_TEXT_LIMIT)
        self.assertEqual(len(b[1]["elements"][0]["text"]["text"]), 75)

    def test_result_blocks(self):
        orig = ch.slack_blocks("Reminder: trash", REM_BUTTONS)
        out = ch.slack_result_blocks(orig, "Reminder: trash", "Done!")
        self.assertEqual([x["type"] for x in out], ["section", "context"])
        self.assertEqual(out[1]["elements"][0]["text"], "Done!")
        # no blocks on the original (plain text message)
        out = ch.slack_result_blocks(None, "plain & text", "ok")
        self.assertEqual(out[0]["text"]["text"], "plain &amp; text")

    def test_unescape(self):
        self.assertEqual(ch.slack_user_ids("hi <@U123ABC> and <@W9|x>"), ["U123ABC", "W9"])
        self.assertEqual(ch.slack_unescape("!remind tomorrow 8am pills <@U123ABC>", {"U123ABC": "budi"}),
                         "!remind tomorrow 8am pills @budi")
        self.assertEqual(ch.slack_unescape("<@U999>"), "@U999")
        self.assertEqual(ch.slack_unescape("see <https://a.b/c|the link> &amp; <#C1|general> <!here>"),
                         "see the link & #general @here")


class SlackEvents(unittest.TestCase):
    def payload(self, **ev):
        base = {"type": "message", "user": "U1", "text": "!today", "channel": "C1", "channel_type": "channel"}
        base.update(ev)
        return {"event_id": "Ev1", "event": base}

    def test_message(self):
        self.assertEqual(ch.slack_message_event(self.payload(), "UBOT"), ("C1", "U1", "!today", False, "Ev1"))
        self.assertTrue(ch.slack_message_event(self.payload(channel_type="im"), "UBOT")[3])

    def test_filtered(self):
        for ev in ({"bot_id": "B1"}, {"subtype": "message_changed"}, {"subtype": "bot_message"}, {"user": "UBOT"},
                   {"text": "  "}, {"type": "reaction_added"}, {"user": None}):
            self.assertIsNone(ch.slack_message_event(self.payload(**ev), "UBOT"), ev)
        self.assertIsNone(ch.slack_message_event({}, "UBOT"))

    def test_allowed(self):
        self.assertTrue(ch.slack_allowed("C1", "C1", False))
        self.assertFalse(ch.slack_allowed("C2", "C1", False))              # other channel, plain text
        self.assertTrue(ch.slack_allowed("C2", "C1", False, "link"))       # ANYWHERE command
        self.assertFalse(ch.slack_allowed("C2", "C1", False, "today"))
        self.assertTrue(ch.slack_allowed("D1", "C1", True, "help"))
        self.assertFalse(ch.slack_allowed("D1", "C1", True, "today"))
        self.assertFalse(ch.slack_allowed("D1", "C1", True))
        self.assertTrue(ch.slack_allowed("C9", "", False, "today"))        # no channel configured: any channel

    def test_block_action(self):
        p = {"type": "block_actions", "user": {"id": "U1"}, "channel": {"id": "C1"},
             "message": {"ts": "1.2", "text": "t", "blocks": [{"type": "section"}]},
             "actions": [{"action_id": "rem:12:done", "value": "rem:12:done"}]}
        self.assertEqual(ch.slack_block_action(p), ("rem:12:done", "U1", "C1", "1.2", [{"type": "section"}], "t"))
        self.assertIsNone(ch.slack_block_action({"type": "view_submission"}))
        self.assertIsNone(ch.slack_block_action({"type": "block_actions", "actions": []}))


class MatrixButtons(unittest.TestCase):
    def test_render(self):
        text, mapping, nxt = ch.render_numbered("Reminder", REM_BUTTONS)
        self.assertEqual(text, "Reminder\n\n1) Done\n2) Snooze 1h\n3) Tomorrow\nReply !1 ... !3 to choose.")
        self.assertEqual(mapping, {1: "rem:12:done", 2: "rem:12:snooze60", 3: "rem:12:snooze_tomorrow"})
        self.assertEqual(nxt, 4)
        self.assertEqual(ch.render_numbered("plain", []), ("plain", {}, 1))
        self.assertIn("Reply !5 to choose", ch.render_numbered("x", [[("Only", "a:1")]], start=5)[0])

    def test_parse_choice(self):
        m = {1: "a", 2: "b"}
        self.assertEqual(ch.parse_choice("!2", m), "b")
        self.assertEqual(ch.parse_choice(" !1 ", m), "a")
        self.assertEqual(ch.parse_choice("2", m), "b")          # bare number while a list is pending
        self.assertIsNone(ch.parse_choice("2", {}))             # ... but not otherwise
        self.assertEqual(ch.parse_choice("!2", {}) , None)
        self.assertIsNone(ch.parse_choice("!9", m))
        self.assertIsNone(ch.parse_choice("!1 more", m))
        self.assertIsNone(ch.parse_choice("50k lunch", m))

    def test_memory(self):
        now = [1000.0]
        mem = ch.ChoiceMemory(ttl=60, clock=lambda: now[0])
        mem.start_batch()
        t1 = mem.add("a?", [[("Food", "spc:t1:1"), ("Skip", "spx:t1")]])
        t2 = mem.add("b?", [[("Food", "spc:t2:1")]])
        self.assertIn("1) Food", t1)
        self.assertIn("3) Food", t2)                            # numbering continues within a batch
        self.assertEqual(mem.pick("!3"), "spc:t2:1")
        self.assertIsNone(mem.pick("!3"))                       # used up
        self.assertEqual(mem.pick("2"), "spx:t1")
        now[0] += 61
        self.assertIsNone(mem.pick("!1"))                       # expired
        mem.start_batch()
        mem.add("c", [[("Z", "x:1")]])
        self.assertEqual(mem.pick("!1"), "x:1")

    def test_parse_rem(self):
        self.assertEqual(ch.parse_rem("!rem 12 done"), "rem:12:done")
        self.assertEqual(ch.parse_rem("!rem 12 SNOOZE"), "rem:12:snooze60")
        self.assertEqual(ch.parse_rem("/rem 7 tomorrow"), "rem:7:snooze_tomorrow")
        self.assertIsNone(ch.parse_rem("!reminders"))
        self.assertIsNone(ch.parse_rem("remember milk"))
        for bad in ("!rem", "!rem 12", "!rem x done", "!rem 12 later"):
            with self.assertRaises(ValueError):
                ch.parse_rem(bad)


class MatrixApi(unittest.TestCase):
    def test_urls(self):
        self.assertEqual(ch.matrix_url("https://m.example.org/", "/sync"), "https://m.example.org/_matrix/client/v3/sync")
        self.assertEqual(ch.matrix_send_path("!abc:example.org", "t 1"),
                         "/rooms/%21abc%3Aexample.org/send/m.room.message/t%201")
        self.assertEqual(ch.matrix_join_path("!abc:example.org"), "/join/%21abc%3Aexample.org")
        a, b = ch.matrix_txn_id(), ch.matrix_txn_id()
        self.assertNotEqual(a, b)
        self.assertTrue(a.startswith("kinkit-"))

    def test_homeserver(self):
        self.assertTrue(ch.valid_homeserver("https://matrix.example.org"))
        self.assertTrue(ch.valid_homeserver("http://synapse:8008"))
        for bad in ("", "matrix.example.org", "ftp://x", "https://a b", None):
            self.assertFalse(ch.valid_homeserver(bad), bad)

    def test_filter(self):
        import json
        f = json.loads(ch.matrix_filter('!a"b:x'))
        self.assertEqual(f["room"]["rooms"], ['!a"b:x'])
        self.assertEqual(f["room"]["timeline"]["types"], ["m.room.message"])

    def test_reply_fallback(self):
        self.assertEqual(ch.strip_reply_fallback("> <@a:x> hi\n> there\n\n!done"), "!done")
        self.assertEqual(ch.strip_reply_fallback("plain"), "plain")

    def sync(self, events, room="!r:x", invite=None):
        out = {"rooms": {"join": {room: {"timeline": {"events": events}}}}}
        if invite:
            out["rooms"]["invite"] = {invite: {}}
        return out

    def ev(self, sender="@bob:x", body="!today", **kw):
        e = {"type": "m.room.message", "event_id": "$1", "sender": sender, "content": {"msgtype": "m.text", "body": body}}
        e.update(kw)
        return e

    def test_events(self):
        s = self.sync([self.ev(), self.ev(sender="@bot:x", body="mine"),
                       self.ev(content={"msgtype": "m.notice", "body": "n"}),
                       self.ev(content={"msgtype": "m.text", "body": "* edit", "m.relates_to": {"rel_type": "m.replace"}}),
                       self.ev(type="m.room.encrypted"), self.ev(body="   "),
                       self.ev(event_id="$2", body="> <@a:x> q\n\n50k lunch")])
        self.assertEqual(ch.matrix_events(s, "!r:x", "@bot:x"), [("$1", "@bob:x", "!today"), ("$2", "@bob:x", "50k lunch")])
        self.assertEqual(ch.matrix_events(s, "!other:x", "@bot:x"), [])
        self.assertEqual(ch.matrix_events({}, "!r:x", "@bot:x"), [])

    def test_invites(self):
        self.assertTrue(ch.matrix_invited(self.sync([], invite="!r:x"), "!r:x"))
        self.assertFalse(ch.matrix_invited(self.sync([]), "!r:x"))


class Multiplexer(unittest.TestCase):
    def test_signature(self):
        a = {"account_id": 1, "bot_token": "b", "platform_token": "t", "chat_id": "C", "extra": "e"}
        self.assertEqual(ch.integration_signature(a), ch.integration_signature(dict(a)))
        self.assertNotEqual(ch.integration_signature(a), ch.integration_signature({**a, "extra": "e2"}))
        self.assertEqual(ch.integration_signature(a), ch.integration_signature({**a, "account_id": 2}))


if __name__ == "__main__":
    unittest.main()
