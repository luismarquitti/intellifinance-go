import React from 'react'
import { Trash2, X } from 'lucide-react'

interface DeleteConfirmModalProps {
  isOpen: boolean
  count: number
  description?: string
  onClose: () => void
  onConfirm: () => void
  isDeleting?: boolean
}

export const DeleteConfirmModal: React.FC<DeleteConfirmModalProps> = ({
  isOpen,
  count,
  description,
  onClose,
  onConfirm,
  isDeleting,
}) => {
  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/65 backdrop-blur-sm">
      <div className="bui-card max-w-md w-full overflow-hidden shadow-2xl p-6 border border-[var(--bui-border-strong)]">
        <div className="flex items-start justify-between">
          <div className="w-11 h-11 rounded-2xl bui-badge-rose flex items-center justify-center">
            <Trash2 className="w-5 h-5" />
          </div>
          <button onClick={onClose} className="bui-pill-control p-1.5">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="mt-4">
          <h3 className="text-base font-bold text-[var(--bui-text-primary)]">
            {count > 1 ? `Excluir ${count} transações?` : 'Excluir transação?'}
          </h3>
          <p className="text-xs text-[var(--bui-text-secondary)] mt-1.5 leading-relaxed">
            {count > 1 ? (
              `As ${count} transações selecionadas serão removidas permanentemente e o saldo das respectivas contas será recalculado.`
            ) : (
              <>
                A transação{' '}
                <span className="font-semibold text-[var(--bui-text-primary)]">
                  "{description}"
                </span>{' '}
                será removida e o saldo da conta será recalculado.
              </>
            )}
          </p>
        </div>

        <div className="mt-6 flex items-center justify-end gap-2.5">
          <button
            type="button"
            onClick={onClose}
            className="bui-pill-control px-4 py-2 text-xs font-semibold"
          >
            Cancelar
          </button>
          <button
            type="button"
            disabled={isDeleting}
            onClick={onConfirm}
            className="px-5 py-2 rounded-xl bg-rose-600 hover:bg-rose-700 text-white font-bold text-xs shadow-sm transition disabled:opacity-40"
          >
            {isDeleting ? 'Excluindo...' : 'Confirmar Exclusão'}
          </button>
        </div>
      </div>
    </div>
  )
}
