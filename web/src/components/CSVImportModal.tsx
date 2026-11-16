import React, { useState, useRef, useEffect, useId } from 'react'
import { Upload, X, FileText, CheckCircle2, AlertTriangle, ArrowRight } from 'lucide-react'
import { Account, CSVIngestResult } from '../types'
import { importCSV } from '../api'
import { ModalDialog } from './ModalDialog'

interface CSVImportModalProps {
  isOpen: boolean
  accounts: Account[]
  onClose: () => void
  onSuccess: () => void
}

export const CSVImportModal: React.FC<CSVImportModalProps> = ({
  isOpen,
  accounts,
  onClose,
  onSuccess,
}) => {
  const [selectedAccountId, setSelectedAccountId] = useState<string>(
    accounts[0]?.id || ''
  )
  const [file, setFile] = useState<File | null>(null)
  const [isUploading, setIsUploading] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [result, setResult] = useState<CSVIngestResult | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const uid = useId()
  const titleId = `${uid}-title`
  const accountSelectId = `${uid}-account`
  const fileInputId = `${uid}-file`

  useEffect(() => {
    if (isOpen && !selectedAccountId && accounts.length > 0) {
      setSelectedAccountId(accounts[0].id)
    }
  }, [isOpen, selectedAccountId, accounts])

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      const f = e.target.files[0]
      if (!f.name.toLowerCase().endsWith('.csv')) {
        setErrorMessage('Por favor, selecione um arquivo válido com extensão .csv')
        return
      }
      setFile(f)
      setErrorMessage(null)
    }
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      const f = e.dataTransfer.files[0]
      if (!f.name.toLowerCase().endsWith('.csv')) {
        setErrorMessage('Por favor, selecione um arquivo com extensão .csv')
        return
      }
      setFile(f)
      setErrorMessage(null)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const targetAccId = selectedAccountId || accounts[0]?.id
    if (!targetAccId) {
      setErrorMessage('Selecione uma conta de destino para as transações.')
      return
    }
    if (!file) {
      setErrorMessage('Selecione um arquivo CSV para importar.')
      return
    }

    try {
      setIsUploading(true)
      setErrorMessage(null)
      const res = await importCSV(file, targetAccId)
      setResult(res)
      onSuccess()
    } catch (err: any) {
      setErrorMessage(err.message || 'Falha ao importar arquivo CSV.')
    } finally {
      setIsUploading(false)
    }
  }

  const handleResetAndClose = () => {
    setFile(null)
    setResult(null)
    setErrorMessage(null)
    onClose()
  }

  return (
    <ModalDialog isOpen={isOpen} onClose={handleResetAndClose} labelledBy={titleId}>
      <div className="bui-card w-full max-h-[90dvh] flex flex-col overflow-hidden shadow-2xl border border-[var(--bui-border-strong)]">
        {/* Header */}
        <div className="px-6 py-4 border-b border-[var(--bui-border)] flex items-center justify-between shrink-0">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 bui-inner flex items-center justify-center text-[var(--bui-accent-blue)]">
              <Upload className="w-4 h-4" aria-hidden="true" />
            </div>
            <div>
              <h2 id={titleId} className="text-base font-bold text-[var(--bui-text-primary)]">
                Importar Extrato CSV
              </h2>
              <p className="text-xs text-[var(--bui-text-muted)]">
                Extratos bancários ou faturas com deduplicação SHA-256
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={handleResetAndClose}
            className="bui-pill-control p-1.5"
            aria-label="Fechar modal"
          >
            <X className="w-4 h-4" aria-hidden="true" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 overflow-y-auto">
          {result ? (
            <div className="space-y-4">
              <div className="bui-inner p-4 text-center">
                <CheckCircle2 className="w-10 h-10 text-[var(--bui-lime)] mx-auto mb-2" aria-hidden="true" />
                <h3 className="text-sm font-bold text-[var(--bui-text-primary)]">
                  Importação Concluída com Sucesso!
                </h3>
                <p className="text-xs text-[var(--bui-text-muted)] mt-1">
                  O arquivo foi processado e o saldo da conta foi atualizado.
                </p>
              </div>

              <div className="grid grid-cols-3 gap-3 text-center">
                <div className="bui-inner p-3">
                  <span className="text-xs text-[var(--bui-text-muted)] block">Lidas</span>
                  <span className="text-lg font-bold font-mono-num text-[var(--bui-text-primary)]">
                    {result.total_parsed}
                  </span>
                </div>
                <div className="p-3 rounded-xl bui-badge-lime">
                  <span className="text-xs block">Inseridas</span>
                  <span className="text-lg font-bold font-mono-num">{result.inserted}</span>
                </div>
                <div className="p-3 rounded-xl bui-badge-amber">
                  <span className="text-xs block">Duplicadas</span>
                  <span className="text-lg font-bold font-mono-num">{result.duplicates}</span>
                </div>
              </div>

              <button
                type="button"
                onClick={handleResetAndClose}
                className="w-full py-2.5 rounded-xl bg-[var(--bui-accent-blue)] text-white font-bold text-xs transition"
              >
                Concluir
              </button>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-4">
              {errorMessage && (
                <div role="alert" className="p-3 rounded-xl bui-badge-rose text-xs flex items-center gap-2">
                  <AlertTriangle className="w-4 h-4 shrink-0" aria-hidden="true" />
                  <span>{errorMessage}</span>
                </div>
              )}

              <div>
                <label
                  htmlFor={accountSelectId}
                  className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1"
                >
                  Conta de Destino *
                </label>
                <select
                  id={accountSelectId}
                  value={selectedAccountId || accounts[0]?.id || ''}
                  onChange={(e) => setSelectedAccountId(e.target.value)}
                  className="w-full py-2.5 px-3 rounded-xl bg-[var(--bui-bg-inner)] border border-[var(--bui-border)] text-xs font-medium text-[var(--bui-text-primary)]"
                >
                  {accounts.map((acc) => (
                    <option key={acc.id} value={acc.id}>
                      {acc.name} ({acc.type})
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label
                  htmlFor={fileInputId}
                  className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1"
                >
                  Arquivo CSV *
                </label>
                <button
                  type="button"
                  onDragOver={(e) => e.preventDefault()}
                  onDrop={handleDrop}
                  onClick={() => fileInputRef.current?.click()}
                  className="w-full border-2 border-dashed border-[var(--bui-border-strong)] hover:border-[var(--bui-accent-blue)] rounded-2xl p-6 text-center cursor-pointer transition bg-[var(--bui-bg-inner)]"
                >
                  <input
                    id={fileInputId}
                    ref={fileInputRef}
                    type="file"
                    accept=".csv"
                    onChange={handleFileChange}
                    className="sr-only"
                  />
                  {file ? (
                    <div className="flex items-center justify-center gap-2 text-[var(--bui-accent-blue)]">
                      <FileText className="w-6 h-6 shrink-0" aria-hidden="true" />
                      <div className="text-left min-w-0">
                        <p className="text-xs font-bold text-[var(--bui-text-primary)] truncate max-w-xs">
                          {file.name}
                        </p>
                        <p className="text-[11px] text-[var(--bui-text-muted)]">
                          {Math.round(file.size / 1024)} KB
                        </p>
                      </div>
                    </div>
                  ) : (
                    <div>
                      <Upload className="w-7 h-7 text-[var(--bui-text-muted)] mx-auto mb-2" aria-hidden="true" />
                      <p className="text-xs text-[var(--bui-text-primary)] font-semibold">
                        Arraste seu arquivo CSV aqui ou clique para selecionar
                      </p>
                      <p className="text-[11px] text-[var(--bui-text-muted)] mt-1">
                        Compatível com Itaú, Nubank, Bradesco, Inter, Santander e C6
                      </p>
                    </div>
                  )}
                </button>
              </div>

              <button
                type="submit"
                disabled={isUploading || !file}
                className="w-full py-2.5 rounded-xl bg-[var(--bui-accent-blue)] hover:bg-[var(--bui-accent-blue-hover)] text-white font-bold text-xs shadow-sm transition disabled:opacity-40 flex items-center justify-center gap-2"
              >
                {isUploading ? (
                  <span>Processando arquivo...</span>
                ) : (
                  <>
                    <span>Importar Extrato</span>
                    <ArrowRight className="w-4 h-4" aria-hidden="true" />
                  </>
                )}
              </button>
            </form>
          )}
        </div>
      </div>
    </ModalDialog>
  )
}
