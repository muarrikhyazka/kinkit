import { useState, useEffect, useRef, useId } from 'react'

function formatThousands(n) {
  if (n === '' || n === null || n === undefined) return ''
  const num = Number(n)
  if (Number.isNaN(num)) return ''
  return num.toLocaleString('id-ID')
}

function digitsOnly(s) {
  const digits = String(s).replace(/[^\d-]/g, '')
  return digits === '' ? '' : Number(digits)
}

const SUFFIX = { k: 1e3, rb: 1e3, ribu: 1e3, jt: 1e6, juta: 1e6, m: 1e6, mio: 1e6 }

/**
 * Amount shorthand: "25k" = 25000, "2rb" = 2000, "1.5jt" / "1,5jt" / "1.5m" = 1500000.
 * Returns a number, or null when the text isn't valid shorthand.
 */
export function parseShorthand(text) {
  const m = String(text).toLowerCase().replace(/\s+/g, '').match(/^(\d+(?:[.,]\d+)?)(k|rb|ribu|jt|juta|m|mio)$/)
  if (!m) return null
  return Math.round(parseFloat(m[1].replace(',', '.')) * SUFFIX[m[2]])
}

// Decide what to show and what number it means for the raw text typed so far.
function interpret(raw) {
  if (/[a-z]/i.test(raw)) {
    const v = parseShorthand(raw)
    return { display: raw, value: v ?? '', shorthand: true, parsed: v }
  }
  // "1." / "1.5" may be the start of "1.5jt" - don't reformat it away yet.
  if (/[.,]\d?$/.test(raw.trim())) return { display: raw, value: digitsOnly(raw), shorthand: false }
  const v = digitsOnly(raw)
  return { display: formatThousands(v), value: v, shorthand: false }
}

/**
 * Number input that auto-formats with thousands separators (id-ID: 1.000.000)
 * and accepts shorthand (25k, 2rb, 1.5jt, 1.5m). A small hint bubble shows the
 * resolved amount while shorthand is being typed.
 * onChange(value) fires while typing, onBlur(value) on blur; value is a number or ''.
 */
export default function MoneyInput({ value, onChange, onBlur, className = '', ...props }) {
  const [display, setDisplay] = useState(formatThousands(value))
  const emitted = useRef(value)
  const hintId = useId()

  // Sync from the parent only when the value changed from outside (not as an
  // echo of what we just emitted) - otherwise "25k" would be reformatted mid-typing.
  useEffect(() => {
    if (value !== emitted.current) {
      emitted.current = value
      setDisplay(formatThousands(value))
    }
  }, [value])

  function handleChange(e) {
    const r = interpret(e.target.value)
    setDisplay(r.display)
    emitted.current = r.value
    onChange && onChange(r.value)
  }

  function handleBlur(e) {
    const r = interpret(e.target.value)
    if (r.shorthand && r.parsed == null) { onBlur && onBlur(r.value); return } // keep text so the user can fix it
    setDisplay(formatThousands(r.value))
    onBlur && onBlur(r.value)
  }

  const r = /[a-z]/i.test(display) ? interpret(display) : null

  return (
    <span className={`money-input ${className}`}>
      <input
        inputMode="decimal"
        {...props}
        type="text"
        autoComplete="off"
        value={display}
        onChange={handleChange}
        onBlur={handleBlur}
        aria-describedby={r ? hintId : props['aria-describedby']}
      />
      {r && (
        <span id={hintId} className={`money-hint ${r.parsed == null ? 'invalid' : ''}`}>
          {r.parsed == null ? 'Try 25k, 2rb, 1.5jt' : `= ${formatThousands(r.parsed)}`}
        </span>
      )}
    </span>
  )
}
