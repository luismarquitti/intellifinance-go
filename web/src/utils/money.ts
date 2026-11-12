/**
 * String-First & BigInt-Cents Financial Precision Utilities (FE-NFR-04)
 * Strictly avoids floating-point conversion for monetary values.
 */

export function decimalToCents(raw: string | number | undefined | null): bigint {
  if (raw === undefined || raw === null) return 0n
  const str = String(raw).trim()
  if (!str) return 0n

  // Normalize Brazilian comma or dot decimal separator
  let normalized = str.replace(/\s+/g, '')
  if (normalized.includes(',') && normalized.includes('.')) {
    // e.g. 1.234,56 -> 1234.56
    normalized = normalized.replace(/\./g, '').replace(',', '.')
  } else if (normalized.includes(',')) {
    normalized = normalized.replace(',', '.')
  }

  const isNegative = normalized.startsWith('-')
  const clean = normalized.replace(/^[+-]/, '')
  if (!/^\d+(\.\d+)?$/.test(clean)) {
    return 0n
  }

  const [intPartStr = '0', fracPartRaw = ''] = clean.split('.')
  const fracPadded = (fracPartRaw + '00').slice(0, 2)

  try {
    const cents = BigInt(intPartStr) * 100n + BigInt(fracPadded)
    return isNegative ? -cents : cents
  } catch {
    return 0n
  }
}

export function centsToDecimalString(cents: bigint): string {
  const isNegative = cents < 0n
  const abs = isNegative ? -cents : cents
  const intPart = abs / 100n
  const fracPart = abs % 100n
  const fracStr = fracPart.toString().padStart(2, '0')
  return `${isNegative ? '-' : ''}${intPart.toString()}.${fracStr}`
}

export function absDecimalString(raw: string | undefined | null): string {
  const cents = decimalToCents(raw)
  const abs = cents < 0n ? -cents : cents
  return centsToDecimalString(abs)
}

export function isValidPositiveMoneyInput(raw: string): boolean {
  const cents = decimalToCents(raw)
  return cents > 0n
}

export function toSignedDecimalString(raw: string, isExpense: boolean): string {
  const cents = decimalToCents(raw)
  const abs = cents < 0n ? -cents : cents
  return centsToDecimalString(isExpense ? -abs : abs)
}

export function isNegativeCents(raw: string | undefined | null): boolean {
  return decimalToCents(raw) < 0n
}

export function isNonNegativeCents(raw: string | undefined | null): boolean {
  return decimalToCents(raw) >= 0n
}

function formatThousandDots(intStr: string): string {
  return intStr.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
}

export function formatCurrency(
  valueStr: string | number | bigint | undefined | null,
  isPrivacyMode = false,
  options?: { showPlusSign?: boolean }
): string {
  if (isPrivacyMode) {
    return 'R$ •••••'
  }
  const cents = typeof valueStr === 'bigint' ? valueStr : decimalToCents(valueStr)
  const isNeg = cents < 0n
  const abs = isNeg ? -cents : cents
  const intPart = (abs / 100n).toString()
  const fracPart = (abs % 100n).toString().padStart(2, '0')
  const formattedInt = formatThousandDots(intPart)

  if (isNeg) {
    return `-R$ ${formattedInt},${fracPart}`
  }
  if (options?.showPlusSign && abs > 0n) {
    return `+R$ ${formattedInt},${fracPart}`
  }
  return `R$ ${formattedInt},${fracPart}`
}

export interface DailyBudgetMetrics {
  monthlyLimitCents: bigint
  spentCents: bigint
  remainingCents: bigint
  dailyAvailableCents: bigint
  daysRemaining: number
  usagePercentage: number
  status: 'SAFE' | 'WARNING' | 'CRITICAL'
}

export function calculateDailyBudgetMetrics(
  monthlyLimitStr: string,
  totalExpenseStr: string,
  referenceDate: Date = new Date()
): DailyBudgetMetrics {
  const limitCents = decimalToCents(monthlyLimitStr)
  const rawExpense = decimalToCents(totalExpenseStr)
  const spentCents = rawExpense < 0n ? -rawExpense : rawExpense
  const remainingCents = limitCents - spentCents

  const year = referenceDate.getFullYear()
  const month = referenceDate.getMonth()
  const lastDayOfMonth = new Date(year, month + 1, 0).getDate()
  const currentDay = referenceDate.getDate()
  const daysRemaining = Math.max(1, lastDayOfMonth - currentDay + 1)

  const dailyAvailableCents =
    remainingCents > 0n ? remainingCents / BigInt(daysRemaining) : 0n

  let usagePercentage = 0
  if (limitCents > 0n) {
    usagePercentage = Number((spentCents * 100n) / limitCents)
  }

  let status: 'SAFE' | 'WARNING' | 'CRITICAL' = 'SAFE'
  if (usagePercentage >= 100) {
    status = 'CRITICAL'
  } else if (usagePercentage >= 85) {
    status = 'WARNING'
  }

  return {
    monthlyLimitCents: limitCents,
    spentCents,
    remainingCents,
    dailyAvailableCents,
    daysRemaining,
    usagePercentage,
    status,
  }
}
