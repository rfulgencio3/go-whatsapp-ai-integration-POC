package agro

import "strings"

type confirmationDecision string
type helpTopic string

type animalRegistrationCommand struct {
	AnimalCode       string
	AnimalType       string
	Sex              string
	BirthDate        string
	MotherAnimalCode string
	FirstCalvingDate string
}

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

func parseAnimalRegistrationCommand(text string) (animalRegistrationCommand, bool) {
	normalized := normalizeText(text)
	switch {
	case strings.HasPrefix(normalized, "cadastrar vaca "):
		return parseAnimalRegistrationDetails(normalized, "cadastrar vaca ", "vaca", "female")
	case strings.HasPrefix(normalized, "cadastrar matriz "):
		return parseAnimalRegistrationDetails(normalized, "cadastrar matriz ", "vaca", "female")
	case strings.HasPrefix(normalized, "cadastrar novilha "):
		return parseAnimalRegistrationDetails(normalized, "cadastrar novilha ", "novilha", "female")
	case strings.HasPrefix(normalized, "cadastrar bezerra "):
		return parseAnimalRegistrationDetails(normalized, "cadastrar bezerra ", "bezerra", "female")
	case strings.HasPrefix(normalized, "cadastrar bezerro "):
		return parseAnimalRegistrationDetails(normalized, "cadastrar bezerro ", "bezerro", "male")
	case strings.HasPrefix(normalized, "cadastrar animal "):
		return parseAnimalRegistrationDetails(normalized, "cadastrar animal ", "", "")
	default:
		return animalRegistrationCommand{}, false
	}
}

func parseAnimalRegistrationDetails(text, prefix, animalType, sex string) (animalRegistrationCommand, bool) {
	value := strings.TrimSpace(strings.TrimPrefix(text, prefix))
	if value == "" {
		return animalRegistrationCommand{}, false
	}

	command := animalRegistrationCommand{
		AnimalType: animalType,
		Sex:        sex,
	}
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return animalRegistrationCommand{}, false
	}
	command.AnimalCode = strings.ToUpper(strings.TrimSpace(parts[0]))
	remainder := strings.TrimSpace(strings.TrimPrefix(value, parts[0]))

	switch {
	case strings.Contains(remainder, "filha da vaca "):
		command.MotherAnimalCode = extractDelimitedValue(remainder, "filha da vaca ", " nascida em ")
	case strings.Contains(remainder, "filho da vaca "):
		command.MotherAnimalCode = extractDelimitedValue(remainder, "filho da vaca ", " nascido em ")
	case strings.Contains(remainder, "filha de "):
		command.MotherAnimalCode = extractDelimitedValue(remainder, "filha de ", " nascida em ")
	case strings.Contains(remainder, "filho de "):
		command.MotherAnimalCode = extractDelimitedValue(remainder, "filho de ", " nascido em ")
	}

	switch {
	case strings.Contains(remainder, "nascida em "):
		command.BirthDate = extractDelimitedValue(remainder, "nascida em ", " primeiro parto em ")
	case strings.Contains(remainder, "nascido em "):
		command.BirthDate = extractDelimitedValue(remainder, "nascido em ", " primeiro parto em ")
	}

	if strings.Contains(remainder, "primeiro parto em ") {
		command.FirstCalvingDate = extractDelimitedValue(remainder, "primeiro parto em ", "")
	}

	return command, true
}

func extractDelimitedValue(text, startMarker, endMarker string) string {
	value := text
	if startMarker != "" {
		startIndex := strings.Index(value, startMarker)
		if startIndex < 0 {
			return ""
		}
		value = value[startIndex+len(startMarker):]
	}
	if endMarker != "" {
		if endIndex := strings.Index(value, endMarker); endIndex >= 0 {
			value = value[:endIndex]
		}
	}
	return strings.ToUpper(strings.TrimSpace(value))
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
