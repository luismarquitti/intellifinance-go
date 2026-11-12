import React from 'react'
import {
  Upload,
  Sparkles,
  Plus,
  Sun,
  Moon,
  Eye,
  EyeOff,
  ChevronRight,
} from 'lucide-react'
import { AppSection, Session } from '../types'

interface HeaderProps {
  session: Session | null
  activeSection: AppSection
  pendingCount?: number
  isDark: boolean
  onToggleTheme: () => void
  isPrivacyMode: boolean
  onTogglePrivacy: () => void
  onOpenQuickEntryModal: () => void
  onOpenImportModal: () => void
  onOpenPendingModal?: () => void
}

const SECTION_TITLES: Record<AppSection, string> = {
  overview: 'Visão Geral',
  accounts: 'Contas & Faturas',
  transactions: 'Transações',
  budgets: 'Orçamentos & Fixas',
  hitl: 'Revisão IA (HITL)',
}

export const Header: React.FC<HeaderProps> = ({
  session,
  activeSection,
  pendingCount = 0,
  isDark,
  onToggleTheme,
  isPrivacyMode,
  onTogglePrivacy,
  onOpenQuickEntryModal,
  onOpenImportModal,
  onOpenPendingModal,
}) => {
  return (
    <header
      className="h-16 border-b px-4 sm:px-6 lg:px-8 flex items-center justify-between sticky top-0 z-30 backdrop-blur-md"
      style={{
        backgroundColor: isDark ? 'rgba(18, 18, 18, 0.88)' : 'rgba(255, 255, 255, 0.88)',
        borderColor: 'var(--bui-border)',
      }}
    >
      {/* Breadcrumb & Active Section */}
      <div className="flex items-center gap-2 text-xs text-[var(--bui-text-muted)] min-w-0">
        <span className="font-semibold text-[var(--bui-text-secondary)] hidden sm:inline">
          IntelliFinance
        </span>
        <ChevronRight className="w-3.5 h-3.5 hidden sm:inline opacity-50" />
        <span className="hidden md:inline truncate">
          {session?.tenant_name || 'Personal'}
        </span>
        <ChevronRight className="w-3.5 h-3.5 hidden md:inline opacity-50" />
        <span className="text-[var(--bui-text-primary)] font-bold text-sm truncate">
          {SECTION_TITLES[activeSection]}
        </span>
      </div>

      {/* Action Controls */}
      <div className="flex items-center gap-2 sm:gap-2.5">
        {/* Privacy Mode Pill */}
        <button
          onClick={onTogglePrivacy}
          className="bui-pill-control px-2.5 py-1.5 text-xs font-medium flex items-center gap-1.5"
          title={isPrivacyMode ? 'Mostrar valores monetários' : 'Ocultar valores monetários'}
        >
          {isPrivacyMode ? (
            <EyeOff className="w-3.5 h-3.5 text-[var(--bui-amber)]" />
          ) : (
            <Eye className="w-3.5 h-3.5" />
          )}
          <span className="hidden md:inline">
            {isPrivacyMode ? 'Valores Ocultos' : 'Privacidade'}
          </span>
        </button>

        {/* Theme Switcher Pill */}
        <button
          onClick={onToggleTheme}
          className="bui-pill-control px-2.5 py-1.5 text-xs font-medium flex items-center gap-1.5"
          title={isDark ? 'Alternar para Tema Claro' : 'Alternar para Tema Escuro'}
        >
          {isDark ? (
            <>
              <Sun className="w-3.5 h-3.5 text-amber-400" />
              <span className="hidden xl:inline">Claro</span>
            </>
          ) : (
            <>
              <Moon className="w-3.5 h-3.5 text-[var(--bui-accent-blue)]" />
              <span className="hidden xl:inline">Escuro</span>
            </>
          )}
        </button>

        {/* HITL Queue Trigger */}
        {onOpenPendingModal && (
          <button
            onClick={onOpenPendingModal}
            className={`px-3 py-1.5 rounded-[10px] text-xs font-semibold flex items-center gap-1.5 transition ${
              pendingCount > 0 ? 'bui-badge-amber' : 'bui-pill-control'
            }`}
          >
            <Sparkles className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">HITL</span>
            {pendingCount > 0 && (
              <span className="font-mono-num font-bold px-1.5 py-0.2 rounded-full text-[11px] bg-amber-500/20">
                {pendingCount}
              </span>
            )}
          </button>
        )}

        {/* CSV Import Button */}
        <button
          onClick={onOpenImportModal}
          className="bui-pill-control px-3 py-1.5 text-xs font-semibold flex items-center gap-1.5"
        >
          <Upload className="w-3.5 h-3.5" />
          <span className="hidden sm:inline">Importar CSV</span>
        </button>

        {/* Primary CTA: Quick Entry */}
        <button
          onClick={onOpenQuickEntryModal}
          className="px-3.5 py-1.5 rounded-[10px] bg-[var(--bui-accent-blue)] hover:bg-[var(--bui-accent-blue-hover)] text-white text-xs font-semibold flex items-center gap-1.5 shadow-sm transition"
        >
          <Plus className="w-3.5 h-3.5" />
          <span>Novo Lançamento</span>
        </button>
      </div>
    </header>
  )
}
