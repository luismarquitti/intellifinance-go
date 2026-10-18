import React from 'react'
import { Search, X, RotateCcw, Calendar, Filter } from 'lucide-react'
import { Account, Category, TransactionFilters } from '../types'

interface FilterBarProps {
  accounts: Account[]
  categories: Category[]
  filters: TransactionFilters
  onFilterChange: (newFilters: Partial<TransactionFilters>) => void
  onResetFilters: () => void
}

export const FilterBar: React.FC<FilterBarProps> = ({
  accounts,
  categories,
  filters,
  onFilterChange,
  onResetFilters,
}) => {
  const hasActiveFilters = Boolean(
    filters.search ||
    filters.account_id ||
    filters.category_id ||
    filters.uncategorized ||
    filters.type ||
    filters.from_date ||
    filters.to_date
  )

  const handleSetCurrentMonth = () => {
    const now = new Date()
    const firstDay = new Date(now.getFullYear(), now.getMonth(), 1).toISOString().split('T')[0]
    const lastDay = new Date(now.getFullYear(), now.getMonth() + 1, 0).toISOString().split('T')[0]
    onFilterChange({ from_date: firstDay, to_date: lastDay })
  }

  return (
    <div className="bg-[#161D18] border border-[#2C362F] rounded-2xl p-4 shadow-sm">
      <div className="flex flex-col lg:flex-row gap-3">
        {/* Search input */}
        <div className="relative flex-1">
          <Search className="w-4 h-4 text-[#8E9E92] absolute left-3.5 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            placeholder="Buscar por descrição, notas..."
            value={filters.search || ''}
            onChange={(e) => onFilterChange({ search: e.target.value, page: 1 })}
            className="w-full pl-10 pr-9 py-2.5 rounded-xl bg-[#1F2721] border border-[#2C362F] text-sm text-[#E8EFEA] placeholder-[#8E9E92] focus:outline-none focus:border-[#A5D6A7] transition-colors"
          />
          {filters.search && (
            <button
              onClick={() => onFilterChange({ search: '', page: 1 })}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-[#8E9E92] hover:text-[#E8EFEA]"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>

        {/* Dropdowns row */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-2.5 flex-wrap">
          {/* Account Filter */}
          <select
            value={filters.account_id || ''}
            onChange={(e) => onFilterChange({ account_id: e.target.value || undefined, page: 1 })}
            className="py-2.5 px-3 rounded-xl bg-[#1F2721] border border-[#2C362F] text-xs font-medium text-[#E8EFEA] focus:outline-none focus:border-[#A5D6A7] transition-colors"
          >
            <option value="">Todas as Contas</option>
            {accounts.map((acc) => (
              <option key={acc.id} value={acc.id}>
                {acc.name}
              </option>
            ))}
          </select>

          {/* Category Filter */}
          <select
            value={filters.uncategorized ? '__uncategorized__' : (filters.category_id || '')}
            onChange={(e) => {
              const val = e.target.value
              if (val === '__uncategorized__') {
                onFilterChange({ uncategorized: true, category_id: undefined, page: 1 })
              } else {
                onFilterChange({ uncategorized: undefined, category_id: val || undefined, page: 1 })
              }
            }}
            className="py-2.5 px-3 rounded-xl bg-[#1F2721] border border-[#2C362F] text-xs font-medium text-[#E8EFEA] focus:outline-none focus:border-[#A5D6A7] transition-colors"
          >
            <option value="">Todas as Categorias</option>
            <option value="__uncategorized__">⚠️ Sem Categoria</option>
            {categories.map((cat) => (
              <option key={cat.id} value={cat.id}>
                {cat.name} ({cat.type === 'INCOME' ? 'Receita' : 'Despesa'})
              </option>
            ))}
          </select>

          {/* Type Filter */}
          <select
            value={filters.type || ''}
            onChange={(e) => onFilterChange({ type: e.target.value || undefined, page: 1 })}
            className="py-2.5 px-3 rounded-xl bg-[#1F2721] border border-[#2C362F] text-xs font-medium text-[#E8EFEA] focus:outline-none focus:border-[#A5D6A7] transition-colors"
          >
            <option value="">Todos os Tipos</option>
            <option value="INCOME">Receitas</option>
            <option value="EXPENSE">Despesas</option>
            <option value="TRANSFER">Transferências</option>
          </select>

          {/* Date range trigger / Current Month shortcut */}
          <button
            onClick={handleSetCurrentMonth}
            className="flex items-center justify-center gap-1.5 py-2 px-3 rounded-xl bg-[#1F2721] border border-[#2C362F] text-xs text-[#A3B4A7] hover:text-[#A5D6A7] hover:border-[#A5D6A7]/40 transition-colors"
            title="Filtrar pelo mês atual"
          >
            <Calendar className="w-3.5 h-3.5 text-[#A5D6A7]" />
            <span>Este Mês</span>
          </button>
        </div>
      </div>

      {/* Date Pickers & Reset Row */}
      <div className="mt-3 pt-3 border-t border-[#2C362F]/60 flex flex-wrap items-center justify-between gap-3 text-xs">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-[#8E9E92] flex items-center gap-1 font-medium">
            <Filter className="w-3.5 h-3.5" /> Período:
          </span>
          <div className="flex items-center gap-1.5">
            <input
              type="date"
              value={filters.from_date || ''}
              onChange={(e) => onFilterChange({ from_date: e.target.value || undefined, page: 1 })}
              className="py-1 px-2.5 rounded-lg bg-[#1F2721] border border-[#2C362F] text-xs text-[#E8EFEA] focus:outline-none focus:border-[#A5D6A7]"
            />
            <span className="text-[#8E9E92]">até</span>
            <input
              type="date"
              value={filters.to_date || ''}
              onChange={(e) => onFilterChange({ to_date: e.target.value || undefined, page: 1 })}
              className="py-1 px-2.5 rounded-lg bg-[#1F2721] border border-[#2C362F] text-xs text-[#E8EFEA] focus:outline-none focus:border-[#A5D6A7]"
            />
          </div>
        </div>

        {hasActiveFilters && (
          <button
            onClick={onResetFilters}
            className="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-rose-500/10 text-rose-400 hover:bg-rose-500/20 border border-rose-500/20 transition-colors ml-auto font-medium"
          >
            <RotateCcw className="w-3.5 h-3.5" />
            <span>Limpar Filtros</span>
          </button>
        )}
      </div>
    </div>
  )
}
