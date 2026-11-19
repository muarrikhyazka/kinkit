package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestNormalizeLayout(t *testing.T) {
	out, err := normalizeLayout([]byte(`{"widgets":[{"id":"a","type":"clock","w":99,"h":0},{"id":"a","type":"note","w":4,"h":1,"text":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var l struct {
		Widgets []widgetLayoutItem `json:"widgets"`
	}
	if err := json.Unmarshal(out, &l); err != nil || len(l.Widgets) != 2 || l.Widgets[0].W != 12 || l.Widgets[0].H != 1 || l.Widgets[1].ID == "a" {
		t.Fatalf("normalized: %v %+v", err, l)
	}
	for _, bad := range []string{`{"widgets":[{"type":"bogus"}]}`, `[1]`, `nope`} {
		if _, err := normalizeLayout([]byte(bad)); err == nil {
			t.Errorf("%s should be rejected", bad)
		}
	}
}

func TestWidgetBoardsAndTV(t *testing.T) {
	_, srv := newTestServer(t)
	owner := newClient(t, srv)
	owner.signup()
	kidEmail, _ := addMember(t, owner, "kid", nil)
	kid := newClient(t, srv)
	loginAs(t, kid, kidEmail, "temp-pass-123")

	var cat struct {
		Widgets []widgetType   `json:"widgets"`
		Presets []widgetPreset `json:"presets"`
	}
	if code := kid.do("GET", "/api/widgets/catalog", nil, &cat); code != 200 || len(cat.Widgets) < 8 || len(cat.Presets) != 4 {
		t.Fatalf("catalog: %d %+v", code, cat)
	}
	if code := kid.do("POST", "/api/widgets/boards", map[string]any{"name": "Mine"}, nil); code != 403 {
		t.Fatalf("kid create board: %d", code)
	}
	var b widgetBoard
	if code := owner.do("POST", "/api/widgets/boards", map[string]any{"preset": "tv"}, &b); code != 201 || b.Name != "Living-room TV" || b.Device != "tv" {
		t.Fatalf("create from preset: %d %+v", code, b)
	}
	if code := owner.do("POST", "/api/widgets/boards", map[string]any{"name": "x", "layout": map[string]any{"widgets": []map[string]any{{"type": "nope"}}}}, nil); code != 400 {
		t.Fatalf("bad layout: %d", code)
	}
	layout := map[string]any{"widgets": []map[string]any{{"id": "n1", "type": "note", "w": 6, "h": 1, "text": "Wi-Fi: kinkit123"}, {"id": "a1", "type": "agenda", "w": 6, "h": 3}, {"id": "b1", "type": "budget", "w": 4, "h": 2}}}
	if code := owner.do("PATCH", fmt.Sprintf("/api/widgets/boards/%d", b.ID), map[string]any{"name": "Kitchen TV", "layout": layout}, &b); code != 200 || b.Name != "Kitchen TV" {
		t.Fatalf("patch: %d %+v", code, b)
	}
	var list struct {
		Boards []widgetBoard `json:"boards"`
	}
	kid.do("GET", "/api/widgets/boards", nil, &list)
	if len(list.Boards) != 1 || list.Boards[0].Name != "Kitchen TV" {
		t.Fatalf("kid list: %+v", list.Boards)
	}
	if code := kid.do("DELETE", fmt.Sprintf("/api/widgets/boards/%d", b.ID), nil, nil); code != 403 {
		t.Fatalf("kid delete: %d", code)
	}

	// data: an activity today shows in the agenda; kids never get money widgets
	var act activityRow
	owner.do("POST", "/api/activities", map[string]any{"title": "Piano lesson", "start_date": "2000-01-01", "repeat": "daily"}, &act)
	var data struct {
		Cards map[string]widgetCard `json:"cards"`
	}
	if code := owner.do("GET", "/api/widgets/data?types=agenda,budget,goals,family,meals", nil, &data); code != 200 {
		t.Fatalf("data: %d", code)
	}
	if c := data.Cards["agenda"]; len(c.Rows) == 0 || c.Rows[0].Title != "Piano lesson" {
		t.Fatalf("agenda card: %+v", c)
	}
	if _, ok := data.Cards["budget"]; !ok || data.Cards["family"].Big == "" || len(data.Cards["meals"].Rows) == 0 {
		t.Fatalf("owner cards: %+v", data.Cards)
	}
	var kidData struct {
		Cards map[string]widgetCard `json:"cards"`
	}
	kid.do("GET", "/api/widgets/data?types=agenda,budget,goals", nil, &kidData)
	if _, ok := kidData.Cards["budget"]; ok {
		t.Fatalf("kid got a money widget")
	}
	if _, ok := kidData.Cards["agenda"]; !ok {
		t.Fatalf("kid should get the agenda")
	}

	// TV link tied to the board
	var tok tvTokenRow
	if code := owner.do("POST", "/api/tv-tokens", map[string]any{"name": "Kitchen", "board_id": b.ID}, &tok); code != 200 || tok.BoardID == nil || *tok.BoardID != b.ID {
		t.Fatalf("token with board: %d %+v", code, tok)
	}
	var tv struct {
		Board     *widgetBoard          `json:"board"`
		Cards     map[string]widgetCard `json:"cards"`
		Household string                `json:"household_name"`
		Timezone  string                `json:"timezone"`
	}
	if code := newClient(t, srv).do("GET", "/api/tv/data?k="+tok.Token, nil, &tv); code != 200 || tv.Board == nil || tv.Board.Name != "Kitchen TV" || tv.Cards["agenda"].Title == "" || tv.Cards["budget"].Type != "budget" {
		t.Fatalf("tv data: %d %+v", code, tv)
	}
	// switch the link back to the classic view, and reject someone else's board
	if code := owner.do("PATCH", fmt.Sprintf("/api/tv-tokens/%d", tok.ID), map[string]any{"board_id": nil}, nil); code != 200 {
		t.Fatalf("detach: %d", code)
	}
	var tv2 struct {
		Board *widgetBoard `json:"board"`
	}
	newClient(t, srv).do("GET", "/api/tv/data?k="+tok.Token, nil, &tv2)
	if tv2.Board != nil {
		t.Fatalf("board should be detached")
	}
	other := newClient(t, srv)
	other.signup()
	if code := other.do("PATCH", fmt.Sprintf("/api/tv-tokens/%d", tok.ID), map[string]any{"board_id": b.ID}, nil); code != 404 && code != 400 {
		t.Fatalf("foreign token/board: %d", code)
	}
	if code := owner.do("DELETE", fmt.Sprintf("/api/widgets/boards/%d", b.ID), nil, nil); code != 200 {
		t.Fatalf("delete board: %d", code)
	}
}
