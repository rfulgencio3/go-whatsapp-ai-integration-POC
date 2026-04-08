package agro

import "strings"

type confirmationDecision string
type helpTopic string

const (
	confirmationAccepted confirmationDecision = "accepted"
	confirmationRejected confirmationDecision = "rejected"

	helpTopicGeneral    helpTopic = "general"
	helpTopicTreatments helpTopic = "treatments"
	helpTopicPurchases  helpTopic = "purchases"
	helpTopicQueries    helpTopic = "queries"
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

func isHelpCommand(text string) bool {
	return parseHelpTopic(text) != ""
}

func parseAnimalRegistrationCommand(text string) (string, bool) {
	normalized := normalizeText(text)
	switch {
	case strings.HasPrefix(normalized, "cadastrar vaca "):
		return extractTrailingAnimalCode(normalized, "cadastrar vaca ")
	case strings.HasPrefix(normalized, "cadastrar matriz "):
		return extractTrailingAnimalCode(normalized, "cadastrar matriz ")
	case strings.HasPrefix(normalized, "cadastrar animal "):
		return extractTrailingAnimalCode(normalized, "cadastrar animal ")
	default:
		return "", false
	}
}

func extractTrailingAnimalCode(text, prefix string) (string, bool) {
	value := strings.TrimSpace(strings.TrimPrefix(text, prefix))
	if value == "" {
		return "", false
	}
	return strings.ToUpper(value), true
}

func parseHelpTopic(text string) helpTopic {
	normalized := normalizeText(text)
	switch normalized {
	case "ajuda", "help", "socorro", "o que posso registrar", "o que eu posso registrar", "exemplos", "como funciona":
		return helpTopicGeneral
	case "consultas disponiveis", "quais consultas posso fazer", "quais consultas posso consultar":
		return helpTopicQueries
	case "exemplos de tratamento", "exemplo de tratamento", "como registrar tratamento", "como lancar tratamento":
		return helpTopicTreatments
	case "exemplos de compras", "exemplo de compra", "como registrar compra", "como lancar compra":
		return helpTopicPurchases
	}

	switch {
	case strings.Contains(normalized, "consulta") && strings.Contains(normalized, "disponiv"):
		return helpTopicQueries
	case strings.Contains(normalized, "quais consultas"):
		return helpTopicQueries
	case strings.Contains(normalized, "exemplo") && strings.Contains(normalized, "tratamento"):
		return helpTopicTreatments
	case strings.Contains(normalized, "registrar tratamento"):
		return helpTopicTreatments
	case strings.Contains(normalized, "exemplo") && strings.Contains(normalized, "compra"):
		return helpTopicPurchases
	case strings.Contains(normalized, "registrar compra"):
		return helpTopicPurchases
	default:
		return ""
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
