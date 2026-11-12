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

  const accountMap = new Map(accounts.map((a) => [a.id, a]))
  const categoryMap = new Map(categories.map((c) => [c.id, c]))

  const handleSetCurrentMonth = () => {
    const now = new Date()
    const firstDay = new Date(now.getFullYear(), now.getMonth(), 1)
      .toISOString()
      .split('T')[0]
    const lastDay = new Date(now.getFullYear(), now.getMonth() + 1, 0)
      .toISOString()
      .split('T')[0]
    onFilterChange({ from_date: firstDay, to_date: lastDay, page: 1 })
  }

  return (
    <div className="bui-card p-4 space-y-3">
      <div className="flex flex-col lg:flex-row gap-2.5">
        {/* Search input */}
        <div className="relative flex-1">
          <Search className="w-4 h-4 text-[var(--bui-text-muted)] absolute left-3.5 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            placeholder="Buscar por estabelecimento, descrição ou notas..."
            value={filters.search || ''}
            onChange={(e) => onFilterChange({ search: e.target.value, page: 1 })}
            className="w-full pl-10 pr-9 py-2 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] placeholder-[var(--bui-text-muted)] focus:outline-none focus:border-[var(--bui-accent-blue)] transition"
          />
          {filters.search && (
            <button
              onClick={() => onFilterChange({ search: '', page: 1 })}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--bui-text-muted)] hover:text-[var(--bui-text-primary)]"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>

        {/* Dropdowns row */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
          {/* Account Filter */}
          <select
            value={filters.account_id || ''}
            onChange={(e) =>
              onFilterChange({ account_id: e.target.value || undefined, page: 1 })
            }
            className="py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-medium text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
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
            value={
              filters.uncategorized
                ? '__uncategorized__'
                : filters.category_id || ''
            }
            onChange={(e) => {
              const val = e.target.value
              if (val === '__uncategorized__') {
                onFilterChange({
                  uncategorized: true,
                  category_id: undefined,
                  page: 1,
                })
              } else {
                onFilterChange({
                  uncategorized: undefined,
                  category_id: val || undefined,
                  page: 1,
                })
              }
            }}
            className="py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-medium text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
          >
            <option value="">Todas as Categorias</option>
            <option value="__uncategorized__">⚠️ Sem Categoria</option>
            {categories.map((cat) => (
              <option key={cat.id} value={cat.id}>
                {cat.name}
              </option>
            ))}
          </select>

          {/* Type Filter */}
          <select
            value={filters.type || ''}
            onChange={(e) =>
              onFilterChange({ type: e.target.value || undefined, page: 1 })
            }
            className="py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-medium text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
          >
            <option value="">Todos os Tipos</option>
            <option value="INCOME">Receitas (+)</option>
            <option value="EXPENSE">Despesas (-)</option>
            <option value="TRANSFER">Transferências</option>
          </select>

          {/* Current Month Shortcut */}
          <button
            onClick={handleSetCurrentMonth}
            className="bui-pill-control py-2 px-3 text-xs font-medium flex items-center justify-center gap-1.5"
            title="Filtrar pelo mês atual"
          >
            <Calendar className="w-3.5 h-3.5 text-[var(--bui-accent-blue)]" />
            <span>Mês Atual</span>
          </button>
        </div>
      </div>

      {/* Date Range & Active Filter Chips Bar */}
      <div className="pt-2.5 border-t border-[var(--bui-border)] flex flex-wrap items-center justify-between gap-2 text-xs">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-[var(--bui-text-muted)] flex items-center gap-1 font-medium">
            <Filter className="w-3 h-3" /> Período:
          </span>
          <input
            type="date"
            value={filters.from_date || ''}
            onChange={(e) =>
              onFilterChange({ from_date: e.target.value || undefined, page: 1 })
            }
            className="py-1 px-2 rounded-lg bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-mono-num text-[var(--bui-text-primary)]"
          />
          <span className="text-[var(--bui-text-muted)]">até</span>
          <input
            type="date"
            value={filters.to_date || ''}
            onChange={(e) =>
              onFilterChange({ to_date: e.target.value || undefined, page: 1 })
            }
            className="py-1 px-2 rounded-lg bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-mono-num text-[var(--bui-text-primary)]"
          />
        </div>

        {/* Active Filter Chips (1-click removal) */}
        <div className="flex items-center gap-1.5 flex-wrap">
          {filters.account_id && (
            <button
              onClick={() => onFilterChange({ account_id: undefined, page: 1 })}
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bui-badge-blue text-[11px] font-semibold"
            >
              <span>Conta: {accountMap.get(filters.account_id)?.name || 'Selecionada'}</span>
              <X className="w-3 h-3" />
            </button>
          )}
          {filters.category_id && (
            <button
              onClick={() => onFilterChange({ category_id: undefined, page: 1 })}
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bui-badge-lime text-[11px] font-semibold"
            >
              <span>
                Categoria: {categoryMap.get(filters.category_id)?.name || 'Selecionada'}
              </span>
              <X className="w-3 h-3" />
            </button>
          )}
          {filters.uncategorized && (
            <button
              onClick={() => onFilterChange({ uncategorized: undefined, page: 1 })}
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bui-badge-amber text-[11px] font-semibold"
            >
              <span>Sem Categoria</span>
              <X className="w-3 h-3" />
            </button>
          )}
          {filters.type && (
            <button
              onClick={() => onFilterChange({ type: undefined, page: 1 })}
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bui-pill-control text-[11px] font-semibold"
            >
              <span>Tipo: {filters.type}</span>
              <X className="w-3 h-3" />
            </button>
          )}
          {filters.search && (
            <button
              onClick={() => onFilterChange({ search: undefined, page: 1 })}
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bui-pill-control text-[11px] font-semibold"
            >
              <span>Busca: "{filters.search}"</span>
              <X className="w-3 h-3" />
            </button>
          )}

          {hasActiveFilters && (
            <button
              onClick={onResetFilters}
              className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-md bui-badge-rose text-[11px] font-semibold"
            >
              <RotateCcw className="w-3 h-3" />
              <span>Limpar Tudo</span>
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
