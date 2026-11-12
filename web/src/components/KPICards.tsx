import React from 'react'
import {
  Wallet,
  ArrowDownLeft,
  ArrowUpRight,
  CreditCard,
  CalendarClock,
} from 'lucide-react'
import { CashflowProjection, TransactionSummary } from '../types'
import {
  calculateDailyBudgetMetrics,
  decimalToCents,
  formatCurrency,
  isNonNegativeCents,
} from '../utils/money'

export { formatCurrency } from '../utils/money'

interface KPICardsProps {
  summary: TransactionSummary | null
  projection?: CashflowProjection | null
  monthlyBudgetLimit?: string
  isPrivacyMode?: boolean
  isLoading?: boolean
  onInspectInvoices?: () => void
  onInspectBudgets?: () => void
}

export const KPICards: React.FC<KPICardsProps> = ({
  summary,
  projection,
  monthlyBudgetLimit = '12000.00',
  isPrivacyMode = false,
  isLoading = false,
  onInspectInvoices,
  onInspectBudgets,
}) => {
  const liquidBalance = projection?.available_balance || '0.00'
  const pendingBills = projection?.total_pending_bills || '0.00'
  const projectedNet = projection?.projected_net || summary?.net_balance || '0.00'
  const income = summary?.total_income || '0.00'
  const expense = summary?.total_expense || '0.00'
  const count = summary?.count || 0

  const dailyMetrics = calculateDailyBudgetMetrics(monthlyBudgetLimit, expense)
  const netPositive = isNonNegativeCents(projectedNet)

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-5 gap-3.5">
      {/* 1. Saldo em Contas Líquidas */}
      <div className="bui-card p-4 flex flex-col justify-between">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 bui-inner flex items-center justify-center text-[var(--bui-text-secondary)] shrink-0">
            <Wallet className="w-4 h-4" />
          </div>
          <span className="text-xs font-medium text-[var(--bui-text-secondary)] truncate">
            Saldo em Contas Líquidas
          </span>
        </div>
        <div className="mt-3.5 flex items-end justify-between gap-2">
          <div className="text-xl font-bold font-mono-num tracking-tight text-[var(--bui-text-primary)]">
            {isLoading ? (
              <span className="opacity-40 animate-pulse">...</span>
            ) : (
              formatCurrency(liquidBalance, isPrivacyMode)
            )}
          </div>
          <span className="text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-lime shrink-0">
            Disponível
          </span>
        </div>
      </div>

      {/* 2. Receitas no Período */}
      <div className="bui-card p-4 flex flex-col justify-between">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 bui-inner flex items-center justify-center text-[var(--bui-lime)] shrink-0">
            <ArrowDownLeft className="w-4 h-4" />
          </div>
          <span className="text-xs font-medium text-[var(--bui-text-secondary)] truncate">
            Receitas no Período
          </span>
        </div>
        <div className="mt-3.5 flex items-end justify-between gap-2">
          <div className="text-xl font-bold font-mono-num tracking-tight text-[var(--bui-text-primary)]">
            {isLoading ? (
              <span className="opacity-40 animate-pulse">...</span>
            ) : (
              formatCurrency(income, isPrivacyMode)
            )}
          </div>
          <span className="text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-lime shrink-0">
            Entradas
          </span>
        </div>
      </div>

      {/* 3. Despesas no Período */}
      <div className="bui-card p-4 flex flex-col justify-between">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 bui-inner flex items-center justify-center text-[var(--bui-rose)] shrink-0">
            <ArrowUpRight className="w-4 h-4" />
          </div>
          <span className="text-xs font-medium text-[var(--bui-text-secondary)] truncate">
            Despesas ({count} lanç.)
          </span>
        </div>
        <div className="mt-3.5 flex items-end justify-between gap-2">
          <div className="text-xl font-bold font-mono-num tracking-tight text-[var(--bui-text-primary)]">
            {isLoading ? (
              <span className="opacity-40 animate-pulse">...</span>
            ) : (
              formatCurrency(
                decimalToCents(expense) < 0n ? -decimalToCents(expense) : decimalToCents(expense),
                isPrivacyMode
              )
            )}
          </div>
          <span className="text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-rose shrink-0">
            Saídas
          </span>
        </div>
      </div>

      {/* 4. Faturas Pendentes & Liquidez Projetada */}
      <div
        onClick={onInspectInvoices}
        className={`bui-card p-4 flex flex-col justify-between ${
          onInspectInvoices ? 'cursor-pointer hover:border-[var(--bui-border-strong)]' : ''
        }`}
      >
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 bui-inner flex items-center justify-center text-[var(--bui-amber)] shrink-0">
            <CreditCard className="w-4 h-4" />
          </div>
          <span className="text-xs font-medium text-[var(--bui-text-secondary)] truncate">
            Faturas em Aberto
          </span>
        </div>
        <div className="mt-3.5 flex items-end justify-between gap-2">
          <div>
            <div className="text-xl font-bold font-mono-num tracking-tight text-[var(--bui-text-primary)]">
              {isLoading ? (
                <span className="opacity-40 animate-pulse">...</span>
              ) : (
                formatCurrency(pendingBills, isPrivacyMode)
              )}
            </div>
            <div className="text-[10px] text-[var(--bui-text-muted)] mt-0.5">
              Líq. Projetado:{' '}
              <span
                className={`font-mono-num font-semibold ${
                  netPositive ? 'text-[var(--bui-lime)]' : 'text-[var(--bui-rose)]'
                }`}
              >
                {formatCurrency(projectedNet, isPrivacyMode)}
              </span>
            </div>
          </div>
          <span className="text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-amber shrink-0">
            {projection?.upcoming_bills?.length || 0} abertas
          </span>
        </div>
      </div>

      {/* 5. Disponível por Dia (Spendee Pattern) */}
      <div
        onClick={onInspectBudgets}
        className={`bui-card p-4 flex flex-col justify-between ${
          onInspectBudgets ? 'cursor-pointer hover:border-[var(--bui-border-strong)]' : ''
        }`}
      >
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 bui-inner flex items-center justify-center text-[var(--bui-accent-blue)] shrink-0">
            <CalendarClock className="w-4 h-4" />
          </div>
          <span className="text-xs font-medium text-[var(--bui-text-secondary)] truncate">
            Disponível / Dia
          </span>
        </div>
        <div className="mt-3.5 flex items-end justify-between gap-2">
          <div>
            <div className="text-xl font-bold font-mono-num tracking-tight text-[var(--bui-text-primary)]">
              {isLoading ? (
                <span className="opacity-40 animate-pulse">...</span>
              ) : (
                formatCurrency(dailyMetrics.dailyAvailableCents, isPrivacyMode)
              )}
            </div>
            <div className="text-[10px] text-[var(--bui-text-muted)] mt-0.5">
              Restam {dailyMetrics.daysRemaining} dias no mês
            </div>
          </div>
          <span
            className={`text-[10px] font-semibold px-2 py-0.5 rounded-md shrink-0 ${
              dailyMetrics.status === 'SAFE'
                ? 'bui-badge-blue'
                : dailyMetrics.status === 'WARNING'
                ? 'bui-badge-amber'
                : 'bui-badge-rose'
            }`}
          >
            {dailyMetrics.usagePercentage}% meta
          </span>
        </div>
      </div>
    </div>
  )
}
