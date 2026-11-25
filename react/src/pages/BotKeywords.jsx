import { useMemo, useState } from 'react'
import PageHeader from '../PageHeader.jsx'
import { SkeletonCard } from '../Skeleton.jsx'
import { useToast } from '../Toast.jsx'
import { useApi, useSetup, useDebounced } from '../useApi'
import { mealApi } from '../api'
import { IconClose } from '../icons.jsx'
import '../mealplan.css'

/**
 * Settings › Chat keywords: words the chat bot uses to pick a category when
 * someone types "lunch 15k". The bot learns them when a category button is
 * pressed for an unknown expense; here they can be added, re-mapped or removed.
 * Backed by backend-go/bot_extras.go (category_keywords table).
 */
export default function BotKeywords() {
  const toast = useToast()
  const { categories } = useSetup()
  const { data, error, setData } = useApi(() => mealApi.keywords(), [])
  const [q, setQ] = useState('')
  const query = useDebounced(q.trim().toLowerCase(), 150)
  const [kw, setKw] = useState('')
  const [cat, setCat] = useState('')

  const items = data?.items
  const filtered = useMemo(() => (items || []).filter(it =>
    !query || it.keyword.includes(query) || it.category.toLowerCase().includes(query)), [items, query])

  const save = async (keyword, category) => {
    try {
      const r = await mealApi.saveKeyword(keyword, category)
      setData(d => {
        const rest = (d?.items || []).filter(x => x.id !== r.id)
        return { items: [...rest, { id: r.id, keyword: r.keyword, category: r.category }].sort((a, b) => a.keyword.localeCompare(b.keyword)) }
      })
      return true
    } catch (err) { toast.error(err.message); return false }
  }

  const add = async (e) => {
    e.preventDefault()
    if (!kw.trim() || !cat) return
    if (await save(kw, cat)) { toast.success(`"${kw.trim().toLowerCase()}" -> ${cat}`); setKw('') }
  }

  const remove = async (it) => {
    try {
      await mealApi.deleteKeyword(it.id)
      setData(d => ({ items: (d?.items || []).filter(x => x.id !== it.id) }))
    } catch (err) { toast.error(err.message) }
  }

  return (
    <div className="kw-page">
      <PageHeader title="Chat keywords" subtitle={items ? `${items.length} keywords` : undefined} />
      <section className="card">
        <p className="muted meal-note">
          When someone types an expense in the family chat, e.g. <kbd>lunch 15k</kbd>, the bot looks for one of these words to
          pick the category. Unknown expenses get category buttons, and the words of the description are learned from the
          button that is pressed.
        </p>
        <form className="meal-add" onSubmit={add}>
          <label className="sr-only" htmlFor="kw-word">Keyword</label>
          <input id="kw-word" value={kw} onChange={e => setKw(e.target.value)} placeholder="Keyword, e.g. fuel" maxLength={60} />
          <label className="sr-only" htmlFor="kw-cat">Category</label>
          <select id="kw-cat" value={cat} onChange={e => setCat(e.target.value)}>
            <option value="">Category</option>
            {categories.map(c => <option key={c.id} value={c.name}>{c.name}</option>)}
          </select>
          <button type="submit" disabled={!kw.trim() || !cat}>Save</button>
        </form>
      </section>

      <section className="card">
        <label className="sr-only" htmlFor="kw-search">Search keywords</label>
        <input id="kw-search" className="kw-search" type="search" value={q} onChange={e => setQ(e.target.value)} placeholder="Search keywords or categories" />
        {error && <div className="error-box">{error}</div>}
        {!items ? <SkeletonCard /> : filtered.length === 0 ? <p className="muted">No keywords{query ? ' match' : ' yet'}.</p> : (
          <ul className="kw-list">
            {filtered.slice(0, 300).map(it => (
              <li key={it.id} className="kw-row">
                <span className="kw-word">{it.keyword}</span>
                <select aria-label={`Category for ${it.keyword}`} value={it.category} onChange={e => save(it.keyword, e.target.value)}>
                  {!categories.some(c => c.name === it.category) && <option value={it.category}>{it.category}</option>}
                  {categories.map(c => <option key={c.id} value={c.name}>{c.name}</option>)}
                </select>
                <button className="icon-btn" onClick={() => remove(it)} aria-label={`Delete ${it.keyword}`}><IconClose width={12} height={12} /></button>
              </li>
            ))}
          </ul>
        )}
        {filtered.length > 300 && <p className="muted meal-note">Showing 300 of {filtered.length}. Search to narrow down.</p>}
      </section>
    </div>
  )
}
