import React, { useEffect, useState, useCallback } from 'react'
import { Header } from './components/Header'
import { KPICards } from './components/KPICards'
import { FilterBar } from './components/FilterBar'
import { TransactionTable } from './components/TransactionTable'
import { BulkActionsBar } from './components/BulkActionsBar'
import { CSVImportModal } from './components/CSVImportModal'
import { EditTransactionModal } from './components/EditTransactionModal'
import { DeleteConfirmModal } from './components/DeleteConfirmModal'
import {
  Account,
  Category,
  Pagination,
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
  fetchCategories,
  fetchSession,
  fetchSummary,
  fetchTransactions,
  updateTransaction,
} from './api'

export const App: React.FC = () => {
  const [session, setSession] = useState<Session | null>(null)
  const [accounts, setAccounts] = useState<Account[]>([])
  const [categories, setCategories] = useState<Category[]>([])
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [summary, setSummary] = useState<TransactionSummary | null>(null)
  const [pagination, setPagination] = useState<Pagination>({
    page: 1,
    limit: 50,
    total_items: 0,
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
  const [isImportModalOpen, setIsImportModalOpen] = useState(false)
  const [editingTx, setEditingTx] = useState<Transaction | null>(null)
  const [deletingTx, setDeletingTx] = useState<Transaction | null>(null)
  const [isBatchDeleteModalOpen, setIsBatchDeleteModalOpen] = useState(false)
  const [isActionLoading, setIsActionLoading] = useState(false)

  // Load static context (Session, Accounts, Categories)
  const loadStaticData = useCallback(async () => {
    try {
      const [sess, accs, cats] = await Promise.all([
        fetchSession().catch(() => null),
        fetchAccounts().catch(() => []),
        fetchCategories().catch(() => []),
      ])
      if (sess) setSession(sess)
      setAccounts(accs)
      setCategories(cats)
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
    setFilters((prev) => ({ ...prev, ...newFilters }))
  }

  const handleResetFilters = () => {
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
  const handleInlineCategoryChange = async (txId: string, categoryId: string | null) => {
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

  return (
    <div className="min-h-screen bg-[#0E1410] text-[#E8EFEA] flex flex-col font-sans">
      {/* Header */}
      <Header
        session={session}
        onOpenImportModal={() => setIsImportModalOpen(true)}
      />

      {/* Main Container */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-6">
        {/* KPI Summary Cards */}
        <KPICards summary={summary} isLoading={isLoading} />

        {/* Filter & Search Bar */}
        <FilterBar
          accounts={accounts}
          categories={categories}
          filters={filters}
          onFilterChange={handleFilterChange}
          onResetFilters={handleResetFilters}
        />

        {/* Transaction Table */}
        <TransactionTable
          transactions={transactions}
          accounts={accounts}
          categories={categories}
          pagination={pagination}
          selectedIds={selectedIds}
          sortBy={filters.sort_by || 'date'}
          sortOrder={filters.sort_order || 'desc'}
          onToggleSelect={handleToggleSelect}
          onToggleSelectAll={handleToggleSelectAll}
          onSort={handleSort}
          onPageChange={(page) => handleFilterChange({ page })}
          onEdit={(tx) => setEditingTx(tx)}
          onDelete={(tx) => setDeletingTx(tx)}
          onInlineCategoryChange={handleInlineCategoryChange}
          isLoading={isLoading}
        />
      </main>

      {/* Floating Bulk Actions Bar */}
      <BulkActionsBar
        selectedCount={selectedIds.length}
        categories={categories}
        onClearSelection={() => setSelectedIds([])}
        onBatchDelete={() => setIsBatchDeleteModalOpen(true)}
        onBatchCategory={handleBatchCategory}
      />

      {/* Modals */}
      <CSVImportModal
        isOpen={isImportModalOpen}
        accounts={accounts}
        onClose={() => setIsImportModalOpen(false)}
        onSuccess={handleRefresh}
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
