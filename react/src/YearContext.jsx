import { createContext, useContext, useState } from 'react'

const YearContext = createContext(null)

// Periods are UTC everywhere (the app is used worldwide; the backend buckets
// by UTC date), so the initial "current" month is the UTC one.
export function currentUTCPeriod() {
  const now = new Date()
  return { year: now.getUTCFullYear(), month: now.getUTCMonth() + 1 }
}

export function YearProvider({ children }) {
  const [year, setYear] = useState(() => currentUTCPeriod().year)
  const [month, setMonth] = useState(() => currentUTCPeriod().month)
  const [valueMax, setValueMax] = useState('all') // 'all' | '10jt' | '100jt'
  return (
    <YearContext.Provider value={{ year, setYear, month, setMonth, valueMax, setValueMax }}>
      {children}
    </YearContext.Provider>
  )
}

export function useYearMonth() {
  return useContext(YearContext)
}
