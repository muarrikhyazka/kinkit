import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { IconClose } from './icons.jsx'

const ToastCtx = createContext(null)
let seq = 0

/**
 * App-wide toasts.
 *   const toast = useToast()
 *   toast.success('Saved')               // auto-dismiss
 *   toast.error('Something broke')       // stays a bit longer
 *   toast.info('Queued offline')
 *   toast.show({ message, type, duration, action: { label, onClick } })
 *   toast.undoable({ message, commit, onUndo, onCommitted, onError, delay })
 *
 * undoable(): "deferred delete". Nothing is sent until `delay` (5s) passes;
 * clicking Undo cancels it and calls onUndo. Callers hide the item locally
 * meanwhile. Pending commits are flushed if the page is closed.
 */
export function ToastProvider({ children }) {
  const [toasts, setToasts] = useState([])
  const pending = useRef(new Map()) // id -> { timer, run }

  const dismiss = useCallback(id => setToasts(ts => ts.filter(t => t.id !== id)), [])

  const show = useCallback(({ message, type = 'info', duration = 3500, action }) => {
    const id = ++seq
    setToasts(ts => [...ts.slice(-4), { id, message, type, action, duration }])
    if (duration) setTimeout(() => dismiss(id), duration)
    return id
  }, [dismiss])

  const api = useMemo(() => {
    const success = message => show({ message, type: 'success' })
    const error = message => show({ message, type: 'error', duration: 7000 })
    const info = message => show({ message, type: 'info', duration: 5000 })

    function undoable({ message, commit, onUndo, onCommitted, onError, delay = 5000 }) {
      const id = ++seq
      const run = async () => {
        pending.current.delete(id)
        dismiss(id)
        try {
          await commit()
          onCommitted && onCommitted()
        } catch (e) {
          if (onError) onError(e)
          else { onUndo && onUndo(); error(e.message) }
        }
      }
      const timer = setTimeout(run, delay)
      pending.current.set(id, { timer, run })
      const undo = () => {
        clearTimeout(timer)
        pending.current.delete(id)
        dismiss(id)
        onUndo && onUndo()
      }
      setToasts(ts => [...ts.slice(-4), { id, message, type: 'undo', duration: delay, action: { label: 'Undo', onClick: undo } }])
      return id
    }

    return { show, success, error, info, undoable, dismiss }
  }, [show, dismiss])

  // Leaving the page shouldn't silently cancel a delete the user asked for.
  useEffect(() => {
    const flush = () => pending.current.forEach(p => { clearTimeout(p.timer); p.run() })
    window.addEventListener('pagehide', flush)
    return () => window.removeEventListener('pagehide', flush)
  }, [])

  return (
    <ToastCtx.Provider value={api}>
      {children}
      <div className="toast-region" role="status" aria-live="polite">
        {toasts.map(t => (
          <div key={t.id} className={`toast toast-${t.type}`}>
            <span className="toast-message">{t.message}</span>
            {t.action && (
              <button className="toast-action" onClick={() => { t.action.onClick(); if (t.type !== 'undo') dismiss(t.id) }}>
                {t.action.label}
              </button>
            )}
            {t.type !== 'undo' && (
              <button className="icon-btn toast-close" onClick={() => dismiss(t.id)} aria-label="Dismiss notification">
                <IconClose width={12} height={12} />
              </button>
            )}
            {t.type === 'undo' && <span className="toast-timer" style={{ animationDuration: `${t.duration}ms` }} />}
          </div>
        ))}
      </div>
    </ToastCtx.Provider>
  )
}

export function useToast() {
  return useContext(ToastCtx)
}

/**
 * Helper for lists with undo-able deletes: keeps a set of ids hidden locally
 * while the deferred delete is pending.
 *   const [hidden, hide, unhide] = useHiddenIds()
 */
export function useHiddenIds() {
  const [hidden, setHidden] = useState(() => new Set())
  const hide = useCallback(id => setHidden(s => new Set(s).add(id)), [])
  const unhide = useCallback(id => setHidden(s => { const n = new Set(s); n.delete(id); return n }), [])
  return [hidden, hide, unhide]
}
