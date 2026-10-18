import React, { useState, useEffect } from 'react'
import { X, Save, AlertTriangle } from 'lucide-react'
import { Account, Category, Transaction, TransactionType } from '../types'
import { updateTransaction } from '../api'

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
  const [notes, setNotes] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  useEffect(() => {
    if (transaction) {
      setDescription(transaction.description)
      // Display positive amount in input, sign is determined by type
      const num = parseFloat(transaction.amount)
      setAmount(Math.abs(num).toFixed(2))
      setType(transaction.type)
      setDate(transaction.date.split('T')[0])
      setAccountId(transaction.account_id)
      setCategoryId(transaction.category_id || '')
      setNotes(transaction.notes || '')
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
    const parsedAmount = parseFloat(amount.replace(',', '.'))
    if (isNaN(parsedAmount) || parsedAmount <= 0) {
      setErrorMessage('Informe um valor monetário válido maior que zero.')
      return
    }

    // Backend domain rules: Expense is negative, Income is positive
    const finalAmount = type === 'EXPENSE' ? `-${parsedAmount.toFixed(2)}` : parsedAmount.toFixed(2)

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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-[#161D18] border border-[#2C362F] rounded-3xl max-w-lg w-full overflow-hidden shadow-2xl animate-in zoom-in-95 duration-200">
        {/* Header */}
        <div className="p-6 border-b border-[#2C362F] flex items-center justify-between">
          <h2 className="text-base font-bold text-[#E8EFEA]">Editar Transação</h2>
          <button
            onClick={onClose}
            className="p-1 rounded-xl text-[#8E9E92] hover:text-[#E8EFEA] hover:bg-[#1F2721] transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          {errorMessage && (
            <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/20 text-xs text-rose-400 flex items-center gap-2">
              <AlertTriangle className="w-4 h-4 shrink-0" />
              <span>{errorMessage}</span>
            </div>
          )}

          {/* Description */}
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-[#8E9E92] mb-1.5">
              Descrição *
            </label>
            <input
              type="text"
              required
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="w-full py-2.5 px-3 rounded-xl bg-[#1F2721] border border-[#2C362F] text-xs text-[#E8EFEA] focus:outline-none focus:border-[#A5D6A7]"
            />
          </div>

          {/* Amount and Type Grid */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-[#8E9E92] mb-1.5">
                Valor (R$) *
              </label>
              <input
                type="text"
                required
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                placeholder="0.00"
                className="w-full py-2.5 px-3 rounded-xl bg-[#1F2721] border border-[#2C362F] text-xs text-[#E8EFEA] font-mono focus:outline-none focus:border-[#A5D6A7]"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-[#8E9E92] mb-1.5">
                Tipo *
              </label>
              <select
                value={type}
                onChange={(e) => setType(e.target.value as TransactionType)}
                className="w-full py-2.5 px-3 rounded-xl bg-[#1F2721] border border-[#2C362F] text-xs text-[#E8EFEA] focus:outline-none focus:border-[#A5D6A7]"
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
              <label className="block text-xs font-semibold uppercase tracking-wider text-[#8E9E92] mb-1.5">
                Data *
              </label>
              <input
                type="date"
                required
                value={date}
                onChange={(e) => setDate(e.target.value)}
                className="w-full py-2.5 px-3 rounded-xl bg-[#1F2721] border border-[#2C362F] text-xs text-[#E8EFEA] focus:outline-none focus:border-[#A5D6A7]"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-[#8E9E92] mb-1.5">
                Conta *
              </label>
              <select
                value={accountId}
                onChange={(e) => setAccountId(e.target.value)}
                className="w-full py-2.5 px-3 rounded-xl bg-[#1F2721] border border-[#2C362F] text-xs text-[#E8EFEA] focus:outline-none focus:border-[#A5D6A7]"
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
            <label className="block text-xs font-semibold uppercase tracking-wider text-[#8E9E92] mb-1.5">
              Categoria
            </label>
            <select
              value={categoryId}
              onChange={(e) => setCategoryId(e.target.value)}
              className="w-full py-2.5 px-3 rounded-xl bg-[#1F2721] border border-[#2C362F] text-xs text-[#E8EFEA] focus:outline-none focus:border-[#A5D6A7]"
            >
              <option value="">Sem Categoria</option>
              {categories.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name} ({c.type === 'INCOME' ? 'Receita' : 'Despesa'})
                </option>
              ))}
            </select>
          </div>

          {/* Notes */}
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-[#8E9E92] mb-1.5">
              Observações / Notas
            </label>
            <textarea
              rows={2}
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              className="w-full py-2 px-3 rounded-xl bg-[#1F2721] border border-[#2C362F] text-xs text-[#E8EFEA] focus:outline-none focus:border-[#A5D6A7]"
              placeholder="Anotações opcionais..."
            />
          </div>

          {/* Actions */}
          <div className="pt-2 flex items-center justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2.5 rounded-xl bg-[#1F2721] text-[#8E9E92] hover:text-[#E8EFEA] text-xs font-semibold transition-colors"
            >
              Cancelar
            </button>
            <button
              type="submit"
              disabled={isSaving}
              className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-[#A5D6A7] hover:bg-[#81C784] text-[#0E1410] font-bold text-xs shadow-md transition-all disabled:opacity-40"
            >
              <Save className="w-4 h-4" />
              <span>{isSaving ? 'Salvando...' : 'Salvar Alterações'}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
