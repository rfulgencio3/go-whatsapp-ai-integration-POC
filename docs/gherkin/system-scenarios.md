# System Scenarios

These Gherkin scenarios describe internal behavior, persistence, integrations, and resilience rules.

## Feature: Membership and context resolution

```gherkin
Feature: Resolve farm membership from inbound phone
  As the capture system
  I want to resolve the active farm from the inbound phone number
  So that every message is processed under the correct farm context

  Scenario: Membership lookup succeeds
    Given the inbound phone number matches one active farm membership
    When the system resolves the phone
    Then the system should load the membership
    And the system should continue the flow under that farm

  Scenario: Membership lookup is ambiguous
    Given the inbound phone number matches more than one active farm membership
    When the system resolves the phone
    Then the system should persist pending context options
    And the system should stop the business flow until the user selects one option
```

## Feature: Source message persistence

```gherkin
Feature: Persist inbound source messages
  As the capture system
  I want every processed inbound message to be stored
  So that the audit trail is preserved

  Scenario: Text message is stored as a source message
    Given an inbound text message arrives for a resolved farm context
    When the message is processed
    Then the system should create a source message record
    And the record should keep provider, provider message id, sender phone, and raw text

  Scenario: Audio message is stored with media metadata
    Given an inbound audio message arrives
    When the message is processed
    Then the system should create a source message record
    And the record should keep media url, content type, and filename when available
```

## Feature: Transcription persistence

```gherkin
Feature: Persist transcriptions before interpretation
  As the capture system
  I want audio transcriptions stored independently from the original source message
  So that downstream interpretation works over text

  Scenario: Audio message produces a transcription record
    Given the inbound audio already has transcription text and metadata
    When the system persists the inbound message
    Then the system should create a transcription linked to the source message
    And the system should keep provider reference, language, and duration
```

## Feature: Deterministic interpretation

```gherkin
Feature: Interpret common agro intents without AI
  As the capture system
  I want deterministic classification for common user messages
  So that the main flows do not depend on external AI for routine operations

  Scenario: Input purchase is classified deterministically
    Given the inbound text is "Comprei 10 sacos de racao por 850 reais"
    When the rule-based interpreter runs
    Then the normalized intent should be "finance.input_purchase"
    And amount, quantity, and unit should be extracted

  Scenario: Health treatment is classified deterministically
    Given the inbound text describes mastitis, hoof treatment, or bloat
    When the rule-based interpreter runs
    Then the normalized intent should be a "health.*" intent
    And health attributes should be extracted when available

  Scenario: Insemination is classified deterministically
    Given the inbound text contains "insemin"
    When the rule-based interpreter runs
    Then the normalized intent should be "reproduction.insemination"
    And the animal code should be extracted when present
```

## Feature: Insemination expected calving date

```gherkin
Feature: Compute expected calving date for insemination
  As the capture system
  I want insemination events to include a calving forecast
  So that the reproductive timeline is available for later queries

  Scenario: Expected calving date is derived from the insemination date
    Given the interpreter resolved an insemination occurred date
    When the interpreter finalizes the interpretation result
    Then it should calculate expected calving date as occurred date plus 283 days
    And it should store the result in interpretation attributes

  Scenario: Expected calving date survives draft creation
    Given a draft insemination business event is created
    When event attributes are persisted
    Then the expected calving date should be saved with the business event
```

## Feature: Draft business event persistence

```gherkin
Feature: Persist interpreted drafts before user confirmation
  As the capture system
  I want interpreted records stored as draft business events
  So that the user can confirm or reject them later

  Scenario: Interpreted text creates a draft business event
    Given interpretation returned a normalized intent and extracted fields
    When the system persists the interpretation result
    Then it should create an interpretation run
    And it should create a draft business event
    And it should create event attributes when extracted attributes exist
    And it should store the pending confirmation pointer in the conversation
```

## Feature: Confirmation and correction lifecycle

```gherkin
Feature: Confirm or reject draft business events
  As the capture system
  I want a draft lifecycle for inbound records
  So that only user-approved events become confirmed history

  Scenario: Draft event is confirmed
    Given a conversation has a pending confirmation event id
    And the referenced business event is still draft
    When the user sends "SIM"
    Then the system should mark the event as confirmed
    And the system should clear the pending confirmation pointer

  Scenario: Draft event is rejected
    Given a conversation has a pending confirmation event id
    And the referenced business event is still draft
    When the user sends "NAO"
    Then the system should mark the event as rejected
    And the system should clear the pending confirmation pointer
    And the system should set the pending correction pointer
```

## Feature: Health guided state persistence

```gherkin
Feature: Persist health treatment guided state
  As the capture system
  I want health treatment collection to survive multiple messages
  So that missing fields can be collected step by step

  Scenario: Guided treatment state is created
    Given a health text was classified with animal and issue type
    When no existing health treatment state is found
    Then the system should create a health treatment state
    And the first step should be waiting for diagnosis date or medicine depending on extracted data

  Scenario: Guided treatment state is completed
    Given a health treatment state exists for the phone
    And the final required field has just been received
    When the flow is finalized
    Then the system should create a draft health business event
    And the system should delete the health treatment state
```

## Feature: Correlated expense state persistence

```gherkin
Feature: Persist correlated expense collection state
  As the capture system
  I want optional related cost capture after health confirmation
  So that treatment costs can be recorded incrementally

  Scenario: Correlated expense state is created after health confirmation
    Given a draft health event has just been confirmed
    When the post-confirmation reply is prepared
    Then the system should create a correlated expense state waiting for the decision

  Scenario: Correlated expense state creates finance events
    Given a correlated expense state exists and all required amounts were collected
    When the flow is finalized
    Then the system should create confirmed finance expense events
    And each expense event should reference the health event through attributes
    And the system should delete the correlated expense state
```

## Feature: Animal registry validation for insemination

```gherkin
Feature: Validate insemination animal existence
  As the capture system
  I want insemination confirmation blocked when the animal does not exist
  So that reproductive records are always linked to known animals

  Scenario: Missing animal blocks insemination confirmation
    Given the pending confirmation event is an insemination event with animal code
    And the farm animal repository does not return an active animal
    When the user sends "SIM"
    Then the system should keep the event as draft
    And the system should not clear the pending confirmation pointer
    And the system should send a deterministic registration hint

  Scenario: Existing animal allows insemination confirmation
    Given the pending confirmation event is an insemination event with animal code
    And the farm animal repository returns an active animal
    When the user sends "SIM"
    Then the system should confirm the event
    And the system should update the animal last seen timestamp
```

## Feature: Animal registry enrichment

```gherkin
Feature: Store enriched animal registration data
  As the capture system
  I want farm animal records to keep lifecycle and lineage fields
  So that later reproductive and health flows have richer context

  Scenario: Animal registration stores lifecycle details
    Given the user sends a deterministic animal registration command
    When the command includes animal type, birth date, or mother animal code
    Then the system should upsert those fields into the farm animal record

  Scenario: Vaccination schema is prepared separately from the animal table
    Given the database schema is ensured
    When the schema setup runs
    Then the system should create the animal_vaccinations table
    And vaccination records should remain separate from farm_animals
```

## Feature: Post-confirmation ranking

```gherkin
Feature: Show recent ranking after confirmation
  As the capture system
  I want each confirmation reply to include recent similar events
  So that the user can validate the new record against recent history

  Scenario: Ranking is loaded from confirmed events
    Given a draft business event was just confirmed
    When the system builds the post-confirmation reply
    Then it should query the 5 most recent confirmed events with the same category and subcategory
    And it should order them by descending business date
```

## Feature: Deterministic operational queries

```gherkin
Feature: Query confirmed farm data without AI
  As the capture system
  I want operational WhatsApp queries to be deterministic
  So that frequent lookups are cheap and stable

  Scenario: Active milk withdrawal query
    Given confirmed health events exist with milk withdrawal attributes
    When the query "Quais vacas nao podem tirar leite?" is received
    Then the system should load the relevant confirmed events
    And it should derive active withdrawal windows from treatment data

  Scenario: Monthly medicine and vet expense queries
    Given confirmed finance expense events exist with expense_type attributes
    When the corresponding monthly query is received
    Then the system should sum only the matching expense type within the current month
```

## Feature: Legacy chat persistence

```gherkin
Feature: Persist legacy chat history for user and assistant messages
  As the capture system
  I want both inbound and outbound chat messages mirrored into legacy stores
  So that auditability is preserved while the POC evolves

  Scenario: Deterministic reply is persisted into legacy chat history
    Given the system sends a deterministic reply
    When the interaction is completed
    Then the user message should be appended to the legacy conversation history
    And the assistant reply should be appended to the legacy conversation history
    And both messages should be recorded in the legacy archive
```

## Feature: AI fallback behavior

```gherkin
Feature: Preserve user experience when AI is unavailable
  As the capture system
  I want failures in external AI calls to degrade gracefully
  So that deterministic flows keep working

  Scenario: Gemini quota is exceeded
    Given the chatbot path requires Gemini
    And Gemini returns a 429 quota error
    When the inbound message is processed
    Then the system should not crash the inbound pipeline
    And the system should return a short fallback reply

  Scenario: Deterministic flows bypass Gemini
    Given the inbound message matches onboarding, confirmation, health, animal, help, or query flows
    When the system processes the message
    Then the flow should complete without depending on Gemini
```
