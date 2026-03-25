package httpapi

import "net/http"

const privacyPolicyHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Privacy Policy | go-whatsapp-ai-integration-POC</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f6f3ec;
      --panel: #fffdf8;
      --ink: #1f2933;
      --muted: #5b6875;
      --line: #d8d2c4;
      --accent: #0d6b5f;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: Georgia, "Times New Roman", serif;
      background: linear-gradient(180deg, #efe8db 0%, var(--bg) 100%);
      color: var(--ink);
      line-height: 1.65;
    }
    main {
      max-width: 860px;
      margin: 0 auto;
      padding: 40px 20px 72px;
    }
    .card {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 18px;
      padding: 32px;
      box-shadow: 0 18px 40px rgba(31, 41, 51, 0.08);
    }
    h1, h2 { line-height: 1.2; }
    h1 {
      margin-top: 0;
      font-size: clamp(2rem, 4vw, 3rem);
      color: var(--accent);
    }
    h2 {
      margin-top: 32px;
      font-size: 1.25rem;
    }
    p, li { font-size: 1rem; }
    .meta {
      color: var(--muted);
      margin-bottom: 24px;
    }
    a { color: var(--accent); }
    ul { padding-left: 22px; }
    code {
      background: #f1ece3;
      padding: 0.12rem 0.35rem;
      border-radius: 6px;
      font-size: 0.95em;
    }
  </style>
</head>
<body>
  <main>
    <section class="card">
      <h1>Privacy Policy</h1>
      <p class="meta">Last updated: March 25, 2026</p>
      <p>This Privacy Policy describes how <strong>go-whatsapp-ai-integration-POC</strong> handles data when users interact with the service through WhatsApp and related API endpoints.</p>

      <h2>1. Data We Process</h2>
      <p>When a user sends a message to the service, we may process:</p>
      <ul>
        <li>phone number or WhatsApp identifier;</li>
        <li>message content sent by the user;</li>
        <li>message identifiers and delivery metadata;</li>
        <li>technical logs and diagnostic information related to requests and processing.</li>
      </ul>

      <h2>2. How We Use Data</h2>
      <p>We process this data to:</p>
      <ul>
        <li>receive and respond to WhatsApp messages;</li>
        <li>generate automated answers with AI assistance;</li>
        <li>maintain short conversation context for better replies;</li>
        <li>monitor system health, reliability, and abuse prevention;</li>
        <li>store operational and historical records required for the service.</li>
      </ul>

      <h2>3. Third-Party Services</h2>
      <p>This service may use third-party providers to operate, including:</p>
      <ul>
        <li>Meta / WhatsApp Business Platform for message delivery and webhook events;</li>
        <li>Google Gemini for AI-generated responses;</li>
        <li>Railway or similar hosting providers for application deployment;</li>
        <li>Redis for short-lived operational data such as context, idempotency, and queue processing;</li>
        <li>Postgres for persisted chat history and operational records.</li>
      </ul>

      <h2>4. Data Retention</h2>
      <p>Short-lived operational data may be stored temporarily in Redis using expiration policies. Persisted conversation records may be stored in Postgres for audit, troubleshooting, and service continuity purposes. Retention periods may vary based on technical and business needs.</p>

      <h2>5. Security and Access</h2>
      <p>Reasonable technical measures are used to protect processed data. However, no system can guarantee absolute security, and users should avoid sending unnecessary sensitive personal information through the service.</p>

      <h2>6. User Rights and Contact</h2>
      <p>If you need information about your data, or want to request deletion or clarification, contact the operator of this service through the business contact channel associated with the WhatsApp number or application.</p>

      <h2>7. Policy Changes</h2>
      <p>This Privacy Policy may be updated to reflect changes in the service, providers, legal requirements, or operational practices. The latest published version at <code>/privacy-policy</code> should be considered the active version.</p>
    </section>
  </main>
</body>
</html>`

func (h *Handler) handlePrivacyPolicy(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(responseWriter, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	responseWriter.WriteHeader(http.StatusOK)
	_, _ = responseWriter.Write([]byte(privacyPolicyHTML))
}
