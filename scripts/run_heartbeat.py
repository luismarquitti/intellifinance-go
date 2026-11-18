#!/usr/bin/env python3
import json
import subprocess
import os
import sys

REPO = "luismarquitti/intellifinance-go"
DATA_DIR = os.path.expanduser("~/.openclaw/data/sdlc")
STATE_FILE = os.path.join(DATA_DIR, "state.json")

def run_cmd(cmd):
    res = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, shell=True)
    return res.stdout.strip()

# 1. Carregar estado
state = {"notified_prs": {}, "triaged_issues": {}}
if os.path.exists(STATE_FILE):
    try:
        with open(STATE_FILE, "r") as f:
            state = json.load(f)
    except Exception:
        pass

# 2. Rodar polling
poll_raw = run_cmd("python3 ~/.openclaw/workspace/skills/intellifinance-sdlc/scripts/poll_repo.py")
try:
    poll_data = json.loads(poll_raw)
except Exception as e:
    print(f"Erro ao analisar polling: {e}")
    sys.exit(0)

notifications = []

# 3. Triagem de Novas Issues
issues = poll_data.get("issues_needing_triage", [])
for iss in issues:
    inum = str(iss["number"])
    title = iss["title"]
    
    if inum in state.get("triaged_issues", {}):
        continue
    
    # Classificação de Dificuldade e Roteamento de Agente
    title_lower = title.lower()
    if any(k in title_lower for k in ["ddl", "rls", "persistência", "persistencia", "domínio", "dominio", "ports", "secret", "deploy", "serverless", "co-orçamento", "worker"]):
        diff = "difficulty:high"
        agent = "agent:antigravity"
    elif any(k in title_lower for k in ["fixturegen", "parser", "ofx", "csv", "excel", "swagger", "openapi", "docs", "showcase"]):
        diff = "difficulty:medium" if "parser" in title_lower else "difficulty:low"
        agent = "agent:jules"
    else:
        diff = "difficulty:medium"
        agent = "agent:antigravity"
    
    # Aplicar labels na issue
    labels_to_add = f"status:triaged,{diff},{agent}"
    run_cmd(f"gh issue edit {inum} --add-label '{labels_to_add}' -R {REPO}")
    
    # Se for para o Jules, adicionar comentário de acionamento
    if agent == "agent:jules":
        comment = "@jules please implement this issue following the specification in the description."
        run_cmd(f"gh issue comment {inum} --body '{comment}' -R {REPO}")
    
    state.setdefault("triaged_issues", {})[inum] = {
        "title": title,
        "difficulty": diff,
        "agent": agent
    }

# 4. Processar PRs
prs = poll_data.get("prs_needing_action", [])
for pr in prs:
    pnum = str(pr["number"])
    title = pr["title"]
    branch = pr["branch"]
    risk = pr["risk"]
    can_merge = pr["can_auto_merge"]
    reason = pr["reason"]
    ci = pr["ci_status"]
    
    # Se for de alto risco e requer aprovação humana
    if not can_merge:
        if pnum not in state.get("notified_prs", {}):
            run_cmd(f"gh pr edit {pnum} --add-label 'pr:requires-human-approval' -R {REPO}")
            msg = (
                f"⚠️ [IntelliFinance Gate] PR #{pnum} requer sua aprovação manual!\n"
                f"• Título: {title}\n"
                f"• Branch: {branch}\n"
                f"• Classificação: {risk} RISK\n"
                f"• Motivo: {reason}\n"
                f"• CI Status: {ci}\n"
                f"👉 Revise e aprove: https://github.com/{REPO}/pull/{pnum}"
            )
            notifications.append(msg)
            state.setdefault("notified_prs", {})[pnum] = {
                "title": title,
                "risk": risk
            }
    else:
        # Se puder auto-mergear e CI estiver verde
        if ci == "passing":
            merge_res = run_cmd(f"gh pr merge {pnum} --merge --auto --delete-branch -R {REPO}")
            msg = f"✅ [IntelliFinance] PR #{pnum} auto-mergeado em dev com sucesso!\n• Título: {title}"
            notifications.append(msg)

# 5. Salvar estado atualizado
with open(STATE_FILE, "w") as f:
    json.dump(state, f, indent=2)

# 6. Emitir notificações se houver
if notifications:
    print("\n\n".join(notifications))
else:
    sys.exit(0)
