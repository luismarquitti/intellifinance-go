import React, { useEffect, useMemo, useRef, useState } from 'react'
import {
  CreditCard,
  Calendar,
  CheckCircle2,
  Clock,
  AlertCircle,
  Tag,
  Receipt,
  ChevronLeft,
  ChevronRight,
  Landmark,
  RotateCcw,
  Check,
} from 'lucide-react'
import { Account, Category, Invoice, Transaction } from '../types'
import { fetchInvoiceDetails, payInvoice, unpayInvoice } from '../api'
import { formatCurrency, isNegativeCents } from '../utils/money'

interface InvoicesTimelineProps {
  invoices: Invoice[]
  accounts: Account[]
  categories: Category[]
  isPrivacyMode?: boolean
  selectedMonthCursor?: Date
  onSelectMonthCursor?: (date: Date) => void
  onInlineCategoryChange: (txId: string, categoryId: string | null) => void
  onRefresh?: () => void
}

const MONTHS_SHORT = [
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

const MONTHS_FULL = [
  'Janeiro',
  'Fevereiro',
  'Março',
  'Abril',
  'Maio',
  'Junho',
  'Julho',
  'Agosto',
  'Setembro',
  'Outubro',
  'Novembro',
  'Dezembro',
]

export const InvoicesTimeline: React.FC<InvoicesTimelineProps> = ({
  invoices,
  accounts,
  categories,
  isPrivacyMode = false,
  selectedMonthCursor,
  onSelectMonthCursor,
  onInlineCategoryChange,
  onRefresh,
}) => {
  const creditCards = useMemo(
    () => accounts.filter((a) => a.type === 'CREDIT_CARD' && a.is_active),
    [accounts]
  )
  const liquidAccounts = useMemo(
    () =>
      accounts.filter(
        (a) =>
          (a.type === 'CHECKING' || a.type === 'SAVINGS' || a.type === 'CASH') &&
          a.is_active
      ),
    [accounts]
  )

  const [selectedCardId, setSelectedCardId] = useState<string | null>(
    creditCards[0]?.id || invoices[0]?.account_id || null
  )
  const [selectedInvoiceId, setSelectedInvoiceId] = useState<string | null>(null)
  const [selectedPayAccountId, setSelectedPayAccountId] = useState<string>(
    liquidAccounts[0]?.id || ''
  )
  const [invoiceTransactions, setInvoiceTransactions] = useState<Transaction[]>([])
  const [isLoadingDetails, setIsLoadingDetails] = useState(false)
  const [isMutatingStatus, setIsMutatingStatus] = useState(false)
  const [statusError, setStatusError] = useState<string | null>(null)
  const [editingCatTxId, setEditingCatTxId] = useState<string | null>(null)

  useEffect(() => {
    if (!selectedCardId && (creditCards.length > 0 || invoices.length > 0)) {
      setSelectedCardId(creditCards[0]?.id || invoices[0]?.account_id || null)
    }
  }, [creditCards, invoices, selectedCardId])

  useEffect(() => {
    if (!selectedPayAccountId && liquidAccounts.length > 0) {
      setSelectedPayAccountId(liquidAccounts[0].id)
    }
  }, [liquidAccounts, selectedPayAccountId])

  // Invoices for the active credit card sorted chronologically (oldest -> newest)
  const cardInvoices = useMemo(() => {
    const filtered = selectedCardId
      ? invoices.filter((inv) => inv.account_id === selectedCardId)
      : invoices
    return [...filtered].sort((a, b) => a.due_date.localeCompare(b.due_date))
  }, [invoices, selectedCardId])

  const lastCursorKeyRef = useRef<string | null>(null)

  // Select invoice matching global month cursor on mount/change, or default to current/latest
  useEffect(() => {
    if (cardInvoices.length === 0) {
      setSelectedInvoiceId(null)
      return
    }

    const cursorKey = selectedMonthCursor
      ? `${selectedMonthCursor.getFullYear()}-${String(
          selectedMonthCursor.getMonth() + 1
        ).padStart(2, '0')}`
      : null

    const cursorChanged = cursorKey !== lastCursorKeyRef.current
    lastCursorKeyRef.current = cursorKey

    if (cursorKey && (cursorChanged || !selectedInvoiceId)) {
      const match = cardInvoices.find((inv) => inv.due_date.startsWith(cursorKey))
      if (match) {
        setSelectedInvoiceId(match.id)
        return
      }
    }

    if (!selectedInvoiceId || !cardInvoices.some((i) => i.id === selectedInvoiceId)) {
      const now = new Date()
      const currentYM = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
      const currentMatch =
        cardInvoices.find((inv) => inv.due_date.startsWith(currentYM)) ||
        cardInvoices.find((inv) => inv.status !== 'PAID') ||
        cardInvoices[cardInvoices.length - 1]
      setSelectedInvoiceId(currentMatch?.id || null)
    }
  }, [cardInvoices, selectedMonthCursor, selectedInvoiceId])

  useEffect(() => {
    if (!selectedInvoiceId) {
      setInvoiceTransactions([])
      return
    }
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

  const accountMap = useMemo(() => new Map(accounts.map((a) => [a.id, a])), [accounts])
  const categoryMap = useMemo(() => new Map(categories.map((c) => [c.id, c])), [categories])

  const selectedIndex = cardInvoices.findIndex((i) => i.id === selectedInvoiceId)
  const selectedInvoice = selectedIndex >= 0 ? cardInvoices[selectedIndex] : null

  const selectInvoiceAndNotify = (inv: Invoice) => {
    setSelectedInvoiceId(inv.id)
    setStatusError(null)
    if (onSelectMonthCursor) {
      const parts = inv.due_date.split('T')[0].split('-')
      if (parts.length === 3) {
        onSelectMonthCursor(new Date(Number(parts[0]), Number(parts[1]) - 1, 1))
      }
    }
  }

  const handlePrevInvoice = () => {
    if (selectedIndex > 0) {
      selectInvoiceAndNotify(cardInvoices[selectedIndex - 1])
    }
  }

  const handleNextInvoice = () => {
    if (selectedIndex >= 0 && selectedIndex < cardInvoices.length - 1) {
      selectInvoiceAndNotify(cardInvoices[selectedIndex + 1])
    }
  }

  const handleMarkAsPaid = async () => {
    if (!selectedInvoice) return
    try {
      setIsMutatingStatus(true)
      setStatusError(null)
      await payInvoice(selectedInvoice.id, {
        account_id: selectedPayAccountId || undefined,
      })
      onRefresh?.()
    } catch (err: any) {
      setStatusError(err?.message || 'Erro ao marcar fatura como paga.')
    } finally {
      setIsMutatingStatus(false)
    }
  }

  const handleMarkAsOpen = async () => {
    if (!selectedInvoice) return
    try {
      setIsMutatingStatus(true)
      setStatusError(null)
      await unpayInvoice(selectedInvoice.id)
      onRefresh?.()
    } catch (err: any) {
      setStatusError(err?.message || 'Erro ao reabrir fatura.')
    } finally {
      setIsMutatingStatus(false)
    }
  }

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '-'
    const parts = String(dateStr).split('T')[0].split('-')
    if (parts.length === 3) {
      return `${parts[2]}/${parts[1]}/${parts[0]}`
    }
    return String(dateStr)
  }

  const formatMonthShort = (dateStr?: string) => {
    if (!dateStr) return '-'
    const parts = dateStr.split('T')[0].split('-')
    if (parts.length !== 3) return dateStr
    const mIdx = Math.max(0, Math.min(11, parseInt(parts[1], 10) - 1))
    return `${MONTHS_SHORT[mIdx]} ${parts[0].slice(2)}`
  }

  const formatMonthFull = (dateStr?: string) => {
    if (!dateStr) return '-'
    const parts = dateStr.split('T')[0].split('-')
    if (parts.length !== 3) return dateStr
    const mIdx = Math.max(0, Math.min(11, parseInt(parts[1], 10) - 1))
    return `${MONTHS_FULL[mIdx]} ${parts[0]}`
  }

  const renderStatusBadge = (status: Invoice['status']) => {
    switch (status) {
      case 'PAID':
        return (
          <span className="inline-flex items-center gap-1 text-[11px] font-semibold px-2.5 py-0.5 rounded-md bui-badge-lime">
            <CheckCircle2 className="w-3 h-3" /> Paga
          </span>
        )
      case 'OPEN':
        return (
          <span className="inline-flex items-center gap-1 text-[11px] font-semibold px-2.5 py-0.5 rounded-md bui-badge-blue">
            <Clock className="w-3 h-3" /> Em Aberto
          </span>
        )
      case 'CLOSED':
        return (
          <span className="inline-flex items-center gap-1 text-[11px] font-semibold px-2.5 py-0.5 rounded-md bui-badge-amber">
            <AlertCircle className="w-3 h-3" /> Fechada (Em Aberto)
          </span>
        )
      default:
        return (
          <span className="inline-flex items-center gap-1 text-[11px] font-semibold px-2.5 py-0.5 rounded-md bui-badge-rose">
            <AlertCircle className="w-3 h-3" /> Vencida
          </span>
        )
    }
  }

  return (
    <div className="space-y-5">
      {/* Credit Card Selector Strip */}
      {creditCards.length > 0 && (
        <div className="bui-card p-4">
          <div className="flex flex-wrap items-center justify-between gap-3 mb-3">
            <div>
              <h3 className="text-sm font-bold text-[var(--bui-text-primary)]">
                Cartões de Crédito ({creditCards.length})
              </h3>
              <p className="text-xs text-[var(--bui-text-muted)]">
                Selecione um cartão e navegue mês a mês pelas faturas, como no Organizze.
              </p>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
            {creditCards.map((card) => {
              const isSelected = card.id === selectedCardId
              const invs = invoices.filter((i) => i.account_id === card.id)
              const paidCount = invs.filter((i) => i.status === 'PAID').length
              const openCount = invs.length - paidCount
              return (
                <button
                  key={card.id}
                  onClick={() => setSelectedCardId(card.id)}
                  className={`bui-inner p-3.5 text-left flex items-center justify-between gap-3 transition ${
                    isSelected
                      ? 'ring-2 ring-[var(--bui-accent-blue)] border-transparent'
                      : 'hover:border-[var(--bui-border-strong)]'
                  }`}
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <div
                      className="w-10 h-10 rounded-xl flex items-center justify-center text-white shrink-0"
                      style={{ backgroundColor: card.color_hex || '#18181B' }}
                    >
                      <CreditCard className="w-5 h-5" />
                    </div>
                    <div className="min-w-0">
                      <div className="text-xs font-bold text-[var(--bui-text-primary)] truncate">
                        {card.name}
                      </div>
                      <div className="text-[11px] text-[var(--bui-text-muted)] mt-0.5 flex items-center gap-2">
                        <span className="text-[var(--bui-lime)] font-semibold">
                          {paidCount} pagas
                        </span>
                        <span>•</span>
                        <span className="text-[var(--bui-amber)] font-semibold">
                          {openCount} em aberto
                        </span>
                      </div>
                    </div>
                  </div>
                  <div className="text-right shrink-0">
                    <div className="text-xs font-bold font-mono-num text-[var(--bui-amber)]">
                      {formatCurrency(card.balance, isPrivacyMode)}
                    </div>
                    <div className="text-[10px] text-[var(--bui-text-muted)]">
                      Em aberto
                    </div>
                  </div>
                </button>
              )
            })}
          </div>
        </div>
      )}

      {/* Organizze-Style Monthly Invoice Inspector */}
      <div className="bui-card p-5 space-y-5">
        {cardInvoices.length === 0 ? (
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
            {/* Prominent Month Selector (< Mês Anterior | Novembro 2026 | Próximo Mês >) */}
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-4 border-b border-[var(--bui-border)]">
              <div className="flex items-center gap-2">
                <button
                  onClick={handlePrevInvoice}
                  disabled={selectedIndex <= 0}
                  className="bui-pill-control p-2 disabled:opacity-35 disabled:cursor-not-allowed"
                  title="Fatura do mês anterior"
                >
                  <ChevronLeft className="w-4 h-4" />
                </button>
                <div className="bui-inner px-4 py-1.5 min-w-[200px] text-center">
                  <span className="text-[10px] uppercase tracking-wider text-[var(--bui-text-muted)] block font-semibold">
                    Fatura Mensal
                  </span>
                  <span className="text-sm font-bold text-[var(--bui-text-primary)]">
                    {selectedInvoice ? formatMonthFull(selectedInvoice.due_date) : '-'}
                  </span>
                </div>
                <button
                  onClick={handleNextInvoice}
                  disabled={selectedIndex < 0 || selectedIndex >= cardInvoices.length - 1}
                  className="bui-pill-control p-2 disabled:opacity-35 disabled:cursor-not-allowed"
                  title="Fatura do próximo mês"
                >
                  <ChevronRight className="w-4 h-4" />
                </button>
              </div>

              {selectedInvoice && (
                <div className="flex items-center gap-2.5 flex-wrap">
                  {renderStatusBadge(selectedInvoice.status)}
                  <span className="text-xs text-[var(--bui-text-muted)]">
                    {accountMap.get(selectedInvoice.account_id)?.name || 'Cartão de Crédito'}
                  </span>
                </div>
              )}
            </div>

            {/* Chronological Month Pills Selector */}
            <div className="flex items-stretch gap-2 overflow-x-auto pb-1">
              {cardInvoices.map((inv) => {
                const isSelected = inv.id === selectedInvoiceId
                const isPaid = inv.status === 'PAID'
                return (
                  <button
                    key={inv.id}
                    onClick={() => selectInvoiceAndNotify(inv)}
                    className={`min-w-[122px] text-left px-3 py-2.5 rounded-xl border transition-all shrink-0 flex flex-col justify-between gap-1 ${
                      isSelected
                        ? 'bg-[var(--bui-bg-inner)] border-[var(--bui-accent-blue)] ring-2 ring-[var(--bui-accent-blue)]/25'
                        : 'bui-inner hover:border-[var(--bui-border-strong)]'
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-xs font-bold text-[var(--bui-text-primary)]">
                        {formatMonthShort(inv.due_date)}
                      </span>
                      <span
                        className={`w-2 h-2 rounded-full ${
                          isPaid ? 'bg-[var(--bui-lime)]' : 'bg-[var(--bui-amber)]'
                        }`}
                        title={isPaid ? 'Fatura Paga' : 'Em Aberto'}
                      />
                    </div>
                    <div className="text-xs font-bold font-mono-num text-[var(--bui-text-secondary)]">
                      {formatCurrency(inv.total_amount, isPrivacyMode)}
                    </div>
                  </button>
                )
              })}
            </div>

            {/* Selected Invoice Details & Payment Action Bar */}
            {selectedInvoice && (
              <div className="bui-inner overflow-hidden">
                {/* Summary Header */}
                <div className="p-4 border-b border-[var(--bui-border)] flex flex-col lg:flex-row lg:items-center justify-between gap-4">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2.5 flex-wrap">
                      <h4 className="text-sm font-bold text-[var(--bui-text-primary)]">
                        Fatura de {formatMonthFull(selectedInvoice.due_date)}
                      </h4>
                      {renderStatusBadge(selectedInvoice.status)}
                    </div>
                    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-[var(--bui-text-muted)]">
                      {selectedInvoice.closing_date && (
                        <span className="flex items-center gap-1">
                          <Calendar className="w-3.5 h-3.5" />
                          Fechamento: <strong className="text-[var(--bui-text-secondary)]">{formatDate(selectedInvoice.closing_date)}</strong>
                        </span>
                      )}
                      <span className="flex items-center gap-1">
                        <Calendar className="w-3.5 h-3.5" />
                        Vencimento: <strong className="text-[var(--bui-text-secondary)]">{formatDate(selectedInvoice.due_date)}</strong>
                      </span>
                      <span>
                        Pagamento mínimo:{' '}
                        <strong className="font-mono-num text-[var(--bui-text-secondary)]">
                          {formatCurrency(selectedInvoice.minimum_payment, isPrivacyMode)}
                        </strong>
                      </span>
                    </div>
                  </div>

                  <div className="flex flex-col sm:flex-row sm:items-center gap-4">
                    <div className="sm:text-right">
                      <span className="text-[10px] uppercase tracking-wider text-[var(--bui-text-muted)] block">
                        Valor Total da Fatura ({invoiceTransactions.length} lanç.)
                      </span>
                      <span className="text-xl font-bold font-mono-num text-[var(--bui-text-primary)]">
                        {formatCurrency(selectedInvoice.total_amount, isPrivacyMode)}
                      </span>
                    </div>
                  </div>
                </div>

                {/* Payment Reconciliation / Action Bar */}
                <div className="px-4 py-3 bg-[var(--bui-bg-card)]/60 border-b border-[var(--bui-border)] flex flex-wrap items-center justify-between gap-3">
                  {selectedInvoice.status === 'PAID' ? (
                    <>
                      <div className="flex items-center gap-2 text-xs text-[var(--bui-text-secondary)]">
                        <CheckCircle2 className="w-4 h-4 text-[var(--bui-lime)] shrink-0" />
                        <span>
                          Fatura paga via{' '}
                          <strong className="text-[var(--bui-text-primary)]">
                            {String(selectedInvoice.metadata?.payment_account_name || 'Conta Corrente')}
                          </strong>{' '}
                          em{' '}
                          <strong className="text-[var(--bui-text-primary)]">
                            {formatDate(
                              (selectedInvoice.metadata?.paid_at as string) ||
                                selectedInvoice.due_date
                            )}
                          </strong>
                          {' • '}
                          Valor refletido no saldo da conta corrente.
                        </span>
                      </div>
                      <button
                        onClick={handleMarkAsOpen}
                        disabled={isMutatingStatus}
                        className="bui-pill-control px-3 py-1.5 text-xs font-semibold flex items-center gap-1.5 hover:border-[var(--bui-amber)]"
                      >
                        <RotateCcw className="w-3.5 h-3.5" />
                        <span>{isMutatingStatus ? 'Processando...' : 'Marcar em Aberto'}</span>
                      </button>
                    </>
                  ) : (
                    <>
                      <div className="flex items-center gap-2 text-xs text-[var(--bui-text-secondary)]">
                        <Landmark className="w-4 h-4 text-[var(--bui-accent-blue)] shrink-0" />
                        <span>Conta de pagamento:</span>
                        <select
                          value={selectedPayAccountId}
                          onChange={(e) => setSelectedPayAccountId(e.target.value)}
                          className="text-xs py-1 px-2.5 rounded-lg bg-[var(--bui-bg-card)] border border-[var(--bui-border)] text-[var(--bui-text-primary)]"
                        >
                          {liquidAccounts.map((acc) => (
                            <option key={acc.id} value={acc.id}>
                              {acc.name} ({formatCurrency(acc.balance, isPrivacyMode)})
                            </option>
                          ))}
                        </select>
                      </div>
                      <button
                        onClick={handleMarkAsPaid}
                        disabled={isMutatingStatus || liquidAccounts.length === 0}
                        className="px-3.5 py-1.5 rounded-lg bg-[var(--bui-accent-blue)] hover:bg-[var(--bui-accent-blue-hover)] text-white text-xs font-semibold flex items-center gap-1.5 transition disabled:opacity-50"
                      >
                        <Check className="w-3.5 h-3.5" />
                        <span>
                          {isMutatingStatus ? 'Registrando...' : 'Marcar Fatura como Paga'}
                        </span>
                      </button>
                    </>
                  )}
                </div>

                {statusError && (
                  <div className="px-4 py-2 text-xs text-[var(--bui-rose)] bg-[var(--bui-rose)]/10 border-b border-[var(--bui-border)]">
                    {statusError}
                  </div>
                )}

                {/* Invoice Transactions Table */}
                <div className="max-h-[460px] overflow-y-auto divide-y divide-[var(--bui-border)]">
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
                      const isCredit = tx.type === 'INCOME' || !isNegativeCents(tx.amount)
                      const instNo = Number(tx.metadata?.installment_no || 0)
                      const instTot = Number(tx.metadata?.installments_tot || 0)

                      return (
                        <div
                          key={tx.id}
                          className="px-4 py-2.5 flex items-center justify-between gap-3 hover:bg-[var(--bui-bg-hover)]/50 transition text-xs"
                        >
                          <div className="flex items-center gap-3 min-w-0">
                            <span className="font-mono-num text-[11px] text-[var(--bui-text-muted)] shrink-0 w-20">
                              {formatDate(tx.date)}
                            </span>
                            <div className="min-w-0 flex items-center gap-2">
                              <span className="font-medium text-[var(--bui-text-primary)] truncate">
                                {tx.description}
                              </span>
                              {instNo > 0 && instTot > 0 && (
                                <span className="text-[10px] font-mono-num px-1.5 py-0.5 rounded bg-[var(--bui-bg-card)] border border-[var(--bui-border)] text-[var(--bui-text-muted)] shrink-0">
                                  {instNo}/{instTot}
                                </span>
                              )}
                              {isCredit && (
                                <span className="text-[10px] font-semibold px-1.5 py-0.5 rounded bui-badge-lime shrink-0">
                                  Estorno / Crédito
                                </span>
                              )}
                            </div>
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
                                isCredit
                                  ? 'text-[var(--bui-lime)]'
                                  : 'text-[var(--bui-text-primary)]'
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

                {/* Footer Summary */}
                <div className="px-4 py-3 border-t border-[var(--bui-border)] bg-[var(--bui-bg-card)]/40 flex items-center justify-between text-xs">
                  <span className="text-[var(--bui-text-muted)]">
                    Total de {invoiceTransactions.length} lançamentos na fatura
                  </span>
                  <div className="flex items-center gap-2">
                    <span className="text-[var(--bui-text-secondary)] font-medium">
                      Total da Fatura:
                    </span>
                    <span className="text-sm font-bold font-mono-num text-[var(--bui-text-primary)]">
                      {formatCurrency(selectedInvoice.total_amount, isPrivacyMode)}
                    </span>
                  </div>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
