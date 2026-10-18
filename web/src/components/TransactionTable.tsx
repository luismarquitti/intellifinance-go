import React, { useState } from 'react'
import {
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  Edit2,
  Trash2,
  Tag,
  ChevronLeft,
  ChevronRight,
  HelpCircle,
} from 'lucide-react'
import { Account, Category, Pagination, Transaction } from '../types'
import { formatCurrency } from './KPICards'

interface TransactionTableProps {
  transactions: Transaction[]
  accounts: Account[]
  categories: Category[]
  pagination: Pagination
  selectedIds: string[]
  sortBy: string
  sortOrder: string
  onToggleSelect: (id: string) => void
  onToggleSelectAll: () => void
  onSort: (column: string) => void
  onPageChange: (newPage: number) => void
  onEdit: (tx: Transaction) => void
  onDelete: (tx: Transaction) => void
  onInlineCategoryChange: (txId: string, categoryId: string | null) => void
  isLoading?: boolean
}

export const TransactionTable: React.FC<TransactionTableProps> = ({
  transactions,
  accounts,
  categories,
  pagination,
  selectedIds,
  sortBy,
  sortOrder,
  onToggleSelect,
  onToggleSelectAll,
  onSort,
  onPageChange,
  onEdit,
  onDelete,
  onInlineCategoryChange,
  isLoading,
}) => {
  const [activeInlineTxId, setActiveInlineTxId] = useState<string | null>(null)

  const accountMap = new Map(accounts.map((a) => [a.id, a]))
  const categoryMap = new Map(categories.map((c) => [c.id, c]))

  const allSelected =
    transactions.length > 0 && transactions.every((t) => selectedIds.includes(t.id))

  const renderSortIcon = (col: string) => {
    if (sortBy !== col) return <ArrowUpDown className="w-3.5 h-3.5 opacity-40 ml-1 inline" />
    if (sortOrder === 'asc') return <ArrowUp className="w-3.5 h-3.5 text-[#A5D6A7] ml-1 inline" />
    return <ArrowDown className="w-3.5 h-3.5 text-[#A5D6A7] ml-1 inline" />
  }

  const formatDate = (dateStr: string) => {
    if (!dateStr) return '-'
    const parts = dateStr.split('T')[0].split('-')
    if (parts.length === 3) {
      return `${parts[2]}/${parts[1]}/${parts[0]}`
    }
    return dateStr
  }

  return (
    <div className="bg-[#161D18] border border-[#2C362F] rounded-2xl overflow-hidden shadow-sm">
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm text-[#E8EFEA] border-collapse">
          <thead>
            <tr className="border-b border-[#2C362F] bg-[#1F2721]/60 text-xs font-semibold text-[#8E9E92] uppercase tracking-wider">
              <th className="py-3.5 px-4 w-12 text-center">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={onToggleSelectAll}
                  className="rounded border-[#2C362F] bg-[#0E1410] text-[#A5D6A7] focus:ring-0 focus:ring-offset-0 cursor-pointer"
                />
              </th>
              <th
                onClick={() => onSort('date')}
                className="py-3.5 px-4 cursor-pointer hover:text-[#E8EFEA] transition-colors select-none"
              >
                Data {renderSortIcon('date')}
              </th>
              <th
                onClick={() => onSort('description')}
                className="py-3.5 px-4 cursor-pointer hover:text-[#E8EFEA] transition-colors select-none"
              >
                Descrição {renderSortIcon('description')}
              </th>
              <th className="py-3.5 px-4">Conta</th>
              <th className="py-3.5 px-4">Categoria</th>
              <th className="py-3.5 px-4">Tipo</th>
              <th
                onClick={() => onSort('amount')}
                className="py-3.5 px-4 text-right cursor-pointer hover:text-[#E8EFEA] transition-colors select-none"
              >
                Valor {renderSortIcon('amount')}
              </th>
              <th className="py-3.5 px-4 text-center w-24">Ações</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[#2C362F]/60">
            {isLoading ? (
              <tr>
                <td colSpan={8} className="py-12 text-center text-[#8E9E92]">
                  <div className="flex flex-col items-center justify-center gap-2">
                    <div className="w-6 h-6 border-2 border-[#A5D6A7] border-t-transparent rounded-full animate-spin" />
                    <span className="text-xs">Carregando transações...</span>
                  </div>
                </td>
              </tr>
            ) : transactions.length === 0 ? (
              <tr>
                <td colSpan={8} className="py-16 text-center text-[#8E9E92]">
                  <div className="flex flex-col items-center justify-center gap-3">
                    <div className="w-12 h-12 rounded-2xl bg-[#1F2721] flex items-center justify-center border border-[#2C362F] text-[#8E9E92]">
                      <HelpCircle className="w-6 h-6" />
                    </div>
                    <div>
                      <p className="font-semibold text-sm text-[#E8EFEA]">Nenhuma transação encontrada</p>
                      <p className="text-xs mt-1">Importe um extrato CSV ou ajuste os filtros acima.</p>
                    </div>
                  </div>
                </td>
              </tr>
            ) : (
              transactions.map((tx) => {
                const isSelected = selectedIds.includes(tx.id)
                const acc = accountMap.get(tx.account_id)
                const cat = tx.category_id ? categoryMap.get(tx.category_id) : null
                const numAmount = parseFloat(tx.amount)
                const isIncome = tx.type === 'INCOME' || numAmount > 0

                return (
                  <tr
                    key={tx.id}
                    className={`hover:bg-[#1F2721]/50 transition-colors ${
                      isSelected ? 'bg-[#A5D6A7]/5' : ''
                    }`}
                  >
                    {/* Checkbox */}
                    <td className="py-3 px-4 text-center">
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => onToggleSelect(tx.id)}
                        className="rounded border-[#2C362F] bg-[#0E1410] text-[#A5D6A7] focus:ring-0 focus:ring-offset-0 cursor-pointer"
                      />
                    </td>

                    {/* Data */}
                    <td className="py-3 px-4 font-mono text-xs text-[#A3B4A7] whitespace-nowrap">
                      {formatDate(tx.date)}
                    </td>

                    {/* Descrição */}
                    <td className="py-3 px-4">
                      <div className="font-medium text-[#E8EFEA] max-w-xs md:max-w-md truncate" title={tx.description}>
                        {tx.description}
                      </div>
                      {tx.notes && (
                        <div className="text-xs text-[#8E9E92] truncate mt-0.5">{tx.notes}</div>
                      )}
                    </td>

                    {/* Conta */}
                    <td className="py-3 px-4 whitespace-nowrap">
                      {acc ? (
                        <span
                          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium border"
                          style={{
                            backgroundColor: `${acc.color_hex}15`,
                            borderColor: `${acc.color_hex}30`,
                            color: acc.color_hex,
                          }}
                        >
                          <span
                            className="w-1.5 h-1.5 rounded-full"
                            style={{ backgroundColor: acc.color_hex }}
                          />
                          {acc.name}
                        </span>
                      ) : (
                        <span className="text-xs text-[#8E9E92]">-</span>
                      )}
                    </td>

                    {/* Categoria (Inline 1-Click Selector) */}
                    <td className="py-3 px-4 relative">
                      {activeInlineTxId === tx.id ? (
                        <div className="flex items-center gap-1.5">
                          <select
                            autoFocus
                            defaultValue={tx.category_id || ''}
                            onChange={(e) => {
                              const val = e.target.value
                              onInlineCategoryChange(tx.id, val ? val : null)
                              setActiveInlineTxId(null)
                            }}
                            onBlur={() => setActiveInlineTxId(null)}
                            className="text-xs py-1 px-2 rounded-lg bg-[#0E1410] border border-[#A5D6A7] text-[#E8EFEA] focus:outline-none"
                          >
                            <option value="">Sem Categoria</option>
                            {categories.map((c) => (
                              <option key={c.id} value={c.id}>
                                {c.name}
                              </option>
                            ))}
                          </select>
                        </div>
                      ) : (
                        <button
                          onClick={() => setActiveInlineTxId(tx.id)}
                          className={`group inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium transition-all ${
                            cat
                              ? 'bg-[#1F2721] text-[#E8EFEA] hover:bg-[#28332B] border border-[#2C362F]'
                              : 'bg-amber-500/10 text-amber-300 hover:bg-amber-500/20 border border-amber-500/30'
                          }`}
                          title="Clique para alterar categoria"
                        >
                          <Tag className="w-3 h-3 text-[#A5D6A7]" />
                          <span>{cat ? cat.name : 'Classificar'}</span>
                        </button>
                      )}
                    </td>

                    {/* Tipo */}
                    <td className="py-3 px-4 whitespace-nowrap">
                      <span
                        className={`inline-block text-[10px] font-bold uppercase tracking-wider px-2 py-0.5 rounded-full ${
                          isIncome
                            ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                            : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                        }`}
                      >
                        {isIncome ? 'Receita' : 'Despesa'}
                      </span>
                    </td>

                    {/* Valor */}
                    <td
                      className={`py-3 px-4 text-right font-mono font-semibold whitespace-nowrap ${
                        isIncome ? 'text-emerald-400' : 'text-rose-400'
                      }`}
                    >
                      {formatCurrency(tx.amount)}
                    </td>

                    {/* Ações */}
                    <td className="py-3 px-4 text-center whitespace-nowrap">
                      <div className="flex items-center justify-center gap-1">
                        <button
                          onClick={() => onEdit(tx)}
                          className="p-1.5 rounded-lg text-[#8E9E92] hover:text-[#A5D6A7] hover:bg-[#1F2721] transition-colors"
                          title="Editar transação"
                        >
                          <Edit2 className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => onDelete(tx)}
                          className="p-1.5 rounded-lg text-[#8E9E92] hover:text-rose-400 hover:bg-rose-500/10 transition-colors"
                          title="Excluir transação"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination Footer */}
      <div className="p-4 border-t border-[#2C362F] flex flex-wrap items-center justify-between gap-3 text-xs text-[#8E9E92]">
        <div>
          Mostrando página <span className="font-bold text-[#E8EFEA]">{pagination.page}</span> de{' '}
          <span className="font-bold text-[#E8EFEA]">{pagination.total_pages || 1}</span> ({pagination.total_items} registros no total)
        </div>

        <div className="flex items-center gap-2">
          <button
            disabled={pagination.page <= 1}
            onClick={() => onPageChange(pagination.page - 1)}
            className="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-[#1F2721] border border-[#2C362F] text-[#E8EFEA] disabled:opacity-40 disabled:cursor-not-allowed hover:bg-[#28332B] transition-colors"
          >
            <ChevronLeft className="w-3.5 h-3.5" />
            <span>Anterior</span>
          </button>
          <button
            disabled={pagination.page >= pagination.total_pages}
            onClick={() => onPageChange(pagination.page + 1)}
            className="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-[#1F2721] border border-[#2C362F] text-[#E8EFEA] disabled:opacity-40 disabled:cursor-not-allowed hover:bg-[#28332B] transition-colors"
          >
            <span>Próxima</span>
            <ChevronRight className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>
  )
}
