import React, { useEffect, useState, useCallback, useMemo } from 'react'
import { Sidebar } from './components/Sidebar'
import { Header } from './components/Header'
import { KPICards } from './components/KPICards'
import { CashflowCard } from './components/CashflowCard'
import { AccountsRail } from './components/AccountsRail'
import { InvoicesTimeline } from './components/InvoicesTimeline'
import { CategoryBreakdownPanel } from './components/CategoryBreakdownPanel'
import { BudgetOverviewPanel } from './components/BudgetOverviewPanel'
import { FilterBar } from './components/FilterBar'
import { TransactionTable } from './components/TransactionTable'
import { BulkActionsBar } from './components/BulkActionsBar'
import { QuickTransactionModal } from './components/QuickTransactionModal'
import { CSVImportModal } from './components/CSVImportModal'
import { EditTransactionModal } from './components/EditTransactionModal'
import { DeleteConfirmModal } from './components/DeleteConfirmModal'
import { PendingActionsModal } from './components/PendingActionsModal'
import {
  Account,
  AppSection,
  Category,
  CashflowProjection,
  Invoice,
  Pagination,
  PendingAction,
  Session,
  Transaction,
  TransactionFilters,
  TransactionSummary,
} from './types'
import {
  batchDeleteTransactions,
  batchUpdateCategory,
  deleteTransaction,
  fetchAccounts,
  fetchCashflowProjection,
  fetchCategories,
  fetchInvoices,
  fetchPendingActions,
  fetchSession,
  fetchSummary,
  fetchTransactions,
  updateTransaction,
} from './api'

const MONTH_NAMES = [
  'Jan',
  'Fev',
  'Mar',
  'Abr',
  'Mai',
  'Jun',
  'Jul',
  'Ago',
  'Set',
  'Out',
  'Nov',
  'Dez',
]

export const App: React.FC = () => {
  // Theme state: Light Mode default (false), Dark Mode optional (true)
  const [isDark, setIsDark] = useState(false)
  const [isPrivacyMode, setIsPrivacyMode] = useState(false)
  const [activeSection, setActiveSection] = useState<AppSection>('overview')

  // Global Month Selector state (defaults to All Periods so all Open Finance data is immediately visible, or user can step month-by-month)
  const [monthCursor, setMonthCursor] = useState<Date>(() => new Date())
  const [isAllPeriods, setIsAllPeriods] = useState(true)
  const [monthlyBudgetLimit, setMonthlyBudgetLimit] = useState('12000.00')

  // Domain Data state
  const [session, setSession] = useState<Session | null>(null)
  const [accounts, setAccounts] = useState<Account[]>([])
  const [categories, setCategories] = useState<Category[]>([])
  const [invoices, setInvoices] = useState<Invoice[]>([])
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [summary, setSummary] = useState<TransactionSummary | null>(null)
  const [projection, setProjection] = useState<CashflowProjection | null>(null)
  const [pendingActions, setPendingActions] = useState<PendingAction[]>([])
  const [pagination, setPagination] = useState<Pagination>({
    page: 1,
    limit: 50,
    total: 0,
    total_pages: 1,
  })

  const [filters, setFilters] = useState<TransactionFilters>({
    page: 1,
    limit: 50,
    sort_by: 'date',
    sort_order: 'desc',
  })

  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [isLoading, setIsLoading] = useState(false)

  // Modals state
  const [isQuickEntryOpen, setIsQuickEntryOpen] = useState(false)
  const [isImportModalOpen, setIsImportModalOpen] = useState(false)
  const [isPendingModalOpen, setIsPendingModalOpen] = useState(false)
  const [editingTx, setEditingTx] = useState<Transaction | null>(null)
  const [deletingTx, setDeletingTx] = useState<Transaction | null>(null)
  const [isBatchDeleteModalOpen, setIsBatchDeleteModalOpen] = useState(false)
  const [isActionLoading, setIsActionLoading] = useState(false)

  // Synchronize HTML root `.dark` class with React state (Light Mode is default)
  useEffect(() => {
    document.documentElement.classList.toggle('dark', isDark)
  }, [isDark])

  const selectedMonthLabel = useMemo(() => {
    return `${MONTH_NAMES[monthCursor.getMonth()]} ${monthCursor.getFullYear()}`
  }, [monthCursor])

  const applyMonthBoundsToFilters = useCallback((cursor: Date) => {
    const y = cursor.getFullYear()
    const m = cursor.getMonth()
    const firstDay = new Date(y, m, 1).toISOString().split('T')[0]
    const lastDay = new Date(y, m + 1, 0).toISOString().split('T')[0]
    setIsAllPeriods(false)
    setFilters((prev) => ({
      ...prev,
      from_date: firstDay,
      to_date: lastDay,
      page: 1,
    }))
  }, [])

  const handlePrevMonth = () => {
    const prev = new Date(monthCursor.getFullYear(), monthCursor.getMonth() - 1, 1)
    setMonthCursor(prev)
    applyMonthBoundsToFilters(prev)
  }

  const handleNextMonth = () => {
    const next = new Date(monthCursor.getFullYear(), monthCursor.getMonth() + 1, 1)
    setMonthCursor(next)
    applyMonthBoundsToFilters(next)
  }

  const handleToggleAllPeriods = () => {
    if (isAllPeriods) {
      applyMonthBoundsToFilters(monthCursor)
    } else {
      setIsAllPeriods(true)
      setFilters((prev) => ({
        ...prev,
        from_date: undefined,
        to_date: undefined,
        page: 1,
      }))
    }
  }

  // Load static context (Session, Accounts, Categories, Cashflow, Invoices, Pending Actions)
  const loadStaticData = useCallback(async () => {
    try {
      const [sess, accs, cats, proj, invs, acts] = await Promise.all([
        fetchSession().catch(() => null),
        fetchAccounts().catch(() => []),
        fetchCategories().catch(() => []),
        fetchCashflowProjection().catch(() => null),
        fetchInvoices().catch(() => []),
        fetchPendingActions().catch(() => []),
      ])
      if (sess) setSession(sess)
      setAccounts(accs)
      setCategories(cats)
      setProjection(proj)
      setInvoices(invs)
      setPendingActions(acts)
    } catch (err) {
      console.error('Failed to load initial data:', err)
    }
  }, [])

  useEffect(() => {
    loadStaticData()
  }, [loadStaticData])

  // Load dynamic transactions & KPI summary based on filters
  const loadTransactionsData = useCallback(async () => {
    try {
      setIsLoading(true)
      const [txList, summ] = await Promise.all([
        fetchTransactions(filters),
        fetchSummary(filters),
      ])
      setTransactions(txList.items)
      setPagination(txList.pagination)
      setSummary(summ)
    } catch (err) {
      console.error('Failed to load transactions:', err)
    } finally {
      setIsLoading(false)
    }
  }, [filters])

  useEffect(() => {
    loadTransactionsData()
  }, [loadTransactionsData])

  // Reload everything after mutation
  const handleRefresh = useCallback(() => {
    loadStaticData()
    loadTransactionsData()
    setSelectedIds([])
  }, [loadStaticData, loadTransactionsData])

  // Filter actions
  const handleFilterChange = (newFilters: Partial<TransactionFilters>) => {
    if (newFilters.from_date !== undefined || newFilters.to_date !== undefined) {
      setIsAllPeriods(!newFilters.from_date && !newFilters.to_date)
    }
    setFilters((prev) => ({ ...prev, ...newFilters }))
  }

  const handleResetFilters = () => {
    setIsAllPeriods(true)
    setFilters({
      page: 1,
      limit: 50,
      sort_by: 'date',
      sort_order: 'desc',
    })
  }

  // Sorting
  const handleSort = (column: string) => {
    setFilters((prev) => {
      let order = 'desc'
      if (prev.sort_by === column) {
        order = prev.sort_order === 'asc' ? 'desc' : 'asc'
      }
      return { ...prev, sort_by: column, sort_order: order, page: 1 }
    })
  }

  // Selection
  const handleToggleSelect = (id: string) => {
    setSelectedIds((prev) =>
      prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id]
    )
  }

  const handleToggleSelectAll = () => {
    if (transactions.length === 0) return
    const allSelected = transactions.every((t) => selectedIds.includes(t.id))
    if (allSelected) {
      setSelectedIds([])
    } else {
      setSelectedIds(transactions.map((t) => t.id))
    }
  }

  // Inline category classification (1-click)
  const handleInlineCategoryChange = async (
    txId: string,
    categoryId: string | null
  ) => {
    try {
      await updateTransaction(txId, { category_id: categoryId })
      handleRefresh()
    } catch (err) {
      console.error('Failed to update category inline:', err)
    }
  }

  // Bulk actions
  const handleBatchCategory = async (categoryId: string | null) => {
    if (selectedIds.length === 0) return
    try {
      await batchUpdateCategory(selectedIds, categoryId)
      handleRefresh()
    } catch (err) {
      console.error('Failed to update category in batch:', err)
    }
  }

  const handleConfirmBatchDelete = async () => {
    if (selectedIds.length === 0) return
    try {
      setIsActionLoading(true)
      await batchDeleteTransactions(selectedIds)
      setIsBatchDeleteModalOpen(false)
      handleRefresh()
    } catch (err) {
      console.error('Failed to batch delete transactions:', err)
    } finally {
      setIsActionLoading(false)
    }
  }

  const handleConfirmDelete = async () => {
    if (!deletingTx) return
    try {
      setIsActionLoading(true)
      await deleteTransaction(deletingTx.id)
      setDeletingTx(null)
      handleRefresh()
    } catch (err) {
      console.error('Failed to delete transaction:', err)
    } finally {
      setIsActionLoading(false)
    }
  }

  const hasActiveFilters = Boolean(
    filters.search ||
      filters.account_id ||
      filters.category_id ||
      filters.uncategorized ||
      filters.type ||
      filters.from_date ||
      filters.to_date
  )

  return (
    <div className="min-h-screen flex flex-col lg:flex-row font-sans bg-[var(--bui-bg-canvas)] text-[var(--bui-text-primary)]">
      {/* Left Board UI Sidebar */}
      <Sidebar
        activeSection={activeSection}
        onSelectSection={setActiveSection}
        session={session}
        isDark={isDark}
        onToggleTheme={() => setIsDark((prev) => !prev)}
        isPrivacyMode={isPrivacyMode}
        onTogglePrivacy={() => setIsPrivacyMode((prev) => !prev)}
        selectedMonthLabel={selectedMonthLabel}
        isAllPeriods={isAllPeriods}
        onPrevMonth={handlePrevMonth}
        onNextMonth={handleNextMonth}
        onToggleAllPeriods={handleToggleAllPeriods}
        invoicesCount={invoices.length}
        transactionsTotal={pagination.total}
        pendingCount={pendingActions.length}
      />

      {/* Right Main Workspace */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Topbar */}
        <Header
          session={session}
          activeSection={activeSection}
          pendingCount={pendingActions.length}
          isDark={isDark}
          onToggleTheme={() => setIsDark((prev) => !prev)}
          isPrivacyMode={isPrivacyMode}
          onTogglePrivacy={() => setIsPrivacyMode((prev) => !prev)}
          onOpenQuickEntryModal={() => setIsQuickEntryOpen(true)}
          onOpenImportModal={() => setIsImportModalOpen(true)}
          onOpenPendingModal={() => setIsPendingModalOpen(true)}
        />

        {/* Section Content */}
        <main className="flex-1 max-w-[1440px] w-full mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-5">
          {/* Top KPI Row is visible on Overview, Accounts, and Transactions */}
          {activeSection !== 'hitl' && (
            <KPICards
              summary={summary}
              projection={projection}
              monthlyBudgetLimit={monthlyBudgetLimit}
              isPrivacyMode={isPrivacyMode}
              isLoading={isLoading}
              onInspectInvoices={() => setActiveSection('accounts')}
              onInspectBudgets={() => setActiveSection('budgets')}
            />
          )}

          {/* SECTION 1: OVERVIEW (VISÃO GERAL) */}
          {activeSection === 'overview' && (
            <>
              <CashflowCard
                projection={projection}
                isPrivacyMode={isPrivacyMode}
                isLoading={isLoading}
                onInspectInvoices={() => setActiveSection('accounts')}
              />

              <CategoryBreakdownPanel
                transactions={transactions}
                categories={categories}
                selectedCategoryId={filters.category_id}
                uncategorizedFilter={filters.uncategorized}
                isPrivacyMode={isPrivacyMode}
                onSelectCategory={(categoryId, uncategorized) =>
                  handleFilterChange({
                    category_id: categoryId,
                    uncategorized: uncategorized || undefined,
                    page: 1,
                  })
                }
              />

              <AccountsRail
                accounts={accounts}
                invoices={invoices}
                selectedAccountId={filters.account_id}
                isPrivacyMode={isPrivacyMode}
                onSelectAccount={(accountId) =>
                  handleFilterChange({ account_id: accountId, page: 1 })
                }
                onOpenInvoicesHub={() => setActiveSection('accounts')}
                onAccountCreated={handleRefresh}
              />

              <FilterBar
                accounts={accounts}
                categories={categories}
                filters={filters}
                onFilterChange={handleFilterChange}
                onResetFilters={handleResetFilters}
              />

              <TransactionTable
                transactions={transactions}
                accounts={accounts}
                categories={categories}
                pagination={pagination}
                selectedIds={selectedIds}
                sortBy={filters.sort_by || 'date'}
                sortOrder={filters.sort_order || 'desc'}
                isPrivacyMode={isPrivacyMode}
                hasActiveFilters={hasActiveFilters}
                onResetFilters={handleResetFilters}
                onToggleSelect={handleToggleSelect}
                onToggleSelectAll={handleToggleSelectAll}
                onSort={handleSort}
                onPageChange={(page) => handleFilterChange({ page })}
                onEdit={(tx) => setEditingTx(tx)}
                onDelete={(tx) => setDeletingTx(tx)}
                onInlineCategoryChange={handleInlineCategoryChange}
                isLoading={isLoading}
              />
            </>
          )}

          {/* SECTION 2: ACCOUNTS & INVOICES HUB */}
          {activeSection === 'accounts' && (
            <>
              <AccountsRail
                accounts={accounts}
                invoices={invoices}
                selectedAccountId={filters.account_id}
                isPrivacyMode={isPrivacyMode}
                onSelectAccount={(accountId) =>
                  handleFilterChange({ account_id: accountId, page: 1 })
                }
                onAccountCreated={handleRefresh}
              />

              <InvoicesTimeline
                invoices={invoices}
                accounts={accounts}
                categories={categories}
                isPrivacyMode={isPrivacyMode}
                onInlineCategoryChange={handleInlineCategoryChange}
              />
            </>
          )}

          {/* SECTION 3: TRANSACTIONS EXPLORER */}
          {activeSection === 'transactions' && (
            <>
              <FilterBar
                accounts={accounts}
                categories={categories}
                filters={filters}
                onFilterChange={handleFilterChange}
                onResetFilters={handleResetFilters}
              />

              <TransactionTable
                transactions={transactions}
                accounts={accounts}
                categories={categories}
                pagination={pagination}
                selectedIds={selectedIds}
                sortBy={filters.sort_by || 'date'}
                sortOrder={filters.sort_order || 'desc'}
                isPrivacyMode={isPrivacyMode}
                hasActiveFilters={hasActiveFilters}
                onResetFilters={handleResetFilters}
                onToggleSelect={handleToggleSelect}
                onToggleSelectAll={handleToggleSelectAll}
                onSort={handleSort}
                onPageChange={(page) => handleFilterChange({ page })}
                onEdit={(tx) => setEditingTx(tx)}
                onDelete={(tx) => setDeletingTx(tx)}
                onInlineCategoryChange={handleInlineCategoryChange}
                isLoading={isLoading}
              />
            </>
          )}

          {/* SECTION 4: BUDGETS & RECURRING COMMITMENTS */}
          {activeSection === 'budgets' && (
            <BudgetOverviewPanel
              summary={summary}
              transactions={transactions}
              categories={categories}
              monthlyBudgetLimit={monthlyBudgetLimit}
              onChangeMonthlyBudgetLimit={setMonthlyBudgetLimit}
              isPrivacyMode={isPrivacyMode}
            />
          )}

          {/* SECTION 5: HITL AI REVIEW CENTER (INLINE) */}
          {activeSection === 'hitl' && (
            <PendingActionsModal
              isOpen={true}
              inlineMode={true}
              isPrivacyMode={isPrivacyMode}
              actions={pendingActions}
              onClose={() => setActiveSection('overview')}
              onRefresh={handleRefresh}
            />
          )}
        </main>
      </div>

      {/* Floating Bulk Actions Bar */}
      <BulkActionsBar
        selectedCount={selectedIds.length}
        categories={categories}
        onClearSelection={() => setSelectedIds([])}
        onBatchDelete={() => setIsBatchDeleteModalOpen(true)}
        onBatchCategory={handleBatchCategory}
      />

      {/* Modals */}
      <QuickTransactionModal
        isOpen={isQuickEntryOpen}
        accounts={accounts}
        categories={categories}
        onClose={() => setIsQuickEntryOpen(false)}
        onSuccess={handleRefresh}
      />

      <CSVImportModal
        isOpen={isImportModalOpen}
        accounts={accounts}
        onClose={() => setIsImportModalOpen(false)}
        onSuccess={handleRefresh}
      />

      <PendingActionsModal
        isOpen={isPendingModalOpen}
        isPrivacyMode={isPrivacyMode}
        actions={pendingActions}
        onClose={() => setIsPendingModalOpen(false)}
        onRefresh={handleRefresh}
      />

      <EditTransactionModal
        isOpen={Boolean(editingTx)}
        transaction={editingTx}
        accounts={accounts}
        categories={categories}
        onClose={() => setEditingTx(null)}
        onSuccess={handleRefresh}
      />

      {/* Delete Single Modal */}
      <DeleteConfirmModal
        isOpen={Boolean(deletingTx)}
        count={1}
        description={deletingTx?.description}
        onClose={() => setDeletingTx(null)}
        onConfirm={handleConfirmDelete}
        isDeleting={isActionLoading}
      />

      {/* Delete Batch Modal */}
      <DeleteConfirmModal
        isOpen={isBatchDeleteModalOpen}
        count={selectedIds.length}
        onClose={() => setIsBatchDeleteModalOpen(false)}
        onConfirm={handleConfirmBatchDelete}
        isDeleting={isActionLoading}
      />
    </div>
  )
}

export default App
