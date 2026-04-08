# WhatsApp Queries

Consultas operacionais já suportadas no canal WhatsApp da POC.

## Saúde animal

### Vacas com restrição de leite
Exemplos:
- `Quais vacas nao podem tirar leite?`
- `Quais animais estao sem poder tirar leite?`

Resposta esperada:
- lista de animais com restrição ativa
- tetas afetadas, quando houver
- data limite estimada do bloqueio, quando houver `treatment_days`

### Últimos tratamentos
Exemplos:
- `Quais foram os ultimos tratamentos?`
- `Tratamentos recentes`

Resposta esperada:
- últimos tratamentos confirmados de saúde animal
- animal
- tipo de tratamento
- data
- tetas afetadas, quando aplicável

## Financeiro

### Gasto com medicamento no mês
Exemplos:
- `Quanto gastei com medicamento esse mes?`
- `Quanto foi gasto com remedio este mes?`

Resposta esperada:
- soma dos eventos `finance.expense` confirmados com `expense_type=medicine`
- período calculado no mês corrente da fazenda/ambiente

### Gasto com veterinário no mês
Exemplos:
- `Quanto gastei com veterinario esse mes?`
- `Quanto foi gasto com consulta veterinaria neste mes?`

Resposta esperada:
- soma dos eventos `finance.expense` confirmados com `expense_type=vet_consultation`

### Últimas compras
Exemplos:
- `Quais foram as ultimas compras?`
- `Ultimas compras de insumos`

Resposta esperada:
- últimas compras confirmadas de `finance.input_purchase`
- descrição
- valor
- quantidade/unidade quando houver
- data

## Observações de implementação

- As consultas são determinísticas e não dependem de IA.
- O lookup usa apenas eventos confirmados.
- Os fluxos ficam em `internal/usecase/agro/capture_query.go`.
- A identificação das frases de consulta fica em `internal/usecase/agro/capture_workflow.go`.
- A leitura consolidada do Postgres fica em `internal/adapters/storage/postgres/repositories.go`.
