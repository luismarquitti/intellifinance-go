import React, { useState, useMemo } from 'react'
import {
  X,
  Check,
  CheckCheck,
  RefreshCw,
  Sparkles,
  ArrowRightLeft,
  Tag,
  Layers,
  List,
} from 'lucide-react'
import { PendingAction } from '../types'
import { decimalToCents, formatCurrency, isNonNegativeCents } from '../utils/money'
import {
  approvePendingAction,
  rejectPendingAction,
  batchApprovePendingActions,
  batchRejectPendingActions,
  triggerAutoCategorizeScan,
  triggerReconcileScan,
} from '../api'

interface PendingActionsModalProps {
  isOpen: boolean
  inlineMode?: boolean
  isPrivacyMode?: boolean
  actions: PendingAction[]
  onClose: () => void
  onRefresh: () => void
}

interface ActionGroup {
  key: string
  merchantLabel: string
  suggestedCategory: string
  actionType: PendingAction['action_type']
  actionIds: string[]
  totalCents: bigint
  items: PendingAction[]
}

export const PendingActionsModal: React.FC<PendingActionsModalProps> = ({
  isOpen,
  inlineMode = false,
  isPrivacyMode = false,
  actions,
  onClose,
  onRefresh,
}) => {
  const [isProcessing, setIsProcessing] = useState(false)
  const [activeTab, setActiveTab] = useState<'ALL' | 'CATEGORIES' | 'TRANSFERS'>('ALL')
  const [viewMode, setViewMode] = useState<'GROUPED' | 'INDIVIDUAL'>('GROUPED')

  const filteredActions = useMemo(() => {
    return actions.filter((act) => {
      if (activeTab === 'CATEGORIES') return act.action_type === 'CATEGORIZE_TRANSACTION'
      if (activeTab === 'TRANSFERS') return act.action_type === 'RECONCILE_TRANSFER'
      return true
    })
  }, [actions, activeTab])

  const groupedActions = useMemo<ActionGroup[]>(() => {
    const map = new Map<string, ActionGroup>()
    for (const act of filteredActions) {
      const p = act.payload || {}
      const merchant = (p.clean_description || p.description || 'Transação').trim()
      const catName =
        act.action_type === 'CATEGORIZE_TRANSACTION'
          ? p.category_name || 'Categoria Sugerida'
          : p.reconciliation_type === 'BILL_PAYMENT'
          ? 'Pagamento de Fatura'
          : 'Transferência Entre Contas'
      const groupKey = `${act.action_type}::${merchant}::${catName}`
      const c = decimalToCents(p.amount)
      const absC = c < 0n ? -c : c

      const existing = map.get(groupKey)
      if (existing) {
        existing.actionIds.push(act.id)
        existing.totalCents += absC
        existing.items.push(act)
      } else {
        map.set(groupKey, {
          key: groupKey,
          merchantLabel: merchant,
          suggestedCategory: catName,
          actionType: act.action_type,
          actionIds: [act.id],
          totalCents: absC,
          items: [act],
        })
      }
    }
    return Array.from(map.values()).sort((a, b) => b.actionIds.length - a.actionIds.length)
  }, [filteredActions])

  if (!isOpen && !inlineMode) return null

  const handleApproveSingle = async (id: string) => {
    try {
      setIsProcessing(true)
      await approvePendingAction(id)
      onRefresh()
    } catch (err) {
      console.error('Failed to approve action:', err)
    } finally {
      setIsProcessing(false)
    }
  }

  const handleRejectSingle = async (id: string) => {
    try {
      setIsProcessing(true)
      await rejectPendingAction(id, 'Rejeitado pelo usuário no painel')
      onRefresh()
    } catch (err) {
      console.error('Failed to reject action:', err)
    } finally {
      setIsProcessing(false)
    }
  }

  const handleApproveGroup = async (actionIds: string[]) => {
    if (actionIds.length === 0) return
    try {
      setIsProcessing(true)
      await batchApprovePendingActions({ action_ids: actionIds })
      onRefresh()
    } catch (err) {
      console.error('Failed to approve group:', err)
    } finally {
      setIsProcessing(false)
    }
  }

  const handleRejectGroup = async (actionIds: string[]) => {
    if (actionIds.length === 0) return
    try {
      setIsProcessing(true)
      await batchRejectPendingActions({
        action_ids: actionIds,
        reason: 'Grupo rejeitado pelo usuário',
      })
      onRefresh()
    } catch (err) {
      console.error('Failed to reject group:', err)
    } finally {
      setIsProcessing(false)
    }
  }

  const handleApproveAll = async () => {
    if (actions.length === 0) return
    try {
      setIsProcessing(true)
      await batchApprovePendingActions({ all: true })
      onRefresh()
    } catch (err) {
      console.error('Failed to batch approve:', err)
    } finally {
      setIsProcessing(false)
    }
  }

  const handleTriggerScans = async () => {
    try {
      setIsProcessing(true)
      await Promise.all([
        triggerAutoCategorizeScan().catch(() => null),
        triggerReconcileScan().catch(() => null),
      ])
      onRefresh()
    } catch (err) {
      console.error('Failed to trigger scans:', err)
    } finally {
      setIsProcessing(false)
    }
  }

  const content = (
    <div
      className={`bui-card flex flex-col overflow-hidden ${
        inlineMode ? 'w-full' : 'w-full max-w-4xl max-h-[85vh] shadow-2xl'
      }`}
    >
      {/* Header */}
      <div className="p-5 border-b border-[var(--bui-border)] flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-xl bg-[var(--bui-accent-blue)] flex items-center justify-center text-white">
            <Sparkles className="w-4 h-4" />
          </div>
          <div>
            <h2 className="text-base font-bold text-[var(--bui-text-primary)] flex items-center gap-2">
              Central de Aprovações IA (HITL)
              <span className="px-2 py-0.5 text-xs font-mono-num font-bold rounded-full bui-badge-amber">
                {actions.length} {actions.length === 1 ? 'pendência' : 'pendências'}
              </span>
            </h2>
            <p className="text-xs text-[var(--bui-text-muted)]">
              Aprove sugestões agrupadas por estabelecimento em 1 clique ou revise individualmente.
            </p>
          </div>
        </div>
        {!inlineMode && (
          <button onClick={onClose} className="bui-pill-control p-1.5">
            <X className="w-4 h-4" />
          </button>
        )}
      </div>

      {/* Toolbar */}
      <div
        className="px-5 py-3 border-b border-[var(--bui-border)] flex flex-wrap items-center justify-between gap-3"
        style={{ backgroundColor: 'var(--bui-bg-inner)' }}
      >
        {/* Filter Tabs */}
        <div className="flex items-center gap-1.5 flex-wrap">
          <button
            onClick={() => setActiveTab('ALL')}
            className={`px-3 py-1 rounded-lg text-xs font-semibold transition ${
              activeTab === 'ALL'
                ? 'bg-[var(--bui-accent-blue)] text-white'
                : 'bui-pill-control'
            }`}
          >
            Todas ({actions.length})
          </button>
          <button
            onClick={() => setActiveTab('CATEGORIES')}
            className={`px-3 py-1 rounded-lg text-xs font-semibold transition ${
              activeTab === 'CATEGORIES'
                ? 'bg-[var(--bui-accent-blue)] text-white'
                : 'bui-pill-control'
            }`}
          >
            Categorias (
            {actions.filter((a) => a.action_type === 'CATEGORIZE_TRANSACTION').length})
          </button>
          <button
            onClick={() => setActiveTab('TRANSFERS')}
            className={`px-3 py-1 rounded-lg text-xs font-semibold transition ${
              activeTab === 'TRANSFERS'
                ? 'bg-[var(--bui-accent-blue)] text-white'
                : 'bui-pill-control'
            }`}
          >
            Transferências & Faturas (
            {actions.filter((a) => a.action_type === 'RECONCILE_TRANSFER').length})
          </button>
        </div>

        {/* View Mode & Batch Actions */}
        <div className="flex items-center gap-2 flex-wrap">
          <div className="flex items-center rounded-lg border border-[var(--bui-border)] p-0.5 bg-[var(--bui-bg-card)]">
            <button
              onClick={() => setViewMode('GROUPED')}
              className={`px-2.5 py-1 rounded-md text-[11px] font-semibold flex items-center gap-1 transition ${
                viewMode === 'GROUPED'
                  ? 'bg-[var(--bui-bg-inner)] text-[var(--bui-text-primary)] shadow-sm'
                  : 'text-[var(--bui-text-muted)]'
              }`}
            >
              <Layers className="w-3 h-3" /> Agrupado ({groupedActions.length})
            </button>
            <button
              onClick={() => setViewMode('INDIVIDUAL')}
              className={`px-2.5 py-1 rounded-md text-[11px] font-semibold flex items-center gap-1 transition ${
                viewMode === 'INDIVIDUAL'
                  ? 'bg-[var(--bui-bg-inner)] text-[var(--bui-text-primary)] shadow-sm'
                  : 'text-[var(--bui-text-muted)]'
              }`}
            >
              <List className="w-3 h-3" /> Individual
            </button>
          </div>

          <button
            onClick={handleTriggerScans}
            disabled={isProcessing}
            className="bui-pill-control px-3 py-1.5 text-xs font-semibold flex items-center gap-1.5 disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isProcessing ? 'animate-spin' : ''}`} />
            <span>Escanear</span>
          </button>

          {actions.length > 0 && (
            <button
              onClick={handleApproveAll}
              disabled={isProcessing}
              className="px-3.5 py-1.5 rounded-lg text-xs font-bold text-white bg-[var(--bui-accent-blue)] hover:bg-[var(--bui-accent-blue-hover)] flex items-center gap-1.5 transition disabled:opacity-50"
            >
              <CheckCheck className="w-3.5 h-3.5" />
              <span>Aprovar Todas ({actions.length})</span>
            </button>
          )}
        </div>
      </div>

      {/* Body List */}
      <div className="flex-1 overflow-y-auto p-5 space-y-2.5 max-h-[600px]">
        {filteredActions.length === 0 ? (
          <div className="py-12 text-center">
            <CheckCheck className="w-12 h-12 text-[var(--bui-lime)] mx-auto mb-3 opacity-60" />
            <h3 className="text-sm font-bold text-[var(--bui-text-primary)]">
              Zero pendências no momento
            </h3>
            <p className="text-xs text-[var(--bui-text-muted)] mt-1">
              Todas as sugestões da inteligência foram revisadas e conciliadas.
            </p>
          </div>
        ) : viewMode === 'GROUPED' ? (
          groupedActions.map((grp) => (
            <div
              key={grp.key}
              className="bui-inner p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3 hover:border-[var(--bui-border-strong)] transition"
            >
              <div className="space-y-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="px-2 py-0.5 rounded-md text-[11px] font-mono-num font-bold bui-badge-blue">
                    {grp.actionIds.length}x lançamentos
                  </span>
                  <span className="text-xs font-bold text-[var(--bui-text-primary)] truncate">
                    {grp.merchantLabel}
                  </span>
                </div>
                <div className="text-xs text-[var(--bui-text-secondary)] flex items-center gap-2">
                  <span>Sugestão IA:</span>
                  <span className="font-semibold text-[var(--bui-lime)]">
                    {grp.suggestedCategory}
                  </span>
                </div>
              </div>

              <div className="flex items-center justify-between sm:justify-end gap-3 shrink-0">
                <div className="text-right">
                  <span className="text-[10px] uppercase text-[var(--bui-text-muted)] block">
                    Volume do Grupo
                  </span>
                  <span className="text-sm font-bold font-mono-num text-[var(--bui-text-primary)]">
                    {formatCurrency(grp.totalCents, isPrivacyMode)}
                  </span>
                </div>

                <div className="flex items-center gap-1.5">
                  <button
                    onClick={() => handleRejectGroup(grp.actionIds)}
                    disabled={isProcessing}
                    className="p-1.5 rounded-lg bui-badge-rose transition disabled:opacity-50"
                    title="Rejeitar grupo"
                  >
                    <X className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => handleApproveGroup(grp.actionIds)}
                    disabled={isProcessing}
                    className="px-3 py-1.5 rounded-lg text-xs font-bold text-white bg-[var(--bui-accent-blue)] hover:bg-[var(--bui-accent-blue-hover)] flex items-center gap-1 transition disabled:opacity-50"
                  >
                    <Check className="w-3.5 h-3.5" />
                    <span>Aprovar Grupo ({grp.actionIds.length})</span>
                  </button>
                </div>
              </div>
            </div>
          ))
        ) : (
          filteredActions.map((act) => {
            const p = act.payload || {}
            const isCat = act.action_type === 'CATEGORIZE_TRANSACTION'
            const isPositive = isNonNegativeCents(p.amount)

            return (
              <div
                key={act.id}
                className="bui-inner p-3.5 flex flex-col sm:flex-row sm:items-center justify-between gap-3"
              >
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    {isCat ? (
                      <span className="flex items-center gap-1 text-[10px] font-semibold uppercase px-2 py-0.5 rounded bui-badge-amber">
                        <Tag className="w-3 h-3" /> Categorização
                      </span>
                    ) : (
                      <span className="flex items-center gap-1 text-[10px] font-semibold uppercase px-2 py-0.5 rounded bui-badge-blue">
                        <ArrowRightLeft className="w-3 h-3" />
                        {p.reconciliation_type === 'BILL_PAYMENT'
                          ? 'Pagamento de Fatura'
                          : 'Transferência'}
                      </span>
                    )}
                    <span className="text-xs font-mono-num text-[var(--bui-text-muted)]">
                      {p.date || p.outflow_date}
                    </span>
                  </div>

                  <div className="text-xs font-bold text-[var(--bui-text-primary)]">
                    {p.clean_description || p.description}
                  </div>

                  <div className="text-xs text-[var(--bui-text-secondary)]">
                    {isCat ? (
                      <>
                        Sugerido:{' '}
                        <strong className="text-[var(--bui-lime)]">{p.category_name}</strong> •{' '}
                        <span className="italic">{p.reason}</span>
                      </>
                    ) : (
                      <span>
                        {p.source_account_name} ➔ {p.destination_account_name}
                      </span>
                    )}
                  </div>
                </div>

                <div className="flex items-center justify-between sm:justify-end gap-3">
                  <div
                    className={`text-sm font-bold font-mono-num ${
                      isPositive
                        ? 'text-[var(--bui-lime)]'
                        : 'text-[var(--bui-text-primary)]'
                    }`}
                  >
                    {formatCurrency(p.amount, isPrivacyMode)}
                  </div>

                  <div className="flex items-center gap-1.5">
                    <button
                      onClick={() => handleRejectSingle(act.id)}
                      disabled={isProcessing}
                      className="p-1.5 rounded-lg bui-badge-rose"
                      title="Rejeitar"
                    >
                      <X className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleApproveSingle(act.id)}
                      disabled={isProcessing}
                      className="px-3 py-1.5 rounded-lg text-xs font-bold text-white bg-[var(--bui-accent-blue)] hover:bg-[var(--bui-accent-blue-hover)] flex items-center gap-1"
                    >
                      <Check className="w-3.5 h-3.5" />
                      <span>Aprovar</span>
                    </button>
                  </div>
                </div>
              </div>
            )
          })
        )}
      </div>
    </div>
  )

  if (inlineMode) {
    return content
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/65 backdrop-blur-sm">
      {content}
    </div>
  )
}
