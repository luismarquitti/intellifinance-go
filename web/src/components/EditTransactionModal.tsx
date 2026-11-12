import React, { useState, useEffect } from 'react'
import { X, Save, AlertTriangle } from 'lucide-react'
import { Account, Category, Transaction, TransactionType } from '../types'
import { updateTransaction } from '../api'
import {
  absDecimalString,
  isValidPositiveMoneyInput,
  toSignedDecimalString,
} from '../utils/money'

interface EditTransactionModalProps {
  isOpen: boolean
  transaction: Transaction | null
  accounts: Account[]
  categories: Category[]
  onClose: () => void
  onSuccess: () => void
}

export const EditTransactionModal: React.FC<EditTransactionModalProps> = ({
  isOpen,
  transaction,
  accounts,
  categories,
  onClose,
  onSuccess,
}) => {
  const [description, setDescription] = useState('')
  const [amount, setAmount] = useState('')
  const [type, setType] = useState<TransactionType>('EXPENSE')
  const [date, setDate] = useState('')
  const [accountId, setAccountId] = useState('')
  const [categoryId, setCategoryId] = useState('')
  const [tagsInput, setTagsInput] = useState('')
  const [projectInput, setProjectInput] = useState('')
  const [notes, setNotes] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  useEffect(() => {
    if (transaction) {
      setDescription(transaction.description)
      setAmount(absDecimalString(transaction.amount))
      setType(transaction.type)
      setDate(transaction.date.split('T')[0])
      setAccountId(transaction.account_id)
      setCategoryId(transaction.category_id || '')
      setNotes(transaction.notes || '')
      const tags: string[] = Array.isArray(transaction.metadata?.tags)
        ? transaction.metadata.tags
        : []
      setTagsInput(tags.join(', '))
      setProjectInput(transaction.metadata?.project || '')
      setErrorMessage(null)
    }
  }, [transaction])

  if (!isOpen || !transaction) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!description.trim()) {
      setErrorMessage('Descrição é obrigatória.')
      return
    }
    if (!isValidPositiveMoneyInput(amount)) {
      setErrorMessage('Informe um valor monetário válido maior que zero.')
      return
    }

    const finalAmount = toSignedDecimalString(amount, type === 'EXPENSE')
    const metadata: Record<string, any> = {
      ...(transaction.metadata || {}),
    }
    if (tagsInput.trim()) {
      metadata.tags = tagsInput
        .split(',')
        .map((t) => t.trim())
        .filter(Boolean)
    } else {
      delete metadata.tags
    }
    if (projectInput.trim()) {
      metadata.project = projectInput.trim()
    } else {
      delete metadata.project
    }

    try {
      setIsSaving(true)
      setErrorMessage(null)
      await updateTransaction(transaction.id, {
        description: description.trim(),
        amount: finalAmount,
        type,
        date,
        account_id: accountId,
        category_id: categoryId ? categoryId : null,
        notes: notes.trim(),
        metadata,
      })
      onSuccess()
      onClose()
    } catch (err: any) {
      setErrorMessage(err.message || 'Falha ao salvar alterações da transação.')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/65 backdrop-blur-sm">
      <div className="bui-card max-w-lg w-full overflow-hidden shadow-2xl border border-[var(--bui-border-strong)]">
        {/* Header */}
        <div className="px-6 py-4 border-b border-[var(--bui-border)] flex items-center justify-between">
          <h2 className="text-base font-bold text-[var(--bui-text-primary)]">
            Editar Lançamento
          </h2>
          <button onClick={onClose} className="bui-pill-control p-1.5">
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          {errorMessage && (
            <div className="p-3 rounded-xl bui-badge-rose text-xs flex items-center gap-2">
              <AlertTriangle className="w-4 h-4 shrink-0" />
              <span>{errorMessage}</span>
            </div>
          )}

          {/* Description */}
          <div>
            <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
              Descrição *
            </label>
            <input
              type="text"
              required
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="w-full py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
            />
          </div>

          {/* Amount and Type Grid */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
                Valor (R$) *
              </label>
              <input
                type="text"
                required
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                placeholder="0.00"
                className="w-full py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-mono-num text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
              />
            </div>

            <div>
              <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
                Tipo *
              </label>
              <select
                value={type}
                onChange={(e) => setType(e.target.value as TransactionType)}
                className="w-full py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] focus:outline-none"
              >
                <option value="EXPENSE">Despesa</option>
                <option value="INCOME">Receita</option>
                <option value="TRANSFER">Transferência</option>
              </select>
            </div>
          </div>

          {/* Date and Account Grid */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
                Data *
              </label>
              <input
                type="date"
                required
                value={date}
                onChange={(e) => setDate(e.target.value)}
                className="w-full py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-mono-num text-[var(--bui-text-primary)] focus:outline-none"
              />
            </div>

            <div>
              <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
                Conta *
              </label>
              <select
                value={accountId}
                onChange={(e) => setAccountId(e.target.value)}
                className="w-full py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] focus:outline-none"
              >
                {accounts.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* Category */}
          <div>
            <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
              Categoria
            </label>
            <select
              value={categoryId}
              onChange={(e) => setCategoryId(e.target.value)}
              className="w-full py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] focus:outline-none"
            >
              <option value="">Sem Categoria</option>
              {categories.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name} ({c.type === 'INCOME' ? 'Receita' : 'Despesa'})
                </option>
              ))}
            </select>
          </div>

          {/* Tags & Project */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
                Tags (vírgula)
              </label>
              <input
                type="text"
                value={tagsInput}
                onChange={(e) => setTagsInput(e.target.value)}
                placeholder="fixo, viagem"
                className="w-full py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] focus:outline-none"
              />
            </div>

            <div>
              <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
                Projeto / Centro
              </label>
              <input
                type="text"
                value={projectInput}
                onChange={(e) => setProjectInput(e.target.value)}
                placeholder="Ex.: Férias 2026"
                className="w-full py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] focus:outline-none"
              />
            </div>
          </div>

          {/* Notes */}
          <div>
            <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
              Observações / Notas
            </label>
            <textarea
              rows={2}
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              className="w-full py-2 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] focus:outline-none"
              placeholder="Anotações opcionais..."
            />
          </div>

          {/* Actions */}
          <div className="pt-2 flex items-center justify-end gap-2.5">
            <button
              type="button"
              onClick={onClose}
              className="bui-pill-control px-4 py-2 text-xs font-semibold"
            >
              Cancelar
            </button>
            <button
              type="submit"
              disabled={isSaving}
              className="flex items-center gap-1.5 px-5 py-2 rounded-xl bg-[var(--bui-accent-blue)] hover:bg-[var(--bui-accent-blue-hover)] text-white font-bold text-xs shadow-sm transition disabled:opacity-40"
            >
              <Save className="w-3.5 h-3.5" />
              <span>{isSaving ? 'Salvando...' : 'Salvar Alterações'}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
