/**
 * Key-number card.
 *   <StatCard label="Total Income" value={formatRupiah(x)} tone="income" sub="vs last month +4%" />
 * tone: income | expense | savings | positive | negative | (none = primary)
 */
export default function StatCard({ label, value, tone, sub }) {
  return (
    <div className={`card stat ${tone ? `stat-${tone}` : ''}`}>
      <div className="muted">{label}</div>
      <div className="stat-value">{value}</div>
      {sub && <div className="stat-sub">{sub}</div>}
    </div>
  )
}
