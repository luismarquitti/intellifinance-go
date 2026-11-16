import React, { useEffect, useId, useRef, useState } from 'react'
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
  Clock,
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
  const selectAllRef = useRef<HTMLInputElement>(null)
  const captionId = useId()

  const accountMap = new Map(accounts.map((a) => [a.id, a]))
  const categoryMap = new Map(categories.map((c) => [c.id, c]))

  const allSelected =
    transactions.length > 0 &&
    transactions.every((t) => selectedIds.includes(t.id))
  const someSelected =
    transactions.some((t) => selectedIds.includes(t.id)) && !allSelected

  useEffect(() => {
    if (selectAllRef.current) {
      selectAllRef.current.indeterminate = someSelected
    }
  }, [someSelected])

  const renderSortIcon = (col: string) => {
    if (sortBy !== col) {
      return <ArrowUpDown className="w-3.5 h-3.5 opacity-40 inline" aria-hidden="true" />
    }
    if (sortOrder === 'asc') {
      return <ArrowUp className="w-3.5 h-3.5 text-[var(--bui-accent-blue)] inline" aria-hidden="true" />
    }
    return <ArrowDown className="w-3.5 h-3.5 text-[var(--bui-accent-blue)] inline" aria-hidden="true" />
  }

  const getAriaSort = (col: string): 'ascending' | 'descending' | undefined => {
    if (sortBy !== col) return undefined
    return sortOrder === 'asc' ? 'ascending' : 'descending'
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
      {/* Mobile Stacked Card Layout (< md) */}
      <div className="md:hidden divide-y divide-[var(--bui-border)]">
        {transactions.length > 0 && !isLoading && (
          <div className="px-4 py-2.5 bg-[var(--bui-bg-inner)] flex items-center justify-between text-xs">
            <label className="inline-flex items-center gap-2 font-semibold text-[var(--bui-text-secondary)] cursor-pointer">
              <input
                type="checkbox"
                checked={allSelected}
                onChange={onToggleSelectAll}
                className="rounded border-[var(--bui-border-strong)] cursor-pointer"
              />
              <span>Selecionar página ({transactions.length})</span>
            </label>
            <div className="flex items-center gap-1.5">
              <button
                type="button"
                onClick={() => onSort('date')}
                className="bui-pill-control px-2 py-1 text-[11px] font-semibold inline-flex items-center gap-1"
              >
                Data {renderSortIcon('date')}
              </button>
              <button
                type="button"
                onClick={() => onSort('amount')}
                className="bui-pill-control px-2 py-1 text-[11px] font-semibold inline-flex items-center gap-1"
              >
                Valor {renderSortIcon('amount')}
              </button>
            </div>
          </div>
        )}

        {isLoading ? (
          <div className="py-12 text-center text-[var(--bui-text-muted)]">
            <div className="flex flex-col items-center justify-center gap-2">
              <div className="w-6 h-6 border-2 border-[var(--bui-accent-blue)] border-t-transparent rounded-full animate-spin" />
              <span className="text-xs">Carregando lançamentos...</span>
            </div>
          </div>
        ) : transactions.length === 0 ? (
          <div className="py-12 px-4 text-center">
            <div className="flex flex-col items-center justify-center gap-3 max-w-sm mx-auto">
              <div className="w-11 h-11 bui-inner flex items-center justify-center text-[var(--bui-text-muted)]">
                <HelpCircle className="w-5 h-5" aria-hidden="true" />
              </div>
              <div>
                <p className="font-bold text-sm text-[var(--bui-text-primary)]">
                  Nenhuma transação encontrada para este filtro
                </p>
                <p className="text-xs text-[var(--bui-text-muted)] mt-1">
                  Ajuste os filtros aplicados, selecione &quot;Todo o Histórico&quot; no cabeçalho ou registre um novo lançamento.
                </p>
              </div>
              {hasActiveFilters && onResetFilters && (
                <button
                  type="button"
                  onClick={onResetFilters}
                  className="bui-pill-control px-3.5 py-2 text-xs font-semibold flex items-center gap-1.5"
                >
                  <RotateCcw className="w-3.5 h-3.5" aria-hidden="true" />
                  <span>Limpar Filtros Ativos</span>
                </button>
              )}
            </div>
          </div>
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
            const isFutureOrPending =
              tx.status === 'PENDING' || Boolean(tx.metadata?.is_future)

            return (
              <article
                key={tx.id}
                className={`p-4 space-y-2.5 ${
                  isSelected
                    ? 'bg-[var(--bui-blue-bg)]'
                    : isFutureOrPending
                    ? 'bg-amber-500/[0.04] border-l-2 border-l-[var(--bui-amber)]'
                    : ''
                }`}
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="flex items-start gap-2.5 min-w-0">
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={() => onToggleSelect(tx.id)}
                      aria-label={`Selecionar ${tx.description}`}
                      className="mt-1 rounded border-[var(--bui-border-strong)] cursor-pointer shrink-0"
                    />
                    <div className="min-w-0">
                      <div className="text-xs font-bold text-[var(--bui-text-primary)] break-words">
                        {tx.description}
                      </div>
                      <div className="text-[11px] font-mono-num text-[var(--bui-text-muted)] mt-0.5 flex items-center gap-2 flex-wrap">
                        <span>{formatDate(tx.date)}</span>
                        {acc && <span>• {acc.name}</span>}
                      </div>
                    </div>
                  </div>
                  <div
                    className={`text-sm font-mono-num font-bold shrink-0 ${
                      isFutureOrPending
                        ? 'text-[var(--bui-amber)]'
                        : isTransfer
                        ? 'text-[var(--bui-accent-blue)]'
                        : isIncome
                        ? 'text-[var(--bui-lime)]'
                        : 'text-[var(--bui-text-primary)]'
                    }`}
                  >
                    {formatCurrency(tx.amount, isPrivacyMode, {
                      showPlusSign: isIncome && !isTransfer,
                    })}
                  </div>
                </div>

                <div className="flex items-center justify-between gap-2 flex-wrap pl-6">
                  <div className="flex items-center gap-1.5 flex-wrap">
                    {activeInlineTxId === tx.id ? (
                      <select
                        autoFocus
                        aria-label={`Categoria para ${tx.description}`}
                        defaultValue={tx.category_id || ''}
                        onChange={(e) => {
                          const val = e.target.value
                          onInlineCategoryChange(tx.id, val ? val : null)
                          setActiveInlineTxId(null)
                        }}
                        onBlur={() => setActiveInlineTxId(null)}
                        className="text-xs py-1 px-2 rounded-lg bg-[var(--bui-bg-inner)] border border-[var(--bui-accent-blue)] text-[var(--bui-text-primary)]"
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
                        type="button"
                        onClick={() => setActiveInlineTxId(tx.id)}
                        className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-lg text-[11px] font-medium ${
                          cat ? 'bui-pill-control' : 'bui-badge-amber'
                        }`}
                      >
                        <Tag className="w-3 h-3" aria-hidden="true" />
                        <span>{cat ? cat.name : 'Classificar'}</span>
                      </button>
                    )}

                    {isTransfer ? (
                      <span className="text-[11px] font-semibold px-2 py-0.5 rounded-md bui-badge-blue">
                        Transferência
                      </span>
                    ) : isIncome ? (
                      <span className="text-[11px] font-semibold px-2 py-0.5 rounded-md bui-badge-lime">
                        Receita
                      </span>
                    ) : (
                      <span className="text-[11px] font-semibold px-2 py-0.5 rounded-md bui-badge-rose">
                        Despesa
                      </span>
                    )}
                  </div>

                  <div className="flex items-center gap-1">
                    <button
                      type="button"
                      onClick={() => onEdit(tx)}
                      aria-label={`Editar ${tx.description}`}
                      className="p-2 rounded-lg text-[var(--bui-text-muted)] hover:text-[var(--bui-accent-blue)] hover:bg-[var(--bui-bg-hover)] transition"
                    >
                      <Edit2 className="w-4 h-4" aria-hidden="true" />
                    </button>
                    <button
                      type="button"
                      onClick={() => onDelete(tx)}
                      aria-label={`Excluir ${tx.description}`}
                      className="p-2 rounded-lg text-[var(--bui-text-muted)] hover:text-[var(--bui-rose)] hover:bg-[var(--bui-rose-bg)] transition"
                    >
                      <Trash2 className="w-4 h-4" aria-hidden="true" />
                    </button>
                  </div>
                </div>
              </article>
            )
          })
        )}
      </div>

      {/* Desktop Semantic Table (>= md) */}
      <div
        tabIndex={0}
        role="region"
        aria-labelledby={captionId}
        className="hidden md:block bui-table-container overflow-x-auto"
      >
        <table className="w-full text-left text-xs border-collapse">
          <caption id={captionId} className="sr-only">
            Lançamentos financeiros filtrados
          </caption>
          <thead>
            <tr className="border-b border-[var(--bui-border)] bg-[var(--bui-bg-inner)] text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)]">
              <th scope="col" className="py-3 px-4 w-10 text-center">
                <input
                  ref={selectAllRef}
                  type="checkbox"
                  checked={allSelected}
                  onChange={onToggleSelectAll}
                  aria-label="Selecionar todas as transações desta página"
                  className="rounded border-[var(--bui-border-strong)] cursor-pointer"
                />
              </th>
              <th scope="col" aria-sort={getAriaSort('date')} className="px-2">
                <button
                  type="button"
                  onClick={() => onSort('date')}
                  className="w-full py-3 px-2 text-left inline-flex items-center gap-1 font-bold uppercase tracking-wider hover:text-[var(--bui-text-primary)] transition"
                >
                  <span>Data</span>
                  {renderSortIcon('date')}
                </button>
              </th>
              <th scope="col" aria-sort={getAriaSort('description')} className="px-2">
                <button
                  type="button"
                  onClick={() => onSort('description')}
                  className="w-full py-3 px-2 text-left inline-flex items-center gap-1 font-bold uppercase tracking-wider hover:text-[var(--bui-text-primary)] transition"
                >
                  <span>Estabelecimento / Descrição</span>
                  {renderSortIcon('description')}
                </button>
              </th>
              <th scope="col" className="py-3 px-4">
                Conta
              </th>
              <th scope="col" className="py-3 px-4">
                Categoria
              </th>
              <th scope="col" className="py-3 px-4">
                Status / Origem
              </th>
              <th scope="col" aria-sort={getAriaSort('amount')} className="px-2 text-right">
                <button
                  type="button"
                  onClick={() => onSort('amount')}
                  className="w-full py-3 px-2 justify-end inline-flex items-center gap-1 font-bold uppercase tracking-wider hover:text-[var(--bui-text-primary)] transition"
                >
                  <span>Valor</span>
                  {renderSortIcon('amount')}
                </button>
              </th>
              <th scope="col" className="py-3 px-4 text-center w-24">
                Ações
              </th>
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
                      <HelpCircle className="w-5 h-5" aria-hidden="true" />
                    </div>
                    <div>
                      <p className="font-bold text-sm text-[var(--bui-text-primary)]">
                        Nenhuma transação encontrada para este filtro
                      </p>
                      <p className="text-xs text-[var(--bui-text-muted)] mt-1">
                        Ajuste os filtros aplicados, selecione &quot;Todo o Histórico&quot; no cabeçalho ou registre um novo lançamento.
                      </p>
                    </div>
                    {hasActiveFilters && onResetFilters && (
                      <button
                        type="button"
                        onClick={onResetFilters}
                        className="bui-pill-control px-3.5 py-1.5 text-xs font-semibold flex items-center gap-1.5"
                      >
                        <RotateCcw className="w-3.5 h-3.5" aria-hidden="true" />
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
                const isFutureOrPending =
                  tx.status === 'PENDING' || Boolean(tx.metadata?.is_future)
                const tags: string[] = Array.isArray(tx.metadata?.tags)
                  ? tx.metadata.tags
                  : []
                const project: string | undefined = tx.metadata?.project

                return (
                  <tr
                    key={tx.id}
                    className={`hover:bg-[var(--bui-bg-hover)]/45 transition-colors ${
                      isSelected
                        ? 'bg-[var(--bui-blue-bg)]'
                        : isFutureOrPending
                        ? 'bg-amber-500/[0.04] border-l-2 border-l-[var(--bui-amber)]'
                        : ''
                    }`}
                  >
                    {/* Checkbox */}
                    <td className="py-3 px-4 text-center">
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => onToggleSelect(tx.id)}
                        aria-label={`Selecionar transação ${tx.description}`}
                        className="rounded border-[var(--bui-border-strong)] cursor-pointer"
                      />
                    </td>

                    {/* Data */}
                    <td className="py-3 px-4 font-mono-num text-[11px] text-[var(--bui-text-secondary)] whitespace-nowrap">
                      <div>{formatDate(tx.date)}</div>
                      {isFutureOrPending && (
                        <div className="mt-0.5 inline-flex items-center gap-1 text-[11px] font-sans font-semibold text-[var(--bui-amber)]">
                          <Clock className="w-3 h-3" aria-hidden="true" />
                          <span>Agendado</span>
                        </div>
                      )}
                    </td>

                    {/* Descrição + Tags / Project */}
                    <td className="py-3 px-4">
                      <div
                        className="font-semibold text-[var(--bui-text-primary)] max-w-xs lg:max-w-md truncate"
                        title={tx.description}
                      >
                        {tx.description}
                      </div>
                      {(tx.notes || tags.length > 0 || project) && (
                        <div className="flex items-center gap-1.5 flex-wrap mt-1">
                          {project && (
                            <span className="inline-flex items-center gap-1 text-[11px] px-1.5 py-0.5 rounded bui-badge-blue font-medium">
                              <FolderKanban className="w-3 h-3" aria-hidden="true" /> {project}
                            </span>
                          )}
                          {tags.map((t) => (
                            <span
                              key={t}
                              className="text-[11px] px-1.5 py-0.5 rounded bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-[var(--bui-text-secondary)]"
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
                            aria-hidden="true"
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
                          aria-label={`Categoria para ${tx.description}`}
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
                          type="button"
                          onClick={() => setActiveInlineTxId(tx.id)}
                          aria-label={`Alterar categoria de ${tx.description}`}
                          className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] font-medium transition ${
                            cat ? 'bui-pill-control' : 'bui-badge-amber'
                          }`}
                          title="Clique para classificar em 1 clique"
                        >
                          <Tag className="w-3 h-3" aria-hidden="true" />
                          <span>{cat ? cat.name : 'Classificar'}</span>
                        </button>
                      )}
                    </td>

                    {/* Status / Origem */}
                    <td className="py-3 px-4 whitespace-nowrap">
                      <div className="flex items-center gap-1.5 flex-wrap">
                        {isTransfer ? (
                          <span className="text-[11px] font-semibold px-2 py-0.5 rounded-md bui-badge-blue">
                            Transferência
                          </span>
                        ) : isIncome ? (
                          <span className="text-[11px] font-semibold px-2 py-0.5 rounded-md bui-badge-lime">
                            Receita
                          </span>
                        ) : (
                          <span className="text-[11px] font-semibold px-2 py-0.5 rounded-md bui-badge-rose">
                            Despesa
                          </span>
                        )}
                        {isFutureOrPending && (
                          <span
                            className="inline-flex items-center gap-1 text-[11px] font-semibold px-2 py-0.5 rounded-md bui-badge-amber"
                            title="Lançamento agendado para data futura — ainda não consolidado no saldo atual"
                          >
                            <Clock className="w-3 h-3" aria-hidden="true" />
                            <span>Futuro</span>
                          </span>
                        )}
                        {tx.is_ai_categorized && (
                          <span
                            className="p-1 rounded bui-badge-amber"
                            title="Categorizado via Inteligência / Regra"
                            aria-label="Categorizado por IA"
                          >
                            <Sparkles className="w-3 h-3" aria-hidden="true" />
                          </span>
                        )}
                      </div>
                    </td>

                    {/* Valor */}
                    <td
                      className={`py-3 px-4 text-right font-mono-num font-bold whitespace-nowrap ${
                        isFutureOrPending
                          ? 'text-[var(--bui-amber)]'
                          : isTransfer
                          ? 'text-[var(--bui-accent-blue)]'
                          : isIncome
                          ? 'text-[var(--bui-lime)]'
                          : 'text-[var(--bui-text-primary)]'
                      }`}
                    >
                      <div>
                        {formatCurrency(tx.amount, isPrivacyMode, {
                          showPlusSign: isIncome && !isTransfer,
                        })}
                      </div>
                      {isFutureOrPending && (
                        <div className="text-[11px] font-sans font-normal text-[var(--bui-text-muted)]">
                          Não consolidado
                        </div>
                      )}
                    </td>

                    {/* Ações */}
                    <td className="py-3 px-4 text-center whitespace-nowrap">
                      <div className="flex items-center justify-center gap-1">
                        <button
                          type="button"
                          onClick={() => onEdit(tx)}
                          aria-label={`Editar ${tx.description}`}
                          className="p-1.5 rounded-lg text-[var(--bui-text-muted)] hover:text-[var(--bui-accent-blue)] hover:bg-[var(--bui-bg-hover)] transition"
                          title="Editar transação"
                        >
                          <Edit2 className="w-3.5 h-3.5" aria-hidden="true" />
                        </button>
                        <button
                          type="button"
                          onClick={() => onDelete(tx)}
                          aria-label={`Excluir ${tx.description}`}
                          className="p-1.5 rounded-lg text-[var(--bui-text-muted)] hover:text-[var(--bui-rose)] hover:bg-[var(--bui-rose-bg)] transition"
                          title="Excluir transação"
                        >
                          <Trash2 className="w-3.5 h-3.5" aria-hidden="true" />
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
      <div className="p-4 border-t border-[var(--bui-border)] bg-[var(--bui-bg-inner)] flex flex-wrap items-center justify-between gap-3 text-xs text-[var(--bui-text-secondary)]">
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
            type="button"
            disabled={pagination.page <= 1}
            onClick={() => onPageChange(pagination.page - 1)}
            className="bui-pill-control px-3 py-1.5 text-xs font-semibold flex items-center gap-1 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <ChevronLeft className="w-3.5 h-3.5" aria-hidden="true" />
            <span>Anterior</span>
          </button>
          <button
            type="button"
            disabled={pagination.page >= pagination.total_pages}
            onClick={() => onPageChange(pagination.page + 1)}
            className="bui-pill-control px-3 py-1.5 text-xs font-semibold flex items-center gap-1 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <span>Próxima</span>
            <ChevronRight className="w-3.5 h-3.5" aria-hidden="true" />
          </button>
        </div>
      </div>
    </div>
  )
}
