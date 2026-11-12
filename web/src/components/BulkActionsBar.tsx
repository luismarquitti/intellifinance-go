import React, { useState } from 'react'
import { Trash2, Tag, X, Check } from 'lucide-react'
import { Category } from '../types'

interface BulkActionsBarProps {
  selectedCount: number
  categories: Category[]
  onClearSelection: () => void
  onBatchDelete: () => void
  onBatchCategory: (categoryId: string | null) => void
}

export const BulkActionsBar: React.FC<BulkActionsBarProps> = ({
  selectedCount,
  categories,
  onClearSelection,
  onBatchDelete,
  onBatchCategory,
}) => {
  const [showCategoryMenu, setShowCategoryMenu] = useState(false)

  if (selectedCount === 0) return null

  return (
    <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-40 bui-card px-5 py-3 shadow-2xl border border-[var(--bui-accent-blue)] flex items-center gap-4">
      <div className="flex items-center gap-2 pr-3 border-r border-[var(--bui-border)] text-xs font-bold text-[var(--bui-accent-blue)]">
        <Check className="w-4 h-4" />
        <span>
          {selectedCount} selecionada{selectedCount > 1 ? 's' : ''}
        </span>
      </div>

      <div className="flex items-center gap-2 relative">
        <div className="relative">
          <button
            onClick={() => setShowCategoryMenu(!showCategoryMenu)}
            className="bui-pill-control px-3 py-1.5 text-xs font-semibold flex items-center gap-1.5"
          >
            <Tag className="w-3.5 h-3.5 text-[var(--bui-accent-blue)]" />
            <span>Definir Categoria</span>
          </button>

          {showCategoryMenu && (
            <div className="absolute bottom-full mb-2 left-0 w-56 max-h-64 overflow-y-auto bui-card shadow-xl p-1.5 z-50 border border-[var(--bui-border-strong)]">
              <button
                onClick={() => {
                  onBatchCategory(null)
                  setShowCategoryMenu(false)
                }}
                className="w-full text-left px-3 py-1.5 rounded-lg text-xs text-[var(--bui-text-muted)] hover:bg-[var(--bui-bg-hover)] hover:text-[var(--bui-text-primary)] transition"
              >
                Remover Categoria
              </button>
              {categories.map((c) => (
                <button
                  key={c.id}
                  onClick={() => {
                    onBatchCategory(c.id)
                    setShowCategoryMenu(false)
                  }}
                  className="w-full text-left px-3 py-1.5 rounded-lg text-xs text-[var(--bui-text-primary)] hover:bg-[var(--bui-bg-hover)] transition flex items-center gap-2"
                >
                  <span
                    className="w-2 h-2 rounded-full"
                    style={{ backgroundColor: c.color_hex }}
                  />
                  <span>{c.name}</span>
                </button>
              ))}
            </div>
          )}
        </div>

        <button
          onClick={onBatchDelete}
          className="px-3 py-1.5 rounded-xl bui-badge-rose text-xs font-semibold flex items-center gap-1.5 transition"
        >
          <Trash2 className="w-3.5 h-3.5" />
          <span>Excluir</span>
        </button>

        <button
          onClick={onClearSelection}
          className="bui-pill-control p-1.5 ml-1"
          title="Desmarcar todas"
        >
          <X className="w-4 h-4" />
        </button>
      </div>
    </div>
  )
}
