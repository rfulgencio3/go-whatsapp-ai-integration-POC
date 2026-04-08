# User Scenarios

These Gherkin scenarios describe what the producer or farm user experiences in WhatsApp.

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
    And the system should ask the user to resend the business message
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
    Then the system should send a structured confirmation summary
    And the system should wait for "SIM" or "NAO"

  Scenario: User confirms a financial draft
    Given the latest pending confirmation event is a draft input purchase
    When the user sends "SIM"
    Then the system should reply with a saved summary
    And the system should include the top 5 most recent events from the same category and subcategory

  Scenario: User rejects a financial draft
    Given the latest pending confirmation event is a draft financial event
    When the user sends "NAO"
    Then the system should ask the user to send the correction in a single message
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
    Then the system should continue the normal confirmation flow
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
    Then the system should ask for the diagnosis date

  Scenario: User completes the health treatment guided flow
    Given a health treatment state exists for the phone
    And the flow is waiting for the diagnosis date
    When the user sends "Hoje"
    Then the system should ask for the medicine
    When the user sends "Mastclin"
    Then the system should ask for the treatment duration
    When the user sends "5 dias"
    Then the system should send a structured confirmation summary
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
    Then the system should reply with the saved summary
    And the system should ask whether correlated expenses should also be recorded

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
    Then the system should reply with a summary of the recorded costs

  Scenario: User declines correlated expense capture
    Given a correlated expense state exists for the phone
    And the state is waiting for the decision
    When the user sends "NAO"
    Then the system should confirm that related costs were skipped
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
    Then the system should show the expected calving date in the confirmation draft

  Scenario: Confirmed insemination summary still shows expected calving date
    Given the latest pending confirmation event is a draft insemination event
    When the user sends "SIM"
    Then the saved summary should include the expected calving date
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
    Then the system should instruct the user to send "CADASTRAR VACA 32"

  Scenario: Confirming insemination for an existing cow
    Given the latest pending confirmation event is a draft insemination event for animal "32"
    And the animal registry contains an active animal "32" for the active farm
    When the user sends "SIM"
    Then the system should confirm the insemination event
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
    Then the system should reply that cow 32 was registered

  Scenario: Register a calf with mother and birth date
    Given the phone number is linked to an active farm
    When the user sends "cadastrar bezerra 45 filha da vaca 32 nascida em 08/04/2026"
    Then the system should reply with the registered calf summary
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

  Scenario: Query recent health treatments
    Given confirmed health events exist
    When the user sends "Quais foram os ultimos tratamentos?"
    Then the system should list the most recent health treatments

  Scenario: Query medicine expenses for the current month
    Given confirmed finance expense events exist with "expense_type=medicine"
    When the user sends "Quanto gastei com medicamento esse mes?"
    Then the system should reply with the total amount

  Scenario: Query veterinarian expenses for the current month
    Given confirmed finance expense events exist with "expense_type=vet_consultation"
    When the user sends "Quanto gastei com veterinario esse mes?"
    Then the system should reply with the total amount

  Scenario: Query recent input purchases
    Given confirmed input purchase events exist
    When the user sends "Quais foram as ultimas compras?"
    Then the system should list the most recent confirmed purchases
```
