import React from 'react'
import { ArrowDownLeft, ArrowUpRight, Scale, ReceiptText } from 'lucide-react'
import { TransactionSummary } from '../types'

interface KPICardsProps {
  summary: TransactionSummary | null
  isLoading?: boolean
}

export function formatCurrency(valueStr: string | number): string {
  const num = typeof valueStr === 'string' ? parseFloat(valueStr) : valueStr
  if (isNaN(num)) return 'R$ 0,00'
  return new Intl.NumberFormat('pt-BR', {
    style: 'currency',
    currency: 'BRL',
  }).format(num)
}

export const KPICards: React.FC<KPICardsProps> = ({ summary, isLoading }) => {
  const income = summary ? parseFloat(summary.total_income || '0') : 0
  const expense = summary ? parseFloat(summary.total_expense || '0') : 0
  const net = summary ? parseFloat(summary.net_balance || '0') : 0
  const count = summary ? summary.count : 0

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      {/* Receitas */}
      <div className="p-5 rounded-2xl bg-[#161D18] border border-[#2C362F] hover:border-[#3D4D41] transition-all duration-200">
        <div className="flex items-center justify-between">
          <span className="text-xs font-semibold uppercase tracking-wider text-[#8E9E92]">
            Total de Receitas
          </span>
          <div className="w-9 h-9 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
            <ArrowDownLeft className="w-5 h-5" />
          </div>
        </div>
        <div className="mt-3">
          <div className="text-2xl font-extrabold text-[#E8EFEA] tracking-tight">
            {isLoading ? <span className="opacity-40 animate-pulse">Carregando...</span> : formatCurrency(income)}
          </div>
          <p className="text-xs text-emerald-400 mt-1 flex items-center gap-1">
            <span>● Entradas no período</span>
          </p>
        </div>
      </div>

      {/* Despesas */}
      <div className="p-5 rounded-2xl bg-[#161D18] border border-[#2C362F] hover:border-[#3D4D41] transition-all duration-200">
        <div className="flex items-center justify-between">
          <span className="text-xs font-semibold uppercase tracking-wider text-[#8E9E92]">
            Total de Despesas
          </span>
          <div className="w-9 h-9 rounded-xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-400">
            <ArrowUpRight className="w-5 h-5" />
          </div>
        </div>
        <div className="mt-3">
          <div className="text-2xl font-extrabold text-[#E8EFEA] tracking-tight">
            {isLoading ? <span className="opacity-40 animate-pulse">Carregando...</span> : formatCurrency(expense)}
          </div>
          <p className="text-xs text-rose-400 mt-1 flex items-center gap-1">
            <span>● Saídas no período</span>
          </p>
        </div>
      </div>

      {/* Saldo Líquido */}
      <div className="p-5 rounded-2xl bg-[#161D18] border border-[#2C362F] hover:border-[#3D4D41] transition-all duration-200">
        <div className="flex items-center justify-between">
          <span className="text-xs font-semibold uppercase tracking-wider text-[#8E9E92]">
            Saldo Líquido
          </span>
          <div className={`w-9 h-9 rounded-xl flex items-center justify-center border ${
            net >= 0
              ? 'bg-[#A5D6A7]/10 border-[#A5D6A7]/20 text-[#A5D6A7]'
              : 'bg-rose-500/10 border-rose-500/20 text-rose-400'
          }`}>
            <Scale className="w-5 h-5" />
          </div>
        </div>
        <div className="mt-3">
          <div className={`text-2xl font-extrabold tracking-tight ${
            net >= 0 ? 'text-[#A5D6A7]' : 'text-rose-400'
          }`}>
            {isLoading ? <span className="opacity-40 animate-pulse">Carregando...</span> : formatCurrency(net)}
          </div>
          <p className="text-xs text-[#8E9E92] mt-1">
            {net >= 0 ? 'Superávit acumulado' : 'Déficit no filtro'}
          </p>
        </div>
      </div>

      {/* Total de Transações */}
      <div className="p-5 rounded-2xl bg-[#161D18] border border-[#2C362F] hover:border-[#3D4D41] transition-all duration-200">
        <div className="flex items-center justify-between">
          <span className="text-xs font-semibold uppercase tracking-wider text-[#8E9E92]">
            Volume de Transações
          </span>
          <div className="w-9 h-9 rounded-xl bg-blue-500/10 border border-blue-500/20 flex items-center justify-center text-blue-400">
            <ReceiptText className="w-5 h-5" />
          </div>
        </div>
        <div className="mt-3">
          <div className="text-2xl font-extrabold text-[#E8EFEA] tracking-tight">
            {isLoading ? <span className="opacity-40 animate-pulse">...</span> : count}
          </div>
          <p className="text-xs text-[#8E9E92] mt-1">
            Itens encontrados
          </p>
        </div>
      </div>
    </div>
  )
}
