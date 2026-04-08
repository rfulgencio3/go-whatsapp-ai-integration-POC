package agro

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	interpreterProvider = "rule-engine"
	interpreterModel    = "agro-v1"
	promptVersion       = "rules-v1"
	cattleGestationDays = 283
)

var (
	currencyBeforePattern = regexp.MustCompile(`r\$\s*(\d+(?:[.,]\d{1,2})?)`)
	currencyAfterPattern  = regexp.MustCompile(`(\d+(?:[.,]\d{1,2})?)\s*(?:reais|real)`)
	unitPricePattern      = regexp.MustCompile(`(?:r\$\s*(\d+(?:[.,]\d{1,2})?)|(\d+(?:[.,]\d{1,2})?)\s*(?:reais|real))\s*cada\b`)
	quantityPattern       = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*(sacos?|kg|quilo(?:s|gramas?)?|litros?|l|toneladas?|ton|unidades?|un|cabecas?|cabecas?|cabecas?)`)
	animalPattern         = regexp.MustCompile(`(?:vaca|matriz|novilha|animal)\s+([a-z0-9-]+)`)
	teatPattern           = regexp.MustCompile(`t(?:eta)?\s*([1-4])`)
)

type RuleBasedInterpreter struct{}

type interpretationPayload struct {
	Provider             string            `json:"provider"`
	Model                string            `json:"model"`
	PromptVersion        string            `json:"prompt_version"`
	NormalizedIntent     string            `json:"normalized_intent"`
	Category             string            `json:"category"`
	Subcategory          string            `json:"subcategory"`
	Description          string            `json:"description"`
	AnimalCode           string            `json:"animal_code,omitempty"`
	Confidence           float64           `json:"confidence"`
	RequiresConfirmation bool              `json:"requires_confirmation"`
	Amount               *float64          `json:"amount,omitempty"`
	Currency             string            `json:"currency,omitempty"`
	Quantity             *float64          `json:"quantity,omitempty"`
	Unit                 string            `json:"unit,omitempty"`
	OccurredAt           *string           `json:"occurred_at,omitempty"`
	Attributes           map[string]string `json:"attributes,omitempty"`
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
		AnimalCode:  extractAnimalCode(text),
		Attributes:  make(map[string]string),
	}

	switch {
	case isMastitisTreatment(normalizedText):
		result.NormalizedIntent = "health.mastitis_treatment"
		result.Category = "health"
		result.Subcategory = "mastitis_treatment"
		result.Confidence = 0.94
		result.RequiresConfirmation = true
		enrichMastitisAttributes(result.Attributes, text, normalizedText)
	case isHoofTreatment(normalizedText):
		result.NormalizedIntent = "health.hoof_treatment"
		result.Category = "health"
		result.Subcategory = "hoof_treatment"
		result.Confidence = 0.90
		result.RequiresConfirmation = true
		result.Attributes["health_issue_type"] = "casco"
	case isBloat(normalizedText):
		result.NormalizedIntent = "health.bloat"
		result.Category = "health"
		result.Subcategory = "bloat"
		result.Confidence = 0.88
		result.RequiresConfirmation = true
		result.Attributes["health_issue_type"] = "gases"
	case containsAny(normalizedText, "insemin"):
		result.NormalizedIntent = "reproduction.insemination"
		result.Category = "reproduction"
		result.Subcategory = "insemination"
		result.Confidence = 0.95
		result.RequiresConfirmation = true
		enrichInseminationAttributes(result.Attributes, result.OccurredAt)
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
	if result.OccurredAt == nil && result.Category != "health" && !input.OccurredAt.IsZero() {
		occurredAt := input.OccurredAt.UTC()
		result.OccurredAt = &occurredAt
	}

	result.Quantity, result.Unit = extractQuantity(text)
	result.Amount = extractAmount(text, result.Quantity)
	if result.Amount != nil {
		result.Currency = "BRL"
	}
	enrichPricingAttributes(result.Attributes, text, result.Quantity, result.Amount)
	if len(result.Attributes) == 0 {
		result.Attributes = nil
	}
	result.RawOutputJSON = buildInterpretationPayload(result)

	return result, nil
}

func isMastitisTreatment(text string) bool {
	return containsAny(text, "teta", "mastite", "ubere", "úbere", "nao pode tirar leite", "não pode tirar leite")
}

func isHoofTreatment(text string) bool {
	return containsAny(text, "casco", "manco", "manqueira", "claudic", "pododermat")
}

func isBloat(text string) bool {
	return containsAny(text, "gases", "estufad", "inchad", "timpan", "empanz")
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

func extractAmount(text string, quantity *float64) *float64 {
	if unitPrice, ok := extractUnitPrice(normalizeText(text)); ok && quantity != nil {
		total := unitPrice * *quantity
		return &total
	}
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

func extractUnitPrice(normalizedText string) (float64, bool) {
	matches := unitPricePattern.FindStringSubmatch(normalizedText)
	if len(matches) < 3 {
		return 0, false
	}
	for _, candidate := range matches[1:] {
		if value, ok := parseDecimal(candidate); ok {
			return value, true
		}
	}
	return 0, false
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

func extractAnimalCode(text string) string {
	matches := animalPattern.FindStringSubmatch(normalizeText(text))
	if len(matches) < 2 {
		return ""
	}

	return strings.ToUpper(strings.TrimSpace(matches[1]))
}

func enrichMastitisAttributes(attributes map[string]string, text, normalizedText string) {
	if attributes == nil {
		return
	}

	teats := extractAffectedTeats(text)
	if len(teats) > 0 {
		attributes["affected_teats"] = strings.Join(teats, ",")
	}
	if containsAny(normalizedText, "nao pode tirar leite", "não pode tirar leite", "descartar leite", "nao tirar leite") {
		attributes["milk_withdrawal"] = "true"
	}
	attributes["health_issue_type"] = "teta"
}

func extractAffectedTeats(text string) []string {
	matches := teatPattern.FindAllStringSubmatch(normalizeText(text), -1)
	if len(matches) == 0 {
		return nil
	}

	unique := make(map[string]struct{})
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		label := "T" + strings.TrimSpace(match[1])
		unique[label] = struct{}{}
	}

	if len(unique) == 0 {
		return nil
	}

	result := make([]string, 0, len(unique))
	for label := range unique {
		result = append(result, label)
	}
	sort.Strings(result)
	return result
}

func resolveOccurredAt(text string, fallback time.Time) *time.Time {
	if containsAny(normalizeText(text), "hoje") && !fallback.IsZero() {
		timestamp := fallback.UTC()
		return &timestamp
	}

	return nil
}

func enrichInseminationAttributes(attributes map[string]string, occurredAt *time.Time) {
	if attributes == nil || occurredAt == nil {
		return
	}

	expectedCalvingDate := occurredAt.UTC().AddDate(0, 0, cattleGestationDays)
	attributes["expected_calving_date"] = expectedCalvingDate.Format("02/01/2006")
}

func enrichPricingAttributes(attributes map[string]string, text string, quantity, amount *float64) {
	if attributes == nil {
		return
	}

	unitPrice, ok := extractUnitPrice(normalizeText(text))
	if !ok {
		return
	}

	attributes["unit_price"] = strconv.FormatFloat(unitPrice, 'f', 2, 64)
	if quantity != nil && amount != nil {
		attributes["amount_inferred_from_unit_price"] = "true"
	}
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
		AnimalCode:           result.AnimalCode,
		Confidence:           result.Confidence,
		RequiresConfirmation: result.RequiresConfirmation,
		Amount:               result.Amount,
		Currency:             result.Currency,
		Quantity:             result.Quantity,
		Unit:                 result.Unit,
		Attributes:           result.Attributes,
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
