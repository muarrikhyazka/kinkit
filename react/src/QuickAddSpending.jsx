import { useState } from 'react'
import Modal from './Modal.jsx'
import SpendingForm from './SpendingForm.jsx'
import { IncomeForm } from './pages/Income.jsx'

/**
 * Quick-add modal (sidebar button / mobile "+" FAB). Spending by default;
 * owners/parents can switch to Income. Closes once everything is saved or queued.
 */
export default function QuickAddSpending({ onClose, onSaved, allowIncome = false }) {
  const [kind, setKind] = useState('spending')
  return (
    <Modal title={kind === 'income' ? 'Add Income' : 'Add Spending'} onClose={onClose} className="quick-add">
      {allowIncome && (
        <div className="segmented" role="tablist" aria-label="What to add">
          {['spending', 'income'].map(k => (
            <button key={k} type="button" role="tab" aria-selected={kind === k}
              className={`segmented-btn ${kind === k ? 'active' : ''}`} onClick={() => setKind(k)}>
              {k === 'spending' ? 'Spending' : 'Income'}
            </button>
          ))}
        </div>
      )}
      {kind === 'spending' ? (
        <SpendingForm
          autoFocus
          onSaved={result => {
            onSaved && onSaved(result)
            if (!result.failed) onClose()
          }}
        />
      ) : (
        <IncomeForm stacked autoFocus onSaved={() => onClose()} />
      )}
    </Modal>
  )
}
