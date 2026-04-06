package agro

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	interpreterProvider = "rule-engine"
	interpreterModel    = "agro-v1"
	promptVersion       = "rules-v1"
)

var (
	currencyBeforePattern = regexp.MustCompile(`r\$\s*(\d+(?:[.,]\d{1,2})?)`)
	currencyAfterPattern  = regexp.MustCompile(`(\d+(?:[.,]\d{1,2})?)\s*(?:reais|real)`)
	quantityPattern       = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*(sacos?|kg|quilo(?:s|gramas?)?|litros?|l|toneladas?|ton|unidades?|un|cabecas?|cabecas?|cabecas?)`)
)

type RuleBasedInterpreter struct{}

type interpretationPayload struct {
	Provider             string   `json:"provider"`
	Model                string   `json:"model"`
	PromptVersion        string   `json:"prompt_version"`
	NormalizedIntent     string   `json:"normalized_intent"`
	Category             string   `json:"category"`
	Subcategory          string   `json:"subcategory"`
	Description          string   `json:"description"`
	Confidence           float64  `json:"confidence"`
	RequiresConfirmation bool     `json:"requires_confirmation"`
	Amount               *float64 `json:"amount,omitempty"`
	Currency             string   `json:"currency,omitempty"`
	Quantity             *float64 `json:"quantity,omitempty"`
	Unit                 string   `json:"unit,omitempty"`
	OccurredAt           *string  `json:"occurred_at,omitempty"`
}

// NewRuleBasedInterpreter creates the first deterministic agro interpreter used by the POC.
func NewRuleBasedInterpreter() *RuleBasedInterpreter {
	return &RuleBasedInterpreter{}
}

// Interpret classifies inbound text into the initial business categories and extracts basic fields.
func (r *RuleBasedInterpreter) Interpret(_ context.Context, input InterpretationInput) (InterpretationResult, error) {
	text := strings.TrimSpace(input.Text)
	if text == "" {
		return InterpretationResult{}, nil
	}

	normalizedText := normalizeText(text)
	result := InterpretationResult{
		Description: text,
		OccurredAt:  resolveOccurredAt(text, input.OccurredAt),
	}

	switch {
	case containsAny(normalizedText, "insemin"):
		result.NormalizedIntent = "reproduction.insemination"
		result.Category = "reproduction"
		result.Subcategory = "insemination"
		result.Confidence = 0.95
		result.RequiresConfirmation = true
	case isInputPurchase(normalizedText):
		result.NormalizedIntent = "finance.input_purchase"
		result.Category = "finance"
		result.Subcategory = "input_purchase"
		result.Confidence = 0.90
		result.RequiresConfirmation = true
	case isRevenue(normalizedText):
		result.NormalizedIntent = "finance.revenue"
		result.Category = "finance"
		result.Subcategory = "revenue"
		result.Confidence = 0.86
		result.RequiresConfirmation = true
	case isExpense(normalizedText):
		result.NormalizedIntent = "finance.expense"
		result.Category = "finance"
		result.Subcategory = "expense"
		result.Confidence = 0.82
		result.RequiresConfirmation = true
	default:
		result.NormalizedIntent = "operations.note"
		result.Category = "operations"
		result.Subcategory = "note"
		result.Confidence = 0.60
		result.RequiresConfirmation = false
	}

	result.Amount = extractAmount(text)
	if result.Amount != nil {
		result.Currency = "BRL"
	}
	result.Quantity, result.Unit = extractQuantity(text)
	result.RawOutputJSON = buildInterpretationPayload(result)

	return result, nil
}

func isInputPurchase(text string) bool {
	if !containsAny(text, "comprei", "compramos", "compra", "adquiri") {
		return false
	}

	return containsAny(text,
		"insumo",
		"racao",
		"ração",
		"adubo",
		"fertiliz",
		"sal mineral",
		"semente",
		"vacina",
		"medicamento",
		"herbicida",
		"defensivo",
		"saco",
		"sacos",
	)
}

func isRevenue(text string) bool {
	return containsAny(text,
		"recebi",
		"recebemos",
		"vendi",
		"vendemos",
		"venda",
		"receita",
		"faturei",
		"entrada de",
	)
}

func isExpense(text string) bool {
	return containsAny(text,
		"gastei",
		"gastamos",
		"paguei",
		"pagamos",
		"despesa",
		"custo",
		"custou",
		"gasto",
	)
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}

	return false
}

func normalizeText(value string) string {
	replacer := strings.NewReplacer(
		"á", "a",
		"à", "a",
		"ã", "a",
		"â", "a",
		"é", "e",
		"ê", "e",
		"í", "i",
		"ó", "o",
		"ô", "o",
		"õ", "o",
		"ú", "u",
		"ç", "c",
		"Ã¡", "a",
		"Ã ", "a",
		"Ã£", "a",
		"Ã¢", "a",
		"Ã©", "e",
		"Ãª", "e",
		"Ã­", "i",
		"Ã³", "o",
		"Ã´", "o",
		"Ãµ", "o",
		"Ãº", "u",
		"Ã§", "c",
	)

	return replacer.Replace(strings.ToLower(strings.TrimSpace(value)))
}

func extractAmount(text string) *float64 {
	if matches := currencyBeforePattern.FindStringSubmatch(normalizeText(text)); len(matches) > 1 {
		if value, ok := parseDecimal(matches[1]); ok {
			return &value
		}
	}
	if matches := currencyAfterPattern.FindStringSubmatch(normalizeText(text)); len(matches) > 1 {
		if value, ok := parseDecimal(matches[1]); ok {
			return &value
		}
	}

	return nil
}

func extractQuantity(text string) (*float64, string) {
	matches := quantityPattern.FindStringSubmatch(normalizeText(text))
	if len(matches) < 3 {
		return nil, ""
	}

	value, ok := parseDecimal(matches[1])
	if !ok {
		return nil, ""
	}

	return &value, normalizeUnit(matches[2])
}

func parseDecimal(value string) (float64, bool) {
	normalized := strings.TrimSpace(value)
	switch {
	case strings.Contains(normalized, ".") && strings.Contains(normalized, ","):
		normalized = strings.ReplaceAll(normalized, ".", "")
		normalized = strings.ReplaceAll(normalized, ",", ".")
	case strings.Contains(normalized, ","):
		normalized = strings.ReplaceAll(normalized, ",", ".")
	}

	parsed, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}

	return parsed, true
}

func normalizeUnit(unit string) string {
	unit = normalizeText(unit)
	switch {
	case strings.HasPrefix(unit, "saco"):
		return "saco"
	case strings.HasPrefix(unit, "quilo"), unit == "kg":
		return "kg"
	case strings.HasPrefix(unit, "litro"), unit == "l":
		return "litro"
	case strings.HasPrefix(unit, "ton"):
		return "tonelada"
	case strings.HasPrefix(unit, "un"):
		return "unidade"
	case strings.HasPrefix(unit, "cabeca"):
		return "cabeca"
	default:
		return unit
	}
}

func resolveOccurredAt(text string, fallback time.Time) *time.Time {
	if containsAny(normalizeText(text), "hoje") && !fallback.IsZero() {
		timestamp := fallback.UTC()
		return &timestamp
	}

	return nil
}

func buildInterpretationPayload(result InterpretationResult) string {
	payload := interpretationPayload{
		Provider:             interpreterProvider,
		Model:                interpreterModel,
		PromptVersion:        promptVersion,
		NormalizedIntent:     result.NormalizedIntent,
		Category:             result.Category,
		Subcategory:          result.Subcategory,
		Description:          result.Description,
		Confidence:           result.Confidence,
		RequiresConfirmation: result.RequiresConfirmation,
		Amount:               result.Amount,
		Currency:             result.Currency,
		Quantity:             result.Quantity,
		Unit:                 result.Unit,
	}
	if result.OccurredAt != nil {
		formatted := result.OccurredAt.UTC().Format(time.RFC3339)
		payload.OccurredAt = &formatted
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return `{"provider":"rule-engine","error":"marshal_failed"}`
	}

	return string(body)
}
