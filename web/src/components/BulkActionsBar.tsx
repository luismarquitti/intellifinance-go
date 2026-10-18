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
    <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-40 bg-[#161D18] border border-[#A5D6A7]/40 rounded-2xl px-5 py-3 shadow-2xl shadow-green-950/60 flex items-center gap-4 animate-in fade-in slide-in-from-bottom-4 duration-200">
      <div className="flex items-center gap-2 pr-3 border-r border-[#2C362F] text-xs font-semibold text-[#A5D6A7]">
        <Check className="w-4 h-4" />
        <span>{selectedCount} selecionada{selectedCount > 1 ? 's' : ''}</span>
      </div>

      <div className="flex items-center gap-2 relative">
        {/* Bulk Category Selector */}
        <div className="relative">
          <button
            onClick={() => setShowCategoryMenu(!showCategoryMenu)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-[#1F2721] hover:bg-[#28332B] text-xs font-medium text-[#E8EFEA] border border-[#2C362F] transition-colors"
          >
            <Tag className="w-3.5 h-3.5 text-[#A5D6A7]" />
            <span>Definir Categoria</span>
          </button>

          {showCategoryMenu && (
            <div className="absolute bottom-full mb-2 left-0 w-56 max-h-64 overflow-y-auto rounded-xl bg-[#1F2721] border border-[#2C362F] shadow-xl p-1 z-50">
              <button
                onClick={() => {
                  onBatchCategory(null)
                  setShowCategoryMenu(false)
                }}
                className="w-full text-left px-3 py-1.5 rounded-lg text-xs text-[#8E9E92] hover:bg-[#28332B] hover:text-[#E8EFEA] transition-colors"
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
                  className="w-full text-left px-3 py-1.5 rounded-lg text-xs text-[#E8EFEA] hover:bg-[#28332B] transition-colors flex items-center gap-2"
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

        {/* Bulk Delete Button */}
        <button
          onClick={onBatchDelete}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 text-xs font-medium border border-rose-500/20 transition-colors"
        >
          <Trash2 className="w-3.5 h-3.5" />
          <span>Excluir</span>
        </button>

        {/* Clear selection */}
        <button
          onClick={onClearSelection}
          className="p-1 rounded-lg text-[#8E9E92] hover:text-[#E8EFEA] transition-colors ml-1"
          title="Desmarcar todas"
        >
          <X className="w-4 h-4" />
        </button>
      </div>
    </div>
  )
}
