import React from 'react'
import { BarChart3, Filter, X } from 'lucide-react'
import { Category, Transaction } from '../types'
import { decimalToCents, formatCurrency } from '../utils/money'

interface CategoryBreakdownPanelProps {
  transactions: Transaction[]
  categories: Category[]
  selectedCategoryId?: string
  uncategorizedFilter?: boolean
  isPrivacyMode?: boolean
  onSelectCategory: (categoryId?: string, uncategorized?: boolean) => void
}

interface CategoryStat {
  id: string
  name: string
  colorHex: string
  isUncategorized?: boolean
  totalCents: bigint
  count: number
  percentage: number
}

export const CategoryBreakdownPanel: React.FC<CategoryBreakdownPanelProps> = ({
  transactions,
  categories,
  selectedCategoryId,
  uncategorizedFilter,
  isPrivacyMode = false,
  onSelectCategory,
}) => {
  const categoryMap = new Map(categories.map((c) => [c.id, c]))

  // Aggregate expenses in integer cents from loaded transactions
  const expenseMap = new Map<string, { cents: bigint; count: number }>()
  let totalExpenseCents = 0n

  for (const tx of transactions) {
    if (tx.type === 'TRANSFER') continue
    const cents = decimalToCents(tx.amount)
    if (tx.type === 'EXPENSE' || cents < 0n) {
      const absCents = cents < 0n ? -cents : cents
      totalExpenseCents += absCents
      const key = tx.category_id || '__uncategorized__'
      const prev = expenseMap.get(key) || { cents: 0n, count: 0 }
      expenseMap.set(key, { cents: prev.cents + absCents, count: prev.count + 1 })
    }
  }

  const stats: CategoryStat[] = Array.from(expenseMap.entries())
    .map(([key, val]) => {
      const isUncat = key === '__uncategorized__'
      const cat = isUncat ? undefined : categoryMap.get(key)
      const pct =
        totalExpenseCents > 0n
          ? Number((val.cents * 100n) / totalExpenseCents)
          : 0
      return {
        id: key,
        name: isUncat ? 'Sem Categoria (Pendente)' : cat?.name || 'Outros',
        colorHex: isUncat ? '#D97706' : cat?.color_hex || '#2563EB',
        isUncategorized: isUncat,
        totalCents: val.cents,
        count: val.count,
        percentage: Math.max(1, pct),
      }
    })
    .sort((a, b) => (b.totalCents > a.totalCents ? 1 : -1))
    .slice(0, 7)

  const maxCents = stats.length > 0 ? stats[0].totalCents : 1n

  return (
    <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
      {/* Left 7 Cols: Board UI Capsule-in-Track Vertical Bar Chart */}
      <div className="lg:col-span-7 bui-card p-5 flex flex-col justify-between">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <span className="text-xs text-[var(--bui-text-secondary)] font-medium block">
              Padrão de Gastos (Board UI Capsule Chart)
            </span>
            <h3 className="text-base font-bold text-[var(--bui-text-primary)] mt-0.5">
              Top Categorias de Despesa na Amostra
            </h3>
          </div>

          <div className="flex items-center gap-3 text-xs text-[var(--bui-text-secondary)]">
            <span className="flex items-center gap-1.5">
              <span className="w-2.5 h-2.5 rounded-sm bg-[var(--bui-lime-bar)] inline-block" />
              Categorizado
            </span>
            <span className="flex items-center gap-1.5">
              <span className="w-2.5 h-2.5 rounded-sm bg-[var(--bui-amber)] inline-block" />
              Sem Categoria
            </span>
          </div>
        </div>

        {stats.length === 0 ? (
          <div className="py-12 text-center text-xs text-[var(--bui-text-muted)]">
            Nenhuma despesa encontrada no filtro atual para compor o gráfico.
          </div>
        ) : (
          <div className="mt-6 pt-2 grid grid-cols-7 gap-3 items-end h-44">
            {stats.map((st, idx) => {
              const heightPct =
                maxCents > 0n
                  ? Math.max(14, Number((st.totalCents * 100n) / maxCents))
                  : 14
              const isSelected =
                (st.isUncategorized && uncategorizedFilter) ||
                (!st.isUncategorized && selectedCategoryId === st.id)

              return (
                <button
                  key={st.id}
                  onClick={() => {
                    if (st.isUncategorized) {
                      onSelectCategory(undefined, !uncategorizedFilter)
                    } else {
                      onSelectCategory(isSelected ? undefined : st.id, false)
                    }
                  }}
                  className="flex flex-col items-center gap-2 h-full justify-end group focus:outline-none"
                  title={`${st.name}: ${formatCurrency(st.totalCents, isPrivacyMode)} (${st.count} lançamentos)`}
                >
                  <span className="text-[10px] font-mono-num text-[var(--bui-text-secondary)] group-hover:text-[var(--bui-text-primary)] font-semibold">
                    {st.percentage}%
                  </span>
                  <div
                    className={`w-full max-w-[38px] h-32 bui-bar-track p-1 flex items-end transition ${
                      isSelected ? 'ring-2 ring-[var(--bui-accent-blue)]' : ''
                    }`}
                  >
                    <div
                      className={`w-full transition-all duration-300 ${
                        st.isUncategorized
                          ? 'bui-bar-fill-amber'
                          : idx % 2 === 0
                          ? 'bui-bar-fill-lime'
                          : 'bui-bar-fill-teal'
                      }`}
                      style={{ height: `${heightPct}%` }}
                    />
                  </div>
                  <span className="text-[11px] font-medium text-[var(--bui-text-secondary)] group-hover:text-[var(--bui-text-primary)] truncate w-full text-center">
                    {st.name.split(' ')[0]}
                  </span>
                </button>
              )
            })}
          </div>
        )}
      </div>

      {/* Right 5 Cols: Numerical Breakdown List (Mobills / Wallet Pattern) */}
      <div className="lg:col-span-5 bui-card p-5 flex flex-col justify-between">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <BarChart3 className="w-4 h-4 text-[var(--bui-accent-blue)]" />
            <h3 className="text-sm font-bold text-[var(--bui-text-primary)]">
              Detalhamento Numérico Equivalente
            </h3>
          </div>
          {(selectedCategoryId || uncategorizedFilter) && (
            <button
              onClick={() => onSelectCategory(undefined, false)}
              className="bui-pill-control px-2 py-1 text-[11px] font-semibold flex items-center gap-1"
            >
              <X className="w-3 h-3" /> Limpar
            </button>
          )}
        </div>

        <div className="my-3 space-y-2">
          {stats.length === 0 ? (
            <div className="py-8 text-center text-xs text-[var(--bui-text-muted)]">
              Sem dados de categorias para exibir.
            </div>
          ) : (
            stats.slice(0, 5).map((st, idx) => {
              const isSelected =
                (st.isUncategorized && uncategorizedFilter) ||
                (!st.isUncategorized && selectedCategoryId === st.id)

              return (
                <button
                  key={st.id}
                  onClick={() => {
                    if (st.isUncategorized) {
                      onSelectCategory(undefined, !uncategorizedFilter)
                    } else {
                      onSelectCategory(isSelected ? undefined : st.id, false)
                    }
                  }}
                  className={`w-full bui-inner p-2.5 flex flex-col gap-1.5 text-left transition ${
                    isSelected
                      ? 'ring-2 ring-[var(--bui-accent-blue)]'
                      : 'hover:border-[var(--bui-border-strong)]'
                  }`}
                >
                  <div className="flex items-center justify-between text-xs">
                    <div className="flex items-center gap-2 min-w-0">
                      <span
                        className="w-2.5 h-2.5 rounded-full shrink-0"
                        style={{ backgroundColor: st.colorHex }}
                      />
                      <span className="font-semibold text-[var(--bui-text-primary)] truncate">
                        {st.name}
                      </span>
                      <span className="text-[10px] text-[var(--bui-text-muted)] shrink-0">
                        ({st.count})
                      </span>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <span className="font-mono-num font-bold text-[var(--bui-text-primary)]">
                        {formatCurrency(st.totalCents, isPrivacyMode)}
                      </span>
                      <span className="text-[10px] font-mono-num px-1.5 py-0.5 rounded bg-[var(--bui-bg-hover)] text-[var(--bui-text-secondary)]">
                        {st.percentage}%
                      </span>
                    </div>
                  </div>
                  <div className="w-full h-1.5 bui-bar-track">
                    <div
                      className={`h-full ${
                        st.isUncategorized
                          ? 'bui-bar-fill-amber'
                          : idx % 2 === 0
                          ? 'bui-bar-fill-lime'
                          : 'bui-bar-fill-teal'
                      }`}
                      style={{ width: `${Math.min(100, st.percentage)}%` }}
                    />
                  </div>
                </button>
              )
            })
          )}
        </div>

        <div className="pt-2 border-t border-[var(--bui-border)] flex items-center justify-between text-[11px] text-[var(--bui-text-muted)]">
          <span className="flex items-center gap-1">
            <Filter className="w-3 h-3" /> Clique em uma barra ou item para filtrar a tabela
          </span>
          <span className="font-mono-num font-semibold text-[var(--bui-text-secondary)]">
            Total: {formatCurrency(totalExpenseCents, isPrivacyMode)}
          </span>
        </div>
      </div>
    </div>
  )
}
