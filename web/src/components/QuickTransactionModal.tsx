import React, { useState, useEffect } from 'react'
import { X, Plus, ChevronDown, ChevronUp, AlertTriangle } from 'lucide-react'
import { Account, Category, TransactionStatus, TransactionType } from '../types'
import { createTransaction } from '../api'
import { isValidPositiveMoneyInput, toSignedDecimalString } from '../utils/money'

interface QuickTransactionModalProps {
  isOpen: boolean
  accounts: Account[]
  categories: Category[]
  onClose: () => void
  onSuccess: () => void
}

export const QuickTransactionModal: React.FC<QuickTransactionModalProps> = ({
  isOpen,
  accounts,
  categories,
  onClose,
  onSuccess,
}) => {
  const [amount, setAmount] = useState('')
  const [type, setType] = useState<TransactionType>('EXPENSE')
  const [description, setDescription] = useState('')
  const [accountId, setAccountId] = useState('')
  const [categoryId, setCategoryId] = useState('')
  const [date, setDate] = useState(() => new Date().toISOString().split('T')[0])

  // Progressive Disclosure (Advanced Fields)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [visibility, setVisibility] = useState('SHARED')
  const [status, setStatus] = useState<TransactionStatus>('COMPLETED')
  const [tagsInput, setTagsInput] = useState('')
  const [projectInput, setProjectInput] = useState('')
  const [notes, setNotes] = useState('')

  const [isSaving, setIsSaving] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  useEffect(() => {
    if (isOpen && !accountId && accounts.length > 0) {
      setAccountId(accounts[0].id)
    }
  }, [isOpen, accountId, accounts])

  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!description.trim()) {
      setErrorMessage('Informe uma descrição para o lançamento.')
      return
    }
    if (!accountId) {
      setErrorMessage('Selecione uma conta de origem/destino.')
      return
    }
    if (!isValidPositiveMoneyInput(amount)) {
      setErrorMessage('Informe um valor monetário válido maior que zero.')
      return
    }

    const signedAmount = toSignedDecimalString(amount, type === 'EXPENSE')
    const metadata: Record<string, any> = {}
    if (tagsInput.trim()) {
      metadata.tags = tagsInput
        .split(',')
        .map((t) => t.trim())
        .filter(Boolean)
    }
    if (projectInput.trim()) {
      metadata.project = projectInput.trim()
    }

    try {
      setIsSaving(true)
      setErrorMessage(null)
      await createTransaction({
        account_id: accountId,
        category_id: categoryId ? categoryId : null,
        date,
        amount: signedAmount,
        description: description.trim(),
        type,
        status,
        visibility,
        notes: notes.trim() || undefined,
        metadata: Object.keys(metadata).length > 0 ? metadata : undefined,
      })
      setAmount('')
      setDescription('')
      setNotes('')
      setTagsInput('')
      setProjectInput('')
      onSuccess()
      onClose()
    } catch (err: any) {
      setErrorMessage(err.message || 'Erro ao criar transação.')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
      <div className="bui-card max-w-lg w-full overflow-hidden shadow-2xl border border-[var(--bui-border-strong)]">
        {/* Header */}
        <div className="px-6 py-4 border-b border-[var(--bui-border)] flex items-center justify-between">
          <div>
            <h2 className="text-base font-bold text-[var(--bui-text-primary)]">
              Novo Lançamento Rápido
            </h2>
            <p className="text-xs text-[var(--bui-text-muted)]">
              Preencha os 5 campos essenciais ou expanda para opções avançadas.
            </p>
          </div>
          <button
            onClick={onClose}
            className="bui-pill-control p-1.5"
            aria-label="Fechar modal"
          >
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

          {/* Type Selector Pills */}
          <div className="grid grid-cols-2 gap-2 p-1 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)]">
            <button
              type="button"
              onClick={() => setType('EXPENSE')}
              className={`py-2 rounded-lg text-xs font-bold transition ${
                type === 'EXPENSE'
                  ? 'bui-badge-rose'
                  : 'text-[var(--bui-text-secondary)] hover:text-[var(--bui-text-primary)]'
              }`}
            >
              Despesa (-)
            </button>
            <button
              type="button"
              onClick={() => setType('INCOME')}
              className={`py-2 rounded-lg text-xs font-bold transition ${
                type === 'INCOME'
                  ? 'bui-badge-lime'
                  : 'text-[var(--bui-text-secondary)] hover:text-[var(--bui-text-primary)]'
              }`}
            >
              Receita (+)
            </button>
          </div>

          {/* 1. Valor & 2. Data */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
                Valor (R$) *
              </label>
              <input
                type="text"
                required
                autoFocus
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                placeholder="0,00"
                className="w-full py-2.5 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-sm font-mono-num font-bold text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
              />
            </div>

            <div>
              <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
                Data *
              </label>
              <input
                type="date"
                required
                value={date}
                onChange={(e) => setDate(e.target.value)}
                className="w-full py-2.5 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-mono-num text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
              />
            </div>
          </div>

          {/* 3. Descrição */}
          <div>
            <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
              Descrição / Estabelecimento *
            </label>
            <input
              type="text"
              required
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Ex.: Supermercado, Almoço, Salário..."
              className="w-full py-2.5 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
            />
          </div>

          {/* 4. Conta & 5. Categoria */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
                Conta *
              </label>
              <select
                value={accountId}
                onChange={(e) => setAccountId(e.target.value)}
                className="w-full py-2.5 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
              >
                {accounts.map((acc) => (
                  <option key={acc.id} value={acc.id}>
                    {acc.name}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
                Categoria
              </label>
              <select
                value={categoryId}
                onChange={(e) => setCategoryId(e.target.value)}
                className="w-full py-2.5 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
              >
                <option value="">Sem Categoria</option>
                {categories.map((cat) => (
                  <option key={cat.id} value={cat.id}>
                    {cat.name}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* Progressive Disclosure Accordion */}
          <div className="pt-1">
            <button
              type="button"
              onClick={() => setShowAdvanced(!showAdvanced)}
              className="flex items-center gap-1.5 text-xs font-semibold text-[var(--bui-accent-blue)] hover:underline"
            >
              {showAdvanced ? (
                <ChevronUp className="w-3.5 h-3.5" />
              ) : (
                <ChevronDown className="w-3.5 h-3.5" />
              )}
              <span>Opções Avançadas (Visibilidade, Status, Tags, Projeto, Notas)</span>
            </button>

            {showAdvanced && (
              <div className="mt-3 p-3.5 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] space-y-3">
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)] mb-1">
                      Visibilidade
                    </label>
                    <select
                      value={visibility}
                      onChange={(e) => setVisibility(e.target.value)}
                      className="w-full py-1.5 px-2.5 rounded-lg bg-[var(--bui-bg-card)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)]"
                    >
                      <option value="SHARED">Compartilhada</option>
                      <option value="PRIVATE">Pessoal (Privada)</option>
                      <option value="RESTRICTED">Restrita</option>
                    </select>
                  </div>

                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)] mb-1">
                      Status
                    </label>
                    <select
                      value={status}
                      onChange={(e) => setStatus(e.target.value as TransactionStatus)}
                      className="w-full py-1.5 px-2.5 rounded-lg bg-[var(--bui-bg-card)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)]"
                    >
                      <option value="COMPLETED">Efetivado</option>
                      <option value="PENDING">Pendente</option>
                      <option value="RECONCILED">Conciliado</option>
                    </select>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)] mb-1">
                      Tags (separadas por vírgula)
                    </label>
                    <input
                      type="text"
                      value={tagsInput}
                      onChange={(e) => setTagsInput(e.target.value)}
                      placeholder="reembolsavel, fixo"
                      className="w-full py-1.5 px-2.5 rounded-lg bg-[var(--bui-bg-card)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)]"
                    />
                  </div>

                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)] mb-1">
                      Projeto / Evento
                    </label>
                    <input
                      type="text"
                      value={projectInput}
                      onChange={(e) => setProjectInput(e.target.value)}
                      placeholder="Ex.: Reforma Casa"
                      className="w-full py-1.5 px-2.5 rounded-lg bg-[var(--bui-bg-card)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)]"
                    />
                  </div>
                </div>

                <div>
                  <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)] mb-1">
                    Observações
                  </label>
                  <textarea
                    rows={2}
                    value={notes}
                    onChange={(e) => setNotes(e.target.value)}
                    placeholder="Notas adicionais..."
                    className="w-full py-1.5 px-2.5 rounded-lg bg-[var(--bui-bg-card)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)]"
                  />
                </div>
              </div>
            )}
          </div>

          {/* Footer Buttons */}
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
              className="px-5 py-2 rounded-xl bg-[var(--bui-accent-blue)] hover:bg-[var(--bui-accent-blue-hover)] text-white text-xs font-bold flex items-center gap-1.5 shadow-sm transition disabled:opacity-50"
            >
              <Plus className="w-4 h-4" />
              <span>{isSaving ? 'Salvando...' : 'Registrar Lançamento'}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
