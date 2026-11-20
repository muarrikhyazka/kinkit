/*
 * Kinkit TV - dependency-free, view-only viewer for Samsung Tizen, LG webOS
 * and any TV browser. Written in ES5 (no arrow functions, fetch, let/const,
 * template literals) so it runs on old TV Chromium/WebKit builds.
 *
 * Data: GET <server>/api/tv/data?k=<token> (read-only, token auth, no session)
 * Optional: GET <server>/api/fx-rates (public) to convert IDR to the display currency.
 */
(function () {
  'use strict';

  var REFRESH_MS = 60000;
  var ROTATE_MS = 15000;
  var LONG_PRESS_MS = 700;
  var TIMEOUT_MS = 15000;
  var PER_BUDGET_PAGE = 6;
  var LS = { server: 'kinkit_tv.server', token: 'kinkit_tv.token', cache: 'kinkit_tv.cache_json',
             cacheAt: 'kinkit_tv.cache_at', fx: 'kinkit_tv.fx_json' };

  var MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
  var MONTHS_LONG = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August',
                     'September', 'October', 'November', 'December'];
  var DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
  var CURRENCY_META = {
    IDR: ['Rp', 'id-ID'], USD: ['$', 'en-US'], EUR: ['€', 'de-DE'], GBP: ['£', 'en-GB'],
    SGD: ['S$', 'en-SG'], MYR: ['RM', 'ms-MY'], JPY: ['¥', 'ja-JP'], AUD: ['A$', 'en-AU'], INR: ['₹', 'en-IN']
  };

  // Remote key codes (Tizen / webOS / browsers).
  var K = {
    LEFT: 37, UP: 38, RIGHT: 39, DOWN: 40, ENTER: 13,
    BACK_TIZEN: 10009, BACK_WEBOS: 461, BACKSPACE: 8, ESC: 27,
    RED: 403, INFO: 457, PLAY_PAUSE: 10252, PLAY: 415, PAUSE: 19,
    CH_UP: 427, CH_DOWN: 428, PAGE_UP: 33, PAGE_DOWN: 34, REW: 412, FF: 417,
    KEY_M: 77, KEY_S: 83, SPACE: 32
  };
  function isBack(c) { return c === K.BACK_TIZEN || c === K.BACK_WEBOS || c === K.ESC || c === K.BACKSPACE; }
  function isMenu(c) { return c === K.RED || c === K.INFO || c === K.KEY_M || c === K.KEY_S; }

  // utils
  function $(id) { return document.getElementById(id); }
  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function lsGet(k) { try { return window.localStorage.getItem(k); } catch (e) { return null; } }
  function lsSet(k, v) { try { window.localStorage.setItem(k, v); } catch (e) { /* storage full / disabled */ } }
  function lsDel(k) { try { window.localStorage.removeItem(k); } catch (e) { /* ignore */ } }
  function pad2(n) { return n < 10 ? '0' + n : '' + n; }
  function hhmm(d) { return pad2(d.getHours()) + ':' + pad2(d.getMinutes()); }
  function toInt(v) {
    if (typeof v === 'number') return isFinite(v) ? Math.round(v) : 0;
    if (typeof v === 'string') { var n = parseFloat(v); return isFinite(n) ? Math.round(n) : 0; }
    return 0;
  }

  // pairing parsing
  // Same rules as tv/flutter/lib/src/config.dart.
  function looksLocal(host) {
    host = host.toLowerCase();
    return host === 'localhost' || /^\d{1,3}(\.\d{1,3}){3}$/.test(host) || /\.(local|lan|home)$/.test(host) ||
      host.indexOf('.') < 0;
  }
  // Minimal URL parser (old TV browsers lack the URL constructor).
  function parseLoose(input) {
    var s = (input || '').replace(/^\s+|\s+$/g, '');
    if (!s) return null;
    if (s.indexOf('://') < 0) {
      var hostProbe = s.split(/[\/?#]/)[0].split(':')[0];
      if (!hostProbe || hostProbe.indexOf('=') >= 0) return null;
      s = (looksLocal(hostProbe) ? 'http' : 'https') + '://' + s;
    }
    var m = /^(https?):\/\/([^\/?#:]+)(:\d+)?([^?#]*)(\?[^#]*)?/i.exec(s);
    if (!m) return null;
    return { scheme: m[1].toLowerCase(), host: m[2], port: m[3] || '', path: m[4] || '', query: m[5] || '' };
  }
  function queryParam(query, name) {
    var q = query.replace(/^\?/, '').split('&');
    for (var i = 0; i < q.length; i++) {
      var kv = q[i].split('=');
      if (kv[0] === name && kv.length > 1) {
        try { return decodeURIComponent(kv.slice(1).join('=').replace(/\+/g, ' ')); } catch (e) { return kv.slice(1).join('='); }
      }
    }
    return null;
  }
  function tokenFromShareUrl(input) {
    var s = (input || '').replace(/^\s+|\s+$/g, '');
    if (s.indexOf('k=') < 0) return null;
    var qi = s.indexOf('?');
    var k = queryParam(qi >= 0 ? s.substring(qi) : s, 'k');
    return k ? k.replace(/^\s+|\s+$/g, '') || null : null;
  }
  function normalizeServer(input) {
    var u = parseLoose(input);
    if (!u) return null;
    var path = u.path, changed = true, suffixes = ['/api/tv/data', '/api', '/tv'];
    while (changed) {
      changed = false;
      while (path.charAt(path.length - 1) === '/') { path = path.substring(0, path.length - 1); changed = true; }
      for (var i = 0; i < suffixes.length; i++) {
        var sx = suffixes[i];
        if (path.length >= sx.length && path.toLowerCase().substring(path.length - sx.length) === sx) {
          path = path.substring(0, path.length - sx.length); changed = true;
        }
      }
    }
    return u.scheme + '://' + u.host + u.port + path;
  }
  function parsePairing(serverInput, tokenInput) {
    var sRaw = (serverInput || '').replace(/^\s+|\s+$/g, '');
    var tRaw = (tokenInput || '').replace(/^\s+|\s+$/g, '');
    var token = tRaw ? (tokenFromShareUrl(tRaw) || safeDecode(tRaw)) : tokenFromShareUrl(sRaw);
    var server = null;
    if (sRaw) {
      server = normalizeServer(sRaw);
      if (!server) return { error: 'Server address is not a valid http(s) URL.' };
    } else if (tRaw.indexOf('://') >= 0 || tRaw.indexOf('/tv?') >= 0) {
      server = normalizeServer(tRaw);
    }
    if (!server) return { error: 'Enter the server address (or paste the full TV link).' };
    if (!token) return { error: 'Enter the TV token (or paste the full TV link).' };
    if (token.length > 200 || !/^[A-Za-z0-9_\-.~]+$/.test(token)) return { error: 'That token does not look right - check for typos.' };
    return { server: server, token: token };
  }
  function safeDecode(s) { try { return decodeURIComponent(s); } catch (e) { return s; } }
  function maskToken(t) { return t.length <= 8 ? new Array(t.length + 1).join('*') : t.substring(0, 4) + '...' + t.substring(t.length - 4); }

  // data model
  function parseData(json) {
    var now = new Date();
    var r = (json && typeof json.report === 'object' && json.report) || {};
    var list = Object.prototype.toString.call(r.expenses_list) === '[object Array]' ? r.expenses_list : [];
    var trend = Object.prototype.toString.call(json.trend) === '[object Array]' ? json.trend : [];
    var y = toInt(json.year), m = toInt(json.month);
    var cur = typeof json.currency === 'string' && json.currency ? json.currency.toUpperCase() : 'IDR';
    var name = typeof json.display_name === 'string' ? json.display_name.replace(/^\s+|\s+$/g, '') : '';
    var family = typeof json.household_name === 'string' ? json.household_name.replace(/^\s+|\s+$/g, '') : '';
    var d = {
      year: y > 0 ? y : now.getUTCFullYear(),
      month: m >= 1 && m <= 12 ? m : now.getUTCMonth() + 1,
      currency: cur,
      displayName: name || null,
      householdName: family || null,
      report: {
        totalIncome: toInt(r.total_income), budgetedExpenses: toInt(r.budgeted_expenses),
        savings: toInt(r.savings), leftForBudgeting: toInt(r.left_for_budgeting),
        expenses: []
      },
      trend: [],
      board: null
    };
    // A link tied to a widget board also carries that board + its widget cards.
    if (json.board && typeof json.board === 'object') {
      var lay = json.board.layout && json.board.layout.widgets;
      var ws = [];
      for (var wi = 0; lay && wi < lay.length; wi++) {
        var wx = lay[wi] || {};
        ws.push({ id: String(wx.id || wi), type: String(wx.type || ''),
          w: Math.max(1, Math.min(12, toInt(wx.w) || 4)), h: Math.max(1, Math.min(4, toInt(wx.h) || 2)),
          text: typeof wx.text === 'string' ? wx.text : '', color: cleanHex(wx.color) });
      }
      d.board = { name: String(json.board.name || ''), widgets: ws,
        background: cleanHex(json.board.layout && json.board.layout.background),
        cards: (json.cards && typeof json.cards === 'object') ? json.cards : {} };
    }
    for (var i = 0; i < list.length; i++) {
      var e = list[i] || {};
      d.report.expenses.push({
        name: typeof e.category_name === 'string' ? e.category_name : 'Uncategorised',
        main: typeof e.main_category === 'string' ? e.main_category : '',
        allocation: toInt(e.allocation), realization: toInt(e.realization)
      });
    }
    for (var j = 0; j < trend.length; j++) {
      var t = trend[j] || {};
      var tm = toInt(t.month);
      d.trend.push({ year: toInt(t.year), month: tm < 1 ? 1 : (tm > 12 ? 12 : tm), income: toInt(t.income), expense: toInt(t.expense) });
    }
    return d;
  }
  function ratioOf(e) { return e.allocation > 0 ? e.realization / e.allocation : null; }
  function totalSpent(rep) { var s = 0; for (var i = 0; i < rep.expenses.length; i++) s += rep.expenses[i].realization; return s; }
  function activeCategories(rep) {
    var out = [];
    for (var i = 0; i < rep.expenses.length; i++) {
      var e = rep.expenses[i];
      if (e.allocation > 0 || e.realization > 0) out.push(e);
    }
    function key(e) { var r = ratioOf(e); return r === null ? (e.realization > 0 ? 99 : 0) : r; }
    out.sort(function (a, b) { return (key(b) - key(a)) || (b.realization - a.realization); });
    return out;
  }
  function topExpenses(rep, n) {
    var out = [];
    for (var i = 0; i < rep.expenses.length; i++) if (rep.expenses[i].realization > 0) out.push(rep.expenses[i]);
    out.sort(function (a, b) { return b.realization - a.realization; });
    return out.slice(0, n);
  }
  function colorFor(ratio, realization) {
    if (ratio === null) return realization > 0 ? '#f87171' : '#9ca3af';
    if (ratio > 1) return '#f87171';
    if (ratio >= 0.8) return '#fbbf24';
    return '#4ade80';
  }

  // money (mirrors react/src/api.js formatRupiah)
  var money = { code: 'IDR', rates: { IDR: 1 } };
  function effective() {
    var code = money.code;
    var rate = code === 'IDR' ? 1 : money.rates[code];
    if (!rate || !CURRENCY_META[code]) return { code: 'IDR', rate: 1, meta: CURRENCY_META.IDR };
    return { code: code, rate: rate, meta: CURRENCY_META[code] };
  }
  function groupDigits(n, locale) {
    try { return n.toLocaleString(locale); } catch (e) { /* no Intl */ }
    var sep = /^(id|de)/.test(locale) ? '.' : ',';
    return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, sep);
  }
  function fmt(idr) {
    var ef = effective();
    var v = Math.round(idr * ef.rate);
    var s = ef.meta[0] + groupDigits(Math.abs(v), ef.meta[1]);
    return v < 0 ? '-' + s : s;
  }
  function fmtCompact(idr) {
    var ef = effective();
    var v = Math.abs(idr * ef.rate), sign = idr < 0 ? '-' : '';
    var units = [[1e12, 'T'], [1e9, 'B'], [1e6, 'M'], [1e3, 'K']];
    for (var i = 0; i < units.length; i++) {
      if (v >= units[i][0]) {
        var x = v / units[i][0];
        return sign + ef.meta[0] + (x >= 100 ? Math.round(x) : Math.round(x * 10) / 10) + units[i][1];
      }
    }
    return sign + ef.meta[0] + Math.round(v);
  }

  // network
  function getJSON(url, cb) {
    var xhr = new XMLHttpRequest();
    var done = false;
    function finish(err, status, body) { if (done) return; done = true; cb(err, status, body); }
    try {
      xhr.open('GET', url, true);
      xhr.timeout = TIMEOUT_MS;
      // No custom headers: keeps this a CORS "simple request" (no preflight).
      xhr.onreadystatechange = function () {
        if (xhr.readyState !== 4) return;
        if (xhr.status === 0) return finish('offline', 0, null);
        var json = null;
        try { json = JSON.parse(xhr.responseText); } catch (e) { /* non-JSON */ }
        finish(null, xhr.status, json);
      };
      xhr.ontimeout = function () { finish('timeout', 0, null); };
      xhr.onerror = function () { finish('offline', 0, null); };
      xhr.send(null);
    } catch (e) { finish('offline', 0, null); }
  }
  function dataUrl(cfg) { return cfg.server + '/api/tv/data?k=' + encodeURIComponent(cfg.token); }
  function fxUrl(cfg) { return cfg.server + '/api/fx-rates'; }

  // Server pre-filled on first pairing: <html data-default-server="https://...">
  function defaultServer() {
    var v = document.documentElement.getAttribute('data-default-server');
    return v ? v.replace(/\/+$/, '') : '';
  }

  // app state
  var state = {
    screen: null, cfg: null, data: null, updatedAt: null, offline: false, offlineReason: '',
    page: 0, pageCount: 1, paused: false, fxAt: 0,
    pair: { server: defaultServer(), token: '', active: 'server', upper: false, busy: false },
    held: null, // { code, since, fired }
    swallow: null // keyCode whose remaining repeats/keyup must be ignored (after a long press)
  };
  var timers = { refresh: null, rotate: null };

  function loadConfig() {
    var s = lsGet(LS.server), t = lsGet(LS.token);
    return s && t ? { server: s, token: t } : null;
  }
  function saveConfig(cfg) {
    var prev = loadConfig();
    lsSet(LS.server, cfg.server); lsSet(LS.token, cfg.token);
    if (!prev || prev.server !== cfg.server || prev.token !== cfg.token) { lsDel(LS.cache); lsDel(LS.cacheAt); }
  }
  function clearToken() { lsDel(LS.token); lsDel(LS.cache); lsDel(LS.cacheAt); }

  // focus / spatial navigation
  var focused = null;
  function focusables() {
    var root = $('screen-' + state.screen);
    if (!root) return [];
    var all = root.querySelectorAll('.focusable'), out = [];
    for (var i = 0; i < all.length; i++) if (all[i].offsetParent !== null) out.push(all[i]);
    return out;
  }
  function setFocus(el) {
    if (focused) focused.className = focused.className.replace(/\s*\bfocused\b/g, '');
    focused = el;
    if (el) {
      el.className += ' focused';
      if (el.getAttribute('data-field')) setActiveField(el.getAttribute('data-field'));
    }
  }
  function moveFocus(dir) {
    var list = focusables();
    if (!list.length) return;
    if (!focused || list.indexOf(focused) < 0) { setFocus(list[0]); return; }
    var a = focused.getBoundingClientRect();
    var ax = a.left + a.width / 2, ay = a.top + a.height / 2;
    var best = null, bestScore = Infinity;
    for (var i = 0; i < list.length; i++) {
      var el = list[i];
      if (el === focused) continue;
      var b = el.getBoundingClientRect();
      var bx = b.left + b.width / 2, by = b.top + b.height / 2;
      var dx = bx - ax, dy = by - ay, main, cross;
      if (dir === 'left') { if (b.right > a.left + 1) continue; main = -dx; cross = Math.abs(dy); }
      else if (dir === 'right') { if (b.left < a.right - 1) continue; main = dx; cross = Math.abs(dy); }
      else if (dir === 'up') { if (b.bottom > a.top + 1) continue; main = -dy; cross = Math.abs(dx); }
      else { if (b.top < a.bottom - 1) continue; main = dy; cross = Math.abs(dx); }
      var score = main + cross * 2.5;
      if (score < bestScore) { bestScore = score; best = el; }
    }
    if (best) setFocus(best);
  }
  function activate(el) {
    if (!el) return;
    var fn = el.__onActivate;
    if (fn) fn();
  }
  function onActivate(el, fn) {
    el.__onActivate = fn;
    el.addEventListener('click', function () { setFocus(el); fn(); });
  }

  // screens
  function show(name) {
    var names = ['pair', 'viewer', 'settings'];
    for (var i = 0; i < names.length; i++) $('screen-' + names[i]).hidden = names[i] !== name;
    state.screen = name;
    setFocus(null);
  }

  // pairing
  function buildKeyboard() {
    var rows = ['1234567890', 'qwertyuiop', 'asdfghjkl-', 'zxcvbnm_.:', '/?=&@~'];
    var html = '';
    for (var r = 0; r < rows.length; r++) {
      html += '<div class="kbd-row">';
      for (var c = 0; c < rows[r].length; c++) {
        var ch = rows[r].charAt(c);
        html += '<div class="key focusable" data-ch="' + esc(ch) + '">' + esc(state.pair.upper ? ch.toUpperCase() : ch) + '</div>';
      }
      html += '</div>';
    }
    html += '<div class="kbd-row">' +
      '<div class="key wide focusable" data-act="shift">' + (state.pair.upper ? 'abc' : 'ABC') + '</div>' +
      '<div class="key wide focusable" data-act="del">Del</div>' +
      '<div class="key wide focusable" data-act="clear">Clear</div></div>' +
      '<div class="kbd-row">' +
      '<div class="key wide focusable" data-str="https://">https://</div>' +
      '<div class="key wide focusable" data-str="http://">http://</div>' +
      '<div class="key wide focusable" data-str=".com">.com</div>' +
      '<div class="key wide focusable" data-str="/tv?k=">/tv?k=</div></div>';
    var box = $('kbd');
    var hadFocus = focused && box.contains(focused) ? focused.getAttribute('data-act') || focused.getAttribute('data-ch') : null;
    box.innerHTML = html;
    var keys = box.querySelectorAll('.key');
    for (var i = 0; i < keys.length; i++) wireKey(keys[i]);
    if (hadFocus) {
      for (var j = 0; j < keys.length; j++) {
        if (keys[j].getAttribute('data-act') === hadFocus || keys[j].getAttribute('data-ch') === hadFocus) { setFocus(keys[j]); break; }
      }
    }
  }
  function wireKey(el) {
    onActivate(el, function () {
      var ch = el.getAttribute('data-ch'), act = el.getAttribute('data-act'), str = el.getAttribute('data-str');
      if (ch) typeText(state.pair.upper ? ch.toUpperCase() : ch);
      else if (str) typeText(str);
      else if (act === 'del') backspace();
      else if (act === 'clear') setFieldValue('');
      else if (act === 'shift') { state.pair.upper = !state.pair.upper; buildKeyboard(); }
    });
  }
  function fieldValue() { return state.pair[state.pair.active]; }
  function setFieldValue(v) { state.pair[state.pair.active] = v; $('pair-error').textContent = ''; renderFields(); }
  function typeText(s) { setFieldValue(fieldValue() + s); }
  function backspace() { var v = fieldValue(); if (v) setFieldValue(v.substring(0, v.length - 1)); }
  function setActiveField(f) { state.pair.active = f; renderFields(); }
  function renderFields() {
    var p = state.pair;
    function render(id, val, placeholder, active) {
      $(id).innerHTML = (val ? esc(val) : (active ? '' : '<span class="ph">' + esc(placeholder) + '</span>')) +
        (active ? '<span class="caret"></span>' : '');
    }
    render('val-server', p.server, 'e.g. https://kinkit.example.com', p.active === 'server');
    render('val-token', p.token, 'the part after ?k=', p.active === 'token');
    $('field-server').className = $('field-server').className.replace(/\s*\bactive\b/g, '') + (p.active === 'server' ? ' active' : '');
    $('field-token').className = $('field-token').className.replace(/\s*\bactive\b/g, '') + (p.active === 'token' ? ' active' : '');
    $('kbd-target').textContent = 'Typing into: ' + (p.active === 'server' ? 'Server address' : 'TV token');
  }
  function openPairing(message, prefill) {
    stopTimers();
    var p = state.pair;
    p.server = prefill ? prefill.server : (lsGet(LS.server) || p.server || '');
    p.token = prefill ? prefill.token : '';
    p.active = 'server';
    state.repairing = !!prefill;
    show('pair');
    $('pair-error').textContent = '';
    $('pair-info').textContent = message || '';
    buildKeyboard();
    renderFields();
    setFocus($('field-server'));
  }
  function connect() {
    var p = state.pair;
    if (p.busy) return;
    var res = parsePairing(p.server, p.token);
    if (res.error) { $('pair-error').textContent = res.error; return; }
    p.server = res.server; p.token = res.token; renderFields();
    p.busy = true;
    $('btn-connect').textContent = 'Connecting...';
    $('pair-error').textContent = ''; $('pair-info').textContent = '';
    getJSON(dataUrl(res), function (err, status, json) {
      p.busy = false;
      $('btn-connect').textContent = 'Connect';
      if (status === 401 || status === 403) {
        $('pair-error').textContent = 'Token rejected: it is wrong or was revoked. Create a new TV link in the web app.';
        return;
      }
      if (err || status !== 200 || !json || typeof json !== 'object') {
        $('pair-error').textContent = (err ? 'Cannot reach the server' : 'Server error (HTTP ' + status + ')') +
          '. Check the address, the network, and that the server allows this app (CORS - see README).';
        return;
      }
      saveConfig(res);
      lsSet(LS.cache, JSON.stringify(json)); lsSet(LS.cacheAt, String(Date.now()));
      startViewer(res);
    });
  }
  var sysKbdOpen = false;
  function openSystemKeyboard() {
    var input = $('syskbd-input');
    $('syskbd-label').textContent = state.pair.active === 'server' ? 'Server address' : 'TV token or link';
    input.value = fieldValue();
    $('syskbd').hidden = false;
    sysKbdOpen = true;
    setTimeout(function () { input.focus(); }, 50);
  }
  function closeSystemKeyboard(accept) {
    var input = $('syskbd-input');
    if (accept) setFieldValue(input.value.replace(/^\s+|\s+$/g, ''));
    input.blur();
    $('syskbd').hidden = true;
    sysKbdOpen = false;
  }

  // viewer
  function startViewer(cfg) {
    state.cfg = cfg;
    state.page = 0;
    state.offline = false;
    show('viewer');
    var cached = lsGet(LS.cache);
    state.data = null;
    if (cached) {
      try { state.data = parseData(JSON.parse(cached)); state.updatedAt = new Date(parseInt(lsGet(LS.cacheAt), 10) || Date.now()); } catch (e) { state.data = null; }
    }
    try { var fx = JSON.parse(lsGet(LS.fx) || 'null'); if (fx) money.rates = fx; } catch (e) { /* ignore */ }
    renderViewer();
    stopTimers();
    refresh();
    timers.refresh = setInterval(refresh, REFRESH_MS);
    restartRotation();
  }
  function stopTimers() {
    if (timers.refresh) clearInterval(timers.refresh);
    if (timers.rotate) clearInterval(timers.rotate);
    timers.refresh = timers.rotate = null;
  }
  function refresh() {
    var cfg = state.cfg;
    if (!cfg) return;
    getJSON(dataUrl(cfg), function (err, status, json) {
      if (state.cfg !== cfg) return; // re-paired meanwhile
      if (status === 401 || status === 403) {
        clearToken();
        state.cfg = null;
        openPairing('This TV link is invalid or was revoked. Create a new one in the web app under Settings > Data & devices.');
        return;
      }
      if (err || status !== 200 || !json || typeof json !== 'object') {
        state.offline = true;
        state.offlineReason = err ? 'Cannot reach the server.' : 'Server error (HTTP ' + status + ').';
      } else {
        state.data = parseData(json);
        state.updatedAt = new Date();
        state.offline = false;
        lsSet(LS.cache, JSON.stringify(json)); lsSet(LS.cacheAt, String(state.updatedAt.getTime()));
        maybeFetchFx(cfg, state.data.currency);
      }
      if (state.screen === 'viewer') renderViewer();
    });
  }
  function maybeFetchFx(cfg, currency) {
    money.code = currency;
    if (currency === 'IDR' || Date.now() - state.fxAt < 6 * 3600 * 1000) return;
    getJSON(fxUrl(cfg), function (err, status, json) {
      if (err || status !== 200 || !json || !json.rates) return;
      var rates = {};
      for (var k in json.rates) if (Object.prototype.hasOwnProperty.call(json.rates, k) && typeof json.rates[k] === 'number') rates[k.toUpperCase()] = json.rates[k];
      money.rates = rates;
      state.fxAt = Date.now();
      lsSet(LS.fx, JSON.stringify(rates));
      if (state.screen === 'viewer') renderViewer();
    });
  }
  function restartRotation() {
    if (timers.rotate) clearInterval(timers.rotate);
    timers.rotate = null;
    if (!state.paused) timers.rotate = setInterval(function () { go(1, true); }, ROTATE_MS);
  }
  function go(delta, auto) {
    if (!state.data) return;
    state.page = ((state.page + delta) % state.pageCount + state.pageCount) % state.pageCount;
    renderViewer();
    if (!auto) restartRotation();
  }
  function togglePause() { state.paused = !state.paused; restartRotation(); renderViewer(); }

  function updateClock() {
    var d = new Date();
    $('clock-time').textContent = hhmm(d);
    $('clock-date').textContent = DAYS[d.getDay()] + ' ' + d.getDate() + ' ' + MONTHS[d.getMonth()];
    var els = document.querySelectorAll ? document.querySelectorAll('[data-clock]') : [];
    for (var i = 0; i < els.length; i++) {
      els[i].textContent = els[i].getAttribute('data-clock') === 'time' ? hhmm(d) : DAYS[d.getDay()] + ', ' + d.getDate() + ' ' + MONTHS_LONG[d.getMonth()];
    }
  }

  // Kiki "thinking" for no-data states (styles.css .mk; same as tv/flutter _Empty).
  var MASCOT_THINK = '<div class="mk"><div class="mk-shadow"></div><img class="mk-img" src="mascot/mascot-think-512.png" alt=""></div>';
  var lastPageKey = '';
  function renderViewer() {
    var d = state.data;
    $('badge-offline').hidden = !state.offline;
    $('badge-offline').textContent = state.updatedAt ? 'OFFLINE - ' + hhmm(state.updatedAt) : 'OFFLINE';
    $('badge-paused').hidden = !state.paused;
    $('v-updated').textContent = state.updatedAt ? 'Updated ' + hhmm(state.updatedAt) : '';
    var body = $('v-body');
    if (!d) {
      $('v-name').textContent = ''; $('v-month').textContent = ''; $('v-dots').innerHTML = '';
      body.innerHTML = state.offline
        ? '<div class="page"><div class="center-msg">' + MASCOT_THINK + '<h1 class="c-warn">Cannot reach Kinkit</h1><p>' + esc(state.offlineReason) +
          '</p><p>Retrying every minute. Server: ' + esc(state.cfg ? state.cfg.server : '') +
          '</p><p class="c-acc">Hold BACK or press RED for settings</p></div></div>'
        : '<div class="page"><div class="center-msg"><div class="spinner"></div></div></div>';
      lastPageKey = '';
      applyBackdrop('');
      return;
    }
    money.code = d.currency;
    $('v-name').textContent = d.householdName || d.displayName || '';
    $('v-month').textContent = MONTHS_LONG[d.month - 1] + ' ' + d.year;

    var cats = activeCategories(d.report);
    var chunks = [];
    for (var i = 0; i < cats.length; i += PER_BUDGET_PAGE) chunks.push(cats.slice(i, i + PER_BUDGET_PAGE));
    if (!chunks.length) chunks.push([]);
    var pages = [function () { return pageOverview(d); }];
    for (var c = 0; c < chunks.length; c++) {
      (function (idx) { pages.push(function () { return pageBudget(chunks[idx], idx, chunks.length); }); })(c);
    }
    pages.push(function () { return pageTrend(d); });
    // A link tied to a widget board shows just that board (web Widgets page).
    if (d.board) {
      pages = [function () { return pageBoard(d); }];
      $('v-month').textContent = d.board.name;
    }
    state.pageCount = pages.length;
    if (state.page >= pages.length) state.page = 0;

    var bt = applyBackdrop(d.board ? d.board.background : '');
    var dots = '';
    for (var p = 0; p < pages.length; p++) {
      dots += '<span' + (p === state.page ? ' class="on"' : '') + (bt ? ' style="background:' + (p === state.page ? bt.fg : bt.track) + '"' : '') + '></span>';
    }
    $('v-dots').innerHTML = dots;

    var html = pages[state.page]();
    // Only replay the slide-in animation when the page actually changes.
    var key = state.page + '/' + pages.length;
    if (key !== lastPageKey) {
      body.innerHTML = '<div class="page">' + html + '</div>';
      lastPageKey = key;
    } else {
      body.firstChild.innerHTML = html;
    }
    if (d.board) updateClock();
  }

  function card(label, value, color, sub) {
    return '<div class="card" style="border-top-color:' + color + '"><div class="card-label">' + esc(label) +
      '</div><div class="card-value" style="color:' + color + '">' + esc(value) + '</div>' +
      (sub ? '<div class="card-sub">' + esc(sub) + '</div>' : '') + '</div>';
  }
  function bar(ratio, color, thick, track) {
    var w = ratio === null || isNaN(ratio) ? 0 : Math.max(0, Math.min(1, ratio));
    return '<div class="bar' + (thick ? ' thick' : '') + '"' + (track ? ' style="background:' + track + '"' : '') + '><div style="width:' + (w * 100).toFixed(1) + '%;background:' + color + '"></div></div>';
  }
  // widget board: 12-column flow grid scaled to fit the screen (same rules as
  // react/src/WidgetBoard.jsx and tv/flutter lib/src/board_page.dart)
  // Widget colours (react/src/widgetColors.js): '#rrggbb' fill per widget, '#rrggbb' board backdrop.
  // Text colour is white or near-black, whichever contrasts better; null tone = default dark look.
  function cleanHex(v) {
    return typeof v === 'string' && /^#[0-9a-fA-F]{6}$/.test(v.replace(/^\s+|\s+$/g, '')) ? v.replace(/^\s+|\s+$/g, '').toLowerCase() : '';
  }
  function hexRgb(h) { return [parseInt(h.substr(1, 2), 16), parseInt(h.substr(3, 2), 16), parseInt(h.substr(5, 2), 16)]; }
  function rgbHex(a) {
    var out = '#';
    for (var i = 0; i < 3; i++) { var n = Math.max(0, Math.min(255, Math.round(a[i]))); out += (n < 16 ? '0' : '') + n.toString(16); }
    return out;
  }
  function relLum(h) {
    var c = hexRgb(h), k = [];
    for (var i = 0; i < 3; i++) { var v = c[i] / 255; k.push(v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4)); }
    return 0.2126 * k[0] + 0.7152 * k[1] + 0.0722 * k[2];
  }
  function readableOn(h) {
    var l = relLum(h);
    return (l + 0.05) / (relLum('#1b1f2b') + 0.05) >= 1.05 / (l + 0.05) ? '#1b1f2b' : '#ffffff';
  }
  function mixHex(a, b, t) {
    var x = hexRgb(a), y = hexRgb(b);
    return rgbHex([x[0] + (y[0] - x[0]) * t, x[1] + (y[1] - x[1]) * t, x[2] + (y[2] - x[2]) * t]);
  }
  function boardTone(hex) {
    if (!hex) return null;
    var fg = readableOn(hex);
    return { bg: hex, fg: fg, muted: mixHex(fg, hex, 0.32), track: mixHex(hex, fg, 0.16), border: mixHex(hex, fg, 0.14),
      danger: fg === '#ffffff' ? '#fecaca' : '#b91c1c' };
  }
  function fgStyle(color) { return color ? ' style="color:' + color + '"' : ''; }
  // Page backdrop for a board with a background colour (footer text/dots follow it).
  function applyBackdrop(hex) {
    var t = boardTone(hex), i;
    document.documentElement.style.background = t ? t.bg : '';
    document.body.style.background = t ? t.bg : '';
    var foot = document.querySelectorAll ? document.querySelectorAll('.v-footer .muted') : [];
    for (i = 0; i < foot.length; i++) foot[i].style.color = t ? t.muted : '';
    return t;
  }
  function boardMoney(idr) { return fmt(idr); }
  function boardCard(c, tone) {
    var mu = fgStyle(tone && tone.muted);
    var big = c.big_amount !== undefined && c.big_amount !== null ? boardMoney(toInt(c.big_amount)) : (c.big || '');
    var html = '<div class="bw-title"' + mu + '>' + esc(c.title || '') + '</div>';
    if (big) html += '<div class="bw-big' + (toInt(c.big_amount) < 0 ? ' c-neg' : '') + '"' + (tone && toInt(c.big_amount) < 0 ? fgStyle(tone.danger) : '') + '>' + esc(big) + '</div>';
    if (c.caption) html += '<div class="bw-cap"' + mu + '>' + esc(c.caption) + '</div>';
    var rows = c.rows || [];
    if (!rows.length) {
      if (!big) html += '<div class="bw-cap"' + mu + '>' + esc(c.empty || 'Nothing to show') + '</div>';
      return html;
    }
    html += '<div class="bw-rows">';
    for (var i = 0; i < rows.length; i++) {
      var r = rows[i] || {};
      html += '<div class="bw-row' + (r.done ? ' done' : '') + (r.alert ? ' alert' : '') + '"><div class="bw-line"><span class="bw-rt"' + (tone && r.alert ? fgStyle(tone.danger) : '') + '>' + esc(r.title || '') + '</span>' +
        (r.amount !== undefined && r.amount !== null ? '<span class="bw-amt"' + mu + '>' + esc(boardMoney(toInt(r.amount))) + '</span>' : '') + '</div>' +
        (r.detail ? '<div class="bw-det"' + mu + '>' + esc(r.detail) + '</div>' : '') +
        (typeof r.pct === 'number' ? bar(r.pct / 100, tone ? (r.alert ? tone.danger : tone.fg) : (r.alert ? '#f87171' : '#818cf8'), false, tone && tone.track) : '') + '</div>';
    }
    return html + '</div>';
  }
  function pageBoard(d) {
    var b = d.board, W = 1760, GAP = 24;
    var H = $('v-body').clientHeight || 760;
    if (!b.widgets.length) return '<div class="empty">' + MASCOT_THINK + 'This board is empty</div>';
    var lines = [[]], used = 0, i, j;
    for (i = 0; i < b.widgets.length; i++) {
      var w = b.widgets[i];
      if (used + w.w > 12 && lines[lines.length - 1].length) { lines.push([]); used = 0; }
      lines[lines.length - 1].push(w); used += w.w;
    }
    var units = 0, gaps = lines.length - 1, lh = [];
    for (i = 0; i < lines.length; i++) {
      var m = 1;
      for (j = 0; j < lines[i].length; j++) if (lines[i][j].h > m) m = lines[i][j].h;
      lh.push(m); units += m; gaps += m - 1;
    }
    var unit = Math.max(60, Math.min(300, (H - gaps * GAP) / units));
    var html = '';
    for (i = 0; i < lines.length; i++) {
      var sum = 0;
      for (j = 0; j < lines[i].length; j++) sum += lines[i][j].w;
      html += '<div class="bw-line-row" style="margin-bottom:' + (i < lines.length - 1 ? GAP : 0) + 'px">';
      for (j = 0; j < lines[i].length; j++) {
        var x = lines[i][j];
        var width = Math.floor((W - GAP * (lines[i].length - 1)) * x.w / sum);
        var height = Math.round(x.h * unit + (x.h - 1) * GAP);
        var inner, tone = boardTone(x.color);
        if (x.type === 'clock') inner = '<div class="bw-clock-time" style="font-size:' + Math.max(40, Math.min(120, Math.round(height * 0.55))) + 'px" data-clock="time">--:--</div><div class="bw-clock-date"' + fgStyle(tone && tone.muted) + ' data-clock="date"></div>';
        else if (x.type === 'note') inner = '<div class="bw-note">' + esc(x.text || 'Empty note') + '</div>';
        else if (b.cards[x.type]) inner = boardCard(b.cards[x.type], tone);
        else inner = '<div class="bw-cap"' + fgStyle(tone && tone.muted) + '>No data</div>';
        html += '<div class="bw-tile" style="width:' + width + 'px;height:' + height + 'px;margin-right:' + (j < lines[i].length - 1 ? GAP : 0) + 'px' +
          (tone ? ';background:' + tone.bg + ';border-color:' + tone.border + ';color:' + tone.fg : '') + '">' + inner + '</div>';
      }
      html += '</div>';
    }
    return html;
  }
  function pageOverview(d) {
    var r = d.report, spent = totalSpent(r), left = r.budgetedExpenses - spent;
    var pct = r.budgetedExpenses > 0 ? Math.round(spent * 100 / r.budgetedExpenses) + '% of budget' : null;
    var top = topExpenses(r, 5);
    var html = '<div class="row cards">' +
      card('Income', fmt(r.totalIncome), '#4ade80') +
      card('Budget', fmt(r.budgetedExpenses), '#818cf8') +
      card('Spent', fmt(spent), spent > r.budgetedExpenses && r.budgetedExpenses > 0 ? '#f87171' : '#fbbf24', pct) +
      card('Savings', fmt(r.savings), '#c4b5fd') + '</div>';
    html += '<div class="row panels">';
    html += '<div class="panel" style="flex:4;-webkit-flex:4"><div class="panel-title">This month</div><div class="panel-body kv-big">' +
      '<div class="k">Left to budget</div><div class="v ' + (r.leftForBudgeting >= 0 ? 'c-pos' : 'c-neg') + '">' + esc(fmt(r.leftForBudgeting)) + '</div>' +
      '<div class="k">' + (left >= 0 ? 'Budget remaining' : 'Over budget by') + '</div><div class="v ' + (left >= 0 ? 'c-pos' : 'c-neg') + '">' + esc(fmt(Math.abs(left))) + '</div>' +
      '<div class="full"></div>' +
      (r.budgetedExpenses > 0 ? bar(spent / r.budgetedExpenses, colorFor(spent / r.budgetedExpenses, spent), true) +
        '<div class="muted" style="font-size:26px;margin-top:8px">Spent ' + esc(fmt(spent)) + ' of ' + esc(fmt(r.budgetedExpenses)) + '</div>' : '') +
      '</div></div>';
    html += '<div class="panel" style="flex:5;-webkit-flex:5"><div class="panel-title">Top expenses</div><div class="panel-body">';
    if (!top.length) html += '<div class="empty">' + MASCOT_THINK + 'No spending yet this month</div>';
    for (var i = 0; i < top.length; i++) {
      html += '<div class="exp-row"><span class="name">' + esc(top[i].name) + '</span><span class="amt">' + esc(fmt(top[i].realization)) + '</span></div>';
    }
    html += '</div></div></div>';
    return html;
  }
  function pageBudget(items, idx, total) {
    var html = '<div class="panel full"><div class="panel-title">Budget vs actual' +
      (total > 1 ? '<span class="right">' + (idx + 1) + ' / ' + total + '</span>' : '') + '</div><div class="panel-body">';
    if (!items.length) html += '<div class="empty">' + MASCOT_THINK + 'No budget set for this month</div>';
    for (var i = 0; i < PER_BUDGET_PAGE && items.length; i++) {
      var e = items[i];
      if (!e) { html += '<div class="bud-row"></div>'; continue; }
      var ratio = ratioOf(e), color = colorFor(ratio, e.realization);
      html += '<div class="bud-row"><div class="bud-line"><span class="name">' + esc(e.name) + '</span>' +
        '<span class="amt">' + esc(fmt(e.realization)) + ' / ' + esc(fmt(e.allocation)) + '</span>' +
        '<span class="pct" style="color:' + color + '">' + (ratio === null ? 'no budget' : Math.round(ratio * 100) + '%') + '</span></div>' +
        bar(ratio === null ? (e.realization > 0 ? 1 : 0) : ratio, color) + '</div>';
    }
    return html + '</div></div>';
  }
  function pageTrend(d) {
    var t = d.trend;
    var html = '<div class="panel full"><div class="panel-title">6-month trend' +
      '<span class="legend" style="margin-left:auto"><i style="background:#4ade80"></i>Income</span>' +
      '<span class="legend"><i style="background:#fbbf24"></i>Spent</span></div><div class="panel-body">';
    if (!t.length) return html + '<div class="empty">' + MASCOT_THINK + 'No history yet</div></div></div>';
    html += '<div class="chart">' + trendSvg(d) + '</div>';
    return html + '</div></div>';
  }
  function trendSvg(d) {
    var t = d.trend, W = 1640, H = 600, left = 170, bottom = 60, top = 20;
    var max = 0;
    for (var i = 0; i < t.length; i++) max = Math.max(max, t[i].income, t[i].expense);
    var maxY = max > 0 ? max * 1.15 : 1;
    var plotH = H - bottom - top, plotW = W - left - 10;
    var svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ' + W + ' ' + H + '" preserveAspectRatio="xMidYMid meet">';
    for (var g = 0; g <= 4; g++) {
      var gy = top + plotH - plotH * g / 4;
      svg += '<line x1="' + left + '" x2="' + W + '" y1="' + gy + '" y2="' + gy + '" stroke="#262d47" stroke-width="2"/>' +
        '<text x="' + (left - 16) + '" y="' + (gy + 9) + '" fill="#9ca3af" font-size="26" text-anchor="end">' + esc(fmtCompact(maxY * g / 4)) + '</text>';
    }
    var slot = plotW / t.length, bw = Math.min(70, slot / 3);
    for (var j = 0; j < t.length; j++) {
      var cx = left + slot * j + slot / 2;
      var hi = plotH * t[j].income / maxY, he = plotH * t[j].expense / maxY;
      svg += '<rect x="' + (cx - bw - 6) + '" y="' + (top + plotH - hi) + '" width="' + bw + '" height="' + hi + '" rx="8" fill="#4ade80"/>';
      svg += '<rect x="' + (cx + 6) + '" y="' + (top + plotH - he) + '" width="' + bw + '" height="' + he + '" rx="8" fill="#fbbf24"/>';
      var cur = t[j].year === d.year && t[j].month === d.month;
      svg += '<text x="' + cx + '" y="' + (H - 16) + '" fill="' + (cur ? '#f1f3f8' : '#9ca3af') + '" font-size="30" font-weight="' +
        (cur ? 800 : 500) + '" text-anchor="middle">' + MONTHS[t[j].month - 1] + '</text>';
    }
    return svg + '</svg>';
  }

  // settings
  function openSettings() {
    if (state.screen !== 'viewer') return;
    // The key that opened us may still be held: ignore its repeats and keyup.
    if (state.held) state.swallow = state.held.code;
    state.held = null;
    var cfg = state.cfg;
    $('s-server').textContent = cfg ? cfg.server : '';
    $('s-token').textContent = cfg ? maskToken(cfg.token) : '';
    $('s-updated').textContent = state.updatedAt ? state.updatedAt.toString().replace(/ GMT.*$/, '') : 'never';
    $('s-status').textContent = state.offline ? 'Offline - ' + state.offlineReason : 'Online';
    show('settings');
    setFocus($('s-back'));
  }
  function closeSettings() { show('viewer'); renderViewer(); }

  function exitApp() {
    try { if (window.tizen) { window.tizen.application.getCurrentApplication().exit(); return; } } catch (e) { /* not Tizen */ }
    try { if (window.PalmSystem && window.PalmSystem.platformBack) { window.PalmSystem.platformBack(); return; } } catch (e) { /* not webOS */ }
    try { if (window.webOS && window.webOS.platformBack) { window.webOS.platformBack(); return; } } catch (e) { /* ignore */ }
    try { window.close(); } catch (e) { /* plain browser: nothing to exit */ }
  }

  // key handling
  function longPressStart(code) {
    if (state.held && state.held.code === code) {
      // auto-repeat keydown while held
      if (!state.held.fired && Date.now() - state.held.since >= LONG_PRESS_MS) { state.held.fired = true; openSettings(); }
      return;
    }
    state.held = { code: code, since: Date.now(), fired: false };
  }
  function longPressEnd(code, shortFn) {
    var h = state.held;
    state.held = null;
    if (!h || h.code !== code || h.fired) return;
    if (Date.now() - h.since >= LONG_PRESS_MS) openSettings(); else shortFn();
  }

  document.addEventListener('keydown', function (e) {
    var c = e.keyCode;
    if (sysKbdOpen) {
      if (c === K.ENTER || c === 65376 /* Tizen IME Done */) { e.preventDefault(); closeSystemKeyboard(true); }
      else if (c === K.BACK_TIZEN || c === K.BACK_WEBOS || c === K.ESC || c === 65385 /* Tizen IME Cancel */) { e.preventDefault(); closeSystemKeyboard(false); }
      return; // let the input handle everything else
    }
    if (state.swallow !== null && c === state.swallow) { e.preventDefault(); return; }
    var handled = true;
    if (state.screen === 'viewer') {
      if (c === K.LEFT || c === K.CH_UP || c === K.PAGE_UP || c === K.REW) go(-1);
      else if (c === K.RIGHT || c === K.CH_DOWN || c === K.PAGE_DOWN || c === K.FF) go(1);
      else if (c === K.PLAY_PAUSE || c === K.PLAY || c === K.PAUSE || c === K.SPACE) { if (!e.repeat) togglePause(); }
      else if (isMenu(c)) openSettings();
      else if (c === K.ENTER || isBack(c)) longPressStart(c);
      else handled = false;
    } else if (state.screen === 'settings' || state.screen === 'pair') {
      if (c === K.LEFT) moveFocus('left');
      else if (c === K.RIGHT) moveFocus('right');
      else if (c === K.UP) moveFocus('up');
      else if (c === K.DOWN) moveFocus('down');
      else if (c === K.ENTER) { if (!state.held) { state.held = { code: c, since: Date.now(), fired: false }; } }
      else if (state.screen === 'pair' && c === K.BACKSPACE) backspace();
      else if (isBack(c)) { if (!state.held) state.held = { code: c, since: Date.now(), fired: false }; }
      else handled = false;
    } else handled = false;
    if (handled) e.preventDefault();
  });

  document.addEventListener('keyup', function (e) {
    if (sysKbdOpen) return;
    var c = e.keyCode;
    if (state.swallow !== null && c === state.swallow) { state.swallow = null; return; }
    if (state.screen === 'viewer') {
      if (c === K.ENTER) longPressEnd(c, togglePause);
      else if (isBack(c)) longPressEnd(c, exitApp);
      return;
    }
    var h = state.held;
    // Only act on presses that started on this screen (the long press that
    // opened settings still delivers its keyup here).
    if (!h || h.code !== c) return;
    state.held = null;
    if (c === K.ENTER) activate(focused);
    else if (isBack(c)) {
      if (state.screen === 'settings') closeSettings();
      else if (state.screen === 'pair') { if (state.repairing && loadConfig()) startViewer(loadConfig()); else exitApp(); }
    }
  });

  // Hardware keyboards on the pairing screen: type straight into the active field.
  document.addEventListener('keypress', function (e) {
    if (sysKbdOpen || state.screen !== 'pair') return;
    var code = e.charCode || e.which;
    if (code > 32 && code !== 127) { typeText(String.fromCharCode(code)); e.preventDefault(); }
  });
  document.addEventListener('paste', function (e) {
    if (sysKbdOpen || state.screen !== 'pair') return;
    var text = (e.clipboardData || window.clipboardData).getData('text');
    if (text) { typeText(text.replace(/^\s+|\s+$/g, '')); e.preventDefault(); }
  });

  // boot
  function registerTizenKeys() {
    try {
      var keys = ['MediaPlayPause', 'MediaPlay', 'MediaPause', 'MediaRewind', 'MediaFastForward',
                  'ChannelUp', 'ChannelDown', 'ColorF0Red', 'Info'];
      for (var i = 0; i < keys.length; i++) {
        try { window.tizen.tvinputdevice.registerKey(keys[i]); } catch (e) { /* key not supported on this model */ }
      }
    } catch (e) { /* not Tizen */ }
  }
  function fitStage() {
    var s = Math.min(window.innerWidth / 1920, window.innerHeight / 1080);
    var tf = 'translate(-50%, -50%) scale(' + s + ')';
    var st = $('stage').style;
    st.webkitTransform = tf; st.transform = tf;
  }

  function boot() {
    fitStage();
    window.addEventListener('resize', fitStage);
    if (window.tizen) registerTizenKeys();
    updateClock();
    setInterval(updateClock, 1000);

    onActivate($('field-server'), function () { setActiveField('server'); });
    onActivate($('field-token'), function () { setActiveField('token'); });
    onActivate($('btn-connect'), connect);
    onActivate($('btn-syskbd'), openSystemKeyboard);
    onActivate($('s-back'), closeSettings);
    onActivate($('s-refresh'), function () { closeSettings(); refresh(); });
    onActivate($('s-repair'), function () { openPairing('', state.cfg); });
    onActivate($('s-unpair'), function () { clearToken(); state.cfg = null; openPairing(''); });
    onActivate($('s-exit'), exitApp);
    $('syskbd-input').addEventListener('blur', function () { if (sysKbdOpen) closeSystemKeyboard(true); });

    // Opened as index.html?server=...&k=... or ?k=... (e.g. hosted on the Kinkit
    // server itself) -> pair automatically.
    var qs = window.location.search || '';
    var qk = queryParam(qs, 'k');
    if (qk) {
      var qServer = queryParam(qs, 'server') ||
        (/^https?:$/.test(window.location.protocol) ? window.location.protocol + '//' + window.location.host : '');
      var res = parsePairing(qServer, qk);
      if (!res.error) { saveConfig(res); startViewer(res); return; }
    }
    var cfg = loadConfig();
    if (cfg) startViewer(cfg); else openPairing('');
  }

  // Exposed for quick checks in a desktop browser console.
  window.KinkitTV = { parsePairing: parsePairing, normalizeServer: normalizeServer, tokenFromShareUrl: tokenFromShareUrl, parseData: parseData };

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot); else boot();
})();
