import React from 'react'
import { Wallet, CreditCard, Sparkles, TrendingUp } from 'lucide-react'
import { CashflowProjection } from '../types'
import { formatCurrency } from './KPICards'

interface CashflowCardProps {
  projection: CashflowProjection | null
  isLoading?: boolean
}

export const CashflowCard: React.FC<CashflowCardProps> = ({ projection, isLoading }) => {
  if (!projection) return null

  const liquidBalance = parseFloat(projection.current_liquid_balance || '0')
  const unpaidInvoices = parseFloat(projection.total_unpaid_invoices || '0')
  const projectedNet = parseFloat(projection.projected_net_liquidity || '0')
  const count = projection.unpaid_invoices_count || 0

  return (
    <div className="p-5 rounded-2xl bg-gradient-to-r from-[#161D18] to-[#1B241E] border border-[#2C362F] shadow-lg">
      <div className="flex flex-col lg:flex-row items-start lg:items-center justify-between gap-4">
        {/* Title & Badge */}
        <div>
          <div className="flex items-center gap-2">
            <span className="text-xs font-semibold uppercase tracking-wider text-[#A5D6A7] flex items-center gap-1.5">
              <Sparkles className="w-3.5 h-3.5" />
              Inteligência de Fluxo de Caixa (Open Finance)
            </span>
          </div>
          <p className="text-xs text-[#8E9E92] mt-0.5">
            Projeção em tempo real abatendo faturas de cartão de crédito pendentes do saldo bancário disponível.
          </p>
        </div>

        {/* Metrics Grid */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 w-full lg:w-auto">
          {/* Saldo em Conta */}
          <div className="px-4 py-2.5 rounded-xl bg-[#111613] border border-[#242D26]">
            <div className="flex items-center gap-1.5 text-xs text-[#8E9E92]">
              <Wallet className="w-3.5 h-3.5 text-emerald-400" />
              <span>Saldo em Contas</span>
            </div>
            <div className="text-base font-bold text-[#E8EFEA] mt-1">
              {isLoading ? '...' : formatCurrency(liquidBalance)}
            </div>
          </div>

          {/* Faturas a Pagar */}
          <div className="px-4 py-2.5 rounded-xl bg-[#111613] border border-[#242D26]">
            <div className="flex items-center gap-1.5 text-xs text-[#8E9E92]">
              <CreditCard className="w-3.5 h-3.5 text-amber-400" />
              <span>Faturas a Vencer ({count})</span>
            </div>
            <div className="text-base font-bold text-amber-400 mt-1">
              {isLoading ? '...' : formatCurrency(unpaidInvoices)}
            </div>
          </div>

          {/* Liquidez Projetada */}
          <div className="px-4 py-2.5 rounded-xl bg-[#111613] border border-[#242D26]">
            <div className="flex items-center gap-1.5 text-xs text-[#8E9E92]">
              <TrendingUp className="w-3.5 h-3.5 text-[#A5D6A7]" />
              <span>Liquidez Projetada</span>
            </div>
            <div className={`text-base font-bold mt-1 ${
              projectedNet >= 0 ? 'text-[#A5D6A7]' : 'text-rose-400'
            }`}>
              {isLoading ? '...' : formatCurrency(projectedNet)}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
