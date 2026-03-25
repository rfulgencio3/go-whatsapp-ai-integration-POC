# Privacy Policy

Last updated: March 25, 2026

This Privacy Policy describes how `go-whatsapp-ai-integration-POC` handles data when users interact with the service through WhatsApp and related API endpoints.

## 1. Data We Process

When a user sends a message to the service, we may process:

- phone number or WhatsApp identifier
- message content sent by the user
- message identifiers and delivery metadata
- technical logs and diagnostic information related to requests and processing

## 2. How We Use Data

We process this data to:

- receive and respond to WhatsApp messages
- generate automated answers with AI assistance
- maintain short conversation context for better replies
- monitor system health, reliability, and abuse prevention
- store operational and historical records required for the service

## 3. Third-Party Services

This service may use third-party providers to operate, including:

- Meta / WhatsApp Business Platform for message delivery and webhook events
- Google Gemini for AI-generated responses
- Railway or similar hosting providers for application deployment
- Redis for short-lived operational data such as context, idempotency, and queue processing
- Postgres for persisted chat history and operational records

## 4. Data Retention

Short-lived operational data may be stored temporarily in Redis using expiration policies. Persisted conversation records may be stored in Postgres for audit, troubleshooting, and service continuity purposes. Retention periods may vary based on technical and business needs.

## 5. Security and Access

Reasonable technical measures are used to protect processed data. However, no system can guarantee absolute security, and users should avoid sending unnecessary sensitive personal information through the service.

## 6. User Rights and Contact

If you need information about your data, or want to request deletion or clarification, contact the operator of this service through the business contact channel associated with the WhatsApp number or application.

## 7. Policy Changes

This Privacy Policy may be updated to reflect changes in the service, providers, legal requirements, or operational practices. The latest published version at `/privacy-policy` should be considered the active version.
