# Architecture Guidelines

## Objetivo

Este documento define regras de evolucao do `go-whatsapp-ai-integration-POC` para reduzir duplicidade, manter coesao e evitar que casos de uso virem pontos centrais de acoplamento.

## Principios

### 1. Um caso de uso deve ter uma responsabilidade principal

- `usecase` deve orquestrar fluxo de negocio, nao concentrar formatacao, persistencia detalhada, parsing de comando, onboarding, confirmacao e selecao de contexto no mesmo arquivo.
- Quando um fluxo crescer, extraia colaboradores explicitos, por exemplo:
  - `MembershipResolver`
  - `OnboardingFlow`
  - `ConfirmationFlow`
  - `HealthTreatmentFlow`
  - `AgroReplyFormatter`

### 2. Regras de dominio compartilhadas devem existir em um unico lugar

- Normalizacao de telefone, candidatos de lookup e regras brasileiras de nono digito nao devem existir duplicadas em pacotes diferentes.
- Regras cross-cutting devem morar em um modulo compartilhado de dominio ou utilitario bem nomeado.

### 3. Adaptadores devem ser finos

- `adapters` devem traduzir protocolo, transporte e persistencia.
- Decisoes de negocio nao devem ficar em `transport/httpapi`, `channel/whatsmeow` ou repositorios Postgres.
- O adaptador deve entregar objetos de dominio ou DTOs simples ao caso de uso.

### 4. Formatacao de resposta nao deve ficar misturada com orquestracao

- Builders de texto como confirmacoes, mensagens de onboarding e respostas de contexto devem ficar em componentes dedicados.
- Casos de uso devem pedir uma resposta para um formatter/presenter, nao montar texto extenso inline.

### 5. Composition root deve montar dependencias, nao conter regra de negocio

- `internal/app/application.go` deve apenas compor implementacoes e ligar interfaces.
- Quando houver muitos `if cfg.Has...`, prefira extrair factories por area:
  - messaging
  - storage
  - chatbot
  - agro
  - channel

### 6. Persistencia deve encapsular variacoes de lookup

- Repositorios devem esconder detalhes de consulta como variacoes de telefone e joins auxiliares.
- O caso de uso deve pedir "resolver membership por telefone", nao conhecer a estrategia SQL.

### 7. Evolucao de dominio deve preferir atributos extensivos a campos rigidos quando a variacao for alta

- Eventos de saude animal podem compartilhar base comum em `business_events` e detalhamento em `event_attributes`.
- Antes de adicionar muitas colunas especificas, avaliar se o dado pertence ao nucleo do evento ou a um atributo complementar.

## Hotspots atuais

### `internal/usecase/agro/capture_service.go`

- Arquivo concentra onboarding, selecao de fazenda, confirmacao, correcao, persistencia operacional e formatacao.
- Proximo refactor recomendado:
  - extrair `capture/onboarding_flow.go`
  - extrair `capture/membership_flow.go`
  - extrair `capture/confirmation_flow.go`
  - extrair `capture/reply_formatter.go`

### `internal/app/application.go`

- Composition root com muitas decisoes e montagem concreta em um unico metodo.
- Proximo refactor recomendado:
  - factories por area
  - reduzir branching na funcao `New`
  - preservar `Application` apenas como bootstrap

### `internal/domain/agro/model.go` e `internal/domain/chat/message.go`

- Existe duplicidade de normalizacao de telefone.
- Regra compartilhada deve ser consolidada em um unico modulo.

### `ARCHITECTURE.md`

- O documento principal esta desatualizado em relacao ao estado atual do projeto.
- Antes de ampliar a documentacao externa, manter este arquivo alinhado com:
  - `whatsmeow`
  - Postgres como persistencia principal
  - transcricao stateless
  - fluxos de captura agro

## Regra pratica para novas features

Antes de adicionar um novo fluxo, validar:

1. O caso de uso novo cabe em um colaborador proprio?
2. Existe alguma regra igual ja implementada em outro pacote?
3. A resposta textual pode ser delegada a um formatter?
4. O repositorio pode esconder a variacao tecnica sem contaminar o caso de uso?
5. O novo evento pode reutilizar `business_events` + `event_attributes`?

## Quando criar um novo pacote

Criar novo pacote quando houver:

- uma regra de negocio reutilizavel por mais de um fluxo;
- um fluxo conversacional com estado proprio;
- uma formatacao com mais de um template relevante;
- um ponto de integracao com mais de uma implementacao concreta.

Evitar criar pacote novo apenas para mover funcoes pequenas sem identidade clara.
