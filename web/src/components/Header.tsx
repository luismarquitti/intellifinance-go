import React from 'react'
import { Upload, Sparkles, Building2, UserCircle2 } from 'lucide-react'
import { Session } from '../types'

interface HeaderProps {
  session: Session | null
  onOpenImportModal: () => void
}

export const Header: React.FC<HeaderProps> = ({ session, onOpenImportModal }) => {
  return (
    <header className="border-b border-[#2C362F] bg-[#161D18]/90 backdrop-blur sticky top-0 z-30">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        {/* Brand / Logo */}
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-[#A5D6A7] to-[#2E7D32] flex items-center justify-center shadow-lg shadow-green-950/40">
            <Sparkles className="w-5 h-5 text-[#0E1410]" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-xl font-bold tracking-tight text-[#E8EFEA]">
                Intelli<span className="text-[#A5D6A7]">Finance</span>
              </h1>
              <span className="text-[10px] font-semibold uppercase tracking-wider px-2 py-0.5 rounded-full bg-[#1F2721] text-[#A5D6A7] border border-[#2C362F]">
                MVP
              </span>
            </div>
            <p className="text-xs text-[#8E9E92]">Painel de Controle Financeiro</p>
          </div>
        </div>

        {/* User / Tenant Info & Action */}
        <div className="flex items-center gap-4">
          {session && (
            <div className="hidden md:flex items-center gap-3 pr-3 border-r border-[#2C362F] text-xs">
              <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-[#1F2721] text-[#A3B4A7] border border-[#2C362F]">
                <Building2 className="w-3.5 h-3.5 text-[#A5D6A7]" />
                <span className="font-medium text-[#E8EFEA]">{session.tenant_name}</span>
              </div>
              <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-[#1F2721] text-[#A3B4A7] border border-[#2C362F]">
                <UserCircle2 className="w-3.5 h-3.5 text-[#A5D6A7]" />
                <span>{session.user_name}</span>
                <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse ml-0.5" title="Online" />
              </div>
            </div>
          )}

          <button
            onClick={onOpenImportModal}
            className="flex items-center gap-2 px-4 py-2 rounded-xl bg-[#A5D6A7] hover:bg-[#81C784] text-[#0E1410] font-semibold text-sm shadow-md shadow-green-950/30 transition-all duration-200 hover:scale-[1.02] active:scale-[0.98]"
          >
            <Upload className="w-4 h-4" />
            <span>Importar CSV</span>
          </button>
        </div>
      </div>
    </header>
  )
}
