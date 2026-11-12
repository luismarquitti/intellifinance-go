import React, { useState, useRef } from 'react'
import { Upload, X, FileText, CheckCircle2, AlertTriangle, ArrowRight } from 'lucide-react'
import { Account, CSVIngestResult } from '../types'
import { importCSV } from '../api'

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

  if (!isOpen) return null

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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/65 backdrop-blur-sm">
      <div className="bui-card max-w-lg w-full overflow-hidden shadow-2xl border border-[var(--bui-border-strong)]">
        {/* Header */}
        <div className="px-6 py-4 border-b border-[var(--bui-border)] flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 bui-inner flex items-center justify-center text-[var(--bui-accent-blue)]">
              <Upload className="w-4 h-4" />
            </div>
            <div>
              <h2 className="text-base font-bold text-[var(--bui-text-primary)]">
                Importar Extrato CSV
              </h2>
              <p className="text-xs text-[var(--bui-text-muted)]">
                Extratos bancários ou faturas com deduplicação SHA-256
              </p>
            </div>
          </div>
          <button onClick={handleResetAndClose} className="bui-pill-control p-1.5">
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6">
          {result ? (
            <div className="space-y-4">
              <div className="bui-inner p-4 text-center">
                <CheckCircle2 className="w-10 h-10 text-[var(--bui-lime)] mx-auto mb-2" />
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
                onClick={handleResetAndClose}
                className="w-full py-2.5 rounded-xl bg-[var(--bui-accent-blue)] text-white font-bold text-xs transition"
              >
                Concluir
              </button>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-4">
              {errorMessage && (
                <div className="p-3 rounded-xl bui-badge-rose text-xs flex items-center gap-2">
                  <AlertTriangle className="w-4 h-4 shrink-0" />
                  <span>{errorMessage}</span>
                </div>
              )}

              <div>
                <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
                  Conta de Destino *
                </label>
                <select
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
                <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--bui-text-secondary)] mb-1">
                  Arquivo CSV *
                </label>
                <div
                  onDragOver={(e) => e.preventDefault()}
                  onDrop={handleDrop}
                  onClick={() => fileInputRef.current?.click()}
                  className="border-2 border-dashed border-[var(--bui-border-strong)] hover:border-[var(--bui-accent-blue)] rounded-2xl p-6 text-center cursor-pointer transition bg-[var(--bui-bg-inner)]"
                >
                  <input
                    ref={fileInputRef}
                    type="file"
                    accept=".csv"
                    onChange={handleFileChange}
                    className="hidden"
                  />
                  {file ? (
                    <div className="flex items-center justify-center gap-2 text-[var(--bui-accent-blue)]">
                      <FileText className="w-6 h-6" />
                      <div className="text-left">
                        <p className="text-xs font-bold text-[var(--bui-text-primary)] truncate max-w-xs">
                          {file.name}
                        </p>
                        <p className="text-[10px] text-[var(--bui-text-muted)]">
                          {Math.round(file.size / 1024)} KB
                        </p>
                      </div>
                    </div>
                  ) : (
                    <div>
                      <Upload className="w-7 h-7 text-[var(--bui-text-muted)] mx-auto mb-2" />
                      <p className="text-xs text-[var(--bui-text-primary)] font-semibold">
                        Arraste seu arquivo CSV aqui ou clique para selecionar
                      </p>
                      <p className="text-[11px] text-[var(--bui-text-muted)] mt-1">
                        Compatível com Itaú, Nubank, Bradesco, Inter, Santander e C6
                      </p>
                    </div>
                  )}
                </div>
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
                    <ArrowRight className="w-4 h-4" />
                  </>
                )}
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  )
}
