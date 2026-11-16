import React, { useEffect, useId, useState } from 'react'
import { Search, X, RotateCcw, Filter } from 'lucide-react'
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
  const [searchDraft, setSearchDraft] = useState(filters.search || '')
  const uid = useId()
  const searchId = `${uid}-search`
  const accountSelectId = `${uid}-account`
  const categorySelectId = `${uid}-category`
  const typeSelectId = `${uid}-type`
  const fromDateId = `${uid}-from-date`
  const toDateId = `${uid}-to-date`

  // Keep local search draft synchronized when filters are reset or cleared externally
  useEffect(() => {
    setSearchDraft(filters.search || '')
  }, [filters.search])

  // Debounce text search (300ms) to avoid firing 2 API requests on every keystroke
  useEffect(() => {
    const currentSearch = filters.search || ''
    if (searchDraft === currentSearch) return

    const timer = window.setTimeout(() => {
      onFilterChange({ search: searchDraft.trim() ? searchDraft : undefined, page: 1 })
    }, 300)

    return () => window.clearTimeout(timer)
  }, [searchDraft, filters.search, onFilterChange])

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

  return (
    <search aria-label="Filtros e busca de transações" className="block bui-card p-4 space-y-3">
      <div className="flex flex-col lg:flex-row gap-2.5">
        {/* Search input */}
        <div className="relative flex-1">
          <label htmlFor={searchId} className="sr-only">
            Buscar transações por estabelecimento, descrição ou notas
          </label>
          <Search
            className="w-4 h-4 text-[var(--bui-text-muted)] absolute left-3.5 top-1/2 -translate-y-1/2 pointer-events-none"
            aria-hidden="true"
          />
          <input
            id={searchId}
            type="search"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            placeholder="Buscar por estabelecimento, descrição ou notas..."
            value={searchDraft}
            onChange={(e) => setSearchDraft(e.target.value)}
            className="w-full pl-10 pr-9 py-2 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] placeholder-[var(--bui-text-muted)] focus:outline-none focus:border-[var(--bui-accent-blue)] transition"
          />
          {searchDraft && (
            <button
              type="button"
              onClick={() => {
                setSearchDraft('')
                onFilterChange({ search: undefined, page: 1 })
              }}
              aria-label="Limpar busca"
              className="absolute right-2.5 top-1/2 -translate-y-1/2 p-1 text-[var(--bui-text-muted)] hover:text-[var(--bui-text-primary)]"
            >
              <X className="w-3.5 h-3.5" aria-hidden="true" />
            </button>
          )}
        </div>

        {/* Dropdowns row */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
          {/* Account Filter */}
          <div>
            <label htmlFor={accountSelectId} className="sr-only">
              Filtrar por conta
            </label>
            <select
              id={accountSelectId}
              value={filters.account_id || ''}
              onChange={(e) =>
                onFilterChange({ account_id: e.target.value || undefined, page: 1 })
              }
              className="w-full py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-medium text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
            >
              <option value="">Todas as Contas</option>
              {accounts.map((acc) => (
                <option key={acc.id} value={acc.id}>
                  {acc.name}
                </option>
              ))}
            </select>
          </div>

          {/* Category Filter */}
          <div>
            <label htmlFor={categorySelectId} className="sr-only">
              Filtrar por categoria
            </label>
            <select
              id={categorySelectId}
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
              className="w-full py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-medium text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
            >
              <option value="">Todas as Categorias</option>
              <option value="__uncategorized__">Sem Categoria (Pendente)</option>
              {categories.map((cat) => (
                <option key={cat.id} value={cat.id}>
                  {cat.name}
                </option>
              ))}
            </select>
          </div>

          {/* Type Filter */}
          <div>
            <label htmlFor={typeSelectId} className="sr-only">
              Filtrar por tipo de transação
            </label>
            <select
              id={typeSelectId}
              value={filters.type || ''}
              onChange={(e) =>
                onFilterChange({ type: e.target.value || undefined, page: 1 })
              }
              className="w-full py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-medium text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
            >
              <option value="">Todos os Tipos</option>
              <option value="INCOME">Receitas (+)</option>
              <option value="EXPENSE">Despesas (-)</option>
              <option value="TRANSFER">Transferências</option>
            </select>
          </div>
        </div>
      </div>

      {/* Date Range & Active Filter Chips Bar */}
      <div className="pt-2.5 border-t border-[var(--bui-border)] flex flex-wrap items-center justify-between gap-2.5 text-xs">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-[var(--bui-text-muted)] flex items-center gap-1 font-medium">
            <Filter className="w-3.5 h-3.5" aria-hidden="true" /> Período:
          </span>
          <label htmlFor={fromDateId} className="sr-only">
            Data inicial
          </label>
          <input
            id={fromDateId}
            type="date"
            value={filters.from_date || ''}
            onChange={(e) =>
              onFilterChange({ from_date: e.target.value || undefined, page: 1 })
            }
            className="py-1 px-2.5 rounded-lg bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-mono-num text-[var(--bui-text-primary)]"
          />
          <span className="text-[var(--bui-text-muted)]">até</span>
          <label htmlFor={toDateId} className="sr-only">
            Data final
          </label>
          <input
            id={toDateId}
            type="date"
            value={filters.to_date || ''}
            onChange={(e) =>
              onFilterChange({ to_date: e.target.value || undefined, page: 1 })
            }
            className="py-1 px-2.5 rounded-lg bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-mono-num text-[var(--bui-text-primary)]"
          />
        </div>

        {/* Active Filter Chips (1-click removal) */}
        <div className="flex items-center gap-1.5 flex-wrap">
          {filters.account_id && (
            <button
              type="button"
              onClick={() => onFilterChange({ account_id: undefined, page: 1 })}
              className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md bui-badge-blue text-[11px] font-semibold"
            >
              <span>Conta: {accountMap.get(filters.account_id)?.name || 'Selecionada'}</span>
              <X className="w-3 h-3" aria-hidden="true" />
            </button>
          )}
          {filters.category_id && (
            <button
              type="button"
              onClick={() => onFilterChange({ category_id: undefined, page: 1 })}
              className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md bui-badge-lime text-[11px] font-semibold"
            >
              <span>
                Categoria: {categoryMap.get(filters.category_id)?.name || 'Selecionada'}
              </span>
              <X className="w-3 h-3" aria-hidden="true" />
            </button>
          )}
          {filters.uncategorized && (
            <button
              type="button"
              onClick={() => onFilterChange({ uncategorized: undefined, page: 1 })}
              className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md bui-badge-amber text-[11px] font-semibold"
            >
              <span>Sem Categoria</span>
              <X className="w-3 h-3" aria-hidden="true" />
            </button>
          )}
          {filters.type && (
            <button
              type="button"
              onClick={() => onFilterChange({ type: undefined, page: 1 })}
              className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md bui-pill-control text-[11px] font-semibold"
            >
              <span>Tipo: {filters.type}</span>
              <X className="w-3 h-3" aria-hidden="true" />
            </button>
          )}
          {filters.search && (
            <button
              type="button"
              onClick={() => {
                setSearchDraft('')
                onFilterChange({ search: undefined, page: 1 })
              }}
              className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md bui-pill-control text-[11px] font-semibold"
            >
              <span>Busca: &quot;{filters.search}&quot;</span>
              <X className="w-3 h-3" aria-hidden="true" />
            </button>
          )}

          {hasActiveFilters && (
            <button
              type="button"
              onClick={() => {
                setSearchDraft('')
                onResetFilters()
              }}
              className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md bui-badge-rose text-[11px] font-semibold"
            >
              <RotateCcw className="w-3 h-3" aria-hidden="true" />
              <span>Limpar Tudo</span>
            </button>
          )}
        </div>
      </div>
    </search>
  )
}
