import React, { useEffect } from 'react'
import { AlertTriangle, CheckCircle2, X } from 'lucide-react'

export interface ToastMessage {
  id: number
  message: string
  type: 'success' | 'error'
}

interface ToastBannerProps {
  toast: ToastMessage | null
  onClose: () => void
}

/**
 * Accessible live notification banner for mutation feedback (`role="alert"` / `role="status"`).
 */
export const ToastBanner: React.FC<ToastBannerProps> = ({ toast, onClose }) => {
  useEffect(() => {
    if (!toast) return
    const timer = window.setTimeout(onClose, 4500)
    return () => window.clearTimeout(timer)
  }, [toast, onClose])

  if (!toast) return null

  const isError = toast.type === 'error'

  return (
    <div
      role={isError ? 'alert' : 'status'}
      aria-live={isError ? 'assertive' : 'polite'}
      className={`fixed top-20 right-4 z-50 max-w-sm w-[calc(100vw-2rem)] bui-card px-4 py-3 shadow-xl border flex items-start gap-2.5 text-xs ${
        isError ? 'bui-badge-rose' : 'bui-badge-lime'
      }`}
    >
      {isError ? (
        <AlertTriangle className="w-4 h-4 shrink-0 mt-0.5" aria-hidden="true" />
      ) : (
        <CheckCircle2 className="w-4 h-4 shrink-0 mt-0.5" aria-hidden="true" />
      )}
      <span className="flex-1 font-medium leading-relaxed">{toast.message}</span>
      <button
        type="button"
        onClick={onClose}
        className="p-1 rounded-md hover:bg-black/10 dark:hover:bg-white/10 transition shrink-0"
        aria-label="Fechar notificação"
      >
        <X className="w-3.5 h-3.5" aria-hidden="true" />
      </button>
    </div>
  )
}
