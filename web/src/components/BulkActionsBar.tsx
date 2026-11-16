import React, { useEffect, useRef, useState } from 'react'
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
  const menuContainerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!showCategoryMenu) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setShowCategoryMenu(false)
      }
    }

    const handleClickOutside = (e: MouseEvent) => {
      if (
        menuContainerRef.current &&
        !menuContainerRef.current.contains(e.target as Node)
      ) {
        setShowCategoryMenu(false)
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    document.addEventListener('mousedown', handleClickOutside)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [showCategoryMenu])

  if (selectedCount === 0) return null

  return (
    <div
      role="region"
      aria-label="Ações em lote"
      className="fixed bottom-20 lg:bottom-6 left-1/2 -translate-x-1/2 z-40 w-[calc(100vw-2rem)] sm:w-auto bui-card px-4 py-2.5 shadow-2xl border border-[var(--bui-accent-blue)] flex flex-wrap items-center justify-between gap-2.5 sm:gap-4"
    >
      <div className="flex items-center gap-2 pr-3 sm:border-r border-[var(--bui-border)] text-xs font-bold text-[var(--bui-accent-blue)]">
        <Check className="w-4 h-4" aria-hidden="true" />
        <span>
          {selectedCount} selecionada{selectedCount > 1 ? 's' : ''}
        </span>
      </div>

      <div className="flex items-center gap-2 flex-wrap relative">
        <div className="relative" ref={menuContainerRef}>
          <button
            type="button"
            aria-expanded={showCategoryMenu}
            aria-haspopup="menu"
            onClick={() => setShowCategoryMenu(!showCategoryMenu)}
            className="bui-pill-control px-3 py-1.5 text-xs font-semibold flex items-center gap-1.5"
          >
            <Tag className="w-3.5 h-3.5 text-[var(--bui-accent-blue)]" aria-hidden="true" />
            <span>Definir Categoria</span>
          </button>

          {showCategoryMenu && (
            <div
              role="menu"
              className="absolute bottom-full mb-2 left-0 w-56 max-h-64 overflow-y-auto bui-card shadow-xl p-1.5 z-50 border border-[var(--bui-border-strong)]"
            >
              <button
                type="button"
                role="menuitem"
                onClick={() => {
                  onBatchCategory(null)
                  setShowCategoryMenu(false)
                }}
                className="w-full text-left px-3 py-2 rounded-lg text-xs text-[var(--bui-text-muted)] hover:bg-[var(--bui-bg-hover)] hover:text-[var(--bui-text-primary)] transition"
              >
                Remover Categoria
              </button>
              {categories.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    onBatchCategory(c.id)
                    setShowCategoryMenu(false)
                  }}
                  className="w-full text-left px-3 py-2 rounded-lg text-xs text-[var(--bui-text-primary)] hover:bg-[var(--bui-bg-hover)] transition flex items-center gap-2"
                >
                  <span
                    className="w-2.5 h-2.5 rounded-full shrink-0"
                    style={{ backgroundColor: c.color_hex }}
                    aria-hidden="true"
                  />
                  <span className="truncate">{c.name}</span>
                </button>
              ))}
            </div>
          )}
        </div>

        <button
          type="button"
          onClick={onBatchDelete}
          className="px-3 py-1.5 rounded-xl bui-badge-rose text-xs font-semibold flex items-center gap-1.5 transition"
        >
          <Trash2 className="w-3.5 h-3.5" aria-hidden="true" />
          <span>Excluir</span>
        </button>

        <button
          type="button"
          onClick={onClearSelection}
          className="bui-pill-control p-1.5"
          aria-label="Desmarcar todas as transações selecionadas"
          title="Desmarcar todas"
        >
          <X className="w-4 h-4" aria-hidden="true" />
        </button>
      </div>
    </div>
  )
}
