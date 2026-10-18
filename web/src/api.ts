import {
  Account,
  Category,
  CSVIngestResult,
  Session,
  Transaction,
  TransactionFilters,
  TransactionListResponse,
  TransactionSummary,
} from './types'

const BASE_URL = '/api/v1'

async function handleResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let errorDetail = `Erro HTTP ${res.status}`
    try {
      const errJson = await res.json()
      if (errJson.detail) errorDetail = errJson.detail
      else if (errJson.title) errorDetail = errJson.title
    } catch {
      // fallback to status text
    }
    throw new Error(errorDetail)
  }
  if (res.status === 204) {
    return {} as T
  }
  return res.json()
}

export async function fetchSession(): Promise<Session> {
  const res = await fetch(`${BASE_URL}/session`)
  return handleResponse<Session>(res)
}

export async function fetchAccounts(): Promise<Account[]> {
  const res = await fetch(`${BASE_URL}/accounts`)
  const data = await handleResponse<{ items: Account[] }>(res)
  return data.items || []
}

export async function fetchCategories(): Promise<Category[]> {
  const res = await fetch(`${BASE_URL}/categories`)
  const data = await handleResponse<{ items: Category[] }>(res)
  return data.items || []
}

export async function fetchTransactions(
  filters: TransactionFilters
): Promise<TransactionListResponse> {
  const params = new URLSearchParams()
  if (filters.page) params.set('page', String(filters.page))
  if (filters.limit) params.set('limit', String(filters.limit))
  if (filters.account_id) params.set('account_id', filters.account_id)
  if (filters.category_id) params.set('category_id', filters.category_id)
  if (filters.uncategorized) params.set('uncategorized', 'true')
  if (filters.type) params.set('type', filters.type)
  if (filters.from_date) params.set('from_date', filters.from_date)
  if (filters.to_date) params.set('to_date', filters.to_date)
  if (filters.search) params.set('search', filters.search)
  if (filters.sort_by) params.set('sort_by', filters.sort_by)
  if (filters.sort_order) params.set('sort_order', filters.sort_order)

  const res = await fetch(`${BASE_URL}/transactions?${params.toString()}`)
  return handleResponse<TransactionListResponse>(res)
}

export async function fetchSummary(
  filters: TransactionFilters
): Promise<TransactionSummary> {
  const params = new URLSearchParams()
  if (filters.account_id) params.set('account_id', filters.account_id)
  if (filters.category_id) params.set('category_id', filters.category_id)
  if (filters.uncategorized) params.set('uncategorized', 'true')
  if (filters.type) params.set('type', filters.type)
  if (filters.from_date) params.set('from_date', filters.from_date)
  if (filters.to_date) params.set('to_date', filters.to_date)
  if (filters.search) params.set('search', filters.search)

  const res = await fetch(`${BASE_URL}/transactions/summary?${params.toString()}`)
  return handleResponse<TransactionSummary>(res)
}

export async function updateTransaction(
  id: string,
  data: Partial<{
    account_id: string
    category_id: string | null
    date: string
    amount: string
    type: string
    description: string
    notes: string
    status: string
    visibility: string
  }>
): Promise<Transaction> {
  const res = await fetch(`${BASE_URL}/transactions/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  return handleResponse<Transaction>(res)
}

export async function deleteTransaction(id: string): Promise<void> {
  const res = await fetch(`${BASE_URL}/transactions/${id}`, {
    method: 'DELETE',
  })
  await handleResponse<void>(res)
}

export async function batchDeleteTransactions(ids: string[]): Promise<void> {
  const res = await fetch(`${BASE_URL}/transactions/batch-delete`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids }),
  })
  await handleResponse<void>(res)
}

export async function batchUpdateCategory(
  ids: string[],
  categoryId: string | null
): Promise<void> {
  const res = await fetch(`${BASE_URL}/transactions/batch-category`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids, category_id: categoryId }),
  })
  await handleResponse<void>(res)
}

export async function importCSV(
  file: File,
  accountId: string
): Promise<CSVIngestResult> {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('account_id', accountId)

  const res = await fetch(`${BASE_URL}/transactions/import/csv`, {
    method: 'POST',
    body: formData,
  })
  return handleResponse<CSVIngestResult>(res)
}
