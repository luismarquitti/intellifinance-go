import React from 'react'
import {
  Wallet,
  ArrowDownLeft,
  ArrowUpRight,
  CreditCard,
  Scale,
} from 'lucide-react'
import { CashflowProjection, TransactionSummary } from '../types'
import {
  decimalToCents,
  formatCurrency,
  isNonNegativeCents,
} from '../utils/money'

export { formatCurrency } from '../utils/money'

interface KPICardsProps {
  summary: TransactionSummary | null
  projection?: CashflowProjection | null
  monthlyBudgetLimit?: string
  selectedMonthLabel?: string
  isAllPeriods?: boolean
  isPrivacyMode?: boolean
  isLoading?: boolean
  showInvoicesCard?: boolean
  onInspectInvoices?: () => void
  onInspectBudgets?: () => void
}

export const KPICards: React.FC<KPICardsProps> = ({
  summary,
  projection,
  selectedMonthLabel,
  isAllPeriods = true,
  isPrivacyMode = false,
  isLoading = false,
  showInvoicesCard = true,
  onInspectInvoices,
}) => {
  const initialBalance = summary?.initial_balance ?? '0.00'
  const income = summary?.total_income || '0.00'
  const expense = summary?.total_expense || '0.00'
  const netBalance = summary?.net_balance || '0.00'
  const finalBalance = summary?.final_balance ?? projection?.available_balance ?? '0.00'
  const pendingBills = projection?.total_pending_bills || '0.00'
  const projectedNet = projection?.projected_net || '0.00'
  const count = summary?.count || 0

  const isCurrent = isAllPeriods || Boolean(summary?.is_current_month)
  const finalPositive = isNonNegativeCents(finalBalance)
  const netPositive = isNonNegativeCents(netBalance)
  const projPositive = isNonNegativeCents(projectedNet)

  const periodTag = !isAllPeriods && selectedMonthLabel ? selectedMonthLabel : 'Período'

  return (
    <div
      className={`grid grid-cols-1 sm:grid-cols-2 ${
        showInvoicesCard ? 'xl:grid-cols-5' : 'xl:grid-cols-4'
      } gap-3.5`}
    >
      {/* 1. Saldo Inicial do Mês / Período */}
      <div className="bui-card p-4 flex flex-col justify-between">
        <div className="flex items-center gap-2.5">
          <div className="w-9 h-9 bui-inner flex items-center justify-center text-[var(--bui-text-secondary)] shrink-0">
            <Wallet className="w-4 h-4" aria-hidden="true" />
          </div>
          <span className="text-xs font-medium text-[var(--bui-text-secondary)] truncate">
            {isAllPeriods ? 'Saldo Inicial (Base)' : `Saldo Inicial (${periodTag})`}
          </span>
        </div>
        <div className="mt-3.5 flex items-end justify-between gap-2">
          <div className="text-xl sm:text-2xl font-bold font-mono-num tracking-tight text-[var(--bui-text-primary)]">
            {isLoading ? (
              <span className="opacity-40 animate-pulse">...</span>
            ) : (
              formatCurrency(initialBalance, isPrivacyMode)
            )}
          </div>
          <span className="text-[11px] font-semibold px-2 py-0.5 rounded-md bui-pill-control shrink-0">
            Abertura
          </span>
        </div>
      </div>

      {/* 2. Entradas no Mês / Período */}
      <div className="bui-card p-4 flex flex-col justify-between">
        <div className="flex items-center gap-2.5">
          <div className="w-9 h-9 bui-inner flex items-center justify-center text-[var(--bui-lime)] shrink-0">
            <ArrowDownLeft className="w-4 h-4" aria-hidden="true" />
          </div>
          <span className="text-xs font-medium text-[var(--bui-text-secondary)] truncate">
            {isAllPeriods ? 'Entradas no Período' : `Entradas (${periodTag})`}
          </span>
        </div>
        <div className="mt-3.5 flex items-end justify-between gap-2">
          <div className="text-xl sm:text-2xl font-bold font-mono-num tracking-tight text-[var(--bui-text-primary)]">
            {isLoading ? (
              <span className="opacity-40 animate-pulse">...</span>
            ) : (
              formatCurrency(income, isPrivacyMode, { showPlusSign: true })
            )}
          </div>
          <span className="text-[11px] font-semibold px-2 py-0.5 rounded-md bui-badge-lime inline-flex items-center gap-1.5 shrink-0">
            <span className="bui-dot-lime" aria-hidden="true" />
            Entradas
          </span>
        </div>
      </div>

      {/* 3. Saídas no Mês / Período */}
      <div className="bui-card p-4 flex flex-col justify-between">
        <div className="flex items-center gap-2.5">
          <div className="w-9 h-9 bui-inner flex items-center justify-center text-[var(--bui-rose)] shrink-0">
            <ArrowUpRight className="w-4 h-4" aria-hidden="true" />
          </div>
          <span className="text-xs font-medium text-[var(--bui-text-secondary)] truncate">
            {isAllPeriods ? `Saídas (${count} lanç.)` : `Saídas (${periodTag})`}
          </span>
        </div>
        <div className="mt-3.5 flex items-end justify-between gap-2">
          <div>
            <div className="text-xl sm:text-2xl font-bold font-mono-num tracking-tight text-[var(--bui-text-primary)]">
              {isLoading ? (
                <span className="opacity-40 animate-pulse">...</span>
              ) : (
                formatCurrency(
                  decimalToCents(expense) < 0n ? -decimalToCents(expense) : decimalToCents(expense),
                  isPrivacyMode
                )
              )}
            </div>
            <div className="text-xs text-[var(--bui-text-muted)] mt-0.5">
              {count} lanç. • inclui faturas pagas
            </div>
          </div>
          <span className="text-[11px] font-semibold px-2 py-0.5 rounded-md bui-badge-rose shrink-0">
            Saídas
          </span>
        </div>
      </div>

      {/* 4. Saldo Final / Saldo Atual */}
      <div className="bui-card p-4 flex flex-col justify-between">
        <div className="flex items-center gap-2.5">
          <div className="w-9 h-9 bui-inner flex items-center justify-center text-[var(--bui-accent-blue)] shrink-0">
            <Scale className="w-4 h-4" aria-hidden="true" />
          </div>
          <span className="text-xs font-medium text-[var(--bui-text-secondary)] truncate">
            {isCurrent
              ? isAllPeriods
                ? 'Saldo Atual (Contas)'
                : `Saldo Atual (${periodTag})`
              : `Saldo Final (${periodTag})`}
          </span>
        </div>
        <div className="mt-3.5 flex items-end justify-between gap-2">
          <div>
            <div
              className={`text-xl sm:text-2xl font-bold font-mono-num tracking-tight ${
                finalPositive ? 'text-[var(--bui-text-primary)]' : 'text-[var(--bui-rose)]'
              }`}
            >
              {isLoading ? (
                <span className="opacity-40 animate-pulse">...</span>
              ) : (
                formatCurrency(finalBalance, isPrivacyMode)
              )}
            </div>
            <div className="text-xs text-[var(--bui-text-muted)] mt-0.5">
              Resultado:{' '}
              <span
                className={`font-mono-num font-semibold ${
                  netPositive ? 'text-[var(--bui-lime)]' : 'text-[var(--bui-rose)]'
                }`}
              >
                {formatCurrency(netBalance, isPrivacyMode, { showPlusSign: true })}
              </span>
            </div>
          </div>
          <span
            className={`text-[11px] font-semibold px-2 py-0.5 rounded-md inline-flex items-center gap-1.5 shrink-0 ${
              isCurrent ? 'bui-badge-blue' : 'bui-badge-lime'
            }`}
          >
            <span className={isCurrent ? 'bui-dot-blue' : 'bui-dot-lime'} aria-hidden="true" />
            {isCurrent ? 'Atual' : 'Fechado'}
          </span>
        </div>
      </div>

      {/* 5. Faturas em Aberto & Liquidez Projetada (omitted when CashflowCard is directly below) */}
      {showInvoicesCard && (
        <button
          type="button"
          onClick={onInspectInvoices}
          disabled={!onInspectInvoices}
          className={`bui-card p-4 flex flex-col justify-between text-left w-full ${
            onInspectInvoices ? 'cursor-pointer hover:border-[var(--bui-border-strong)]' : ''
          }`}
        >
          <div className="flex items-center gap-2.5 w-full">
            <div className="w-9 h-9 bui-inner flex items-center justify-center text-[var(--bui-amber)] shrink-0">
              <CreditCard className="w-4 h-4" aria-hidden="true" />
            </div>
            <span className="text-xs font-medium text-[var(--bui-text-secondary)] truncate">
              Faturas em Aberto
            </span>
          </div>
          <div className="mt-3.5 flex items-end justify-between gap-2 w-full">
            <div>
              <div className="text-xl sm:text-2xl font-bold font-mono-num tracking-tight text-[var(--bui-text-primary)]">
                {isLoading ? (
                  <span className="opacity-40 animate-pulse">...</span>
                ) : (
                  formatCurrency(pendingBills, isPrivacyMode)
                )}
              </div>
              <div className="text-xs text-[var(--bui-text-muted)] mt-0.5">
                Líq. Projetado:{' '}
                <span
                  className={`font-mono-num font-semibold ${
                    projPositive ? 'text-[var(--bui-lime)]' : 'text-[var(--bui-rose)]'
                  }`}
                >
                  {formatCurrency(projectedNet, isPrivacyMode)}
                </span>
              </div>
            </div>
            <span className="text-[11px] font-semibold px-2 py-0.5 rounded-md bui-badge-amber inline-flex items-center gap-1.5 shrink-0">
              <span className="bui-dot-amber" aria-hidden="true" />
              {projection?.upcoming_bills?.length || 0} abertas
            </span>
          </div>
        </button>
      )}
    </div>
  )
}
