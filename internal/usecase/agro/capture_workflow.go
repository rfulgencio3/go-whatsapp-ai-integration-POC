package agro

import "strings"

type confirmationDecision string

const (
	confirmationAccepted confirmationDecision = "accepted"
	confirmationRejected confirmationDecision = "rejected"
)

func classifyConfirmationDecision(text string) confirmationDecision {
	normalized := normalizeText(text)
	switch normalized {
	case "sim", "s", "ok", "confirmar", "confirmado", "pode confirmar", "isso":
		return confirmationAccepted
	case "nao", "nÃ£o", "n", "cancelar", "corrigir", "errado":
		return confirmationRejected
	default:
		return ""
	}
}

func parseContextSelection(text string) int {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return 0
	}
	if len(normalized) != 1 || normalized[0] < '1' || normalized[0] > '9' {
		return 0
	}

	return int(normalized[0] - '0')
}

func isContextSwitchCommand(text string) bool {
	switch normalizeText(text) {
	case "trocar fazenda", "mudar fazenda", "alternar fazenda", "selecionar fazenda", "trocar contexto", "mudar contexto":
		return true
	default:
		return false
	}
}

func isOnboardingStartCommand(text string) bool {
	switch normalizeText(text) {
	case "cadastrar", "cadastro", "quero cadastrar", "iniciar cadastro", "me cadastrar":
		return true
	default:
		return false
	}
}

func isMilkWithdrawalQuery(text string) bool {
	normalized := normalizeText(text)
	switch {
	case strings.Contains(normalized, "quais vacas") && strings.Contains(normalized, "tirar leite"):
		return true
	case strings.Contains(normalized, "quais animais") && strings.Contains(normalized, "tirar leite"):
		return true
	case strings.Contains(normalized, "sem poder tirar leite"):
		return true
	case strings.Contains(normalized, "nao podem tirar leite"):
		return true
	case strings.Contains(normalized, "nao pode tirar leite") && strings.Contains(normalized, "quais"):
		return true
	default:
		return false
	}
}

func isRecentTreatmentsQuery(text string) bool {
	normalized := normalizeText(text)
	switch {
	case strings.Contains(normalized, "ultimos tratamentos"):
		return true
	case strings.Contains(normalized, "ultimos casos"):
		return true
	case strings.Contains(normalized, "tratamentos recentes"):
		return true
	case strings.Contains(normalized, "quais foram os ultimos tratamentos"):
		return true
	default:
		return false
	}
}

func isMedicineExpenseMonthQuery(text string) bool {
	normalized := normalizeText(text)
	hasMedicine := strings.Contains(normalized, "medicamento") || strings.Contains(normalized, "remedio") || strings.Contains(normalized, "remedio")
	hasMonth := strings.Contains(normalized, "esse mes") || strings.Contains(normalized, "este mes") || strings.Contains(normalized, "no mes") || strings.Contains(normalized, "neste mes")
	hasSpend := strings.Contains(normalized, "quanto gastei") || strings.Contains(normalized, "quanto foi gasto") || strings.Contains(normalized, "gasto com")
	return hasMedicine && hasMonth && hasSpend
}

func isVetExpenseMonthQuery(text string) bool {
	normalized := normalizeText(text)
	hasVet := strings.Contains(normalized, "veterinario") || strings.Contains(normalized, "veterinaria") || strings.Contains(normalized, "consulta veterinaria")
	hasMonth := strings.Contains(normalized, "esse mes") || strings.Contains(normalized, "este mes") || strings.Contains(normalized, "no mes") || strings.Contains(normalized, "neste mes")
	hasSpend := strings.Contains(normalized, "quanto gastei") || strings.Contains(normalized, "quanto foi gasto") || strings.Contains(normalized, "gasto com")
	return hasVet && hasMonth && hasSpend
}

func isRecentPurchasesQuery(text string) bool {
	normalized := normalizeText(text)
	switch {
	case strings.Contains(normalized, "ultimas compras"):
		return true
	case strings.Contains(normalized, "ultimas aquisicoes"):
		return true
	case strings.Contains(normalized, "ultimos insumos comprados"):
		return true
	case strings.Contains(normalized, "quais foram as ultimas compras"):
		return true
	default:
		return false
	}
}
