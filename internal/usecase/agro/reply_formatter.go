package agro

import (
	"fmt"
	"strings"
	"time"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
)

func buildConfirmedReply(event domain.BusinessEvent) string {
	switch {
	case event.Category == "finance" && event.Subcategory == "input_purchase" && event.Amount != nil && event.Quantity != nil && strings.TrimSpace(event.Unit) != "":
		return fmt.Sprintf("Perfeito. Registrei a compra de insumos em %s, com %.3g %s.", formatCurrency(event.Amount, event.Currency), *event.Quantity, event.Unit)
	case event.Category == "finance" && event.Subcategory == "expense" && event.Amount != nil:
		return fmt.Sprintf("Perfeito. Registrei a despesa em %s.", formatCurrency(event.Amount, event.Currency))
	case event.Category == "finance" && event.Subcategory == "revenue" && event.Amount != nil:
		return fmt.Sprintf("Perfeito. Registrei a receita em %s.", formatCurrency(event.Amount, event.Currency))
	case event.Category == "reproduction" && event.Subcategory == "insemination":
		return "Perfeito. Registrei o evento de inseminacao."
	default:
		return "Perfeito. Registrei essa informacao."
	}
}

func buildHelpReply(topic helpTopic, registered bool) string {
	lines := []string{"Posso te ajudar com registros e consultas objetivas da fazenda."}

	switch topic {
	case helpTopicTreatments:
		lines = append(lines,
			"Exemplos de tratamento:",
			"- A vaca 32 esta com problema na teta T3 e nao pode tirar leite",
			"- A vaca 18 esta mancando por problema de casco",
			"- A vaca 21 esta com gases e barriga inchada",
			"Quando faltar algum dado, eu posso pedir data do diagnostico, medicamento e dias de tratamento.",
		)
	case helpTopicPurchases:
		lines = append(lines,
			"Exemplos de compras:",
			"- Comprei 10 sacos de racao por 850 reais",
			"- Compramos adubo por 1200 reais",
			"- Adquiri 5 litros de herbicida por 430 reais",
			"Se a mensagem estiver clara, eu monto um resumo para voce confirmar com SIM ou NAO.",
		)
	case helpTopicQueries:
		lines = append(lines,
			"Consultas disponiveis:",
			"- Quais vacas nao podem tirar leite?",
			"- Quais foram os ultimos tratamentos?",
			"- Quanto gastei com medicamento esse mes?",
			"- Quanto gastei com veterinario esse mes?",
			"- Quais foram as ultimas compras?",
		)
	default:
		lines = append(lines,
			"Exemplos de registros:",
			"- Comprei 10 sacos de racao por 850 reais",
			"- A vaca 32 esta com problema na teta T3 e nao pode tirar leite",
			"Exemplos de consultas:",
			"- Quais vacas nao podem tirar leite?",
			"- Quais foram os ultimos tratamentos?",
			"- Quanto gastei com medicamento esse mes?",
			"- Quanto gastei com veterinario esse mes?",
			"- Quais foram as ultimas compras?",
		)
	}
	if !registered {
		lines = append(lines, "Se seu numero ainda nao estiver vinculado, responda CADASTRAR para iniciar o cadastro.")
	}
	return strings.Join(lines, "\n")
}

func buildOnboardingHelpReply(step domain.OnboardingStep) string {
	switch step {
	case domain.OnboardingStepAwaitingProducerName:
		return "Estamos no cadastro inicial. Me envie o nome do produtor ou responsavel para continuar."
	case domain.OnboardingStepAwaitingFarmName:
		return "Estamos quase terminando o cadastro. Agora me envie o nome da fazenda ou negocio."
	default:
		return "Se quiser iniciar o cadastro, responda CADASTRAR."
	}
}

func buildHealthTreatmentHelpReply(state domain.HealthTreatmentState) string {
	base := "Estamos registrando um tratamento de saude animal."
	if strings.TrimSpace(state.AnimalCode) != "" {
		base += " Animal atual: " + strings.TrimSpace(state.AnimalCode) + "."
	}

	switch state.Step {
	case domain.HealthTreatmentStepAwaitingDiagnosisDate:
		return base + " Agora me informe a data do diagnostico. Pode ser, por exemplo, HOJE ou 07/04/2026."
	case domain.HealthTreatmentStepAwaitingMedicine:
		return base + " Agora me informe o medicamento aplicado."
	case domain.HealthTreatmentStepAwaitingTreatmentDays:
		return base + " Agora me informe por quantos dias sera o tratamento. Exemplo: 5 dias."
	default:
		return base + " Pode me enviar as proximas informacoes do tratamento."
	}
}

func buildCorrelatedExpenseHelpReply(state domain.CorrelatedExpenseState) string {
	base := "Estamos registrando os gastos relacionados a esse tratamento."
	if strings.TrimSpace(state.AnimalCode) != "" {
		base += " Animal atual: " + strings.TrimSpace(state.AnimalCode) + "."
	}

	switch state.Step {
	case domain.CorrelatedExpenseStepAwaitingDecision:
		return base + " Responda SIM para lancar os gastos ou NAO para pular essa etapa."
	case domain.CorrelatedExpenseStepAwaitingMedicineAmount:
		return base + " Agora me informe o valor gasto com medicamento. Se nao houve, responda 0."
	case domain.CorrelatedExpenseStepAwaitingVetAmount:
		return base + " Agora me informe o valor da consulta veterinaria. Se nao houve, responda 0."
	case domain.CorrelatedExpenseStepAwaitingExamAmount:
		return base + " Agora me informe o valor de exames. Se nao houve, responda 0."
	default:
		return base
	}
}

func buildHealthExpenseCorrelationPrompt(event domain.BusinessEvent) string {
	return buildConfirmedReply(event) + "\nDeseja lancar tambem os gastos com medicamento, consulta veterinaria e exames? Responda SIM ou NAO."
}

func buildCorrelatedExpenseQuestion(state domain.CorrelatedExpenseState) string {
	switch state.Step {
	case domain.CorrelatedExpenseStepAwaitingMedicineAmount:
		return "Certo. Qual foi o valor gasto com medicamento? Se nao houve, responda 0."
	case domain.CorrelatedExpenseStepAwaitingVetAmount:
		return "Entendi. Qual foi o valor da consulta veterinaria? Se nao houve, responda 0."
	case domain.CorrelatedExpenseStepAwaitingExamAmount:
		return "Perfeito. Qual foi o valor de exames? Se nao houve, responda 0."
	default:
		return "Deseja lancar os gastos relacionados? Responda SIM ou NAO."
	}
}

func buildCorrelatedExpenseDeclinedReply() string {
	return "Tudo bem. Nao vou lancar gastos relacionados a esse tratamento."
}

func buildCorrelatedExpenseRecordedReply(state domain.CorrelatedExpenseState) string {
	lines := []string{"Gastos relacionados registrados:"}
	if state.MedicineAmount != nil && *state.MedicineAmount > 0 {
		lines = append(lines, fmt.Sprintf("Medicamento: %s", formatCurrency(state.MedicineAmount, "BRL")))
	}
	if state.VetAmount != nil && *state.VetAmount > 0 {
		lines = append(lines, fmt.Sprintf("Consulta veterinaria: %s", formatCurrency(state.VetAmount, "BRL")))
	}
	if state.ExamAmount != nil && *state.ExamAmount > 0 {
		lines = append(lines, fmt.Sprintf("Exames: %s", formatCurrency(state.ExamAmount, "BRL")))
	}
	if len(lines) == 1 {
		return "Tudo certo. Nao registrei gastos relacionados para esse tratamento."
	}
	return strings.Join(lines, "\n")
}

func buildMilkWithdrawalQueryReply(items []domain.MilkWithdrawalAnimal, reference time.Time) string {
	if len(items) == 0 {
		return "No momento, nao encontrei vacas com restricao de leite ativa."
	}

	lines := []string{"Vacas com restricao de leite ativa:"}
	for _, item := range items {
		line := "Animal: " + item.AnimalCode
		if len(item.AffectedTeats) > 0 {
			line += " | Tetas: " + strings.Join(item.AffectedTeats, ",")
		}
		if item.ActiveUntil != nil {
			line += " | Ate: " + item.ActiveUntil.In(time.FixedZone("BRT", -3*60*60)).Format("02/01/2006")
		}
		lines = append(lines, line)
	}
	lines = append(lines, "Referencia: "+reference.In(time.FixedZone("BRT", -3*60*60)).Format("02/01/2006 15:04"))
	return strings.Join(lines, "\n")
}

func buildRecentHealthTreatmentsReply(items []domain.HealthTreatmentSummary, reference time.Time) string {
	if len(items) == 0 {
		return "Nao encontrei tratamentos de saude registrados recentemente."
	}

	lines := []string{"Ultimos tratamentos de saude:"}
	for _, item := range items {
		line := "Animal: " + item.AnimalCode
		if item.Subcategory != "" {
			line += " | Tipo: " + humanCategoryLabel("health", item.Subcategory)
		}
		if len(item.AffectedTeats) > 0 {
			line += " | Tetas: " + strings.Join(item.AffectedTeats, ",")
		}
		if item.OccurredAt != nil {
			line += " | Data: " + item.OccurredAt.In(time.FixedZone("BRT", -3*60*60)).Format("02/01/2006")
		}
		lines = append(lines, line)
	}
	lines = append(lines, "Referencia: "+reference.In(time.FixedZone("BRT", -3*60*60)).Format("02/01/2006 15:04"))
	return strings.Join(lines, "\n")
}

func buildMedicineExpenseMonthReply(amount float64, reference time.Time) string {
	monthLabel := reference.In(time.FixedZone("BRT", -3*60*60)).Format("01/2006")
	return fmt.Sprintf("Gasto com medicamento no mes %s: R$ %.2f", monthLabel, amount)
}

func buildVetExpenseMonthReply(amount float64, reference time.Time) string {
	monthLabel := reference.In(time.FixedZone("BRT", -3*60*60)).Format("01/2006")
	return fmt.Sprintf("Gasto com veterinario no mes %s: R$ %.2f", monthLabel, amount)
}

func buildRecentInputPurchasesReply(items []domain.InputPurchaseSummary, reference time.Time) string {
	if len(items) == 0 {
		return "Nao encontrei compras de insumos registradas recentemente."
	}

	lines := []string{"Ultimas compras de insumos:"}
	for _, item := range items {
		line := strings.TrimSpace(item.Description)
		if line == "" {
			line = "Compra registrada"
		}
		if item.Amount != nil {
			line += " | Valor: " + formatCurrency(item.Amount, "BRL")
		}
		if item.Quantity != nil {
			line += " | Quantidade: " + formatQuantity(item.Quantity, item.Unit)
		}
		if item.OccurredAt != nil {
			line += " | Data: " + item.OccurredAt.In(time.FixedZone("BRT", -3*60*60)).Format("02/01/2006")
		}
		lines = append(lines, line)
	}
	lines = append(lines, "Referencia: "+reference.In(time.FixedZone("BRT", -3*60*60)).Format("02/01/2006 15:04"))
	return strings.Join(lines, "\n")
}

func buildRejectedReply() string {
	return "Certo. Nao vou considerar esse registro. Me envie a correcao em uma unica mensagem."
}

func buildUnregisteredNumberReply() string {
	return "Ainda nao encontrei seu numero vinculado a uma fazenda. Se quiser, responda CADASTRAR para iniciar o cadastro."
}

func buildAmbiguousContextReply() string {
	return "Seu numero esta vinculado a mais de uma fazenda. Me envie o numero da fazenda que deseja usar."
}

func buildSingleContextReply(farmName string) string {
	return fmt.Sprintf("Certo. Seu numero ja esta vinculado a %s.", fallbackFarmName(domain.PhoneContextOption{FarmName: farmName}, 1))
}

func buildAmbiguousContextSelectionReply(options []domain.PhoneContextOption) string {
	var builder strings.Builder
	builder.WriteString("Seu numero esta vinculado a mais de uma fazenda. Me responda com o numero da fazenda:\n")
	for index, option := range options {
		builder.WriteString(fmt.Sprintf("%d. %s", index+1, fallbackFarmName(option, index+1)))
		if index < len(options)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func buildSelectedContextReply(farmName string) string {
	return fmt.Sprintf("Pronto. Vou usar o contexto de %s. Pode enviar a informacao novamente.", fallbackFarmName(domain.PhoneContextOption{FarmName: farmName}, 1))
}

func buildAlreadyRegisteredReply() string {
	return "Seu numero ja esta cadastrado. Pode me enviar registros quando quiser."
}

func buildHealthTreatmentQuestion(state domain.HealthTreatmentState) string {
	switch state.Step {
	case domain.HealthTreatmentStepAwaitingDiagnosisDate:
		return "Certo. Qual foi a data do diagnostico?"
	case domain.HealthTreatmentStepAwaitingMedicine:
		return "Perfeito. Qual foi o medicamento aplicado?"
	case domain.HealthTreatmentStepAwaitingTreatmentDays:
		return "Entendi. Por quantos dias sera o tratamento?"
	default:
		return "Pode me enviar as informacoes do tratamento."
	}
}

func fallbackFarmName(option domain.PhoneContextOption, position int) string {
	if strings.TrimSpace(option.FarmName) != "" {
		return strings.TrimSpace(option.FarmName)
	}

	return fmt.Sprintf("Fazenda %d", position)
}

func buildDraftConfirmationPrompt(event domain.BusinessEvent) string {
	return buildDraftConfirmationPromptFromInterpretation(InterpretationResult{
		Category:    event.Category,
		Subcategory: event.Subcategory,
		Description: event.Description,
		AnimalCode:  event.AnimalCode,
		Amount:      event.Amount,
		Currency:    event.Currency,
		Quantity:    event.Quantity,
		Unit:        event.Unit,
		OccurredAt:  event.OccurredAt,
	})
}

func buildDraftConfirmationPromptFromInterpretation(result InterpretationResult) string {
	lines := []string{
		fmt.Sprintf("Categoria: %s", humanCategoryLabel(result.Category, result.Subcategory)),
	}

	if detail := buildConfirmationDetail(result); detail != "" {
		lines = append(lines, detail)
	}
	if result.Amount != nil {
		lines = append(lines, fmt.Sprintf("Valor: %s", formatCurrency(result.Amount, result.Currency)))
	}
	if result.Quantity != nil {
		lines = append(lines, fmt.Sprintf("Quantidade: %s", formatQuantity(result.Quantity, result.Unit)))
	}
	if occurredAt := formatOccurredAt(result.OccurredAt); occurredAt != "" {
		lines = append(lines, fmt.Sprintf("Data: %s", occurredAt))
	}
	lines = append(lines, "Se estiver tudo certo, responda SIM. Se precisar ajustar algo, responda NAO.")

	return strings.Join(lines, "\n")
}

func humanCategoryLabel(category, subcategory string) string {
	switch {
	case category == "health" && subcategory == "mastitis_treatment":
		return "Saude animal"
	case category == "health" && subcategory == "hoof_treatment":
		return "Saude animal"
	case category == "health" && subcategory == "bloat":
		return "Saude animal"
	case category == "finance" && subcategory == "input_purchase":
		return "Compra de insumos"
	case category == "finance" && subcategory == "expense":
		return "Despesa"
	case category == "finance" && subcategory == "revenue":
		return "Receita"
	case category == "reproduction" && subcategory == "insemination":
		return "Manejo reprodutivo"
	case category == "operations" && subcategory == "note":
		return "Observacao operacional"
	default:
		return "Registro operacional"
	}
}

func buildConfirmationDetail(result InterpretationResult) string {
	description := strings.TrimSpace(result.Description)
	switch {
	case result.Category == "health":
		return buildHealthConfirmationDetail(result, description)
	case result.Category == "finance" && result.Subcategory == "input_purchase" && description != "":
		return fmt.Sprintf("Descricao: %s", description)
	case result.Category == "finance" && result.Subcategory == "expense" && description != "":
		return fmt.Sprintf("Descricao: %s", description)
	case result.Category == "finance" && result.Subcategory == "revenue" && description != "":
		return fmt.Sprintf("Descricao: %s", description)
	case result.Category == "reproduction" && result.Subcategory == "insemination":
		if description == "" {
			return "Evento: inseminacao"
		}
		return fmt.Sprintf("Evento: %s", description)
	case description != "":
		return fmt.Sprintf("Descricao: %s", description)
	default:
		return ""
	}
}

func buildHealthConfirmationDetail(result InterpretationResult, description string) string {
	lines := make([]string, 0, 4)
	if strings.TrimSpace(result.AnimalCode) != "" {
		lines = append(lines, fmt.Sprintf("Animal: %s", result.AnimalCode))
	}
	switch result.Subcategory {
	case "mastitis_treatment":
		lines = append(lines, "Problema: teta/mastite")
	case "hoof_treatment":
		lines = append(lines, "Problema: casco/manqueira")
	case "bloat":
		lines = append(lines, "Problema: gases/timpanismo")
	}
	if result.Attributes != nil {
		if teats := strings.TrimSpace(result.Attributes["affected_teats"]); teats != "" {
			lines = append(lines, fmt.Sprintf("Tetas afetadas: %s", teats))
		}
		if strings.EqualFold(strings.TrimSpace(result.Attributes["milk_withdrawal"]), "true") {
			lines = append(lines, "Restricao: nao tirar leite")
		}
		if diagnosisDate := strings.TrimSpace(result.Attributes["diagnosis_date"]); diagnosisDate != "" {
			lines = append(lines, fmt.Sprintf("Data do diagnostico: %s", diagnosisDate))
		}
		if medicine := strings.TrimSpace(result.Attributes["medicine"]); medicine != "" {
			lines = append(lines, fmt.Sprintf("Medicamento: %s", medicine))
		}
		if treatmentDays := strings.TrimSpace(result.Attributes["treatment_days"]); treatmentDays != "" {
			lines = append(lines, fmt.Sprintf("Dias de tratamento: %s", treatmentDays))
		}
	}
	if description != "" {
		lines = append(lines, fmt.Sprintf("Descricao: %s", description))
	}
	return strings.Join(lines, "\n")
}

func formatCurrency(amount *float64, currency string) string {
	if amount == nil {
		return ""
	}
	if strings.TrimSpace(currency) == "" || strings.EqualFold(currency, "BRL") {
		return fmt.Sprintf("R$ %.2f", *amount)
	}
	return fmt.Sprintf("%s %.2f", strings.ToUpper(strings.TrimSpace(currency)), *amount)
}

func formatQuantity(quantity *float64, unit string) string {
	if quantity == nil {
		return ""
	}
	if strings.TrimSpace(unit) == "" {
		return fmt.Sprintf("%.3g", *quantity)
	}
	return fmt.Sprintf("%.3g %s", *quantity, unit)
}

func formatOccurredAt(occurredAt *time.Time) string {
	if occurredAt == nil {
		return ""
	}
	return occurredAt.In(time.FixedZone("BRT", -3*60*60)).Format("02/01/2006 15:04")
}
