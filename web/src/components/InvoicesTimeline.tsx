import React, { useEffect, useState } from 'react'
import {
  CreditCard,
  Calendar,
  CheckCircle2,
  Clock,
  AlertCircle,
  Tag,
  Receipt,
} from 'lucide-react'
import { Account, Category, Invoice, Transaction } from '../types'
import { fetchInvoiceDetails } from '../api'
import { formatCurrency, isNegativeCents } from '../utils/money'

interface InvoicesTimelineProps {
  invoices: Invoice[]
  accounts: Account[]
  categories: Category[]
  isPrivacyMode?: boolean
  onInlineCategoryChange: (txId: string, categoryId: string | null) => void
}

export const InvoicesTimeline: React.FC<InvoicesTimelineProps> = ({
  invoices,
  accounts,
  categories,
  isPrivacyMode = false,
  onInlineCategoryChange,
}) => {
  const [selectedInvoiceId, setSelectedInvoiceId] = useState<string | null>(
    invoices[0]?.id || null
  )
  const [invoiceTransactions, setInvoiceTransactions] = useState<Transaction[]>([])
  const [isLoadingDetails, setIsLoadingDetails] = useState(false)
  const [editingCatTxId, setEditingCatTxId] = useState<string | null>(null)

  useEffect(() => {
    if (!selectedInvoiceId && invoices.length > 0) {
      setSelectedInvoiceId(invoices[0].id)
    }
  }, [invoices, selectedInvoiceId])

  useEffect(() => {
    if (!selectedInvoiceId) return
    let active = true
    const loadDetails = async () => {
      try {
        setIsLoadingDetails(true)
        const details = await fetchInvoiceDetails(selectedInvoiceId)
        if (active) {
          setInvoiceTransactions(details.transactions || [])
        }
      } catch (err) {
        console.error('Failed to load invoice details:', err)
      } finally {
        if (active) setIsLoadingDetails(false)
      }
    }
    loadDetails()
    return () => {
      active = false
    }
  }, [selectedInvoiceId])

  const accountMap = new Map(accounts.map((a) => [a.id, a]))
  const categoryMap = new Map(categories.map((c) => [c.id, c]))
  const selectedInvoice = invoices.find((i) => i.id === selectedInvoiceId) || null

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '-'
    const parts = dateStr.split('T')[0].split('-')
    if (parts.length === 3) {
      return `${parts[2]}/${parts[1]}/${parts[0]}`
    }
    return dateStr
  }

  const formatMonthShort = (dateStr?: string) => {
    if (!dateStr) return '-'
    const parts = dateStr.split('T')[0].split('-')
    if (parts.length !== 3) return dateStr
    const months = [
      'Jan',
      'Fev',
      'Mar',
      'Abr',
      'Mai',
      'Jun',
      'Jul',
      'Ago',
      'Set',
      'Out',
      'Nov',
      'Dez',
    ]
    const mIdx = Math.max(0, Math.min(11, parseInt(parts[1], 10) - 1))
    return `${months[mIdx]} ${parts[0]}`
  }

  const renderStatusBadge = (status: Invoice['status']) => {
    switch (status) {
      case 'PAID':
        return (
          <span className="inline-flex items-center gap-1 text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-lime">
            <CheckCircle2 className="w-3 h-3" /> Paga
          </span>
        )
      case 'OPEN':
        return (
          <span className="inline-flex items-center gap-1 text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-blue">
            <Clock className="w-3 h-3" /> Aberta
          </span>
        )
      case 'CLOSED':
        return (
          <span className="inline-flex items-center gap-1 text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-amber">
            <AlertCircle className="w-3 h-3" /> Fechada
          </span>
        )
      default:
        return (
          <span className="inline-flex items-center gap-1 text-[10px] font-semibold px-2 py-0.5 rounded-md bui-badge-rose">
            <AlertCircle className="w-3 h-3" /> Vencida
          </span>
        )
    }
  }

  return (
    <div className="bui-card p-5 space-y-5">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <div className="w-9 h-9 bui-inner flex items-center justify-center text-[var(--bui-accent-blue)]">
            <CreditCard className="w-4 h-4" />
          </div>
          <div>
            <h3 className="text-sm font-bold text-[var(--bui-text-primary)]">
              Hub de Faturas de Cartão de Crédito ({invoices.length} ciclos)
            </h3>
            <p className="text-xs text-[var(--bui-text-muted)]">
              Selecione qualquer ciclo de fatura para auditar os lançamentos vinculados ao <code className="font-mono-num">bill_id</code>.
            </p>
          </div>
        </div>
      </div>

      {invoices.length === 0 ? (
        <div className="bui-inner p-8 text-center space-y-2">
          <Receipt className="w-8 h-8 text-[var(--bui-text-muted)] mx-auto" />
          <div className="text-sm font-semibold text-[var(--bui-text-primary)]">
            Nenhuma fatura sincronizada ainda
          </div>
          <p className="text-xs text-[var(--bui-text-muted)]">
            As faturas do cartão conectadas via Open Finance aparecerão automaticamente aqui.
          </p>
        </div>
      ) : (
        <>
          {/* Horizontal Scrollable Timeline of Invoices */}
          <div className="flex items-stretch gap-3 overflow-x-auto pb-2">
            {invoices.map((inv) => {
              const isSelected = inv.id === selectedInvoiceId
              const acc = accountMap.get(inv.account_id)
              return (
                <button
                  key={inv.id}
                  onClick={() => setSelectedInvoiceId(inv.id)}
                  className={`min-w-[205px] text-left p-3.5 rounded-xl border transition-all shrink-0 flex flex-col justify-between ${
                    isSelected
                      ? 'bg-[var(--bui-bg-inner)] border-[var(--bui-accent-blue)] ring-2 ring-[var(--bui-accent-blue)]/25'
                      : 'bui-inner hover:border-[var(--bui-border-strong)]'
                  }`}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-xs font-bold text-[var(--bui-text-primary)]">
                      {formatMonthShort(inv.due_date)}
                    </span>
                    {renderStatusBadge(inv.status)}
                  </div>

                  <div className="my-2.5">
                    <div className="text-base font-bold font-mono-num text-[var(--bui-text-primary)]">
                      {formatCurrency(inv.total_amount, isPrivacyMode)}
                    </div>
                    <div className="text-[10px] text-[var(--bui-text-muted)] truncate mt-0.5">
                      {acc?.name || 'Cartão de Crédito'}
                    </div>
                  </div>

                  <div className="flex items-center justify-between text-[10px] text-[var(--bui-text-muted)] pt-2 border-t border-[var(--bui-border)]">
                    <span className="flex items-center gap-1">
                      <Calendar className="w-3 h-3" /> Venc: {formatDate(inv.due_date)}
                    </span>
                  </div>
                </button>
              )
            })}
          </div>

          {/* Selected Invoice Drill-down */}
          {selectedInvoice && (
            <div className="bui-inner overflow-hidden">
              <div className="p-4 border-b border-[var(--bui-border)] flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-3">
                  <div>
                    <div className="flex items-center gap-2">
                      <h4 className="text-xs font-bold uppercase tracking-wider text-[var(--bui-text-primary)]">
                        Fatura {formatMonthShort(selectedInvoice.due_date)}
                      </h4>
                      {renderStatusBadge(selectedInvoice.status)}
                    </div>
                    <p className="text-xs text-[var(--bui-text-muted)] mt-0.5">
                      Vencimento em {formatDate(selectedInvoice.due_date)} • Pagamento mínimo:{' '}
                      <span className="font-mono-num font-semibold text-[var(--bui-text-secondary)]">
                        {formatCurrency(selectedInvoice.minimum_payment, isPrivacyMode)}
                      </span>
                    </p>
                  </div>
                </div>

                <div className="text-right">
                  <span className="text-[10px] uppercase tracking-wider text-[var(--bui-text-muted)] block">
                    Total da Fatura ({invoiceTransactions.length} itens)
                  </span>
                  <span className="text-lg font-bold font-mono-num text-[var(--bui-text-primary)]">
                    {formatCurrency(selectedInvoice.total_amount, isPrivacyMode)}
                  </span>
                </div>
              </div>

              {/* Invoice Transactions List */}
              <div className="max-h-[380px] overflow-y-auto divide-y divide-[var(--bui-border)]">
                {isLoadingDetails ? (
                  <div className="p-8 text-center text-xs text-[var(--bui-text-muted)]">
                    Carregando lançamentos da fatura...
                  </div>
                ) : invoiceTransactions.length === 0 ? (
                  <div className="p-8 text-center text-xs text-[var(--bui-text-muted)]">
                    Nenhum lançamento individual vinculado a este ciclo de fatura.
                  </div>
                ) : (
                  invoiceTransactions.map((tx) => {
                    const cat = tx.category_id ? categoryMap.get(tx.category_id) : null
                    const isExpense = tx.type === 'EXPENSE' || isNegativeCents(tx.amount)

                    return (
                      <div
                        key={tx.id}
                        className="px-4 py-2.5 flex items-center justify-between gap-3 hover:bg-[var(--bui-bg-hover)]/50 transition text-xs"
                      >
                        <div className="flex items-center gap-3 min-w-0">
                          <span className="font-mono-num text-[11px] text-[var(--bui-text-muted)] shrink-0">
                            {formatDate(tx.date)}
                          </span>
                          <span className="font-medium text-[var(--bui-text-primary)] truncate">
                            {tx.description}
                          </span>
                        </div>

                        <div className="flex items-center gap-3 shrink-0">
                          {editingCatTxId === tx.id ? (
                            <select
                              autoFocus
                              defaultValue={tx.category_id || ''}
                              onChange={(e) => {
                                onInlineCategoryChange(
                                  tx.id,
                                  e.target.value ? e.target.value : null
                                )
                                setEditingCatTxId(null)
                              }}
                              onBlur={() => setEditingCatTxId(null)}
                              className="text-xs py-1 px-2 rounded-lg bg-[var(--bui-bg-card)] border border-[var(--bui-accent-blue)] text-[var(--bui-text-primary)]"
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
                              onClick={() => setEditingCatTxId(tx.id)}
                              className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-medium ${
                                cat ? 'bui-pill-control' : 'bui-badge-amber'
                              }`}
                            >
                              <Tag className="w-2.5 h-2.5" />
                              <span>{cat ? cat.name : 'Classificar'}</span>
                            </button>
                          )}

                          <span
                            className={`font-mono-num font-semibold w-28 text-right ${
                              isExpense
                                ? 'text-[var(--bui-text-primary)]'
                                : 'text-[var(--bui-lime)]'
                            }`}
                          >
                            {formatCurrency(tx.amount, isPrivacyMode)}
                          </span>
                        </div>
                      </div>
                    )
                  })
                )}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}
