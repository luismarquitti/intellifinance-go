export type TransactionType = 'INCOME' | 'EXPENSE' | 'TRANSFER'
export type TransactionStatus = 'PENDING' | 'COMPLETED' | 'RECONCILED' | 'VOID'
export type Visibility = 'SHARED' | 'PERSONAL' | 'RESTRICTED'

export interface Account {
  id: string
  tenant_id: string
  name: string
  type: string
  currency: string
  balance: string
  visibility: Visibility
  institution_name?: string
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
  notes?: string
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
  total_items: number
  total_pages: number
}

export interface TransactionListResponse {
  items: Transaction[]
  pagination: Pagination
}
