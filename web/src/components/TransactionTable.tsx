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
  Sparkles,
  FolderKanban,
  RotateCcw,
} from 'lucide-react'
import { Account, Category, Pagination, Transaction } from '../types'
import { formatCurrency, isNonNegativeCents } from '../utils/money'

interface TransactionTableProps {
  transactions: Transaction[]
  accounts: Account[]
  categories: Category[]
  pagination: Pagination
  selectedIds: string[]
  sortBy: string
  sortOrder: string
  isPrivacyMode?: boolean
  hasActiveFilters?: boolean
  onResetFilters?: () => void
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
  isPrivacyMode = false,
  hasActiveFilters = false,
  onResetFilters,
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
    transactions.length > 0 &&
    transactions.every((t) => selectedIds.includes(t.id))

  const renderSortIcon = (col: string) => {
    if (sortBy !== col)
      return <ArrowUpDown className="w-3.5 h-3.5 opacity-40 ml-1 inline" />
    if (sortOrder === 'asc')
      return (
        <ArrowUp className="w-3.5 h-3.5 text-[var(--bui-accent-blue)] ml-1 inline" />
      )
    return (
      <ArrowDown className="w-3.5 h-3.5 text-[var(--bui-accent-blue)] ml-1 inline" />
    )
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
    <div className="bui-card overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs border-collapse">
          <thead>
            <tr
              className="border-b text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)]"
              style={{
                backgroundColor: 'var(--bui-bg-inner)',
                borderColor: 'var(--bui-border)',
              }}
            >
              <th className="py-3 px-4 w-10 text-center">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={onToggleSelectAll}
                  className="rounded border-[var(--bui-border-strong)] cursor-pointer"
                />
              </th>
              <th
                onClick={() => onSort('date')}
                className="py-3 px-4 cursor-pointer hover:text-[var(--bui-text-primary)] transition select-none"
              >
                Data {renderSortIcon('date')}
              </th>
              <th
                onClick={() => onSort('description')}
                className="py-3 px-4 cursor-pointer hover:text-[var(--bui-text-primary)] transition select-none"
              >
                Estabelecimento / Descrição {renderSortIcon('description')}
              </th>
              <th className="py-3 px-4">Conta</th>
              <th className="py-3 px-4">Categoria</th>
              <th className="py-3 px-4">Status / Origem</th>
              <th
                onClick={() => onSort('amount')}
                className="py-3 px-4 text-right cursor-pointer hover:text-[var(--bui-text-primary)] transition select-none"
              >
                Valor {renderSortIcon('amount')}
              </th>
              <th className="py-3 px-4 text-center w-24">Ações</th>
            </tr>
          </thead>

          <tbody className="divide-y divide-[var(--bui-border)]">
            {isLoading ? (
              <tr>
                <td colSpan={8} className="py-12 text-center text-[var(--bui-text-muted)]">
                  <div className="flex flex-col items-center justify-center gap-2">
                    <div className="w-6 h-6 border-2 border-[var(--bui-accent-blue)] border-t-transparent rounded-full animate-spin" />
                    <span className="text-xs">Carregando lançamentos...</span>
                  </div>
                </td>
              </tr>
            ) : transactions.length === 0 ? (
              <tr>
                <td colSpan={8} className="py-14 text-center">
                  <div className="flex flex-col items-center justify-center gap-3 max-w-sm mx-auto">
                    <div className="w-11 h-11 bui-inner flex items-center justify-center text-[var(--bui-text-muted)]">
                      <HelpCircle className="w-5 h-5" />
                    </div>
                    <div>
                      <p className="font-bold text-sm text-[var(--bui-text-primary)]">
                        Nenhuma transação encontrada para este filtro
                      </p>
                      <p className="text-xs text-[var(--bui-text-muted)] mt-1">
                        Ajuste os filtros aplicados, selecione "Ver Todo o Histórico" na barra lateral ou registre um novo lançamento.
                      </p>
                    </div>
                    {hasActiveFilters && onResetFilters && (
                      <button
                        onClick={onResetFilters}
                        className="bui-pill-control px-3.5 py-1.5 text-xs font-semibold flex items-center gap-1.5"
                      >
                        <RotateCcw className="w-3.5 h-3.5" />
                        <span>Limpar Filtros Ativos</span>
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            ) : (
              transactions.map((tx) => {
                const isSelected = selectedIds.includes(tx.id)
                const acc = accountMap.get(tx.account_id)
                const cat = tx.category_id ? categoryMap.get(tx.category_id) : null
                const isIncome =
                  tx.type === 'INCOME' ||
                  (tx.type !== 'EXPENSE' && isNonNegativeCents(tx.amount))
                const isTransfer =
                  tx.type === 'TRANSFER' || Boolean(tx.transfer_peer_transaction_id)
                const tags: string[] = Array.isArray(tx.metadata?.tags)
                  ? tx.metadata.tags
                  : []
                const project: string | undefined = tx.metadata?.project

                return (
                  <tr
                    key={tx.id}
                    className={`hover:bg-[var(--bui-bg-hover)]/45 transition-colors ${
                      isSelected ? 'bg-[var(--bui-blue-bg)]' : ''
                    }`}
                  >
                    {/* Checkbox */}
                    <td className="py-3 px-4 text-center">
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => onToggleSelect(tx.id)}
                        className="rounded border-[var(--bui-border-strong)] cursor-pointer"
                      />
                    </td>

                    {/* Data */}
                    <td className="py-3 px-4 font-mono-num text-[11px] text-[var(--bui-text-secondary)] whitespace-nowrap">
                      {formatDate(tx.date)}
                    </td>

                    {/* Descrição + Tags / Project */}
                    <td className="py-3 px-4">
                      <div
                        className="font-semibold text-[var(--bui-text-primary)] max-w-xs md:max-w-md truncate"
                        title={tx.description}
                      >
                        {tx.description}
                      </div>
                      {(tx.notes || tags.length > 0 || project) && (
                        <div className="flex items-center gap-1.5 flex-wrap mt-1">
                          {project && (
                            <span className="inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded bui-badge-blue font-medium">
                              <FolderKanban className="w-2.5 h-2.5" /> {project}
                            </span>
                          )}
                          {tags.map((t) => (
                            <span
                              key={t}
                              className="text-[10px] px-1.5 py-0.5 rounded bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-[var(--bui-text-secondary)]"
                            >
                              #{t}
                            </span>
                          ))}
                          {tx.notes && (
                            <span className="text-[11px] text-[var(--bui-text-muted)] truncate max-w-xs">
                              {tx.notes}
                            </span>
                          )}
                        </div>
                      )}
                    </td>

                    {/* Conta */}
                    <td className="py-3 px-4 whitespace-nowrap">
                      {acc ? (
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] font-semibold bui-inner">
                          <span
                            className="w-2 h-2 rounded-full"
                            style={{ backgroundColor: acc.color_hex || '#2563EB' }}
                          />
                          {acc.name}
                        </span>
                      ) : (
                        <span className="text-xs text-[var(--bui-text-muted)]">-</span>
                      )}
                    </td>

                    {/* Categoria (Inline 1-Click Selector) */}
                    <td className="py-3 px-4 relative whitespace-nowrap">
                      {activeInlineTxId === tx.id ? (
                        <select
                          autoFocus
                          defaultValue={tx.category_id || ''}
                          onChange={(e) => {
                            const val = e.target.value
                            onInlineCategoryChange(tx.id, val ? val : null)
                            setActiveInlineTxId(null)
                          }}
                          onBlur={() => setActiveInlineTxId(null)}
                          className="text-xs py-1 px-2 rounded-lg bg-[var(--bui-bg-inner)] border border-[var(--bui-accent-blue)] text-[var(--bui-text-primary)] focus:outline-none"
                        >
                          <option value="">Sem Categoria</option>
                          {categories.map((c) => (
                            <option key={c.id} value={c.id}>
                              {c.name}
                            </option>
                          ))}
                        </select>
                      ) : (
                        <button
                          onClick={() => setActiveInlineTxId(tx.id)}
                          className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] font-medium transition ${
                            cat ? 'bui-pill-control' : 'bui-badge-amber'
                          }`}
                          title="Clique para classificar em 1 clique"
                        >
                          <Tag className="w-3 h-3" />
                          <span>{cat ? cat.name : 'Classificar'}</span>
                        </button>
                      )}
                    </td>

                    {/* Status / Origem */}
                    <td className="py-3 px-4 whitespace-nowrap">
                      <div className="flex items-center gap-1.5">
                        {isTransfer ? (
                          <span className="text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-blue">
                            Transferência
                          </span>
                        ) : isIncome ? (
                          <span className="text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-lime">
                            Receita
                          </span>
                        ) : (
                          <span className="text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-rose">
                            Despesa
                          </span>
                        )}
                        {tx.is_ai_categorized && (
                          <span
                            className="p-1 rounded bui-badge-amber"
                            title="Categorizado via Inteligência / Regra"
                          >
                            <Sparkles className="w-2.5 h-2.5" />
                          </span>
                        )}
                      </div>
                    </td>

                    {/* Valor */}
                    <td
                      className={`py-3 px-4 text-right font-mono-num font-bold whitespace-nowrap ${
                        isTransfer
                          ? 'text-[var(--bui-accent-blue)]'
                          : isIncome
                          ? 'text-[var(--bui-lime)]'
                          : 'text-[var(--bui-text-primary)]'
                      }`}
                    >
                      {formatCurrency(tx.amount, isPrivacyMode)}
                    </td>

                    {/* Ações */}
                    <td className="py-3 px-4 text-center whitespace-nowrap">
                      <div className="flex items-center justify-center gap-1">
                        <button
                          onClick={() => onEdit(tx)}
                          className="p-1.5 rounded-lg text-[var(--bui-text-muted)] hover:text-[var(--bui-accent-blue)] hover:bg-[var(--bui-bg-hover)] transition"
                          title="Editar transação"
                        >
                          <Edit2 className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => onDelete(tx)}
                          className="p-1.5 rounded-lg text-[var(--bui-text-muted)] hover:text-[var(--bui-rose)] hover:bg-[var(--bui-rose-bg)] transition"
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
      <div
        className="p-4 border-t flex flex-wrap items-center justify-between gap-3 text-xs text-[var(--bui-text-secondary)]"
        style={{
          backgroundColor: 'var(--bui-bg-inner)',
          borderColor: 'var(--bui-border)',
        }}
      >
        <div>
          Página{' '}
          <span className="font-bold font-mono-num text-[var(--bui-text-primary)]">
            {pagination.page}
          </span>{' '}
          de{' '}
          <span className="font-bold font-mono-num text-[var(--bui-text-primary)]">
            {pagination.total_pages || 1}
          </span>{' '}
          •{' '}
          <span className="font-mono-num font-semibold">
            {pagination.total}
          </span>{' '}
          lançamentos encontrados
        </div>

        <div className="flex items-center gap-2">
          <button
            disabled={pagination.page <= 1}
            onClick={() => onPageChange(pagination.page - 1)}
            className="bui-pill-control px-3 py-1.5 text-xs font-semibold flex items-center gap-1 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <ChevronLeft className="w-3.5 h-3.5" />
            <span>Anterior</span>
          </button>
          <button
            disabled={pagination.page >= pagination.total_pages}
            onClick={() => onPageChange(pagination.page + 1)}
            className="bui-pill-control px-3 py-1.5 text-xs font-semibold flex items-center gap-1 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <span>Próxima</span>
            <ChevronRight className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>
  )
}
