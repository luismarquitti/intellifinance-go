package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var _ ports.CategoryRuleService = (*CategoryMatcher)(nil)

var (
	confidenceHistoryMatch  = decimal.RequireFromString("0.98")
	confidenceHintSalary    = decimal.RequireFromString("0.92")
	confidenceHintStandard  = decimal.RequireFromString("0.90")
	confidenceKeywordStrong = decimal.RequireFromString("0.88")
	confidenceKeywordBase   = decimal.RequireFromString("0.85")
)

// CategoryMatcher implements ports.CategoryRuleService using canonical Open Finance hints,
// historical verified patterns, and descriptive keywords.
type CategoryMatcher struct {
	txRepo           ports.TransactionRepository
	catRepo          ports.CategoryRepository
	pendingActionSvc ports.PendingActionService
}

// NewCategoryMatcher constructs a new CategoryMatcher service instance.
func NewCategoryMatcher(
	txRepo ports.TransactionRepository,
	catRepo ports.CategoryRepository,
	pendingActionSvc ports.PendingActionService,
) *CategoryMatcher {
	return &CategoryMatcher{
		txRepo:           txRepo,
		catRepo:          catRepo,
		pendingActionSvc: pendingActionSvc,
	}
}

type matchRule struct {
	categoryName string
	confidence   decimal.Decimal
	reason       string
}

// ScanAndPropose evaluates all uncategorized transactions and proposes classifications via HITL.
func (m *CategoryMatcher) ScanAndPropose(ctx context.Context, tenantID uuid.UUID) (*ports.CategorizationScanResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}
	if m == nil || m.pendingActionSvc == nil {
		return nil, errors.New("pending action service is required")
	}
	if m.catRepo == nil || m.txRepo == nil {
		return nil, errors.New("transaction and category repositories are required")
	}

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

	// 2. Load existing pending actions index to prevent duplicate proposals
	pendingIdx, err := m.pendingActionSvc.LoadPendingIndex(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to load pending actions index: %w", err)
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
		if pendingIdx.HasCategorizationFor(tx.ID) {
			continue
		}

		matchedCat, conf, reason := m.findMatch(tx, catByName, verifiedHistory)
		if matchedCat == nil {
			continue
		}

		proposal, propErr := domain.NewCategorizeProposalFromEntities(tx, matchedCat, conf, reason)
		if propErr != nil {
			return nil, fmt.Errorf("failed building categorization proposal: %w", propErr)
		}

		action, sErr := m.pendingActionSvc.ProposeCategorization(ctx, tenantID, "ai_categorization_agent", proposal)
		if sErr != nil {
			if errors.Is(sErr, domain.ErrDuplicatePendingAction) {
				continue
			}
			return nil, fmt.Errorf("failed to save categorization proposal: %w", sErr)
		}

		pendingIdx.Record(action)
		res.ProposedActionsCount++
	}

	return res, nil
}

func (m *CategoryMatcher) findMatch(
	tx *domain.Transaction,
	catByName map[string]*domain.Category,
	verifiedHistory map[string]uuid.UUID,
) (*domain.Category, decimal.Decimal, string) {
	normDesc := normalizeKey(tx.CleanDescription)

	// A. Check historical manual confirmation
	if pastCatID, ok := verifiedHistory[normDesc]; ok {
		for _, c := range catByName {
			if c.ID == pastCatID {
				return c, confidenceHistoryMatch, fmt.Sprintf("Baseado em histórico previamente confirmado para '%s'", tx.CleanDescription)
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

	return nil, decimal.Zero, ""
}

func matchHint(hint string) *matchRule {
	h := strings.ToLower(strings.TrimSpace(hint))
	switch {
	case strings.Contains(h, "salary") || strings.Contains(h, "payroll"):
		return &matchRule{categoryName: "salario", confidence: confidenceHintSalary}
	case strings.Contains(h, "housing") || strings.Contains(h, "electricity") || strings.Contains(h, "telecom") || strings.Contains(h, "water"):
		return &matchRule{categoryName: "moradia", confidence: confidenceHintStandard}
	case strings.Contains(h, "fitness") || strings.Contains(h, "gym") || strings.Contains(h, "health") || strings.Contains(h, "pharmacy"):
		return &matchRule{categoryName: "saude", confidence: confidenceHintStandard}
	case strings.Contains(h, "streaming") || strings.Contains(h, "entertainment") || strings.Contains(h, "recreation"):
		return &matchRule{categoryName: "lazer", confidence: confidenceHintStandard}
	case strings.Contains(h, "supermarket") || strings.Contains(h, "restaurant") || strings.Contains(h, "food") || strings.Contains(h, "grocer"):
		return &matchRule{categoryName: "alimentacao", confidence: confidenceHintStandard}
	case strings.Contains(h, "transport") || strings.Contains(h, "fuel") || strings.Contains(h, "gas"):
		return &matchRule{categoryName: "transporte", confidence: confidenceHintStandard}
	case strings.Contains(h, "education") || strings.Contains(h, "course") || strings.Contains(h, "school"):
		return &matchRule{categoryName: "educacao", confidence: confidenceHintStandard}
	case strings.Contains(h, "investment") || strings.Contains(h, "dividend") || strings.Contains(h, "interest"):
		return &matchRule{categoryName: "investimentos", confidence: confidenceHintStandard}
	default:
		return nil
	}
}

func matchKeyword(desc string) *matchRule {
	switch {
	case containsAny(desc, "spotify", "netflix", "disney", "prime video", "amazon prime", "hbo", "max", "youtube", "steam", "playstation", "nintendo"):
		return &matchRule{categoryName: "lazer", confidence: confidenceKeywordStrong, reason: "Assinatura de streaming ou entretenimento digital"}
	case containsAny(desc, "smart fit", "academia", "gym", "farmacia", "drogasil", "drogaria", "raia", "pague menos", "hospital", "laboratorio", "clinica", "dentista"):
		return &matchRule{categoryName: "saude", confidence: confidenceKeywordStrong, reason: "Despesa de saúde, farmácia ou academia"}
	case containsAny(desc, "condominio", "aluguel", "eletrobras", "light", "enel", "sabesp", "copasa", "sanepar", "comgas", "vivo", "claro", "tim", "oi fibra"):
		return &matchRule{categoryName: "moradia", confidence: confidenceKeywordStrong, reason: "Despesa de moradia, contas básicas ou telecom"}
	case containsAny(desc, "salario", "folha", "prolabore", "remuneracao", "honorarios"):
		return &matchRule{categoryName: "salario", confidence: confidenceKeywordStrong, reason: "Receita de remuneração salarial ou pró-labore"}
	case containsAny(desc, "ifood", "rappi", "mercado", "supermercado", "carrefour", "pao de acucar", "assai", "atacadao", "restaurante", "padaria", "pizzaria", "lanchonete", "burger", "mcdonald", "outback"):
		return &matchRule{categoryName: "alimentacao", confidence: confidenceKeywordBase, reason: "Alimentação, mercado ou delivery de refeições"}
	case containsAny(desc, "uber", "99app", "99 tecnologia", "posto", "ipiranga", "shell", "petrobras", "sem parar", "veloe", "estapar", "combustivel", "pedagio"):
		return &matchRule{categoryName: "transporte", confidence: confidenceKeywordBase, reason: "Transporte urbano, combustível ou mobilidade"}
	case containsAny(desc, "alura", "udemy", "coursera", "faculdade", "escola", "curso", "livraria", "kindle"):
		return &matchRule{categoryName: "educacao", confidence: confidenceKeywordBase, reason: "Cursos, capacitação ou educação continuada"}
	case containsAny(desc, "xp investimentos", "btg", "rico", "clear", "nu invest", "inter dtvm", "tesouro direto", "dividendo", "rendimento"):
		return &matchRule{categoryName: "investimentos", confidence: confidenceKeywordBase, reason: "Rendimentos financeiros ou investimentos"}
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
