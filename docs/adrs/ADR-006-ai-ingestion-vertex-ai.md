# ADR-006: AI-Driven Financial Ingestion & Vertex AI Integration with Local LLM Fallback

- **Status**: Accepted
- **Deciders**: Software Architect (Worker M3), Principal Engineer, AI/ML Specialist
- **Date**: 2026-09-24
- **Technical Story**: Milestone 3 - Architecture Decision Records

---

## 1. Context & Problem Statement

Financial transaction descriptions from Brazilian banking institutions are notoriously truncated, cryptic, and unstructured. Examples include:
- `PAG*RestauranteSilva 12/03` (Dining Out)
- `DEB PIX TRANSF 0023491823` (Transfer or Service Payment)
- `IOF COMPRA EXTERIOR` (Bank Tax)
- `PGTO ELETRON COBR NUBANK` (Credit Card Bill Payment)

Traditional categorization approaches rely on rigid regex dictionaries or static substring mappings. These approaches fail constantly as new merchants appear, require manual rule authoring, and cannot infer context (such as distinguishing between a pharmacy purchase for medicine vs personal cosmetics).

Furthermore, document ingestion presents a severe technical hurdle:
- Digital and scanned PDF bank statements, credit card bills, and employer salary paystubs (*holerites*) contain complex multi-column grids, embedded logos, and watermark backgrounds.
- Traditional text extractors (such as `pdf-parse` in the legacy Node.js prototype or Tesseract OCR) produce unordered text streams, corrupting table alignments and dropping negative transaction signs.

To solve this, IntelliFinance requires an AI ingestion engine capable of multimodal document parsing and contextual transaction categorization. However, two infrastructure constraints must be reconciled:
1. **GCP Production**: Must leverage Google Cloud GenAI App Builder / startup credits using the high-speed, cost-effective **Gemini 2.0 Flash** model.
2. **Homelab Dev / Offline**: Developers and household members operating locally must be able to run ingestion and categorization at **$0 incremental cost** without requiring GCP credentials or an internet connection, utilizing local models on `lm-claw` (LiteLLM proxy and Ollama).

---

## 2. Decision Drivers

- **Native Multimodal Understanding**: Ability to process raw PDF byte streams directly without external pre-rasterization pipelines, extracting structured tables and amounts visually.
- **Cost Maximization via GCP Credits**: Consume Google Cloud GenAI credits via Vertex AI in production to maintain zero out-of-pocket operational expense.
- **Local Zero-Cost Fallback**: Seamless fallback to local LLMs (Ollama `qwen2.5-coder` or `llama3.2-vision` via LiteLLM on `lm-claw`) for offline execution.
- **Strict JSON Schema Enforcement**: LLM output must conform 100% to Go domain structs with zero markdown wrapping or conversational hallucinations.
- **Confidence Scoring & Human-in-the-Loop Feedback**: Every categorization prediction must return a calibrated confidence score (0.0 to 1.0) and human review flags.

---

## 3. Considered Options

1. **Rule-Based Regex & Substring Heuristics (Legacy Baseline)**: High maintenance, brittle, cannot handle scanned PDFs.
2. **OpenAI GPT-4o / Azure OpenAI**: High recurring USD billing; does not utilize Google Cloud GenAI App Builder credits.
3. **Official Google Gen AI Go SDK (`google.golang.org/genai`) on Vertex AI with Local LiteLLM/Ollama Fallback (Selected)**.

---

## 4. Evaluation & Comparative Matrix

| Evaluation Criteria | Vertex AI Gemini 2.0 + LiteLLM (Selected) | OpenAI GPT-4o | Local Ollama Only | Traditional Regex + Tesseract |
|---|---|---|---|---|
| **Production Cost** | **$0.00** (Funded by GenAI App Builder credits) | Expensive (~$5-$20/mo out of pocket) | $0.00 | $0.00 |
| **Multimodal PDF/OCR** | **State of the Art** (Visual table grid extraction) | Excellent | Moderate (Requires high GPU VRAM) | Poor (Fails on multi-column grids) |
| **Categorization Latency** | **Fast** (~400-800ms for batch of 25 tx) | Fast (~800-1200ms) | Slow (CPU-bound on low-power hardware) | Instant (< 5ms) |
| **Structured JSON Output** | **Guaranteed** (`ResponseMIMEType: application/json`) | Guaranteed (`response_format: json_object`) | Moderate (Prompt dependent) | N/A |
| **Offline Independence** | **100% via LiteLLM/Ollama on `lm-claw`** | 0% (Requires Internet & API key) | 100% | 100% |
| **SDK Stability in Go** | **Official Unified SDK** (`google.golang.org/genai`) | Third-party community SDKs | OpenAI-compatible HTTP REST | Standard library |

---

## 5. Decision Outcome

**Adopt the unified Google Gen AI Go SDK (`google.golang.org/genai`) targeting Gemini 2.0 Flash on Vertex AI for production**, paired with an **OpenAI-compatible LiteLLM / Ollama fallback adapter for Homelab development**.

### Architectural Flow:

```
                            [ Categorization Service ]
                                        │
                                        ▼
                         [ ports.CategorizationEngine ]
                                        │
                    ┌───────────────────┴───────────────────┐
                    │                                       │
           (AI_PROVIDER=vertexai)                  (AI_PROVIDER=litellm)
                    │                                       │
                    ▼                                       ▼
          [ VertexAIAdapter ]                      [ LiteLLMAdapter ]
       (google.golang.org/genai)                (OpenAI-Compatible HTTP)
                    │                                       │
                    ▼                                       ▼
         [ Vertex AI Gemini 2.0 ]                 [ LiteLLM Proxy ]
       (GenAI App Builder Credits)             (http://192.168.3.10:4000)
                                                            │
                                             ┌──────────────┴──────────────┐
                                             │                             │
                                             ▼                             ▼
                                     [ Local Ollama ]               [ Free Cloud ]
                                     (lm-claw:11434)                (Gemini Key)
```

### SDK Rationalization:
Google recently unified its Go AI SDK landscape into `google.golang.org/genai`. This replaces the legacy `cloud.google.com/go/vertexai/genai` and `github.com/google/generative-ai-go` packages. Setting `Backend: genai.BackendEnterprise` points directly to Vertex AI under Google Cloud project credentials, while maintaining a single, future-proof API surface.

---

## 6. Core Interface Contracts (`internal/ports/ai.go`)

```go
package ports

import (
	"context"
	"io"

	"github.com/google/uuid"
	"intellifinance/internal/domain"
)

type CategorizationRequest struct {
	TransactionID       string
	Description         string
	Amount              domain.Money
	AvailableCategories []domain.CategorySummary
}

type CategorizationResult struct {
	TransactionID string    `json:"transaction_id"`
	CategoryID    uuid.UUID `json:"category_id"`
	Confidence    float64   `json:"confidence"` // 0.00 to 1.00
	Reasoning     string    `json:"reasoning"`
}

type CategorizationEngine interface {
	CategorizeBatch(ctx context.Context, requests []CategorizationRequest) ([]CategorizationResult, error)
}

type DocumentParserEngine interface {
	ExtractTransactionsFromStream(ctx context.Context, mimeType string, r io.Reader) ([]domain.RawTransaction, error)
}
```

---

## 7. Concrete Adapter Implementations

### 7.1 Production: `VertexAIAdapter` (`internal/adapters/outbound/ai/vertexai.go`)

```go
package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
	"intellifinance/internal/ports"
)

type VertexAIAdapter struct {
	client *genai.Client
	model  string
}

func NewVertexAIAdapter(ctx context.Context, projectID, location string) (*VertexAIAdapter, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendEnterprise, // Vertex AI mode consuming GCP credits
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init vertexai client: %w", err)
	}

	return &VertexAIAdapter{
		client: client,
		model:  "gemini-2.0-flash",
	}, nil
}

func (a *VertexAIAdapter) CategorizeBatch(ctx context.Context, requests []ports.CategorizationRequest) ([]ports.CategorizationResult, error) {
	prompt := buildBatchCategorizationPrompt(requests)

	// Enforce strict JSON output
	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		Temperature:      genai.Ptr(0.1), // Low temperature for deterministic classification
	}

	resp, err := a.client.Models.GenerateContent(ctx, a.model, genai.Text(prompt), config)
	if err != nil {
		return nil, fmt.Errorf("gemini categorization generation failed: %w", err)
	}

	var results []ports.CategorizationResult
	if err := json.Unmarshal([]byte(resp.Text), &results); err != nil {
		return nil, fmt.Errorf("failed to parse structured gemini response: %w", err)
	}

	return results, nil
}
```

### 7.2 Homelab Dev: `LiteLLMAdapter` (`internal/adapters/outbound/ai/litellm.go`)

In local development, the `LiteLLMAdapter` makes standard HTTP POST calls to the LiteLLM proxy running on `lm-claw` (`http://192.168.3.10:4000/v1/chat/completions`). LiteLLM routes the request to local Ollama models (`qwen2.5-coder:7b` or `llama3.2:3b`), requiring zero external network calls and zero GCP billing.

---

## 8. Confidence Scoring & Human-in-the-Loop Feedback

To prevent AI hallucinations from corrupting accounting records, IntelliFinance enforces an automated confidence threshold:

1. **High Confidence (`>= 0.85`)**:
   - The transaction category is automatically assigned and saved.
   - Database flags: `is_ai_categorized = TRUE`, `is_manually_verified = FALSE`.
   - Rendered in frontend UI with an AI sparkle icon indicating high-confidence categorization.
2. **Low Confidence (`< 0.85`)**:
   - The transaction is assigned the highest-ranking candidate category, but flagged for user verification.
   - Frontend displays a review badge: *"Review Suggested"*.
3. **User Manual Correction Loop**:
   - When a user modifies a category (`PATCH /api/v1/transactions/{id}/category`), the system marks `is_manually_verified = TRUE` and writes an audit entry into `category_correction_audit`.
   - In subsequent batch prompts, recent user corrections for that household are included as few-shot examples, allowing the model to adapt dynamically to household-specific merchant naming.

---

## 9. Multimodal PDF Bank Statement Extraction

For digital or scanned PDF bank statements:
1. The binary stream is passed directly as inline multimodal data (`application/pdf`) to Gemini 2.0 Flash.
2. Gemini visually parses the document layout, identifying account numbers, statement periods, transaction tables, and ending balances.
3. The response is constrained via JSON schema to output an array of `RawTransaction` items containing `date`, `description`, `amount` (properly signed: negative for debits, positive for credits), and inferred `category_hint`.

---

## 10. Consequences

### Positive Consequences
- **Zero Incremental Cost in Production**: Fully funded by Google Cloud GenAI App Builder and startup credits.
- **Robust Scanned Statement Extraction**: Eliminates failures caused by legacy regex/Tesseract parsers on complex multi-column statements.
- **Total Local Independence**: Developers can test and run the entire ingestion pipeline offline using `lm-claw` LiteLLM/Ollama.
- **Continuous Household Learning**: Few-shot corrections improve accuracy over time without fine-tuning model weights.

### Negative Consequences
- **Model Drift & Non-Determinism**: LLM predictions can occasionally vary; mitigated by temperature=0.1 and strict JSON output schemas.
- **Batch Latency**: AI categorization adds 500ms–2s per statement batch; mitigated by processing asynchronously via Asynq/Cloud Tasks.

### Neutral Consequences
- Prompts must maintain token efficiency by categorizing in batches of 25–50 transactions.

---

## 11. Implementation & Verification Plan

1. **Verify Official SDK Integration**:
   - Test `google.golang.org/genai` initialization using GCP service account credentials in a staging test.
2. **Verify JSON Schema Output**:
   - Send 10 synthetic Brazilian bank descriptions to `VertexAIAdapter` and assert that the response decodes into `[]ports.CategorizationResult` without unmarshaling errors.
3. **Verify LiteLLM Dev Fallback**:
   - Configure `AI_PROVIDER=litellm` and execute categorization against `http://192.168.3.10:4000`. Assert successful response formatting.
