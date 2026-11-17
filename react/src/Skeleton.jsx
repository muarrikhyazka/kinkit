/**
 * Shimmer placeholders shown while data loads.
 *   <Skeleton width="60%" height={14} />        one bar
 *   <SkeletonStats count={3} />                  a row of stat cards
 *   <SkeletonCard lines={4} chart />             a card with title + optional chart block
 *   <SkeletonTable rows={5} cols={4} />          table-ish rows
 */
export function Skeleton({ width = '100%', height = 14, radius = 6, style, className = '' }) {
  return <span className={`skeleton ${className}`} style={{ width, height, borderRadius: radius, ...style }} aria-hidden="true" />
}

export function SkeletonStats({ count = 3 }) {
  return (
    <div className="cards-row" aria-busy="true" aria-label="Loading">
      {Array.from({ length: count }, (_, i) => (
        <div className="card stat" key={i}>
          <Skeleton width="50%" height={12} />
          <Skeleton width="80%" height={24} style={{ marginTop: '.6rem' }} />
        </div>
      ))}
    </div>
  )
}

export function SkeletonCard({ lines = 3, chart = false }) {
  return (
    <div className="card" aria-busy="true" aria-label="Loading">
      <Skeleton width="35%" height={16} style={{ marginBottom: '1rem' }} />
      {chart && <Skeleton height={200} radius={10} style={{ marginBottom: '.8rem' }} />}
      {Array.from({ length: lines }, (_, i) => (
        <Skeleton key={i} width={`${90 - (i % 3) * 15}%`} style={{ marginBottom: '.6rem' }} />
      ))}
    </div>
  )
}

export function SkeletonTable({ rows = 5, cols = 4 }) {
  return (
    <div className="skeleton-table" aria-busy="true" aria-label="Loading">
      {Array.from({ length: rows }, (_, r) => (
        <div className="skeleton-row" key={r} style={{ gridTemplateColumns: `repeat(${cols}, 1fr)` }}>
          {Array.from({ length: cols }, (_, c) => <Skeleton key={c} width={`${60 + ((r + c) % 4) * 10}%`} />)}
        </div>
      ))}
    </div>
  )
}

/** Full-page skeleton (initial auth check, simple pages). */
export function SkeletonPage() {
  return (
    <div className="skeleton-page">
      <Skeleton width={180} height={26} style={{ marginBottom: '1.25rem' }} />
      <SkeletonStats count={3} />
      <SkeletonCard lines={4} chart />
    </div>
  )
}
