import React from 'react'
import { Wallet, CreditCard, Sparkles, TrendingUp, ArrowRight } from 'lucide-react'
import { CashflowProjection } from '../types'
import { formatCurrency, isNonNegativeCents } from '../utils/money'

interface CashflowCardProps {
  projection: CashflowProjection | null
  isPrivacyMode?: boolean
  isLoading?: boolean
  onInspectInvoices?: () => void
}

export const CashflowCard: React.FC<CashflowCardProps> = ({
  projection,
  isPrivacyMode = false,
  isLoading,
  onInspectInvoices,
}) => {
  if (!projection) return null

  const liquidBalance = projection.available_balance || '0.00'
  const unpaidInvoices = projection.total_pending_bills || '0.00'
  const projectedNet = projection.projected_net || '0.00'
  const count = projection.upcoming_bills?.length || 0
  const netPositive = isNonNegativeCents(projectedNet)

  return (
    <div className="bui-card p-5">
      <div className="flex flex-col lg:flex-row items-start lg:items-center justify-between gap-4">
        {/* Title & Subtitle */}
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="text-xs font-bold uppercase tracking-wider text-[var(--bui-accent-blue)] flex items-center gap-1.5">
              <Sparkles className="w-3.5 h-3.5" />
              Projeção de Liquidez Real (Open Finance)
            </span>
            <span className="text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-lime">
              Sem Dupla Contagem
            </span>
          </div>
          <p className="text-xs text-[var(--bui-text-muted)]">
            Saldo bancário líquido menos faturas de cartão de crédito abertas/fechadas pendentes de liquidação.
          </p>
        </div>

        {/* Metrics Grid */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 w-full lg:w-auto">
          {/* Saldo em Contas */}
          <div className="bui-inner px-4 py-2.5">
            <div className="flex items-center gap-1.5 text-[11px] text-[var(--bui-text-secondary)]">
              <Wallet className="w-3.5 h-3.5 text-[var(--bui-lime)]" />
              <span>Reservas Líquidas</span>
            </div>
            <div className="text-sm font-bold font-mono-num text-[var(--bui-text-primary)] mt-1">
              {isLoading ? '...' : formatCurrency(liquidBalance, isPrivacyMode)}
            </div>
          </div>

          {/* Faturas a Pagar */}
          <div
            onClick={onInspectInvoices}
            className={`bui-inner px-4 py-2.5 ${
              onInspectInvoices ? 'cursor-pointer hover:border-[var(--bui-border-strong)]' : ''
            }`}
          >
            <div className="flex items-center justify-between gap-2 text-[11px] text-[var(--bui-text-secondary)]">
              <span className="flex items-center gap-1.5">
                <CreditCard className="w-3.5 h-3.5 text-[var(--bui-amber)]" />
                <span>Faturas Pendentes ({count})</span>
              </span>
              {onInspectInvoices && <ArrowRight className="w-3 h-3 opacity-60" />}
            </div>
            <div className="text-sm font-bold font-mono-num text-[var(--bui-amber)] mt-1">
              {isLoading ? '...' : formatCurrency(unpaidInvoices, isPrivacyMode)}
            </div>
          </div>

          {/* Liquidez Projetada */}
          <div className="bui-inner px-4 py-2.5">
            <div className="flex items-center gap-1.5 text-[11px] text-[var(--bui-text-secondary)]">
              <TrendingUp className="w-3.5 h-3.5 text-[var(--bui-accent-blue)]" />
              <span>Liquidez Livre</span>
            </div>
            <div
              className={`text-sm font-bold font-mono-num mt-1 ${
                netPositive ? 'text-[var(--bui-lime)]' : 'text-[var(--bui-rose)]'
              }`}
            >
              {isLoading ? '...' : formatCurrency(projectedNet, isPrivacyMode)}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
