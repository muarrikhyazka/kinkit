import { categoryColor } from './categoryColor'

// Native <select> can't render colored dots inside <option>, so we show the
// selected value's dot next to the select and keep <option> as plain text.
// `suggested` marks a value that was auto-filled (e.g. remembered from the
// last entry with the same description). Extra props go to the <select>.
export default function CategorySelect({ value, onChange, categories, required, placeholder = 'Category...', suggested = false, ...rest }) {
  return (
    <span className={`category-select ${suggested ? 'suggested' : ''}`} title={suggested ? 'Suggested from your last entry with this description' : undefined}>
      {value && <span className="cat-dot" style={{ background: categoryColor(value) }} />}
      <select aria-label={rest['aria-label'] || 'Category'} {...rest} value={value} onChange={e => onChange(e.target.value)} required={required}>
        <option value="">{placeholder}</option>
        {categories.map(c => <option key={c.id} value={c.name}>{c.name}</option>)}
      </select>
    </span>
  )
}
