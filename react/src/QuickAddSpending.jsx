import Modal from './Modal.jsx'
import SpendingForm from './SpendingForm.jsx'

/** Quick-add modal (sidebar button / mobile "+" FAB). Closes once everything saved or queued. */
export default function QuickAddSpending({ onClose, onSaved }) {
  return (
    <Modal title="Add Spending" onClose={onClose} className="quick-add">
      <SpendingForm
        autoFocus
        onSaved={result => {
          onSaved && onSaved(result)
          if (!result.failed) onClose()
        }}
      />
    </Modal>
  )
}
