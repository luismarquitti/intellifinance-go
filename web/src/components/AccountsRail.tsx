import React, { useState } from 'react'
import {
  Landmark,
  CreditCard,
  Plus,
  Check,
  Shield,
  Users,
  Lock,
  X,
} from 'lucide-react'
import { Account, Invoice } from '../types'
import { createAccount } from '../api'
import { formatCurrency } from '../utils/money'

interface AccountsRailProps {
  accounts: Account[]
  invoices: Invoice[]
  selectedAccountId?: string
  isPrivacyMode?: boolean
  onSelectAccount: (accountId?: string) => void
  onOpenInvoicesHub?: () => void
  onAccountCreated: () => void
}

export const AccountsRail: React.FC<AccountsRailProps> = ({
  accounts,
  invoices,
  selectedAccountId,
  isPrivacyMode = false,
  onSelectAccount,
  onOpenInvoicesHub,
  onAccountCreated,
}) => {
  const [isNewAccountOpen, setIsNewAccountOpen] = useState(false)
  const [name, setName] = useState('')
  const [type, setType] = useState('CHECKING')
  const [visibility, setVisibility] = useState('SHARED')
  const [initialBalance, setInitialBalance] = useState('0.00')
  const [isCreating, setIsCreating] = useState(false)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const liquidAccounts = accounts.filter((a) => a.type !== 'CREDIT_CARD')
  const creditCards = accounts.filter((a) => a.type === 'CREDIT_CARD')

  const openInvoiceCount = invoices.filter(
    (inv) => inv.status === 'OPEN' || inv.status === 'CLOSED' || inv.status === 'OVERDUE'
  ).length

  const renderVisibilityBadge = (vis: string) => {
    if (vis === 'SHARED') {
      return (
        <span className="inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded bg-[var(--bui-bg-hover)] text-[var(--bui-text-secondary)]">
          <Users className="w-2.5 h-2.5" /> Compartilhada
        </span>
      )
    }
    if (vis === 'RESTRICTED') {
      return (
        <span className="inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded bui-badge-amber">
          <Lock className="w-2.5 h-2.5" /> Restrita
        </span>
      )
    }
    return (
      <span className="inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded bui-badge-blue">
        <Shield className="w-2.5 h-2.5" /> Pessoal
      </span>
    )
  }

  const handleCreateAccount = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      setErrorMsg('Informe o nome da conta.')
      return
    }
    try {
      setIsCreating(true)
      setErrorMsg(null)
      await createAccount({
        name: name.trim(),
        type,
        currency: 'BRL',
        visibility,
        color_hex: type === 'CREDIT_CARD' ? '#EC7000' : '#2563EB',
        icon_name: type === 'CREDIT_CARD' ? 'credit-card' : 'landmark',
        initial_balance: initialBalance.replace(',', '.'),
      })
      setName('')
      setInitialBalance('0.00')
      setIsNewAccountOpen(false)
      onAccountCreated()
    } catch (err: any) {
      setErrorMsg(err.message || 'Erro ao criar conta.')
    } finally {
      setIsCreating(false)
    }
  }

  return (
    <div className="bui-card p-5 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-bold text-[var(--bui-text-primary)]">
            Contas Bancárias & Cartões (Open Finance)
          </h3>
          <p className="text-xs text-[var(--bui-text-muted)]">
            Clique em uma conta para filtrar os lançamentos ou inspecione as {invoices.length} faturas sincronizadas.
          </p>
        </div>

        <div className="flex items-center gap-2">
          {selectedAccountId && (
            <button
              onClick={() => onSelectAccount(undefined)}
              className="bui-pill-control px-2.5 py-1 text-xs font-medium flex items-center gap-1"
            >
              <X className="w-3 h-3" />
              <span>Mostrar Todas</span>
            </button>
          )}
          <button
            onClick={() => setIsNewAccountOpen(!isNewAccountOpen)}
            className="bui-pill-control px-3 py-1.5 text-xs font-semibold flex items-center gap-1.5"
          >
            <Plus className="w-3.5 h-3.5 text-[var(--bui-accent-blue)]" />
            <span>Nova Conta</span>
          </button>
        </div>
      </div>

      {/* Inline New Account Form */}
      {isNewAccountOpen && (
        <form
          onSubmit={handleCreateAccount}
          className="bui-inner p-4 space-y-3 border border-[var(--bui-border-strong)]"
        >
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold text-[var(--bui-text-primary)]">
              Cadastrar Conta Manual ou Cartão
            </span>
            <button
              type="button"
              onClick={() => setIsNewAccountOpen(false)}
              className="text-xs text-[var(--bui-text-muted)] hover:text-[var(--bui-text-primary)]"
            >
              Fechar
            </button>
          </div>
          {errorMsg && (
            <div className="text-xs text-[var(--bui-rose)] font-medium">{errorMsg}</div>
          )}
          <div className="grid grid-cols-1 sm:grid-cols-4 gap-3">
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)] mb-1">
                Nome da Conta *
              </label>
              <input
                type="text"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Ex.: Reserva Itaú"
                className="w-full px-3 py-1.5 rounded-lg bg-[var(--bui-bg-card)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] focus:outline-none focus:border-[var(--bui-accent-blue)]"
              />
            </div>
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)] mb-1">
                Tipo *
              </label>
              <select
                value={type}
                onChange={(e) => setType(e.target.value)}
                className="w-full px-3 py-1.5 rounded-lg bg-[var(--bui-bg-card)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] focus:outline-none"
              >
                <option value="CHECKING">Conta Corrente</option>
                <option value="SAVINGS">Poupança / Reserva</option>
                <option value="CREDIT_CARD">Cartão de Crédito</option>
                <option value="INVESTMENT">Investimento</option>
                <option value="CASH">Carteira Física</option>
              </select>
            </div>
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)] mb-1">
                Visibilidade *
              </label>
              <select
                value={visibility}
                onChange={(e) => setVisibility(e.target.value)}
                className="w-full px-3 py-1.5 rounded-lg bg-[var(--bui-bg-card)] border border-[var(--bui-border)] text-xs text-[var(--bui-text-primary)] focus:outline-none"
              >
                <option value="SHARED">Compartilhada</option>
                <option value="PRIVATE">Pessoal (Privada)</option>
                <option value="RESTRICTED">Restrita</option>
              </select>
            </div>
            <div className="flex items-end gap-2">
              <div className="flex-1">
                <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)] mb-1">
                  Saldo Inicial (R$)
                </label>
                <input
                  type="text"
                  value={initialBalance}
                  onChange={(e) => setInitialBalance(e.target.value)}
                  className="w-full px-3 py-1.5 rounded-lg bg-[var(--bui-bg-card)] border border-[var(--bui-border)] text-xs font-mono-num text-[var(--bui-text-primary)] focus:outline-none"
                />
              </div>
              <button
                type="submit"
                disabled={isCreating}
                className="px-3.5 py-1.5 rounded-lg bg-[var(--bui-accent-blue)] text-white text-xs font-semibold hover:bg-[var(--bui-accent-blue-hover)] transition"
              >
                {isCreating ? '...' : 'Criar'}
              </button>
            </div>
          </div>
        </form>
      )}

      {/* Grid: Liquid Accounts vs Credit Cards */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Column 1: Contas Bancárias Líquidas */}
        <div className="space-y-2.5">
          <div className="flex items-center justify-between text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)] px-1">
            <span>Contas Líquidas ({liquidAccounts.length})</span>
            <span>Saldo Atual</span>
          </div>

          {liquidAccounts.length === 0 ? (
            <div className="bui-inner p-4 text-xs text-[var(--bui-text-muted)] text-center">
              Nenhuma conta bancária líquida cadastrada.
            </div>
          ) : (
            liquidAccounts.map((acc) => {
              const isSelected = selectedAccountId === acc.id
              return (
                <div
                  key={acc.id}
                  onClick={() => onSelectAccount(isSelected ? undefined : acc.id)}
                  className={`bui-inner p-3.5 flex items-center justify-between cursor-pointer transition ${
                    isSelected
                      ? 'ring-2 ring-[var(--bui-accent-blue)] border-transparent'
                      : 'hover:border-[var(--bui-border-strong)]'
                  }`}
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <div
                      className="w-9 h-9 rounded-xl flex items-center justify-center text-white font-bold text-xs shrink-0"
                      style={{ backgroundColor: acc.color_hex || '#2563EB' }}
                    >
                      <Landmark className="w-4 h-4" />
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-bold text-[var(--bui-text-primary)] truncate">
                          {acc.name}
                        </span>
                        {isSelected && (
                          <span className="inline-flex items-center gap-0.5 text-[10px] font-bold text-[var(--bui-accent-blue)]">
                            <Check className="w-3 h-3" /> Filtro ativo
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-1.5 mt-1 flex-wrap">
                        <span className="text-[10px] px-1.5 py-0.5 rounded bui-badge-lime font-medium">
                          {acc.institution_name || 'Pluggy Open Finance'}
                        </span>
                        {renderVisibilityBadge(acc.visibility)}
                      </div>
                    </div>
                  </div>

                  <div className="text-right shrink-0 pl-2">
                    <div className="text-sm font-bold font-mono-num text-[var(--bui-text-primary)]">
                      {formatCurrency(acc.balance, isPrivacyMode)}
                    </div>
                    <div className="text-[10px] text-[var(--bui-text-muted)]">
                      {acc.type === 'CHECKING' ? 'Conta Corrente' : acc.type}
                    </div>
                  </div>
                </div>
              )
            })
          )}
        </div>

        {/* Column 2: Cartões de Crédito & Faturas */}
        <div className="space-y-2.5">
          <div className="flex items-center justify-between text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)] px-1">
            <span>Cartões de Crédito ({creditCards.length})</span>
            {onOpenInvoicesHub && (
              <button
                onClick={onOpenInvoicesHub}
                className="text-[var(--bui-accent-blue)] hover:underline font-semibold normal-case"
              >
                Ver todas as {invoices.length} faturas →
              </button>
            )}
          </div>

          {creditCards.length === 0 ? (
            <div className="bui-inner p-4 text-xs text-[var(--bui-text-muted)] text-center">
              Nenhum cartão de crédito conectado.
            </div>
          ) : (
            creditCards.map((card) => {
              const isSelected = selectedAccountId === card.id
              const cardInvoices = invoices.filter((i) => i.account_id === card.id)
              return (
                <div
                  key={card.id}
                  onClick={() => onSelectAccount(isSelected ? undefined : card.id)}
                  className={`bui-inner p-3.5 flex items-center justify-between cursor-pointer transition ${
                    isSelected
                      ? 'ring-2 ring-[var(--bui-accent-blue)] border-transparent'
                      : 'hover:border-[var(--bui-border-strong)]'
                  }`}
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <div
                      className="w-9 h-9 rounded-xl flex items-center justify-center text-white font-bold text-xs shrink-0"
                      style={{ backgroundColor: card.color_hex || '#18181B' }}
                    >
                      <CreditCard className="w-4 h-4" />
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-bold text-[var(--bui-text-primary)] truncate">
                          {card.name}
                        </span>
                        {isSelected && (
                          <span className="inline-flex items-center gap-0.5 text-[10px] font-bold text-[var(--bui-accent-blue)]">
                            <Check className="w-3 h-3" /> Filtro ativo
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-1.5 mt-1 flex-wrap">
                        <span className="text-[10px] px-1.5 py-0.5 rounded bui-badge-amber font-medium">
                          {cardInvoices.length || invoices.length} faturas sincronizadas
                        </span>
                        {openInvoiceCount > 0 && (
                          <span className="text-[10px] text-[var(--bui-text-muted)]">
                            • {openInvoiceCount} em aberto
                          </span>
                        )}
                      </div>
                    </div>
                  </div>

                  <div className="text-right shrink-0 pl-2">
                    <div className="text-sm font-bold font-mono-num text-[var(--bui-amber)]">
                      {formatCurrency(card.balance, isPrivacyMode)}
                    </div>
                    <div className="text-[10px] text-[var(--bui-text-muted)]">
                      Passivo Acumulado
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
