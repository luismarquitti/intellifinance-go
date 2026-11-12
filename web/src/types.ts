export type TransactionType = 'INCOME' | 'EXPENSE' | 'TRANSFER'
export type TransactionStatus = 'PENDING' | 'COMPLETED' | 'RECONCILED' | 'VOID'
export type Visibility = 'SHARED' | 'PERSONAL' | 'PRIVATE' | 'RESTRICTED'
export type AppSection = 'overview' | 'accounts' | 'transactions' | 'budgets' | 'hitl'

export interface Account {
  id: string
  tenant_id: string
  name: string
  type: string
  currency: string
  balance: string
  visibility: Visibility
  institution_id?: string
  institution_name?: string
  account_number_masked?: string
  color_hex: string
  icon_name: string
  is_active: boolean
}

export interface Category {
  id: string
  tenant_id: string
  parent_id?: string
  name: string
  type: TransactionType
  color_hex: string
  icon_name: string
  is_system: boolean
}

export interface Transaction {
  id: string
  tenant_id: string
  account_id: string
  category_id?: string
  date: string
  amount: string
  description: string
  clean_description: string
  type: TransactionType
  status: TransactionStatus
  visibility: Visibility
  source: string
  is_ai_categorized?: boolean
  ai_confidence?: number
  is_manually_verified?: boolean
  transfer_peer_transaction_id?: string
  bill_id?: string
  notes?: string
  metadata?: Record<string, any>
}

export interface TransactionSummary {
  total_income: string
  total_expense: string
  net_balance: string
  count: number
}

export interface Session {
  tenant_id: string
  user_id: string
  tenant_name: string
  user_name: string
  user_email: string
  environment: string
}

export interface CSVIngestResult {
  total_parsed: number
  inserted: number
  duplicates: number
  errors?: string[]
}

export interface TransactionFilters {
  page?: number
  limit?: number
  account_id?: string
  category_id?: string
  uncategorized?: boolean
  type?: string
  from_date?: string
  to_date?: string
  search?: string
  sort_by?: string
  sort_order?: string
}

export interface Pagination {
  page: number
  limit: number
  total: number
  total_pages: number
}

export interface TransactionListResponse {
  items: Transaction[]
  pagination: Pagination
}

export interface Invoice {
  id: string
  tenant_id: string
  account_id: string
  external_id?: string
  due_date: string
  closing_date?: string
  total_amount: string
  minimum_payment: string
  currency: string
  status: 'OPEN' | 'CLOSED' | 'PAID' | 'OVERDUE'
  allows_installments?: boolean
  metadata?: Record<string, any>
}

export interface InvoiceDetailsResponse {
  invoice: Invoice
  transactions: Transaction[]
}

export interface CashflowProjection {
  available_balance: string
  total_pending_bills: string
  projected_net: string
  upcoming_bills: Invoice[]
  liquid_accounts: Account[]
}

export interface PendingAction {
  id: string
  tenant_id: string
  created_by_agent: string
  action_type: 'RECONCILE_TRANSFER' | 'CATEGORIZE_TRANSACTION'
  payload: Record<string, any>
  status: 'PENDING' | 'APPROVED' | 'REJECTED' | 'EXECUTED'
  metadata: Record<string, any>
  created_at: string
}

export interface CreateTransactionPayload {
  account_id: string
  category_id?: string | null
  date: string
  amount: string
  description: string
  type: TransactionType
  status?: TransactionStatus
  visibility?: string
  notes?: string
  metadata?: Record<string, any>
}

export interface CreateAccountPayload {
  name: string
  type: string
  currency: string
  visibility: string
  institution_name?: string
  color_hex?: string
  icon_name?: string
  initial_balance?: string
}
