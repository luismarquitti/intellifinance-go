package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// CategoryMatcher implements ports.CategoryRuleService using canonical Open Finance hints,
// historical verified patterns, and descriptive keywords.
type CategoryMatcher struct {
	txRepo     ports.TransactionRepository
	catRepo    ports.CategoryRepository
	actionRepo ports.PendingActionRepository
}

// NewCategoryMatcher constructs a new CategoryMatcher service instance.
func NewCategoryMatcher(
	txRepo ports.TransactionRepository,
	catRepo ports.CategoryRepository,
	actionRepo ports.PendingActionRepository,
) *CategoryMatcher {
	return &CategoryMatcher{
		txRepo:     txRepo,
		catRepo:    catRepo,
		actionRepo: actionRepo,
	}
}

type matchRule struct {
	categoryName string
	confidence   float64
	reason       string
}

// ScanAndPropose evaluates all uncategorized transactions and proposes classifications via HITL.
func (m *CategoryMatcher) ScanAndPropose(ctx context.Context, tenantID uuid.UUID) (*ports.CategorizationScanResult, error) {
	// 1. Load active categories for the tenant
	categories, err := m.catRepo.List(ctx, tenantID, ports.CategoryFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}

	catByName := make(map[string]*domain.Category)
	for _, c := range categories {
		normName := normalizeKey(c.Name)
		catByName[normName] = c
	}

	// 2. Load existing pending actions to prevent duplicate proposals
	pendingActions, err := m.actionRepo.ListPending(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending actions: %w", err)
	}

	proposedTxIDs := make(map[uuid.UUID]bool)
	for _, act := range pendingActions {
		if act.ActionType == domain.ActionTypeCategorizeTransaction {
			if txIDStr, ok := act.Payload["transaction_id"].(string); ok {
				if id, pErr := uuid.Parse(txIDStr); pErr == nil {
					proposedTxIDs[id] = true
				}
			}
		}
	}

	// 3. Load recent transactions (up to 1000)
	txs, _, err := m.txRepo.List(ctx, tenantID, ports.TransactionFilter{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	// 4. Build historical verified patterns index
	verifiedHistory := make(map[string]uuid.UUID)
	for _, tx := range txs {
		if tx.CategoryID != nil && tx.IsManuallyVerified && tx.CleanDescription != "" {
			normDesc := normalizeKey(tx.CleanDescription)
			verifiedHistory[normDesc] = *tx.CategoryID
		}
	}

	res := &ports.CategorizationScanResult{
		ScannedTransactionsCount: len(txs),
	}

	// 5. Evaluate uncategorized non-transfer transactions
	for _, tx := range txs {
		if tx.Status == domain.TransactionStatusVoid || tx.Type == domain.TransactionTypeTransfer {
			continue
		}
		if tx.CategoryID != nil {
			res.AlreadyCategorizedCount++
			continue
		}
		if proposedTxIDs[tx.ID] {
			continue
		}

		matchedCat, conf, reason := m.findMatch(tx, catByName, verifiedHistory)
		if matchedCat == nil {
			continue
		}

		// Create proposal in pending_actions
		payload := map[string]any{
			"transaction_id":    tx.ID.String(),
			"description":       tx.Description,
			"clean_description": tx.CleanDescription,
			"amount":            tx.Amount.StringFixed(2),
			"date":              tx.Date.Format("2006-01-02"),
			"category_id":       matchedCat.ID.String(),
			"category_name":     matchedCat.Name,
			"confidence":        conf,
			"reason":            reason,
		}

		confDec := decimal.NewFromFloat(conf)
		action := &domain.PendingAction{
			ID:             uuid.New(),
			TenantID:       tenantID,
			CreatedByAgent: "ai_categorization_agent",
			ActionType:     domain.ActionTypeCategorizeTransaction,
			Payload:        payload,
			Status:         domain.ActionStatusPending,
			Metadata: map[string]any{
				"deduplication_key": fmt.Sprintf("categorize_%s", tx.ID),
				"confidence":        confDec.StringFixed(2),
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		if sErr := m.actionRepo.Save(ctx, action); sErr != nil {
			return nil, fmt.Errorf("failed to save categorization proposal: %w", sErr)
		}

		proposedTxIDs[tx.ID] = true
		res.ProposedActionsCount++
	}

	return res, nil
}

func (m *CategoryMatcher) findMatch(
	tx *domain.Transaction,
	catByName map[string]*domain.Category,
	verifiedHistory map[string]uuid.UUID,
) (*domain.Category, float64, string) {
	normDesc := normalizeKey(tx.CleanDescription)

	// A. Check historical manual confirmation
	if pastCatID, ok := verifiedHistory[normDesc]; ok {
		for _, c := range catByName {
			if c.ID == pastCatID {
				return c, 0.98, fmt.Sprintf("Baseado em histórico previamente confirmado para '%s'", tx.CleanDescription)
			}
		}
	}

	// B. Check Open Finance provider category_hint metadata
	if tx.Metadata != nil {
		if hint, ok := tx.Metadata["category_hint"].(string); ok && hint != "" {
			if rule := matchHint(hint); rule != nil {
				targetCat := resolveCategory(rule.categoryName, catByName)
				if targetCat != nil {
					return targetCat, rule.confidence, fmt.Sprintf("Classificação Open Finance Pluggy ('%s')", hint)
				}
			}
		}
	}

	// C. Check Keyword heuristics on clean description
	if rule := matchKeyword(normDesc); rule != nil {
		targetCat := resolveCategory(rule.categoryName, catByName)
		if targetCat != nil {
			return targetCat, rule.confidence, rule.reason
		}
	}

	return nil, 0, ""
}

func matchHint(hint string) *matchRule {
	h := strings.ToLower(strings.TrimSpace(hint))
	switch {
	case strings.Contains(h, "salary") || strings.Contains(h, "payroll"):
		return &matchRule{categoryName: "salario", confidence: 0.92}
	case strings.Contains(h, "housing") || strings.Contains(h, "electricity") || strings.Contains(h, "telecom") || strings.Contains(h, "water"):
		return &matchRule{categoryName: "moradia", confidence: 0.90}
	case strings.Contains(h, "fitness") || strings.Contains(h, "gym") || strings.Contains(h, "health") || strings.Contains(h, "pharmacy"):
		return &matchRule{categoryName: "saude", confidence: 0.90}
	case strings.Contains(h, "streaming") || strings.Contains(h, "entertainment") || strings.Contains(h, "recreation"):
		return &matchRule{categoryName: "lazer", confidence: 0.90}
	case strings.Contains(h, "supermarket") || strings.Contains(h, "restaurant") || strings.Contains(h, "food") || strings.Contains(h, "grocer"):
		return &matchRule{categoryName: "alimentacao", confidence: 0.90}
	case strings.Contains(h, "transport") || strings.Contains(h, "fuel") || strings.Contains(h, "gas"):
		return &matchRule{categoryName: "transporte", confidence: 0.90}
	case strings.Contains(h, "education") || strings.Contains(h, "course") || strings.Contains(h, "school"):
		return &matchRule{categoryName: "educacao", confidence: 0.90}
	case strings.Contains(h, "investment") || strings.Contains(h, "dividend") || strings.Contains(h, "interest"):
		return &matchRule{categoryName: "investimentos", confidence: 0.90}
	default:
		return nil
	}
}

func matchKeyword(desc string) *matchRule {
	switch {
	case containsAny(desc, "spotify", "netflix", "disney", "prime video", "amazon prime", "hbo", "max", "youtube", "steam", "playstation", "nintendo"):
		return &matchRule{categoryName: "lazer", confidence: 0.88, reason: "Assinatura de streaming ou entretenimento digital"}
	case containsAny(desc, "smart fit", "academia", "gym", "farmacia", "drogasil", "drogaria", "raia", "pague menos", "hospital", "laboratorio", "clinica", "dentista"):
		return &matchRule{categoryName: "saude", confidence: 0.88, reason: "Despesa de saúde, farmácia ou academia"}
	case containsAny(desc, "condominio", "aluguel", "eletrobras", "light", "enel", "sabesp", "copasa", "sanepar", "comgas", "vivo", "claro", "tim", "oi fibra"):
		return &matchRule{categoryName: "moradia", confidence: 0.88, reason: "Despesa de moradia, contas básicas ou telecom"}
	case containsAny(desc, "salario", "folha", "prolabore", "remuneracao", "honorarios"):
		return &matchRule{categoryName: "salario", confidence: 0.88, reason: "Receita de remuneração salarial ou pró-labore"}
	case containsAny(desc, "ifood", "rappi", "mercado", "supermercado", "carrefour", "pao de acucar", "assai", "atacadao", "restaurante", "padaria", "pizzaria", "lanchonete", "burger", "mcdonald", "outback"):
		return &matchRule{categoryName: "alimentacao", confidence: 0.85, reason: "Alimentação, mercado ou delivery de refeições"}
	case containsAny(desc, "uber", "99app", "99 tecnologia", "posto", "ipiranga", "shell", "petrobras", "sem parar", "veloe", "estapar", "combustivel", "pedagio"):
		return &matchRule{categoryName: "transporte", confidence: 0.85, reason: "Transporte urbano, combustível ou mobilidade"}
	case containsAny(desc, "alura", "udemy", "coursera", "faculdade", "escola", "curso", "livraria", "kindle"):
		return &matchRule{categoryName: "educacao", confidence: 0.85, reason: "Cursos, capacitação ou educação continuada"}
	case containsAny(desc, "xp investimentos", "btg", "rico", "clear", "nu invest", "inter dtvm", "tesouro direto", "dividendo", "rendimento"):
		return &matchRule{categoryName: "investimentos", confidence: 0.85, reason: "Rendimentos financeiros ou investimentos"}
	default:
		return nil
	}
}

func containsAny(s string, keywords ...string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func resolveCategory(key string, catByName map[string]*domain.Category) *domain.Category {
	key = strings.ToLower(key)
	for name, cat := range catByName {
		if strings.Contains(name, key) {
			return cat
		}
	}
	return nil
}

func normalizeKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(
		"ã", "a", "á", "a", "à", "a", "â", "a",
		"é", "e", "ê", "e",
		"í", "i",
		"ó", "o", "õ", "o", "ô", "o",
		"ú", "u",
		"ç", "c",
	)
	return replacer.Replace(s)
}
