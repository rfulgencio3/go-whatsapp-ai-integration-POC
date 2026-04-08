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
		return fmt.Sprintf("Registro confirmado: compra de insumos de R$ %.2f, %.3g %s.", *event.Amount, *event.Quantity, event.Unit)
	case event.Category == "finance" && event.Subcategory == "expense" && event.Amount != nil:
		return fmt.Sprintf("Registro confirmado: despesa de R$ %.2f.", *event.Amount)
	case event.Category == "finance" && event.Subcategory == "revenue" && event.Amount != nil:
		return fmt.Sprintf("Registro confirmado: receita de R$ %.2f.", *event.Amount)
	case event.Category == "reproduction" && event.Subcategory == "insemination":
		return "Registro confirmado: evento de inseminacao salvo."
	default:
		return "Registro confirmado com sucesso."
	}
}

func buildHealthExpenseCorrelationPrompt(event domain.BusinessEvent) string {
	return buildConfirmedReply(event) + "\nVoce deseja lancar tambem os gastos com medicamento, consulta veterinaria e exames? Responda SIM ou NAO."
}

func buildCorrelatedExpenseQuestion(state domain.CorrelatedExpenseState) string {
	switch state.Step {
	case domain.CorrelatedExpenseStepAwaitingMedicineAmount:
		return "Qual o valor gasto com medicamento? Se nao houve, responda 0."
	case domain.CorrelatedExpenseStepAwaitingVetAmount:
		return "Qual o valor da consulta veterinaria? Se nao houve, responda 0."
	case domain.CorrelatedExpenseStepAwaitingExamAmount:
		return "Qual o valor de exames? Se nao houve, responda 0."
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

func buildRejectedReply() string {
	return "Entendi. Nao vou considerar esse registro. Envie a correcao em uma unica mensagem."
}

func buildUnregisteredNumberReply() string {
	return "Seu numero ainda nao esta vinculado a uma fazenda. Peça o cadastro do seu telefone para continuar."
}

func buildAmbiguousContextReply() string {
	return "Seu numero esta vinculado a mais de uma fazenda. Ajuste o cadastro antes de continuar."
}

func buildSingleContextReply(farmName string) string {
	return fmt.Sprintf("Seu numero ja esta vinculado a %s.", fallbackFarmName(domain.PhoneContextOption{FarmName: farmName}, 1))
}

func buildAmbiguousContextSelectionReply(options []domain.PhoneContextOption) string {
	var builder strings.Builder
	builder.WriteString("Seu numero esta vinculado a mais de uma fazenda. Responda com o numero:\n")
	for index, option := range options {
		builder.WriteString(fmt.Sprintf("%d. %s", index+1, fallbackFarmName(option, index+1)))
		if index < len(options)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func buildSelectedContextReply(farmName string) string {
	return fmt.Sprintf("Contexto definido para %s. Envie a informacao novamente.", fallbackFarmName(domain.PhoneContextOption{FarmName: farmName}, 1))
}

func buildAlreadyRegisteredReply() string {
	return "Seu numero ja esta cadastrado. Pode enviar seus registros normalmente."
}

func buildHealthTreatmentQuestion(state domain.HealthTreatmentState) string {
	switch state.Step {
	case domain.HealthTreatmentStepAwaitingDiagnosisDate:
		return "Qual a data do diagnostico?"
	case domain.HealthTreatmentStepAwaitingMedicine:
		return "Qual o medicamento?"
	case domain.HealthTreatmentStepAwaitingTreatmentDays:
		return "Quantos dias de tratamento?"
	default:
		return "Envie as informacoes do tratamento."
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
	lines = append(lines, "Responda SIM para confirmar ou NAO para corrigir.")

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
