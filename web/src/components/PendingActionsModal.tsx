import React, { useState } from 'react'
import { X, Check, CheckCheck, RefreshCw, Sparkles, ArrowRightLeft, Tag } from 'lucide-react'
import { PendingAction } from '../types'
import { formatCurrency } from './KPICards'
import {
  approvePendingAction,
  rejectPendingAction,
  batchApprovePendingActions,
  triggerAutoCategorizeScan,
  triggerReconcileScan,
} from '../api'

interface PendingActionsModalProps {
  isOpen: boolean
  actions: PendingAction[]
  onClose: () => void
  onRefresh: () => void
}

export const PendingActionsModal: React.FC<PendingActionsModalProps> = ({
  isOpen,
  actions,
  onClose,
  onRefresh,
}) => {
  const [isProcessing, setIsProcessing] = useState(false)
  const [activeTab, setActiveTab] = useState<'ALL' | 'CATEGORIES' | 'TRANSFERS'>('ALL')

  if (!isOpen) return null

  const filteredActions = actions.filter((act) => {
    if (activeTab === 'CATEGORIES') return act.action_type === 'CATEGORIZE_TRANSACTION'
    if (activeTab === 'TRANSFERS') return act.action_type === 'RECONCILE_TRANSFER'
    return true
  })

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

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="w-full max-w-3xl max-h-[85vh] flex flex-col rounded-2xl bg-[#161D18] border border-[#2C362F] shadow-2xl">
        {/* Modal Header */}
        <div className="p-5 border-b border-[#2C362F] flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl bg-gradient-to-br from-[#A5D6A7] to-[#2E7D32] flex items-center justify-center text-[#0E1410]">
              <Sparkles className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-lg font-bold text-[#E8EFEA] flex items-center gap-2">
                Central de Aprovações (HITL)
                <span className="px-2 py-0.5 text-xs font-semibold rounded-full bg-[#1F2721] text-[#A5D6A7] border border-[#2C362F]">
                  {actions.length} {actions.length === 1 ? 'pendência' : 'pendências'}
                </span>
              </h2>
              <p className="text-xs text-[#8E9E92]">
                Revise e aprove sugestões de categorização e transferências detectadas pela inteligência.
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-[#8E9E92] hover:text-[#E8EFEA] hover:bg-[#1F2721] transition"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Toolbar & Filters */}
        <div className="px-5 py-3 border-b border-[#2C362F] flex flex-wrap items-center justify-between gap-3 bg-[#131915]">
          {/* Tabs */}
          <div className="flex items-center gap-1.5">
            <button
              onClick={() => setActiveTab('ALL')}
              className={`px-3 py-1 rounded-lg text-xs font-medium transition ${
                activeTab === 'ALL'
                  ? 'bg-[#A5D6A7] text-[#0E1410] font-semibold'
                  : 'text-[#8E9E92] hover:text-[#E8EFEA] hover:bg-[#1F2721]'
              }`}
            >
              Todas ({actions.length})
            </button>
            <button
              onClick={() => setActiveTab('CATEGORIES')}
              className={`px-3 py-1 rounded-lg text-xs font-medium transition ${
                activeTab === 'CATEGORIES'
                  ? 'bg-[#A5D6A7] text-[#0E1410] font-semibold'
                  : 'text-[#8E9E92] hover:text-[#E8EFEA] hover:bg-[#1F2721]'
              }`}
            >
              Categorias ({actions.filter((a) => a.action_type === 'CATEGORIZE_TRANSACTION').length})
            </button>
            <button
              onClick={() => setActiveTab('TRANSFERS')}
              className={`px-3 py-1 rounded-lg text-xs font-medium transition ${
                activeTab === 'TRANSFERS'
                  ? 'bg-[#A5D6A7] text-[#0E1410] font-semibold'
                  : 'text-[#8E9E92] hover:text-[#E8EFEA] hover:bg-[#1F2721]'
              }`}
            >
              Transferências ({actions.filter((a) => a.action_type === 'RECONCILE_TRANSFER').length})
            </button>
          </div>

          {/* Action Buttons */}
          <div className="flex items-center gap-2">
            <button
              onClick={handleTriggerScans}
              disabled={isProcessing}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-[#8E9E92] hover:text-[#E8EFEA] bg-[#1F2721] hover:bg-[#28332B] border border-[#2C362F] transition disabled:opacity-50"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${isProcessing ? 'animate-spin' : ''}`} />
              <span>Escanear Agora</span>
            </button>
            {actions.length > 0 && (
              <button
                onClick={handleApproveAll}
                disabled={isProcessing}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold text-[#0E1410] bg-[#A5D6A7] hover:bg-[#81C784] transition disabled:opacity-50 shadow-sm"
              >
                <CheckCheck className="w-3.5 h-3.5" />
                <span>Aprovar Todas</span>
              </button>
            )}
          </div>
        </div>

        {/* Action Items List */}
        <div className="flex-1 overflow-y-auto p-5 space-y-3">
          {filteredActions.length === 0 ? (
            <div className="py-12 text-center">
              <CheckCheck className="w-12 h-12 text-emerald-400/40 mx-auto mb-3" />
              <h3 className="text-sm font-semibold text-[#E8EFEA]">Zero pendências no momento</h3>
              <p className="text-xs text-[#8E9E92] mt-1">
                Todas as transações e conciliações estão revisadas ou sincronizadas.
              </p>
            </div>
          ) : (
            filteredActions.map((act) => {
              const p = act.payload
              const isCat = act.action_type === 'CATEGORIZE_TRANSACTION'
              const amountNum = parseFloat(p.amount || '0')
              const confPercent = p.confidence ? Math.round(p.confidence * 100) : null

              return (
                <div
                  key={act.id}
                  className="p-4 rounded-xl bg-[#111613] border border-[#242D26] hover:border-[#334036] transition flex flex-col sm:flex-row sm:items-center justify-between gap-3"
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      {isCat ? (
                        <span className="flex items-center gap-1 text-[11px] font-semibold uppercase px-2 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20">
                          <Tag className="w-3 h-3" />
                          Categorização
                        </span>
                      ) : (
                        <span className="flex items-center gap-1 text-[11px] font-semibold uppercase px-2 py-0.5 rounded bg-blue-500/10 text-blue-400 border border-blue-500/20">
                          <ArrowRightLeft className="w-3 h-3" />
                          {p.reconciliation_type === 'BILL_PAYMENT' ? 'Pagamento de Fatura' : 'Transferência'}
                        </span>
                      )}

                      {confPercent && (
                        <span className="text-[10px] font-medium px-1.5 py-0.5 rounded bg-[#1F2721] text-[#A5D6A7]">
                          {confPercent}% confiança
                        </span>
                      )}

                      <span className="text-xs text-[#8E9E92]">{p.date || p.outflow_date}</span>
                    </div>

                    <div className="text-sm font-semibold text-[#E8EFEA]">
                      {p.clean_description || p.description}
                    </div>

                    <div className="text-xs text-[#8E9E92] flex items-center gap-2">
                      {isCat ? (
                        <>
                          <span>Sugerido:</span>
                          <span className="font-semibold text-emerald-400">{p.category_name}</span>
                          <span className="text-[#556459]">•</span>
                          <span className="italic">{p.reason}</span>
                        </>
                      ) : (
                        <span>
                          {p.source_account_name} ➔ {p.destination_account_name}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Amount & Actions */}
                  <div className="flex items-center justify-between sm:justify-end gap-3 pt-2 sm:pt-0 border-t sm:border-t-0 border-[#1F2721]">
                    <div className={`text-base font-bold ${
                      amountNum >= 0 ? 'text-emerald-400' : 'text-[#E8EFEA]'
                    }`}>
                      {formatCurrency(amountNum)}
                    </div>

                    <div className="flex items-center gap-1.5">
                      <button
                        onClick={() => handleRejectSingle(act.id)}
                        disabled={isProcessing}
                        className="p-1.5 rounded-lg text-rose-400 hover:bg-rose-500/10 border border-rose-500/20 transition disabled:opacity-50"
                        title="Rejeitar"
                      >
                        <X className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => handleApproveSingle(act.id)}
                        disabled={isProcessing}
                        className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-semibold text-[#0E1410] bg-[#A5D6A7] hover:bg-[#81C784] transition disabled:opacity-50 shadow-sm"
                        title="Aprovar"
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
    </div>
  )
}
