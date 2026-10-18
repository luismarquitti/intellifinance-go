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
    if (!selectedAccountId) {
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
      const res = await importCSV(file, selectedAccountId)
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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-[#161D18] border border-[#2C362F] rounded-3xl max-w-lg w-full overflow-hidden shadow-2xl animate-in zoom-in-95 duration-200">
        {/* Header */}
        <div className="p-6 border-b border-[#2C362F] flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-[#A5D6A7]/10 border border-[#A5D6A7]/20 flex items-center justify-center text-[#A5D6A7]">
              <Upload className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-base font-bold text-[#E8EFEA]">Importar Transações CSV</h2>
              <p className="text-xs text-[#8E9E92]">Extratos bancários ou faturas de cartão</p>
            </div>
          </div>
          <button
            onClick={handleResetAndClose}
            className="p-1 rounded-xl text-[#8E9E92] hover:text-[#E8EFEA] hover:bg-[#1F2721] transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6">
          {result ? (
            /* Result State */
            <div className="space-y-4">
              <div className="p-4 rounded-2xl bg-[#1F2721] border border-[#2C362F] text-center">
                <CheckCircle2 className="w-10 h-10 text-emerald-400 mx-auto mb-2" />
                <h3 className="text-sm font-bold text-[#E8EFEA]">Importação Concluída com Sucesso!</h3>
                <p className="text-xs text-[#8E9E92] mt-1">
                  O arquivo foi processado e o saldo da conta foi atualizado.
                </p>
              </div>

              <div className="grid grid-cols-3 gap-3 text-center">
                <div className="p-3 rounded-xl bg-[#0E1410] border border-[#2C362F]">
                  <span className="text-xs text-[#8E9E92] block">Lidas</span>
                  <span className="text-lg font-extrabold text-[#E8EFEA]">{result.total_parsed}</span>
                </div>
                <div className="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/20">
                  <span className="text-xs text-emerald-400 block">Inseridas</span>
                  <span className="text-lg font-extrabold text-emerald-400">{result.inserted}</span>
                </div>
                <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/20">
                  <span className="text-xs text-amber-300 block" title="Ignoradas por idempotência">Duplicadas</span>
                  <span className="text-lg font-extrabold text-amber-300">{result.duplicates}</span>
                </div>
              </div>

              {result.errors && result.errors.length > 0 && (
                <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/20 text-xs text-amber-300 max-h-32 overflow-y-auto">
                  <span className="font-bold flex items-center gap-1 mb-1">
                    <AlertTriangle className="w-3.5 h-3.5" /> Avisos de processamento:
                  </span>
                  <ul className="list-disc pl-4 space-y-0.5">
                    {result.errors.map((err, i) => (
                      <li key={i}>{err}</li>
                    ))}
                  </ul>
                </div>
              )}

              <button
                onClick={handleResetAndClose}
                className="w-full py-2.5 rounded-xl bg-[#A5D6A7] hover:bg-[#81C784] text-[#0E1410] font-bold text-sm transition-colors mt-2"
              >
                Concluir
              </button>
            </div>
          ) : (
            /* Upload Form */
            <form onSubmit={handleSubmit} className="space-y-4">
              {errorMessage && (
                <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/20 text-xs text-rose-400 flex items-center gap-2">
                  <AlertTriangle className="w-4 h-4 shrink-0" />
                  <span>{errorMessage}</span>
                </div>
              )}

              {/* Account selection */}
              <div>
                <label className="block text-xs font-semibold uppercase tracking-wider text-[#8E9E92] mb-1.5">
                  Conta de Destino *
                </label>
                <select
                  value={selectedAccountId}
                  onChange={(e) => setSelectedAccountId(e.target.value)}
                  className="w-full py-2.5 px-3 rounded-xl bg-[#1F2721] border border-[#2C362F] text-xs font-medium text-[#E8EFEA] focus:outline-none focus:border-[#A5D6A7]"
                >
                  {accounts.map((acc) => (
                    <option key={acc.id} value={acc.id}>
                      {acc.name} ({acc.type})
                    </option>
                  ))}
                </select>
              </div>

              {/* File Drop Area */}
              <div>
                <label className="block text-xs font-semibold uppercase tracking-wider text-[#8E9E92] mb-1.5">
                  Arquivo CSV *
                </label>
                <div
                  onDragOver={(e) => e.preventDefault()}
                  onDrop={handleDrop}
                  onClick={() => fileInputRef.current?.click()}
                  className="border-2 border-dashed border-[#2C362F] hover:border-[#A5D6A7] rounded-2xl p-6 text-center cursor-pointer transition-colors bg-[#0E1410]/50"
                >
                  <input
                    ref={fileInputRef}
                    type="file"
                    accept=".csv"
                    onChange={handleFileChange}
                    className="hidden"
                  />
                  {file ? (
                    <div className="flex items-center justify-center gap-2 text-[#A5D6A7]">
                      <FileText className="w-6 h-6" />
                      <div className="text-left">
                        <p className="text-xs font-bold text-[#E8EFEA] truncate max-w-xs">{file.name}</p>
                        <p className="text-[10px] text-[#8E9E92]">{(file.size / 1024).toFixed(1)} KB</p>
                      </div>
                    </div>
                  ) : (
                    <div>
                      <Upload className="w-8 h-8 text-[#8E9E92] mx-auto mb-2" />
                      <p className="text-xs text-[#E8EFEA] font-medium">
                        Arraste seu arquivo CSV aqui ou clique para selecionar
                      </p>
                      <p className="text-[10px] text-[#8E9E92] mt-1">
                        Formatos suportados: Itaú, NuBank, Bradesco, Inter, Santander, C6
                      </p>
                    </div>
                  )}
                </div>
              </div>

              {/* Submit Button */}
              <div className="pt-2">
                <button
                  type="submit"
                  disabled={isUploading || !file}
                  className="w-full py-3 rounded-xl bg-[#A5D6A7] hover:bg-[#81C784] text-[#0E1410] font-bold text-sm shadow-md transition-all disabled:opacity-40 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                >
                  {isUploading ? (
                    <>
                      <div className="w-4 h-4 border-2 border-[#0E1410] border-t-transparent rounded-full animate-spin" />
                      <span>Processando arquivo...</span>
                    </>
                  ) : (
                    <>
                      <span>Importar Extrato</span>
                      <ArrowRight className="w-4 h-4" />
                    </>
                  )}
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  )
}
