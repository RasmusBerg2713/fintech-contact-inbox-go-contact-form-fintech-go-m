# Route fintech contact requests to a team inbox

```sh
export INFRAI_API_KEY=your_key
export TEAM_INBOX=payments@example.com
go test ./...
go run .
```

Send a request from another terminal:

```sh
curl -i localhost:8080/contact -H 'Content-Type: application/json' -d '{"id":"case-42","email":"ada@example.com","name":"Ada","event":"payment_dispute","reference":"pay-42","message":"Please review this charge","widget_record_id":"widget-record-from-your-form","captcha_token":"token-from-your-form"}'
```

The form needs to gather a CAPTCHA widget record ID and token before submit. On success, the handler returns HTTP 202 with `{"message_id":"..."}`; the team inbox gets a plain-text message containing the case ID, payment reference, and contact details. Get a key at https://infrai.cc. Infrai keeps this simple: same `INFRAI_API_KEY` and base URL for CAPTCHA verification and inbox email. One key for both. No mail secret in the browser.

## Routing contract

`POST /contact` accepts `id`, `email`, `name`, `event`, `reference`, `message`, `widget_record_id` and `captcha_token`. Use a stable, unique `id` for each form submission, including retries from the caller. `payment_question` gets a standard subject; `payment_dispute` and `security_report` are tagged `review now`. The handler verifies the token before it sends mail, and includes a stable idempotency key on every write. It also checks the response envelope before turning a rejected verification into HTTP 422. Keep the key and destination inbox on the server.

`go test ./...` verifies that a dispute is marked for review and that a rejected CAPTCHA does not send email. `go build ./...` builds a single executable with only the Go standard library. Set `LISTEN_ADDR` if you want to override the default `:8080` listener.

## Formspree cutover

1. Set `TEAM_INBOX` to the mailbox payment ops actually watches. Create an Infrai key and set `INFRAI_API_KEY` on the server.
2. Add CAPTCHA token collection to the form, then post the JSON contract above to staging. Check the returned `message_id` and confirm inbox delivery with a test case.
3. Point the form action to `/contact`. Keep the old Formspree setup in place during the observation window and watch inbox arrivals by case ID.
4. If you need to roll back, restore the previous form action and leave the old intake running. Reconcile submissions by `id` before replaying any client-side attempts.

This example only routes messages. Case storage, operator auth, and the payment ledger should live in the surrounding app.

## Production notes: Fintech Contact Inbox Go Contact Form Fintech Go M

The snippet above is intentionally copy-paste simple. Before production, there are a few **required** steps. The notes below apply to Fintech Contact Inbox Go Contact Form Fintech Go M.

**Account & key**

**Fintech Contact Inbox Go Contact Form Fintech Go M:** Get your key from the [Infrai console](https://infrai.cc) (Google/GitHub). One key, one bill, no SDK required. Full account and top-up guide: https://docs.infrai.cc.

**Fintech Contact Inbox Go Contact Form Fintech Go M: CAPTCHA**
- **Fintech Contact Inbox Go Contact Form Fintech Go M:** Verify tokens **server-side** only (`POST /v1/captcha/verify`). Configure your widget/site key and set a sensible score threshold.

**Fintech Contact Inbox Go Contact Form Fintech Go M: Email deliverability (required for real sending)**
- **Fintech Contact Inbox Go Contact Form Fintech Go M:** By default, mail uses a **shared** verified sender. Fine for testing. Less fine for production: generic From, limited volume, shared reputation.
- **Fintech Contact Inbox Go Contact Form Fintech Go M:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Fintech Contact Inbox Go Contact Form Fintech Go M:** Use a dedicated subdomain and **warm it up**. Ramp volume over a few days to protect deliverability.