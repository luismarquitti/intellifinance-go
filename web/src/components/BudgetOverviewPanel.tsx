import React, { useState } from 'react'
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
}

export const BudgetOverviewPanel: React.FC<BudgetOverviewPanelProps> = ({
  summary,
  transactions,
  categories,
  monthlyBudgetLimit,
  onChangeMonthlyBudgetLimit,
  isPrivacyMode = false,
}) => {
  const [customLimitInput, setCustomLimitInput] = useState(monthlyBudgetLimit)
  const [isEditingGoal, setIsEditingGoal] = useState(false)

  const totalExpenseStr = summary?.total_expense || '0.00'
  const metrics = calculateDailyBudgetMetrics(monthlyBudgetLimit, totalExpenseStr)

  // Group expenses by category to show category envelopes
  const catExpenseMap = new Map<string, bigint>()
  for (const tx of transactions) {
    if (tx.type === 'EXPENSE' || decimalToCents(tx.amount) < 0n) {
      const c = decimalToCents(tx.amount)
      const absC = c < 0n ? -c : c
      const key = tx.category_id || '__uncat__'
      catExpenseMap.set(key, (catExpenseMap.get(key) || 0n) + absC)
    }
  }

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

  return (
    <div className="space-y-4">
      {/* Top Hero Card: Spendee "Disponível por Dia" & Global Monthly Envelope */}
      <div className="bui-card p-6 space-y-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <span className="text-xs font-bold uppercase tracking-wider text-[var(--bui-accent-blue)] flex items-center gap-1.5">
                <CalendarClock className="w-4 h-4" />
                Motor de Orçamento Acionável (Spendee Pattern)
              </span>
              {metrics.status === 'SAFE' && (
                <span className="text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-lime flex items-center gap-1">
                  <CheckCircle2 className="w-3 h-3" /> Ritmo Seguro
                </span>
              )}
              {metrics.status === 'WARNING' && (
                <span className="text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-amber flex items-center gap-1">
                  <AlertTriangle className="w-3 h-3" /> Atenção (85%+)
                </span>
              )}
              {metrics.status === 'CRITICAL' && (
                <span className="text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-rose flex items-center gap-1">
                  <Flame className="w-3 h-3" /> Meta Excedida
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
              <form onSubmit={handleSaveGoal} className="flex items-center gap-2">
                <input
                  type="text"
                  value={customLimitInput}
                  onChange={(e) => setCustomLimitInput(e.target.value)}
                  className="w-32 px-2.5 py-1.5 rounded-lg bg-[var(--bui-bg-inner)] border border-[var(--bui-accent-blue)] text-xs font-mono-num text-[var(--bui-text-primary)]"
                />
                <button
                  type="submit"
                  className="px-3 py-1.5 rounded-lg bg-[var(--bui-accent-blue)] text-white text-xs font-semibold"
                >
                  Salvar
                </button>
              </form>
            ) : (
              <button
                onClick={() => setIsEditingGoal(true)}
                className="bui-pill-control px-3 py-1.5 text-xs font-semibold flex items-center gap-1.5"
              >
                <Sliders className="w-3.5 h-3.5 text-[var(--bui-accent-blue)]" />
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
          <div className="flex items-center justify-between text-xs text-[var(--bui-text-secondary)]">
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
      </div>

      {/* Bottom 2 Columns: Envelopes por Categoria & Compromissos Recorrentes */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Envelopes por Categoria */}
        <div className="bui-card p-5 space-y-3.5">
          <div>
            <h3 className="text-sm font-bold text-[var(--bui-text-primary)]">
              Envelopes por Categoria
            </h3>
            <p className="text-xs text-[var(--bui-text-muted)]">
              Acompanhamento de consumo relativo ao orçamento por categoria de despesa.
            </p>
          </div>

          <div className="space-y-2.5">
            {categories
              .filter((c) => c.type === 'EXPENSE')
              .slice(0, 6)
              .map((cat, idx) => {
                const spent = catExpenseMap.get(cat.id) || 0n
                // Share of monthly budget (20% envelope reference per top category)
                const catEnvelope =
                  metrics.monthlyLimitCents > 0n ? metrics.monthlyLimitCents / 5n : 200000n
                const pct =
                  catEnvelope > 0n ? Number((spent * 100n) / catEnvelope) : 0

                return (
                  <div key={cat.id} className="bui-inner p-3 space-y-1.5">
                    <div className="flex items-center justify-between text-xs">
                      <span className="font-semibold text-[var(--bui-text-primary)] flex items-center gap-2">
                        <span
                          className="w-2 h-2 rounded-full"
                          style={{ backgroundColor: cat.color_hex || '#2563EB' }}
                        />
                        {cat.name}
                      </span>
                      <span className="font-mono-num text-[var(--bui-text-secondary)]">
                        <strong className="text-[var(--bui-text-primary)]">
                          {formatCurrency(spent, isPrivacyMode)}
                        </strong>{' '}
                        / {formatCurrency(catEnvelope, isPrivacyMode)}
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
        </div>

        {/* Compromissos Recorrentes / Frequentes (Mobills / Organizze Pattern) */}
        <div className="bui-card p-5 space-y-3.5">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-bold text-[var(--bui-text-primary)] flex items-center gap-2">
                <Repeat className="w-4 h-4 text-[var(--bui-accent-blue)]" />
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
                Nenhuma despesa recorrente detectada na página atual.
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
                    <span className="text-[10px] px-1.5 py-0.5 rounded bui-badge-blue">
                      Recorrente
                    </span>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
