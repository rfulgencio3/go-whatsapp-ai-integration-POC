# Agro WhatsApp Scenarios

This document captures the current user and system behavior of the POC in Gherkin format.

## Feature: Phone registration and onboarding

```gherkin
Feature: Register a new producer phone
  As a rural producer
  I want to register my phone through WhatsApp
  So that I can start sending farm records

  Scenario: Unregistered phone receives onboarding guidance
    Given a phone number is not linked to any farm
    When the user sends "Ola"
    Then the system should reply that the number is not linked yet
    And the system should instruct the user to send "CADASTRAR"

  Scenario: User starts onboarding
    Given a phone number is not linked to any farm
    When the user sends "CADASTRAR"
    Then the system should start the onboarding flow
    And the system should ask for the producer name

  Scenario: User provides producer name during onboarding
    Given onboarding is waiting for the producer name
    When the user sends "Ricardo Fulgencio"
    Then the system should store the producer name in the onboarding state
    And the system should ask for the farm name

  Scenario: User completes onboarding
    Given onboarding is waiting for the farm name
    And the onboarding state already contains the producer name
    When the user sends "Fazenda Pao de Acucar"
    Then the system should create the producer
    And the system should create the farm
    And the system should create the primary farm membership for the phone
    And the system should activate the farm context for the phone
    And the system should confirm that the registration is complete
```

## Feature: Farm context selection

```gherkin
Feature: Switch farm context for a shared phone
  As a user linked to more than one farm
  I want to choose the active farm context
  So that new records are saved to the correct farm

  Scenario: Ambiguous context requires explicit selection
    Given the phone number is linked to more than one active farm
    When the user sends a normal inbound message
    Then the system should not process the business message yet
    And the system should list the available farms with numeric options

  Scenario: User selects one farm from the context list
    Given the phone number has pending farm context options
    When the user sends "2"
    Then the system should activate the selected farm for the phone
    And the system should clear the pending options
    And the system should ask the user to resend the business message

  Scenario: User explicitly requests a context switch
    Given the phone number is already linked to more than one active farm
    When the user sends "trocar fazenda"
    Then the system should clear the current active farm
    And the system should present the numeric list of available farms
```

## Feature: Deterministic help flow

```gherkin
Feature: Provide deterministic help without AI
  As a user
  I want quick guidance inside WhatsApp
  So that I know which commands and examples are supported

  Scenario: Generic help for a registered user
    Given the phone number is linked to a farm
    When the user sends "ajuda"
    Then the system should reply with supported record examples
    And the system should reply with supported query examples

  Scenario: Themed help for treatments
    Given the phone number is linked to a farm
    When the user sends "exemplos de tratamento"
    Then the system should reply with treatment-related examples

  Scenario: Themed help for purchases
    Given the phone number is linked to a farm
    When the user sends "exemplos de compras"
    Then the system should reply with purchase examples

  Scenario: Themed help for queries
    Given the phone number is linked to a farm
    When the user sends "consultas disponiveis"
    Then the system should list the supported operational queries

  Scenario: Contextual help during health treatment flow
    Given the phone number has an active health treatment state
    And the state is waiting for the medicine
    When the user sends "ajuda"
    Then the system should explain that the medicine name is required next
```

## Feature: Finance capture with confirmation

```gherkin
Feature: Register financial events
  As a producer
  I want to send a purchase, expense, or revenue in plain text
  So that the farm financial history is captured through WhatsApp

  Scenario: Input purchase creates a draft confirmation
    Given the phone number is linked to an active farm
    When the user sends "Comprei 10 sacos de racao por 850 reais"
    Then the system should classify the message as "finance.input_purchase"
    And the system should create a draft business event
    And the system should send a structured confirmation summary
    And the system should wait for "SIM" or "NAO"

  Scenario: User confirms a financial draft
    Given the latest pending confirmation event is a draft input purchase
    When the user sends "SIM"
    Then the system should mark the event as confirmed
    And the system should clear the pending confirmation pointer
    And the system should reply with a saved summary
    And the system should include the top 5 most recent events from the same category and subcategory

  Scenario: User rejects a financial draft
    Given the latest pending confirmation event is a draft financial event
    When the user sends "NAO"
    Then the system should mark the event as rejected
    And the system should keep a pending correction pointer
    And the system should ask the user to send the correction in a single message
```

## Feature: Audio transcription before interpretation

```gherkin
Feature: Process audio messages through transcription first
  As a user
  I want to send voice notes instead of typing
  So that the system can still register farm events

  Scenario: Audio message with a successful transcription
    Given the inbound message type is audio
    And the transcription service returns text and metadata
    When the system processes the inbound message
    Then the system should persist the source message
    And the system should persist the transcription
    And the system should interpret the transcribed text instead of raw audio
    And the system should continue the normal confirmation flow
```

## Feature: Health treatment guided flow

```gherkin
Feature: Register guided animal health treatments
  As a producer
  I want the system to ask for missing treatment data
  So that the record is structured before confirmation

  Scenario: User starts a mastitis treatment flow
    Given the phone number is linked to an active farm
    When the user sends "A vaca 32 esta com problema na teta T3 e nao pode tirar leite"
    Then the system should classify the message as "health.mastitis_treatment"
    And the system should create a health treatment state for the phone
    And the system should ask for the diagnosis date

  Scenario: User completes the health treatment guided flow
    Given a health treatment state exists for the phone
    And the state already contains the animal and issue type
    And the flow is waiting for the diagnosis date
    When the user sends "Hoje"
    Then the system should ask for the medicine
    When the user sends "Mastclin"
    Then the system should ask for the treatment duration
    When the user sends "5 dias"
    Then the system should create a draft health event
    And the system should persist health attributes such as diagnosis date, medicine, and treatment days
    And the system should send a structured confirmation summary
    And the system should delete the guided health state

  Scenario: User requests help in the middle of a health flow
    Given a health treatment state exists for the phone
    And the flow is waiting for the treatment duration
    When the user sends "ajuda"
    Then the system should explain that the next expected value is the number of treatment days
```

## Feature: Correlated expenses for a health event

```gherkin
Feature: Register expenses correlated to a health treatment
  As a producer
  I want to optionally record medicine, vet, and exam expenses
  So that treatment costs are linked to the original health event

  Scenario: User confirms a health event and receives correlated expense prompt
    Given the latest pending confirmation event is a draft health event
    When the user sends "SIM"
    Then the system should confirm the health event
    And the system should reply with the saved summary
    And the system should include the top 5 most recent events from the same category and subcategory
    And the system should ask whether correlated expenses should also be recorded
    And the system should create a correlated expense state waiting for the decision

  Scenario: User accepts correlated expense capture
    Given a correlated expense state exists for the phone
    And the state is waiting for the decision
    When the user sends "SIM"
    Then the system should ask for the medicine amount
    When the user sends "120"
    Then the system should ask for the vet amount
    When the user sends "80"
    Then the system should ask for the exam amount
    When the user sends "75"
    Then the system should create confirmed "finance.expense" events
    And each expense event should be linked to the health event through attributes
    And the system should reply with a summary of the recorded costs
    And the system should clear the correlated expense state

  Scenario: User declines correlated expense capture
    Given a correlated expense state exists for the phone
    And the state is waiting for the decision
    When the user sends "NAO"
    Then the system should not create finance expense events
    And the system should clear the correlated expense state
    And the system should confirm that related costs were skipped
```

## Feature: Insemination with expected calving date

```gherkin
Feature: Register insemination with a predicted calving date
  As a dairy producer
  I want insemination records to include a calving forecast
  So that reproductive planning is visible in the same workflow

  Scenario: Insemination draft includes expected calving date
    Given the phone number is linked to an active farm
    When the user sends "A vaca 32 foi inseminada hoje"
    Then the system should classify the message as "reproduction.insemination"
    And the system should resolve the occurred date as today
    And the system should calculate the expected calving date as occurred date plus 283 days
    And the system should persist the expected calving date in event attributes
    And the system should show the expected calving date in the confirmation draft

  Scenario: Confirmed insemination summary still shows expected calving date
    Given the latest pending confirmation event is a draft insemination event
    When the user sends "SIM"
    Then the system should confirm the insemination event
    And the saved summary should include the expected calving date
```

## Feature: Insemination animal validation

```gherkin
Feature: Validate insemination against the farm animal registry
  As a producer
  I want insemination to reference a registered cow
  So that reproductive history is linked to a known animal

  Scenario: Confirming insemination for a missing cow
    Given the latest pending confirmation event is a draft insemination event for animal "32"
    And the animal registry does not contain animal "32" for the active farm
    When the user sends "SIM"
    Then the system should keep the insemination event as draft
    And the system should keep the pending confirmation pointer unchanged
    And the system should instruct the user to send "CADASTRAR VACA 32"

  Scenario: Confirming insemination for an existing cow
    Given the latest pending confirmation event is a draft insemination event for animal "32"
    And the animal registry contains an active animal "32" for the active farm
    When the user sends "SIM"
    Then the system should confirm the insemination event
    And the system should update the animal last seen timestamp
```

## Feature: Animal registration through WhatsApp

```gherkin
Feature: Register animals through deterministic commands
  As a producer
  I want to register cows, heifers, and calves from WhatsApp
  So that reproductive and health events can reference known animals

  Scenario: Register a simple cow
    Given the phone number is linked to an active farm
    When the user sends "cadastrar vaca 32"
    Then the system should create or update farm animal "32"
    And the animal should be active
    And the system should reply that cow 32 was registered

  Scenario: Register a calf with mother and birth date
    Given the phone number is linked to an active farm
    When the user sends "cadastrar bezerra 45 filha da vaca 32 nascida em 08/04/2026"
    Then the system should create or update farm animal "45"
    And the animal type should be "bezerra"
    And the sex should be "female"
    And the mother animal code should be "32"
    And the birth date should be "08/04/2026"
    And the system should reply with the registered calf summary
```

## Feature: Operational business queries

```gherkin
Feature: Query operational farm data through WhatsApp
  As a producer
  I want deterministic queries over confirmed data
  So that I can inspect the latest farm status without using AI

  Scenario: Query active milk withdrawal animals
    Given confirmed health events exist with active milk withdrawal
    When the user sends "Quais vacas nao podem tirar leite?"
    Then the system should list the animals with active milk withdrawal
    And the response should include affected teats when available
    And the response should include the active-until date when available

  Scenario: Query recent health treatments
    Given confirmed health events exist
    When the user sends "Quais foram os ultimos tratamentos?"
    Then the system should list the most recent health treatments
    And the response should include animal code, treatment type, and date

  Scenario: Query medicine expenses for the current month
    Given confirmed finance expense events exist with "expense_type=medicine"
    When the user sends "Quanto gastei com medicamento esse mes?"
    Then the system should sum the current month medicine expenses
    And the system should reply with the total amount

  Scenario: Query veterinarian expenses for the current month
    Given confirmed finance expense events exist with "expense_type=vet_consultation"
    When the user sends "Quanto gastei com veterinario esse mes?"
    Then the system should sum the current month veterinarian expenses
    And the system should reply with the total amount

  Scenario: Query recent input purchases
    Given confirmed input purchase events exist
    When the user sends "Quais foram as ultimas compras?"
    Then the system should list the most recent confirmed purchases
    And the response should include description, amount, quantity, unit, and date when available
```

## Feature: Post-confirmation ranking

```gherkin
Feature: Show a saved summary and recent ranking after confirmation
  As a producer
  I want to understand what was stored and what happened recently in the same category
  So that I can validate the new record against recent history

  Scenario: Confirmation reply shows the saved summary plus recent ranking
    Given a draft business event was just confirmed by the user
    When the system builds the post-confirmation reply
    Then the reply should include "Resumo salvo"
    And the reply should include the structured fields of the confirmed event
    And the reply should include "Top 5 mais recentes dessa categoria"
    And the ranking should be sorted by descending event date
```

## Feature: AI fallback behavior

```gherkin
Feature: Preserve user experience when AI is unavailable
  As a user
  I want the system to fail gracefully
  So that the conversation does not break on temporary model issues

  Scenario: Gemini quota is exceeded
    Given the system needs Gemini for a chatbot path
    And Gemini returns a 429 quota error
    When the inbound message is processed
    Then the system should not crash the inbound pipeline
    And the system should reply with a short temporary fallback message

  Scenario: Deterministic flows still work without Gemini
    Given the inbound message matches a deterministic onboarding, confirmation, health, animal, or query flow
    When the message is processed
    Then the system should resolve the flow without depending on Gemini
```

## Notes

- The scenarios above describe the current implemented behavior of the POC.
- Vaccination storage is already prepared at schema level, but the conversational vaccination flow is not implemented yet.
- Animal lifecycle classification such as calf, heifer, and cow is currently driven by registration data and future domain rules, not by a finished automated lifecycle workflow yet.
