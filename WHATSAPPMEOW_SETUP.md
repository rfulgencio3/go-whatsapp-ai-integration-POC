# Whatsmeow POC Setup

Este documento descreve como:

1. configurar um numero central para a POC via `whatsmeow`
2. cadastrar o numero do produtor/trabalhador para resolver o contexto da fazenda

## 1. Variaveis de ambiente

Para subir a POC com `whatsmeow`, configure no minimo:

```env
WHATSAPP_CHANNEL_PROVIDER=whatsmeow
DATABASE_URL=postgres://postgres:postgres@localhost:5432/whatsapp_poc?sslmode=disable
WHATSAPPMEOW_STORE_DSN=postgres://postgres:postgres@localhost:5432/whatsapp_poc?sslmode=disable
GEMINI_API_KEY=your-gemini-key
```

Pareamento por QR code:

```env
WHATSAPPMEOW_PAIR_MODE=qr
WHATSAPPMEOW_CLIENT_NAME=Agro WhatsApp POC
```

Pareamento por codigo no telefone:

```env
WHATSAPPMEOW_PAIR_MODE=code
WHATSAPPMEOW_PAIR_PHONE=5511999999999
WHATSAPPMEOW_CLIENT_NAME=Agro WhatsApp POC
```

Observacoes:

- `WHATSAPPMEOW_STORE_DSN` usa o Postgres para persistir a sessao do WhatsApp.
- no primeiro boot sem sessao salva, o processo vai pedir pareamento.
- depois do pareamento, a sessao fica persistida no banco e o numero central volta a conectar sem novo QR.

## 2. Subir a aplicacao

```powershell
$env:WHATSAPP_CHANNEL_PROVIDER="whatsmeow"
$env:DATABASE_URL="postgres://postgres:postgres@localhost:5432/whatsapp_poc?sslmode=disable"
$env:WHATSAPPMEOW_STORE_DSN=$env:DATABASE_URL
$env:WHATSAPPMEOW_PAIR_MODE="qr"
$env:WHATSAPPMEOW_CLIENT_NAME="Agro WhatsApp POC"
$env:GEMINI_API_KEY="your-gemini-key"
go run .
```

Comportamento esperado:

- se nao existir sessao do WhatsApp no banco, a aplicacao vai logar o QR code ou o pairing code
- depois do scan/pairing, o numero central passa a receber e enviar mensagens

## 3. Cadastrar numero do produtor na fazenda

O numero que envia a mensagem e resolvido por `farm_memberships.phone_number`.

Forma recomendada:

```powershell
$env:DATABASE_URL="postgres://postgres:postgres@localhost:5432/whatsapp_poc?sslmode=disable"
go run ./cmd/register-phone --phone 5511999999999 --producer "Joao da Silva" --farm "Fazenda Boa Vista"
```

Fluxo executado pelo comando:

1. cria o produtor
2. cria a fazenda
3. vincula o telefone em `farm_memberships`
4. ativa o contexto do telefone em `phone_context_states`

Se precisar fazer manualmente, o equivalente e:

```sql
INSERT INTO producers (id, name)
VALUES ('11111111-1111-1111-1111-111111111111', 'Fazenda Boa Vista');

INSERT INTO farms (id, producer_id, name, activity_type)
VALUES (
  '22222222-2222-2222-2222-222222222222',
  '11111111-1111-1111-1111-111111111111',
  'Unidade Leite',
  'leite'
);

INSERT INTO farm_memberships (
  id,
  farm_id,
  person_name,
  phone_number,
  role,
  is_primary,
  status,
  verified_at
)
VALUES (
  '33333333-3333-3333-3333-333333333333',
  '22222222-2222-2222-2222-222222222222',
  'Joao da Silva',
  '5511999999999',
  'owner',
  true,
  'active',
  NOW()
);
```

## 4. Regra pratica de identificacao

- o numero central e o numero pareado no `whatsmeow`
- o numero do usuario rural e quem envia a mensagem para esse numero central
- o backend usa `sender_phone_number` para procurar `farm_memberships.phone_number`
- se encontrar exatamente uma fazenda ativa, a conversa entra no contexto dessa fazenda
- se nao encontrar, o ideal e responder com fluxo de onboarding
- se encontrar mais de uma fazenda para o mesmo numero, o ideal e pedir confirmacao da fazenda

## 5. Limites atuais da POC

Hoje a base do adapter `whatsmeow` ja:

- conecta e persiste sessao
- recebe mensagens
- envia texto
- identifica `text`, `image`, `document` e `audio`

Mas ainda faltam:

- resolver o contexto por `farm_memberships` no fluxo principal
- persistir `source_messages`, `transcriptions`, `interpretation_runs` e `business_events` durante o processamento
- integrar transcricao de audio no caminho `whatsmeow`
- tratar imagem/documento como entrada real de negocio
