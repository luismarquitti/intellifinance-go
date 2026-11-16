import React, { useId, useState } from 'react'
import {
  CalendarClock,
  Sliders,
  CheckCircle2,
  AlertTriangle,
  Flame,
  Repeat,
} from 'lucide-react'
import { Category, Transaction, TransactionSummary } from '../types'
import {
  calculateDailyBudgetMetrics,
  decimalToCents,
  formatCurrency,
} from '../utils/money'

interface BudgetOverviewPanelProps {
  summary: TransactionSummary | null
  transactions: Transaction[]
  categories: Category[]
  monthlyBudgetLimit: string
  onChangeMonthlyBudgetLimit: (newLimit: string) => void
  isPrivacyMode?: boolean
  totalPeriodCount?: number
}

export const BudgetOverviewPanel: React.FC<BudgetOverviewPanelProps> = ({
  summary,
  transactions,
  categories,
  monthlyBudgetLimit,
  onChangeMonthlyBudgetLimit,
  isPrivacyMode = false,
  totalPeriodCount,
}) => {
  const [customLimitInput, setCustomLimitInput] = useState(monthlyBudgetLimit)
  const [isEditingGoal, setIsEditingGoal] = useState(false)
  const limitInputId = useId()

  const totalExpenseStr = summary?.total_expense || '0.00'
  const metrics = calculateDailyBudgetMetrics(monthlyBudgetLimit, totalExpenseStr)

  // Group expenses by category to show category envelopes
  const catExpenseMap = new Map<string, bigint>()
  for (const tx of transactions) {
    if (tx.type === 'TRANSFER' || tx.status === 'PENDING' || tx.status === 'VOID') continue
    if (tx.type === 'EXPENSE' || decimalToCents(tx.amount) < 0n) {
      const c = decimalToCents(tx.amount)
      const absC = c < 0n ? -c : c
      const key = tx.category_id || '__uncat__'
      catExpenseMap.set(key, (catExpenseMap.get(key) || 0n) + absC)
    }
  }

  // Sort expense categories by actual spent descending so active categories are always visible
  const sortedExpenseCategories = [...categories]
    .filter((c) => c.type === 'EXPENSE')
    .sort((a, b) => {
      const spentA = catExpenseMap.get(a.id) || 0n
      const spentB = catExpenseMap.get(b.id) || 0n
      if (spentB > spentA) return 1
      if (spentB < spentA) return -1
      return a.name.localeCompare(b.name)
    })
    .slice(0, 6)

  const envelopeCount = BigInt(Math.max(1, sortedExpenseCategories.length))
  const perCategoryEnvelope =
    metrics.monthlyLimitCents > 0n
      ? metrics.monthlyLimitCents / envelopeCount
      : 200000n

  // Detect recurring merchants (descriptions appearing >= 2 times)
  const merchantFreq = new Map<string, { count: number; totalCents: bigint; lastDate: string }>()
  for (const tx of transactions) {
    if (tx.type === 'TRANSFER') continue
    const c = decimalToCents(tx.amount)
    if (c >= 0n && tx.type !== 'EXPENSE') continue
    const absC = c < 0n ? -c : c
    const key = (tx.clean_description || tx.description).trim()
    if (!key) continue
    const prev = merchantFreq.get(key) || { count: 0, totalCents: 0n, lastDate: tx.date }
    merchantFreq.set(key, {
      count: prev.count + 1,
      totalCents: prev.totalCents + absC,
      lastDate: tx.date > prev.lastDate ? tx.date : prev.lastDate,
    })
  }

  const recurringCommitments = Array.from(merchantFreq.entries())
    .filter(([, val]) => val.count >= 2)
    .sort((a, b) => (b[1].totalCents > a[1].totalCents ? 1 : -1))
    .slice(0, 6)

  const handleSaveGoal = (e: React.FormEvent) => {
    e.preventDefault()
    const cents = decimalToCents(customLimitInput)
    if (cents > 0n) {
      onChangeMonthlyBudgetLimit(customLimitInput.replace(',', '.'))
      setIsEditingGoal(false)
    }
  }

  const isTruncatedSample =
    typeof totalPeriodCount === 'number' && totalPeriodCount > transactions.length

  return (
    <div className="space-y-4">
      {/* Top Hero Card: Spendee "Disponível por Dia" & Global Monthly Envelope */}
      <section aria-label="Orçamento Mensal e Ritmo Diário" className="bui-card p-6 space-y-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-xs font-bold uppercase tracking-wider text-[var(--bui-accent-blue)] flex items-center gap-1.5">
                <CalendarClock className="w-4 h-4" aria-hidden="true" />
                Motor de Orçamento Acionável
              </h2>
              {metrics.status === 'SAFE' && (
                <span className="text-[11px] font-semibold px-2 py-0.5 rounded-md bui-badge-lime flex items-center gap-1">
                  <CheckCircle2 className="w-3 h-3" aria-hidden="true" /> Ritmo Seguro
                </span>
              )}
              {metrics.status === 'WARNING' && (
                <span className="text-[11px] font-semibold px-2 py-0.5 rounded-md bui-badge-amber flex items-center gap-1">
                  <AlertTriangle className="w-3 h-3" aria-hidden="true" /> Atenção (85%+)
                </span>
              )}
              {metrics.status === 'CRITICAL' && (
                <span className="text-[11px] font-semibold px-2 py-0.5 rounded-md bui-badge-rose flex items-center gap-1">
                  <Flame className="w-3 h-3" aria-hidden="true" /> Meta Excedida
                </span>
              )}
            </div>
            <p className="text-xs text-[var(--bui-text-muted)]">
              Converte seu teto mensal em poder de gasto diário para os próximos{' '}
              <strong className="text-[var(--bui-text-primary)]">{metrics.daysRemaining} dias</strong> do mês.
            </p>
          </div>

          {/* Goal Editor */}
          <div>
            {isEditingGoal ? (
              <form onSubmit={handleSaveGoal} className="flex flex-wrap items-center gap-2">
                <label htmlFor={limitInputId} className="sr-only">
                  Teto Mensal em Reais
                </label>
                <input
                  id={limitInputId}
                  type="text"
                  inputMode="decimal"
                  value={customLimitInput}
                  onChange={(e) => setCustomLimitInput(e.target.value)}
                  className="w-36 px-2.5 py-1.5 rounded-lg bg-[var(--bui-bg-inner)] border border-[var(--bui-accent-blue)] text-xs font-mono-num text-[var(--bui-text-primary)]"
                />
                <button
                  type="submit"
                  className="px-3 py-1.5 rounded-lg bg-[var(--bui-accent-blue)] text-white text-xs font-semibold"
                >
                  Salvar
                </button>
                <button
                  type="button"
                  onClick={() => setIsEditingGoal(false)}
                  className="bui-pill-control px-3 py-1.5 text-xs font-medium"
                >
                  Cancelar
                </button>
              </form>
            ) : (
              <button
                type="button"
                onClick={() => setIsEditingGoal(true)}
                className="bui-pill-control px-3 py-1.5 text-xs font-semibold flex items-center gap-1.5"
              >
                <Sliders className="w-3.5 h-3.5 text-[var(--bui-accent-blue)]" aria-hidden="true" />
                <span>Ajustar Teto Mensal ({formatCurrency(monthlyBudgetLimit, isPrivacyMode)})</span>
              </button>
            )}
          </div>
        </div>

        {/* 3 Main Budget Numbers */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div className="bui-inner p-4">
            <span className="text-xs text-[var(--bui-text-secondary)] block">
              Disponível para Gastar por Dia
            </span>
            <div className="text-2xl font-bold font-mono-num text-[var(--bui-text-primary)] mt-1">
              {formatCurrency(metrics.dailyAvailableCents, isPrivacyMode)}
              <span className="text-xs font-normal text-[var(--bui-text-muted)]"> / dia</span>
            </div>
          </div>

          <div className="bui-inner p-4">
            <span className="text-xs text-[var(--bui-text-secondary)] block">
              Saldo Restante do Orçamento
            </span>
            <div
              className={`text-2xl font-bold font-mono-num mt-1 ${
                metrics.remainingCents >= 0n
                  ? 'text-[var(--bui-lime)]'
                  : 'text-[var(--bui-rose)]'
              }`}
            >
              {formatCurrency(metrics.remainingCents, isPrivacyMode)}
            </div>
          </div>

          <div className="bui-inner p-4">
            <span className="text-xs text-[var(--bui-text-secondary)] block">
              Realizado vs. Teto Mensal
            </span>
            <div className="text-2xl font-bold font-mono-num text-[var(--bui-text-primary)] mt-1">
              {metrics.usagePercentage}%
              <span className="text-xs font-normal text-[var(--bui-text-muted)] ml-1.5">
                ({formatCurrency(metrics.spentCents, isPrivacyMode)})
              </span>
            </div>
          </div>
        </div>

        {/* Capsule Progress Bar */}
        <div className="space-y-1.5">
          <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-[var(--bui-text-secondary)]">
            <span>Progresso de Consumo Mensal</span>
            <span className="font-mono-num font-semibold">
              {formatCurrency(metrics.spentCents, isPrivacyMode)} de{' '}
              {formatCurrency(metrics.monthlyLimitCents, isPrivacyMode)}
            </span>
          </div>
          <div className="w-full h-3.5 bui-bar-track p-0.5">
            <div
              className={`h-full transition-all duration-300 ${
                metrics.status === 'SAFE'
                  ? 'bui-bar-fill-lime'
                  : metrics.status === 'WARNING'
                  ? 'bui-bar-fill-amber'
                  : 'bui-bar-fill-rose'
              }`}
              style={{ width: `${Math.min(100, Math.max(2, metrics.usagePercentage))}%` }}
            />
          </div>
        </div>
      </section>

      {/* Bottom 2 Columns: Envelopes por Categoria & Compromissos Recorrentes */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Envelopes por Categoria */}
        <section aria-label="Envelopes por Categoria" className="bui-card p-5 space-y-3.5">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <h3 className="text-sm font-bold text-[var(--bui-text-primary)]">
                Envelopes por Categoria (Top 6)
              </h3>
              <p className="text-xs text-[var(--bui-text-muted)]">
                Consumo proporcional distribuído entre as principais categorias de despesa.
              </p>
            </div>
            {isTruncatedSample && (
              <span className="text-[11px] px-2 py-0.5 rounded-md bui-pill-control">
                Amostra: {transactions.length} lanç.
              </span>
            )}
          </div>

          <div className="space-y-2.5">
            {sortedExpenseCategories.map((cat, idx) => {
              const spent = catExpenseMap.get(cat.id) || 0n
              const pct =
                perCategoryEnvelope > 0n ? Number((spent * 100n) / perCategoryEnvelope) : 0

              return (
                <div key={cat.id} className="bui-inner p-3 space-y-1.5">
                  <div className="flex items-center justify-between gap-2 text-xs">
                    <span className="font-semibold text-[var(--bui-text-primary)] flex items-center gap-2 min-w-0">
                      <span
                        className="w-2.5 h-2.5 rounded-full shrink-0"
                        style={{ backgroundColor: cat.color_hex || '#2563EB' }}
                        aria-hidden="true"
                      />
                      <span className="truncate">{cat.name}</span>
                    </span>
                    <span className="font-mono-num text-[var(--bui-text-secondary)] shrink-0">
                      <strong className="text-[var(--bui-text-primary)]">
                        {formatCurrency(spent, isPrivacyMode)}
                      </strong>{' '}
                      / {formatCurrency(perCategoryEnvelope, isPrivacyMode)}
                    </span>
                  </div>
                  <div className="w-full h-2 bui-bar-track">
                    <div
                      className={`h-full ${
                        pct >= 100
                          ? 'bui-bar-fill-rose'
                          : pct >= 85
                          ? 'bui-bar-fill-amber'
                          : idx % 2 === 0
                          ? 'bui-bar-fill-lime'
                          : 'bui-bar-fill-teal'
                      }`}
                      style={{ width: `${Math.min(100, Math.max(2, pct))}%` }}
                    />
                  </div>
                </div>
              )
            })}
          </div>
        </section>

        {/* Compromissos Recorrentes / Frequentes */}
        <section aria-label="Despesas Recorrentes e Frequentes" className="bui-card p-5 space-y-3.5">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-bold text-[var(--bui-text-primary)] flex items-center gap-2">
                <Repeat className="w-4 h-4 text-[var(--bui-accent-blue)]" aria-hidden="true" />
                Despesas Recorrentes & Frequentes
              </h3>
              <p className="text-xs text-[var(--bui-text-muted)]">
                Estabelecimentos e assinaturas com múltiplas ocorrências no período.
              </p>
            </div>
          </div>

          <div className="space-y-2">
            {recurringCommitments.length === 0 ? (
              <div className="bui-inner p-6 text-center text-xs text-[var(--bui-text-muted)]">
                Nenhuma despesa recorrente detectada no período analisado.
              </div>
            ) : (
              recurringCommitments.map(([merchant, info]) => (
                <div
                  key={merchant}
                  className="bui-inner p-3 flex items-center justify-between gap-3"
                >
                  <div className="min-w-0">
                    <div className="text-xs font-bold text-[var(--bui-text-primary)] truncate">
                      {merchant}
                    </div>
                    <div className="text-[11px] text-[var(--bui-text-muted)] mt-0.5">
                      {info.count} ocorrências • Última em {info.lastDate.split('T')[0]}
                    </div>
                  </div>
                  <div className="text-right shrink-0">
                    <div className="text-xs font-bold font-mono-num text-[var(--bui-text-primary)]">
                      {formatCurrency(info.totalCents, isPrivacyMode)}
                    </div>
                    <span className="text-[11px] px-1.5 py-0.5 rounded bui-badge-blue inline-block mt-0.5">
                      Recorrente
                    </span>
                  </div>
                </div>
              ))
            )}
          </div>
        </section>
      </div>
    </div>
  )
}
