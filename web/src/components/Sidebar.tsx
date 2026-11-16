import React from 'react'
import {
  LayoutGrid,
  CreditCard,
  ArrowLeftRight,
  PieChart,
  Sparkles,
} from 'lucide-react'
import { AppSection, Session } from '../types'

interface SidebarProps {
  activeSection: AppSection
  onSelectSection: (section: AppSection) => void
  session: Session | null
  invoicesCount: number
  transactionsTotal: number
  pendingCount: number
}

export const Sidebar: React.FC<SidebarProps> = ({
  activeSection,
  onSelectSection,
  session,
  invoicesCount,
  transactionsTotal,
  pendingCount,
}) => {
  const navItems: {
    id: AppSection
    label: string
    mobileLabel: string
    icon: React.ReactNode
    badge?: string | number
    badgeType?: 'neutral' | 'amber' | 'blue'
  }[] = [
    {
      id: 'overview',
      label: 'Principal',
      mobileLabel: 'Principal',
      icon: <LayoutGrid className="w-4 h-4" aria-hidden="true" />,
    },
    {
      id: 'accounts',
      label: 'Cartões & Faturas',
      mobileLabel: 'Faturas',
      icon: <CreditCard className="w-4 h-4" aria-hidden="true" />,
      badge: invoicesCount > 0 ? `${invoicesCount} fat.` : undefined,
      badgeType: 'blue',
    },
    {
      id: 'transactions',
      label: 'Transações',
      mobileLabel: 'Transações',
      icon: <ArrowLeftRight className="w-4 h-4" aria-hidden="true" />,
      badge: transactionsTotal > 0 ? transactionsTotal : undefined,
      badgeType: 'neutral',
    },
    {
      id: 'budgets',
      label: 'Orçamentos & Fixas',
      mobileLabel: 'Orçamento',
      icon: <PieChart className="w-4 h-4" aria-hidden="true" />,
    },
    {
      id: 'hitl',
      label: 'Revisão IA (HITL)',
      mobileLabel: 'Revisão IA',
      icon: <Sparkles className="w-4 h-4" aria-hidden="true" />,
      badge: pendingCount > 0 ? pendingCount : undefined,
      badgeType: 'amber',
    },
  ]

  return (
    <>
      {/* Desktop Sidebar */}
      <aside className="w-[252px] shrink-0 border-r border-[var(--bui-border)] bg-[var(--bui-bg-sidebar)] flex-col justify-between p-4 select-none hidden lg:flex">
        <div className="space-y-6">
          {/* User / Workspace Header */}
          <div className="flex items-center gap-2.5 px-1.5 py-1">
            <div className="w-9 h-9 rounded-full bg-gradient-to-tr from-blue-600 to-indigo-500 flex items-center justify-center text-xs font-bold text-white shadow-sm shrink-0">
              {session?.user_name ? session.user_name.slice(0, 2).toUpperCase() : 'LM'}
            </div>
            <div className="min-w-0">
              <div className="text-xs font-semibold truncate text-[var(--bui-text-primary)]">
                {session?.user_name || 'Luis Marquitti'}
              </div>
              <div className="text-[11px] text-[var(--bui-text-muted)] flex items-center gap-1.5 truncate">
                <span className="bui-dot-lime inline-block shrink-0" aria-hidden="true" />
                <span className="truncate">{session?.tenant_name || 'Personal Pilot'}</span>
              </div>
            </div>
          </div>

          {/* Navigation Menu */}
          <div className="space-y-1.5">
            <div className="px-2.5 pb-1 text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-muted)]">
              Navegação
            </div>
            <nav aria-label="Navegação principal" className="space-y-1">
              {navItems.map((item) => {
                const isActive = activeSection === item.id
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => onSelectSection(item.id)}
                    aria-current={isActive ? 'page' : undefined}
                    className={`w-full flex items-center justify-between px-3 py-2.5 rounded-xl text-xs font-medium transition-all ${
                      isActive
                        ? 'bg-[var(--bui-accent-blue)] text-white font-semibold shadow-sm'
                        : 'text-[var(--bui-text-secondary)] hover:text-[var(--bui-text-primary)] hover:bg-[var(--bui-bg-hover)]'
                    }`}
                  >
                    <div className="flex items-center gap-2.5 min-w-0">
                      {item.icon}
                      <span className="truncate">{item.label}</span>
                    </div>
                    {item.badge !== undefined && (
                      <span
                        className={`text-[11px] px-1.5 py-0.5 rounded-md font-mono-num font-bold shrink-0 ${
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
              <span className="text-[11px] px-1.5 py-0.5 rounded font-semibold bui-badge-lime inline-flex items-center gap-1">
                <span className="bui-dot-lime inline-block" aria-hidden="true" />
                Ativo
              </span>
            </div>
            <div className="text-[11px] text-[var(--bui-text-muted)] leading-relaxed">
              Pluggy v2 sincronizado • Contas e faturas conciliadas.
            </div>
          </div>

          <div className="px-1 text-[11px] text-[var(--bui-text-muted)]">
            Board UI Design System
          </div>
        </div>
      </aside>

      {/* Fixed Mobile Bottom Navigation Bar */}
      <nav
        aria-label="Navegação móvel"
        className="lg:hidden fixed bottom-0 inset-x-0 z-40 border-t border-[var(--bui-border)] bg-[var(--bui-bg-sidebar)]/95 backdrop-blur-md px-1.5 pt-1.5 pb-[max(0.375rem,env(safe-area-inset-bottom,0px))] flex items-center justify-around"
      >
        {navItems.map((item) => {
          const isActive = activeSection === item.id
          return (
            <button
              key={item.id}
              type="button"
              onClick={() => onSelectSection(item.id)}
              aria-current={isActive ? 'page' : undefined}
              className={`flex flex-col items-center justify-center gap-0.5 px-2 py-1.5 rounded-xl text-[11px] font-medium transition min-w-[60px] ${
                isActive
                  ? 'text-[var(--bui-accent-blue)] font-bold bg-[var(--bui-blue-bg)]'
                  : 'text-[var(--bui-text-secondary)] hover:text-[var(--bui-text-primary)]'
              }`}
            >
              <div className="relative">
                {item.icon}
                {item.badge !== undefined && item.id === 'hitl' && (
                  <span className="absolute -top-1.5 -right-3 px-1 rounded-full text-[10px] leading-tight font-bold bui-badge-amber">
                    {item.badge}
                  </span>
                )}
              </div>
              <span className="truncate max-w-[70px]">{item.mobileLabel}</span>
            </button>
          )
        })}
      </nav>
    </>
  )
}
