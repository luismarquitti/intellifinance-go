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
  ChevronLeft,
  Calendar,
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
  selectedMonthLabel: string
  isAllPeriods: boolean
  isCurrentMonth?: boolean
  onPrevMonth: () => void
  onNextMonth: () => void
  onSelectCurrentMonth: () => void
  onToggleAllPeriods: () => void
  onOpenQuickEntryModal: () => void
  onOpenImportModal: () => void
  onOpenPendingModal?: () => void
}

const SECTION_TITLES: Record<AppSection, string> = {
  overview: 'Visão Geral',
  accounts: 'Cartões & Faturas',
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
  selectedMonthLabel,
  isAllPeriods,
  isCurrentMonth = false,
  onPrevMonth,
  onNextMonth,
  onSelectCurrentMonth,
  onToggleAllPeriods,
  onOpenQuickEntryModal,
  onOpenImportModal,
  onOpenPendingModal,
}) => {
  return (
    <header className="min-h-16 py-2.5 border-b border-[var(--bui-border)] bg-[var(--bui-bg-header)] px-4 sm:px-6 lg:px-8 sticky top-0 z-30 backdrop-blur-md">
      <div className="flex flex-wrap items-center justify-between gap-2.5">
        {/* Breadcrumb & Single Page <h1> */}
        <div className="flex items-center gap-2 text-xs text-[var(--bui-text-muted)] min-w-0">
          <span className="font-semibold text-[var(--bui-text-secondary)] hidden sm:inline">
            IntelliFinance
          </span>
          <ChevronRight className="w-3.5 h-3.5 hidden sm:inline opacity-50" aria-hidden="true" />
          <span className="hidden md:inline truncate">
            {session?.tenant_name || 'Personal'}
          </span>
          <ChevronRight className="w-3.5 h-3.5 hidden md:inline opacity-50" aria-hidden="true" />
          <h1 className="text-[var(--bui-text-primary)] font-bold text-sm sm:text-base truncate">
            {SECTION_TITLES[activeSection]}
          </h1>
        </div>

        {/* Action Controls (Single Authoritative Location) */}
        <div className="flex items-center gap-1.5 sm:gap-2 flex-wrap justify-end">
          {/* Privacy Mode Pill */}
          <button
            type="button"
            onClick={onTogglePrivacy}
            aria-pressed={isPrivacyMode}
            aria-label={isPrivacyMode ? 'Mostrar valores monetários' : 'Ativar Modo Privacidade'}
            className="bui-pill-control px-2.5 py-1.5 text-xs font-medium flex items-center gap-1.5"
            title={isPrivacyMode ? 'Mostrar valores monetários' : 'Ocultar valores monetários'}
          >
            {isPrivacyMode ? (
              <EyeOff className="w-3.5 h-3.5 text-[var(--bui-amber)]" aria-hidden="true" />
            ) : (
              <Eye className="w-3.5 h-3.5" aria-hidden="true" />
            )}
            <span className="hidden xl:inline">
              {isPrivacyMode ? 'Valores Ocultos' : 'Privacidade'}
            </span>
          </button>

          {/* Theme Switcher Pill */}
          <button
            type="button"
            onClick={onToggleTheme}
            aria-pressed={isDark}
            aria-label={isDark ? 'Alternar para Tema Claro' : 'Alternar para Tema Escuro'}
            className="bui-pill-control px-2.5 py-1.5 text-xs font-medium flex items-center gap-1.5"
            title={isDark ? 'Alternar para Tema Claro' : 'Alternar para Tema Escuro'}
          >
            {isDark ? (
              <>
                <Sun className="w-3.5 h-3.5 text-amber-400" aria-hidden="true" />
                <span className="hidden xl:inline">Claro</span>
              </>
            ) : (
              <>
                <Moon className="w-3.5 h-3.5 text-[var(--bui-accent-blue)]" aria-hidden="true" />
                <span className="hidden xl:inline">Escuro</span>
              </>
            )}
          </button>

          {/* HITL Queue Trigger */}
          {onOpenPendingModal && (
            <button
              type="button"
              onClick={() => {
                if (activeSection !== 'hitl') {
                  onOpenPendingModal()
                }
              }}
              aria-current={activeSection === 'hitl' ? 'page' : undefined}
              aria-label={`Central de Revisão IA (${pendingCount} pendências)`}
              className={`px-2.5 sm:px-3 py-1.5 rounded-[10px] text-xs font-semibold flex items-center gap-1.5 transition ${
                activeSection === 'hitl'
                  ? 'bg-[var(--bui-accent-blue)] text-white cursor-default'
                  : pendingCount > 0
                  ? 'bui-badge-amber'
                  : 'bui-pill-control'
              }`}
            >
              <Sparkles className="w-3.5 h-3.5" aria-hidden="true" />
              <span className="hidden sm:inline">HITL</span>
              {pendingCount > 0 && (
                <span
                  className={`font-mono-num font-bold px-1.5 py-0.5 rounded-full text-[11px] leading-none ${
                    activeSection === 'hitl' ? 'bg-white/20 text-white' : 'bg-amber-500/20'
                  }`}
                >
                  {pendingCount}
                </span>
              )}
            </button>
          )}

          {/* CSV Import Button */}
          <button
            type="button"
            onClick={onOpenImportModal}
            aria-label="Importar Extrato CSV"
            className="bui-pill-control px-2.5 sm:px-3 py-1.5 text-xs font-semibold flex items-center gap-1.5"
          >
            <Upload className="w-3.5 h-3.5" aria-hidden="true" />
            <span className="hidden md:inline">Importar CSV</span>
          </button>

          {/* Primary CTA: Quick Entry */}
          <button
            type="button"
            onClick={onOpenQuickEntryModal}
            className="px-3 sm:px-3.5 py-1.5 rounded-[10px] bg-[var(--bui-accent-blue)] hover:bg-[var(--bui-accent-blue-hover)] text-white text-xs font-semibold flex items-center gap-1.5 shadow-sm transition"
          >
            <Plus className="w-3.5 h-3.5" aria-hidden="true" />
            <span className="hidden sm:inline">Novo Lançamento</span>
            <span className="sm:hidden">Novo</span>
          </button>
        </div>
      </div>

      {/* Unified Global Period / Competence Selector (Single Source of Truth) */}
      <div className="mt-2 pt-2 border-t border-[var(--bui-border)] flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-1.5">
          <button
            type="button"
            onClick={onPrevMonth}
            className="bui-pill-control p-1.5"
            aria-label="Mês anterior"
            title="Mês anterior"
          >
            <ChevronLeft className="w-4 h-4" aria-hidden="true" />
          </button>
          <div className="px-2.5 min-w-[132px] text-center">
            <span className="text-[11px] uppercase tracking-wider text-[var(--bui-text-muted)] block font-semibold leading-tight">
              Competência
            </span>
            <span className="text-xs font-bold text-[var(--bui-text-primary)]">
              {isAllPeriods ? 'Todo o Histórico' : selectedMonthLabel}
            </span>
          </div>
          <button
            type="button"
            onClick={onNextMonth}
            className="bui-pill-control p-1.5"
            aria-label="Próximo mês"
            title="Próximo mês"
          >
            <ChevronRight className="w-4 h-4" aria-hidden="true" />
          </button>
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={onSelectCurrentMonth}
            aria-pressed={!isAllPeriods && isCurrentMonth}
            className={`px-3 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition ${
              !isAllPeriods && isCurrentMonth
                ? 'bg-[var(--bui-accent-blue)] text-white'
                : 'bui-pill-control'
            }`}
          >
            <Calendar className="w-3.5 h-3.5" aria-hidden="true" />
            <span>Mês Atual</span>
          </button>
          <button
            type="button"
            onClick={onToggleAllPeriods}
            aria-pressed={isAllPeriods}
            className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition ${
              isAllPeriods ? 'bui-badge-blue' : 'bui-pill-control'
            }`}
          >
            {isAllPeriods ? 'Todo o Histórico (Ativo)' : 'Todo o Histórico'}
          </button>
        </div>
      </div>
    </header>
  )
}
