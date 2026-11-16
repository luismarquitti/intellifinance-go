import React, { useId } from 'react'
import { Trash2, X } from 'lucide-react'
import { ModalDialog } from './ModalDialog'

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
  const titleId = useId()

  return (
    <ModalDialog
      isOpen={isOpen}
      onClose={onClose}
      labelledBy={titleId}
      maxWidthClass="max-w-md"
    >
      <div className="bui-card w-full overflow-hidden shadow-2xl p-6 border border-[var(--bui-border-strong)]">
        <div className="flex items-start justify-between">
          <div className="w-11 h-11 rounded-2xl bui-badge-rose flex items-center justify-center">
            <Trash2 className="w-5 h-5" aria-hidden="true" />
          </div>
          <button
            type="button"
            onClick={onClose}
            className="bui-pill-control p-1.5"
            aria-label="Fechar modal"
          >
            <X className="w-4 h-4" aria-hidden="true" />
          </button>
        </div>

        <div className="mt-4">
          <h2 id={titleId} className="text-base font-bold text-[var(--bui-text-primary)]">
            {count > 1 ? `Excluir ${count} transações?` : 'Excluir transação?'}
          </h2>
          <p className="text-xs text-[var(--bui-text-secondary)] mt-1.5 leading-relaxed">
            {count > 1 ? (
              `As ${count} transações selecionadas serão removidas permanentemente e o saldo das respectivas contas será recalculado.`
            ) : (
              <>
                A transação{' '}
                <span className="font-semibold text-[var(--bui-text-primary)]">
                  &quot;{description}&quot;
                </span>{' '}
                será removida e o saldo da conta será recalculado.
              </>
            )}
          </p>
        </div>

        <div className="mt-6 flex items-center justify-end gap-2.5">
          {/* Destructive confirmation: focus starts on the safe action */}
          <button
            type="button"
            autoFocus
            onClick={onClose}
            className="bui-pill-control px-4 py-2 text-xs font-semibold"
          >
            Cancelar
          </button>
          <button
            type="button"
            disabled={isDeleting}
            onClick={onConfirm}
            className="px-5 py-2 rounded-xl bg-[var(--bui-rose)] hover:opacity-90 text-white font-bold text-xs shadow-sm transition disabled:opacity-40"
          >
            {isDeleting ? 'Excluindo...' : 'Confirmar Exclusão'}
          </button>
        </div>
      </div>
    </ModalDialog>
  )
}
