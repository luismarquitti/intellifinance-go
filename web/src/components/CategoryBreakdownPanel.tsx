import React from 'react'
import { BarChart3, Filter, X, ArrowRight } from 'lucide-react'
import { Category, Transaction } from '../types'
import { decimalToCents, formatCurrency } from '../utils/money'

interface CategoryBreakdownPanelProps {
  transactions: Transaction[]
  categories: Category[]
  selectedCategoryId?: string
  uncategorizedFilter?: boolean
  isPrivacyMode?: boolean
  totalPeriodCount?: number
  onSelectCategory: (categoryId?: string, uncategorized?: boolean) => void
  onNavigateToTransactions?: () => void
}

interface CategoryStat {
  id: string
  name: string
  colorHex: string
  isUncategorized?: boolean
  isOtherSummary?: boolean
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
  totalPeriodCount,
  onSelectCategory,
  onNavigateToTransactions,
}) => {
  const categoryMap = new Map(categories.map((c) => [c.id, c]))

  // Aggregate expenses in integer cents from loaded period transactions
  const expenseMap = new Map<string, { cents: bigint; count: number }>()
  let totalExpenseCents = 0n

  for (const tx of transactions) {
    if (tx.type === 'TRANSFER' || tx.status === 'PENDING' || tx.status === 'VOID') continue
    const cents = decimalToCents(tx.amount)
    if (tx.type === 'EXPENSE' || cents < 0n) {
      const absCents = cents < 0n ? -cents : cents
      totalExpenseCents += absCents
      const key = tx.category_id || '__uncategorized__'
      const prev = expenseMap.get(key) || { cents: 0n, count: 0 }
      expenseMap.set(key, { cents: prev.cents + absCents, count: prev.count + 1 })
    }
  }

  const allSorted: CategoryStat[] = Array.from(expenseMap.entries())
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

  const topStats = allSorted.slice(0, 8)
  const remainder = allSorted.slice(8)
  if (remainder.length > 0) {
    const remCents = remainder.reduce((acc, s) => acc + s.totalCents, 0n)
    const remCount = remainder.reduce((acc, s) => acc + s.count, 0)
    const remPct =
      totalExpenseCents > 0n
        ? Number((remCents * 100n) / totalExpenseCents)
        : 0
    topStats.push({
      id: '__others_summary__',
      name: `Demais Categorias (${remainder.length})`,
      colorHex: '#6B7280',
      isOtherSummary: true,
      totalCents: remCents,
      count: remCount,
      percentage: Math.max(1, remPct),
    })
  }

  const maxCents = topStats.length > 0 ? topStats[0].totalCents : 1n
  const hasSelection = Boolean(selectedCategoryId || uncategorizedFilter)
  const isTruncatedSample =
    typeof totalPeriodCount === 'number' && totalPeriodCount > transactions.length

  return (
    <section aria-label="Distribuição de Despesas por Categoria" className="bui-card p-5 space-y-4">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <div className="flex items-center gap-2 flex-wrap">
            <BarChart3 className="w-4 h-4 text-[var(--bui-accent-blue)]" aria-hidden="true" />
            <h2 className="text-sm sm:text-base font-bold text-[var(--bui-text-primary)]">
              Distribuição de Despesas por Categoria
            </h2>
            {isTruncatedSample && (
              <span className="text-[11px] font-medium px-2 py-0.5 rounded-md bui-pill-control">
                Amostra: {transactions.length} de {totalPeriodCount} lanç.
              </span>
            )}
          </div>
          <p className="text-xs text-[var(--bui-text-muted)] mt-0.5">
            Visão unificada dos principais centros de gasto. Clique em uma categoria para filtrar ou explorar lançamentos.
          </p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <div className="flex items-center gap-3 text-xs text-[var(--bui-text-secondary)] mr-1">
            <span className="flex items-center gap-1.5">
              <span className="w-2.5 h-2.5 rounded-sm bg-[var(--bui-lime-bar)] inline-block" aria-hidden="true" />
              Categorizado
            </span>
            <span className="flex items-center gap-1.5">
              <span className="w-2.5 h-2.5 rounded-sm bg-[var(--bui-amber)] inline-block" aria-hidden="true" />
              Pendente
            </span>
          </div>

          {hasSelection && (
            <button
              type="button"
              onClick={() => onSelectCategory(undefined, false)}
              className="bui-pill-control px-2.5 py-1.5 text-xs font-semibold flex items-center gap-1"
            >
              <X className="w-3.5 h-3.5" aria-hidden="true" />
              <span>Limpar filtro</span>
            </button>
          )}

          {onNavigateToTransactions && (
            <button
              type="button"
              onClick={onNavigateToTransactions}
              className="px-3 py-1.5 rounded-lg text-xs font-semibold bui-badge-blue flex items-center gap-1.5 hover:opacity-90 transition"
            >
              <span>{hasSelection ? 'Ver filtrados em Transações' : 'Explorar Transações'}</span>
              <ArrowRight className="w-3.5 h-3.5" aria-hidden="true" />
            </button>
          )}
        </div>
      </div>

      {/* Unified Capsule-Track List */}
      {topStats.length === 0 ? (
        <div className="bui-inner py-10 text-center text-xs text-[var(--bui-text-muted)]">
          Nenhuma despesa encontrada no período selecionado para compor a distribuição.
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {topStats.map((st, idx) => {
            const isSelected =
              (st.isUncategorized && Boolean(uncategorizedFilter)) ||
              (!st.isUncategorized && !st.isOtherSummary && selectedCategoryId === st.id)

            const barWidthPct =
              maxCents > 0n
                ? Math.max(6, Math.min(100, Number((st.totalCents * 100n) / maxCents)))
                : 6

            const fillClass = st.isUncategorized
              ? 'bui-bar-fill-amber'
              : st.isOtherSummary
              ? 'bg-[var(--bui-text-muted)] rounded-full'
              : idx % 2 === 0
              ? 'bui-bar-fill-lime'
              : 'bui-bar-fill-teal'

            if (st.isOtherSummary) {
              return (
                <div
                  key={st.id}
                  className="bui-inner p-3.5 flex flex-col gap-2 opacity-90"
                >
                  <div className="flex items-center justify-between gap-2 text-xs">
                    <div className="flex items-center gap-2 min-w-0">
                      <span
                        className="w-2.5 h-2.5 rounded-full shrink-0"
                        style={{ backgroundColor: st.colorHex }}
                        aria-hidden="true"
                      />
                      <span className="font-semibold text-[var(--bui-text-secondary)] truncate">
                        {st.name}
                      </span>
                      <span className="text-[11px] text-[var(--bui-text-muted)] shrink-0">
                        ({st.count} lanç.)
                      </span>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <span className="font-mono-num font-bold text-[var(--bui-text-primary)]">
                        {formatCurrency(st.totalCents, isPrivacyMode)}
                      </span>
                      <span className="text-[11px] font-mono-num px-1.5 py-0.5 rounded bg-[var(--bui-bg-hover)] text-[var(--bui-text-secondary)]">
                        {st.percentage}%
                      </span>
                    </div>
                  </div>
                  <div className="w-full h-2.5 bui-bar-track p-0.5">
                    <div className={`h-full ${fillClass}`} style={{ width: `${barWidthPct}%` }} />
                  </div>
                </div>
              )
            }

            return (
              <button
                key={st.id}
                type="button"
                aria-pressed={isSelected}
                onClick={() => {
                  if (st.isUncategorized) {
                    onSelectCategory(undefined, !uncategorizedFilter)
                  } else {
                    onSelectCategory(isSelected ? undefined : st.id, false)
                  }
                }}
                className={`w-full bui-inner p-3.5 flex flex-col gap-2 text-left transition ${
                  isSelected
                    ? 'ring-2 ring-[var(--bui-accent-blue)] border-transparent'
                    : 'hover:border-[var(--bui-border-strong)]'
                }`}
              >
                <div className="flex items-center justify-between gap-2 text-xs w-full">
                  <div className="flex items-center gap-2 min-w-0">
                    <span
                      className="w-2.5 h-2.5 rounded-full shrink-0"
                      style={{ backgroundColor: st.colorHex }}
                      aria-hidden="true"
                    />
                    <span className="font-semibold text-[var(--bui-text-primary)] truncate">
                      {st.name}
                    </span>
                    <span className="text-[11px] text-[var(--bui-text-muted)] shrink-0">
                      ({st.count})
                    </span>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <span className="font-mono-num font-bold text-[var(--bui-text-primary)]">
                      {formatCurrency(st.totalCents, isPrivacyMode)}
                    </span>
                    <span className="text-[11px] font-mono-num px-1.5 py-0.5 rounded bg-[var(--bui-bg-hover)] text-[var(--bui-text-secondary)] font-semibold">
                      {st.percentage}%
                    </span>
                  </div>
                </div>
                <div className="w-full h-2.5 bui-bar-track p-0.5">
                  <div
                    className={`h-full transition-all duration-300 ${fillClass}`}
                    style={{ width: `${barWidthPct}%` }}
                  />
                </div>
              </button>
            )
          })}
        </div>
      )}

      {/* Footer */}
      <div className="pt-2.5 border-t border-[var(--bui-border)] flex flex-wrap items-center justify-between gap-2 text-xs text-[var(--bui-text-muted)]">
        <span className="flex items-center gap-1.5">
          <Filter className="w-3.5 h-3.5" aria-hidden="true" />
          Clique em uma categoria para filtrar os lançamentos por ela
        </span>
        <span className="font-mono-num font-semibold text-[var(--bui-text-primary)]">
          Total Analisado: {formatCurrency(totalExpenseCents, isPrivacyMode)}
        </span>
      </div>
    </section>
  )
}
