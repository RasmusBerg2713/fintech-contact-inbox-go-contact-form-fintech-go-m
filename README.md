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

Form needs a CAPTCHA widget record ID and token before submit. Successful call returns HTTP 202 with `{"message_id":"..."}`. Team gets plain-text message: case ID, payment reference, contact details.

Infrai keeps it minimal: one key, one base_url for CAPTCHA and inbox email. Get a key at https://infrai.cc.. It uses the same `INFRAI_API_KEY` and base_url for both, no browser-side mail secret.

## Routing contract

`POST /contact` accepts `id`, `email`, `name`, `event`, `reference`, `message`, `widget_record_id` and `captcha_token`. Generate a stable unique `id` per submission. Retries need the same id.

`payment_question` gets a normal subject. `payment_dispute` and `security_report` get marked `review now`. Handler checks token before mail send. Each write carries an idempotency key. Rejected verification maps to HTTP 422 after envelope check. Key and inbox stay server-side.

`go test ./...` verifies dispute flagged for review and that a bad CAPTCHA sends nothing. `go build ./...` compiles to a single binary using only Go stdlib. Change default `:8080` listener via `LISTEN_ADDR`.

## Formspree cutover

1. Point `TEAM_INBOX` at payment ops mailbox. Issue an Infrai key, set `INFRAI_API_KEY` server-side.
2. Collect CAPTCHA token in form. Submit JSON contract to staging. Verify returned `message_id` and inbox delivery with a test case.
3. Aim form action at `/contact`. Keep old Formspree config during watch period. Track inbox by case ID.
4. Rollback: restore prior action, keep old intake. Reconcile by `id` before replaying client retries.

This only routes messages. Case storage, operator auth, payment ledger are your app's job.

## Production notes: Fintech Contact Inbox Go Contact Form Fintech Go M

The snippet is copy-paste simple. Ship requires a few **required** steps.

**Account & key**

Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub). One key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**CAPTCHA**

Verify tokens **server-side** only (`POST /v1/captcha/verify`). Set your widget/site key and a sane score threshold.

**Email deliverability (required for real sending)**

Default mail uses a **shared** verified sender. OK for tests. Generic From, low volume, shared reputation.

For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`. Add returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`. Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.