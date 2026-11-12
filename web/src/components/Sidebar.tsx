import React from 'react'
import {
  LayoutGrid,
  CreditCard,
  ArrowLeftRight,
  PieChart,
  Sparkles,
  ChevronLeft,
  ChevronRight,
  Sun,
  Moon,
  Eye,
  EyeOff,
  Calendar,
} from 'lucide-react'
import { AppSection, Session } from '../types'

interface SidebarProps {
  activeSection: AppSection
  onSelectSection: (section: AppSection) => void
  session: Session | null
  isDark: boolean
  onToggleTheme: () => void
  isPrivacyMode: boolean
  onTogglePrivacy: () => void
  selectedMonthLabel: string
  isAllPeriods: boolean
  onPrevMonth: () => void
  onNextMonth: () => void
  onToggleAllPeriods: () => void
  invoicesCount: number
  transactionsTotal: number
  pendingCount: number
}

export const Sidebar: React.FC<SidebarProps> = ({
  activeSection,
  onSelectSection,
  session,
  isDark,
  onToggleTheme,
  isPrivacyMode,
  onTogglePrivacy,
  selectedMonthLabel,
  isAllPeriods,
  onPrevMonth,
  onNextMonth,
  onToggleAllPeriods,
  invoicesCount,
  transactionsTotal,
  pendingCount,
}) => {
  const navItems: {
    id: AppSection
    label: string
    icon: React.ReactNode
    badge?: string | number
    badgeType?: 'neutral' | 'amber' | 'blue'
  }[] = [
    {
      id: 'overview',
      label: 'Principal',
      icon: <LayoutGrid className="w-4 h-4" />,
    },
    {
      id: 'accounts',
      label: 'Contas & Faturas',
      icon: <CreditCard className="w-4 h-4" />,
      badge: invoicesCount > 0 ? `${invoicesCount} fat.` : undefined,
      badgeType: 'blue',
    },
    {
      id: 'transactions',
      label: 'Transações',
      icon: <ArrowLeftRight className="w-4 h-4" />,
      badge: transactionsTotal > 0 ? transactionsTotal : undefined,
      badgeType: 'neutral',
    },
    {
      id: 'budgets',
      label: 'Orçamentos & Fixas',
      icon: <PieChart className="w-4 h-4" />,
    },
    {
      id: 'hitl',
      label: 'Revisão IA (HITL)',
      icon: <Sparkles className="w-4 h-4" />,
      badge: pendingCount > 0 ? pendingCount : undefined,
      badgeType: 'amber',
    },
  ]

  return (
    <>
      {/* Desktop Sidebar */}
      <aside
        className="w-[258px] shrink-0 border-r flex flex-col justify-between p-4 select-none hidden lg:flex"
        style={{
          backgroundColor: 'var(--bui-bg-sidebar)',
          borderColor: 'var(--bui-border)',
        }}
      >
        <div className="space-y-5">
          {/* User / Workspace Header */}
          <div className="flex items-center justify-between px-1.5 py-1">
            <div className="flex items-center gap-2.5 min-w-0">
              <div className="w-8 h-8 rounded-full bg-gradient-to-tr from-blue-600 to-indigo-500 flex items-center justify-center text-xs font-bold text-white shadow-sm shrink-0">
                {session?.user_name ? session.user_name.slice(0, 2).toUpperCase() : 'LM'}
              </div>
              <div className="min-w-0">
                <div className="text-xs font-semibold truncate text-[var(--bui-text-primary)]">
                  {session?.user_name || 'Luis Marquitti'}
                </div>
                <div className="text-[10px] text-[var(--bui-text-muted)] flex items-center gap-1.5 truncate">
                  <span className="bui-dot-lime inline-block shrink-0" />
                  <span className="truncate">{session?.tenant_name || 'Personal Pilot'}</span>
                </div>
              </div>
            </div>

            <div className="flex items-center gap-1 shrink-0">
              <button
                onClick={onTogglePrivacy}
                className="bui-pill-control p-1.5"
                title={isPrivacyMode ? 'Mostrar valores' : 'Modo Privacidade (ocultar valores)'}
                aria-label="Alternar Modo Privacidade"
              >
                {isPrivacyMode ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
              </button>
              <button
                onClick={onToggleTheme}
                className="bui-pill-control p-1.5"
                title={isDark ? 'Mudar para Tema Claro (Padrão)' : 'Mudar para Tema Escuro'}
                aria-label="Alternar Tema Claro ou Escuro"
              >
                {isDark ? (
                  <Sun className="w-3.5 h-3.5 text-amber-400" />
                ) : (
                  <Moon className="w-3.5 h-3.5 text-[var(--bui-accent-blue)]" />
                )}
              </button>
            </div>
          </div>

          {/* Global Month / Period Selector */}
          <div className="space-y-1.5">
            <div className="bui-inner px-2.5 py-2 flex items-center justify-between">
              <button
                onClick={onPrevMonth}
                className="p-1 rounded-md hover:bg-[var(--bui-bg-hover)] text-[var(--bui-text-secondary)] hover:text-[var(--bui-text-primary)] transition"
                title="Mês anterior"
              >
                <ChevronLeft className="w-3.5 h-3.5" />
              </button>
              <div className="text-center">
                <span className="text-[10px] uppercase tracking-wider text-[var(--bui-text-muted)] block font-semibold">
                  Período Ativo
                </span>
                <span className="text-xs font-bold text-[var(--bui-text-primary)]">
                  {isAllPeriods ? 'Todo o Histórico' : selectedMonthLabel}
                </span>
              </div>
              <button
                onClick={onNextMonth}
                className="p-1 rounded-md hover:bg-[var(--bui-bg-hover)] text-[var(--bui-text-secondary)] hover:text-[var(--bui-text-primary)] transition"
                title="Próximo mês"
              >
                <ChevronRight className="w-3.5 h-3.5" />
              </button>
            </div>

            <button
              onClick={onToggleAllPeriods}
              className={`w-full py-1.5 px-2.5 rounded-lg text-[11px] font-medium flex items-center justify-center gap-1.5 transition border ${
                isAllPeriods
                  ? 'bui-badge-blue font-semibold'
                  : 'border-transparent text-[var(--bui-text-muted)] hover:text-[var(--bui-text-primary)] hover:bg-[var(--bui-bg-hover)]'
              }`}
            >
              <Calendar className="w-3 h-3" />
              <span>{isAllPeriods ? 'Filtrar por Mês Atual' : 'Ver Todo o Histórico'}</span>
            </button>
          </div>

          {/* Navigation Menu */}
          <div className="space-y-1">
            <div className="px-2.5 pb-1 text-[10px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)]">
              Navegação
            </div>
            <nav className="space-y-1">
              {navItems.map((item) => {
                const isActive = activeSection === item.id
                return (
                  <button
                    key={item.id}
                    onClick={() => onSelectSection(item.id)}
                    className={`w-full flex items-center justify-between px-3 py-2.5 rounded-xl text-xs font-medium transition-all ${
                      isActive
                        ? 'bg-[var(--bui-accent-blue)] text-white font-semibold shadow-sm'
                        : 'text-[var(--bui-text-secondary)] hover:text-[var(--bui-text-primary)] hover:bg-[var(--bui-bg-hover)]'
                    }`}
                  >
                    <div className="flex items-center gap-2.5">
                      {item.icon}
                      <span>{item.label}</span>
                    </div>
                    {item.badge !== undefined && (
                      <span
                        className={`text-[10px] px-1.5 py-0.5 rounded-md font-mono-num font-bold ${
                          isActive
                            ? 'bg-white/20 text-white'
                            : item.badgeType === 'amber'
                            ? 'bui-badge-amber'
                            : item.badgeType === 'blue'
                            ? 'bui-badge-blue'
                            : 'bg-[var(--bui-bg-inner)] text-[var(--bui-text-secondary)] border border-[var(--bui-border)]'
                        }`}
                      >
                        {item.badge}
                      </span>
                    )}
                  </button>
                )
              })}
            </nav>
          </div>
        </div>

        {/* Bottom Open Finance Status Card */}
        <div className="space-y-3 pt-4 border-t border-[var(--bui-border)]">
          <div className="bui-card p-3 space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-[11px] font-semibold text-[var(--bui-text-secondary)]">
                Open Finance Brasil
              </span>
              <span className="text-[10px] px-1.5 py-0.5 rounded font-semibold bui-badge-lime">
                Ativo
              </span>
            </div>
            <div className="text-[11px] text-[var(--bui-text-muted)] leading-relaxed">
              Pluggy v2 sincronizado • Contas e faturas conciliadas.
            </div>
          </div>

          <div className="flex items-center justify-between px-1 text-[11px] text-[var(--bui-text-muted)]">
            <span>Board UI Design</span>
            <button
              onClick={onToggleTheme}
              className="font-semibold text-[var(--bui-text-secondary)] hover:text-[var(--bui-text-primary)] transition"
            >
              {isDark ? '☀️ Tema Claro' : '🌙 Tema Escuro'}
            </button>
          </div>
        </div>
      </aside>

      {/* Mobile Navigation Bar */}
      <div
        className="lg:hidden sticky bottom-0 inset-x-0 z-40 border-t px-2 py-1.5 flex items-center justify-around"
        style={{
          backgroundColor: 'var(--bui-bg-sidebar)',
          borderColor: 'var(--bui-border)',
        }}
      >
        {navItems.map((item) => {
          const isActive = activeSection === item.id
          return (
            <button
              key={item.id}
              onClick={() => onSelectSection(item.id)}
              className={`flex flex-col items-center gap-0.5 px-2.5 py-1.5 rounded-xl text-[10px] font-medium transition ${
                isActive
                  ? 'text-[var(--bui-accent-blue)] font-bold'
                  : 'text-[var(--bui-text-secondary)]'
              }`}
            >
              <div className="relative">
                {item.icon}
                {item.badge !== undefined && item.id === 'hitl' && (
                  <span className="absolute -top-1.5 -right-2.5 px-1 rounded-full text-[9px] font-bold bg-amber-500 text-black">
                    {item.badge}
                  </span>
                )}
              </div>
              <span className="truncate max-w-[68px]">{item.label.split(' ')[0]}</span>
            </button>
          )
        })}
      </div>
    </>
  )
}
