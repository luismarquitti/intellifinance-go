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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-[#161D18] border border-[#2C362F] rounded-3xl max-w-md w-full overflow-hidden shadow-2xl p-6 animate-in zoom-in-95 duration-200">
        <div className="flex items-start justify-between">
          <div className="w-12 h-12 rounded-2xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-400">
            <Trash2 className="w-6 h-6" />
          </div>
          <button
            onClick={onClose}
            className="p-1 rounded-xl text-[#8E9E92] hover:text-[#E8EFEA] hover:bg-[#1F2721] transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="mt-4">
          <h3 className="text-base font-bold text-[#E8EFEA]">
            {count > 1 ? `Excluir ${count} transações?` : 'Excluir transação?'}
          </h3>
          <p className="text-xs text-[#8E9E92] mt-1.5 leading-relaxed">
            {count > 1 ? (
              `As ${count} transações selecionadas serão removidas permanentemente e o saldo das respectivas contas será recalculado automaticamente.`
            ) : (
              <>
                A transação <span className="font-semibold text-[#E8EFEA]">"{description}"</span> será removida e o saldo da conta será recalculado.
              </>
            )}
          </p>
        </div>

        <div className="mt-6 flex items-center justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2.5 rounded-xl bg-[#1F2721] text-[#8E9E92] hover:text-[#E8EFEA] text-xs font-semibold transition-colors"
          >
            Cancelar
          </button>
          <button
            type="button"
            disabled={isDeleting}
            onClick={onConfirm}
            className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-rose-500 hover:bg-rose-600 text-white font-bold text-xs shadow-md transition-all disabled:opacity-40"
          >
            {isDeleting ? 'Excluindo...' : 'Confirmar Exclusão'}
          </button>
        </div>
      </div>
    </div>
  )
}
