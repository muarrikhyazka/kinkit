import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { PAGES, pagePath } from './index.js'

/**
 * In-memory search index, built lazily the first time the search box is used:
 * every page is rendered to static HTML once and split into sections at its
 * h2/h3 headings, so a result can deep-link to the heading.
 */
let cache = null

const clean = s => s.replace(/\s+/g, ' ').replace(/\s*#\s*$/, '').trim()

export function buildIndex() {
  if (cache) return cache
  const entries = []
  const parser = new DOMParser()
  for (const page of PAGES) {
    let html = ''
    try {
      html = renderToStaticMarkup(createElement(MemoryRouter, null, createElement(page.Content)))
    } catch (e) {
      console.warn('docs search: could not index', page.slug, e)
    }
    const doc = parser.parseFromString(`<div>${html.replace(/></g, '> <')}</div>`, 'text/html')
    const root = doc.body.firstElementChild
    let cur = { heading: '', id: '', text: '' }
    const first = entries.length
    const flush = () => {
      if (cur.text.trim() || cur.heading) entries.push({ page, heading: cur.heading, id: cur.id, text: clean(cur.text) })
    }
    for (const el of root.children) {
      collect(el)
    }
    function collect(el) {
      const tag = el.tagName
      if (tag === 'H2' || tag === 'H3') {
        flush()
        cur = { heading: clean(el.textContent), id: el.id, text: '' }
      } else if (el.querySelector && el.querySelector('h2,h3')) {
        for (const c of el.children) collect(c)
      } else {
        cur.text += ' ' + el.textContent
      }
    }
    flush()
    // the text before the first heading doubles as the page-level result
    if (entries.length > first && !entries[first].heading) {
      entries[first].isTitle = true
      entries[first].text = clean(page.summary + ' ' + entries[first].text)
    } else {
      entries.splice(first, 0, { page, heading: '', id: '', text: page.summary, isTitle: true })
    }
  }
  cache = entries
  return entries
}

const norm = s => s.toLowerCase()

export function search(query, limit = 8) {
  const tokens = norm(query).split(/\s+/).filter(Boolean)
  if (!tokens.length) return []
  const out = []
  for (const e of buildIndex()) {
    const title = norm(e.page.title), head = norm(e.heading), text = norm(e.text), sec = norm(e.page.sectionTitle)
    let score = 0
    let ok = true
    for (const t of tokens) {
      let s = 0
      if (title.includes(t)) s += e.isTitle ? (title.startsWith(t) ? 14 : 10) : 2
      if (head.includes(t)) s += 7
      if (sec.includes(t)) s += 1
      if (text.includes(t)) s += 2 + Math.min(text.split(t).length - 1, 4) * 0.5
      if (e.isTitle && norm(e.page.slug).includes(t)) s += 2
      if (!s) { ok = false; break }
      score += s
    }
    if (!ok) continue
    if (e.isTitle) score += 3
    out.push({ ...e, score, href: pagePath(e.page.slug) + (e.id ? `#${e.id}` : '') })
  }
  out.sort((a, b) => b.score - a.score)
  // at most 3 hits per page so one long page cannot fill the list
  const per = {}
  const res = []
  for (const r of out) {
    per[r.page.slug] = (per[r.page.slug] || 0) + 1
    if (per[r.page.slug] <= 3) res.push(r)
    if (res.length >= limit) break
  }
  return res
}

export function snippet(entry, query, len = 120) {
  const t = entry.text
  if (!t) return ''
  const tokens = norm(query).split(/\s+/).filter(Boolean)
  const low = norm(t)
  let at = -1
  for (const tok of tokens) { const i = low.indexOf(tok); if (i >= 0) { at = i; break } }
  if (at < 0) return t.slice(0, len) + (t.length > len ? '…' : '')
  const start = Math.max(0, at - 40)
  return (start > 0 ? '…' : '') + t.slice(start, start + len) + (start + len < t.length ? '…' : '')
}
