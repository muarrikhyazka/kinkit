// Optional on-device OCR for receipt photos. tesseract.js (~ several MB with
// its language data) is loaded from the CDN only when the user clicks
// "Read amount" - it is deliberately NOT an npm dependency.
const TESSERACT_SRC = 'https://cdn.jsdelivr.net/npm/tesseract.js@5.1.1/dist/tesseract.min.js'

let loading = null
function loadTesseract() {
  if (window.Tesseract) return Promise.resolve(window.Tesseract)
  if (!loading) {
    loading = new Promise((resolve, reject) => {
      const s = document.createElement('script')
      s.src = TESSERACT_SRC
      s.async = true
      s.crossOrigin = 'anonymous'
      s.onload = () => (window.Tesseract ? resolve(window.Tesseract) : reject(new Error('OCR library failed to load')))
      s.onerror = () => { loading = null; reject(new Error('Could not load the OCR library (are you offline?)')) }
      document.head.appendChild(s)
    })
  }
  return loading
}

// "1.234.567" / "1,234,567.89" / "25.000" / "12,50" -> number (same rules as the CSV importer)
export function parseLooseAmount(raw) {
  let s = String(raw).replace(/[^\d.,]/g, '')
  if (!s) return NaN
  const lastDot = s.lastIndexOf('.')
  const lastComma = s.lastIndexOf(',')
  let decimal = '.'
  if (lastDot >= 0 && lastComma >= 0) decimal = lastDot > lastComma ? '.' : ','
  else if (lastDot >= 0) decimal = (s.split('.').length > 2 || s.length - lastDot - 1 === 3) ? ',' : '.'
  else if (lastComma >= 0) decimal = (s.split(',').length > 2 || s.length - lastComma - 1 === 3) ? '.' : ','
  const thousands = decimal === '.' ? ',' : '.'
  s = s.split(thousands).join('')
  if (decimal === ',') s = s.replace(',', '.')
  return Number(s)
}

const TOTAL_WORDS = /(grand\s*total|total|jumlah|tagihan|amount\s*due|balance\s*due|to\s*pay|netto)/i
const SUBTOTAL = /sub\s*-?\s*total/i
const PAID_WORDS = /(bayar|tunai|cash|debit|kredit|credit|qris|card)/i // what was tendered - may exceed the total
const IGNORE_WORDS = /(kembali|kembalian|change|disc|diskon|hemat|saving|tax\s*id|npwp|telp|phone|no\.|invoice|ref)/i
const NUMBER = /\d{1,3}(?:[.,\s]\d{3})+(?:[.,]\d{1,2})?|\d+(?:[.,]\d{1,2})?/g

/** Picks the most likely total from OCR text: the largest amount on a
 *  "total" line (subtotals only if there's no real total), then a
 *  "paid/cash" line, else the largest plausible amount anywhere. */
export function pickTotal(text) {
  const totals = []
  const subtotals = []
  const paid = []
  const all = []
  for (const line of String(text).split(/\r?\n/)) {
    if (IGNORE_WORDS.test(line)) continue
    const nums = (line.match(NUMBER) || [])
      .map(n => parseLooseAmount(n.replace(/\s/g, '')))
      .filter(n => Number.isFinite(n) && n > 0 && n < 1e10)
    if (!nums.length) continue
    const max = Math.max(...nums)
    all.push(max)
    if (SUBTOTAL.test(line)) subtotals.push(max)
    else if (TOTAL_WORDS.test(line)) totals.push(max)
    else if (PAID_WORDS.test(line)) paid.push(max)
  }
  const plausible = all.filter(n => !(n >= 1900 && n <= 2100 && Number.isInteger(n))) // skip lone years
  const pool = [totals, subtotals, paid, plausible].find(p => p.length) || []
  return pool.length ? Math.round(Math.max(...pool)) : null
}

/** OCR an image File/Blob and return { amount, text }. */
export async function readAmountFromImage(file, onProgress) {
  const T = await loadTesseract()
  const { data } = await T.recognize(file, 'eng', {
    logger: m => { if (onProgress && m.status === 'recognizing text') onProgress(m.progress) },
  })
  return { amount: pickTotal(data.text), text: data.text }
}
