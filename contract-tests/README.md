# Contract test verso staging

- **Contesto Linear:** TECH-894, sotto TECH-867 (allineare e presidiare il contratto API)
- **Fonte:** [Playbook 9. API versioning](https://app.notion.com/p/3c53cef66bf8814989b1ce6d420fdd28), punto aperto 3

`openapi.yaml` vive qui, il codice che lo implementa in `recivu-ingestion-service`,
e nulla li teneva allineati. Questo modulo Go chiama ogni operazione documentata
una volta, con una chiave **test** (sandbox), e verifica che status code, header
e body rispettino lo schema dichiarato per quell'operazione. Non e' un test
funzionale della piattaforma: e' il test del contratto.

## Cosa controlla, per ogni chiamata

1. L'operazione esiste nella spec (route trovata).
2. La richiesta che il test manda rispetta la spec: un test rosso non e' mai
   colpa del test stesso.
3. Lo status code ricevuto e' documentato per quell'operazione.
4. Header e body corrispondono allo schema di quella risposta.
5. Le chiavi JSON che il servizio restituisce e la spec non documenta vengono
   segnalate con `NOTE` nel log, senza far fallire: e' drift nell'altra
   direzione (additivo per il partner, ma la spec e' incompleta).

Il flusso (`flow_test.go`): company → employee → receipt → trigger sandbox →
stato → PDF → merchant → report → casi negativi (404, 400, 401) → cleanup.
Le sottoprove dipendono da quelle precedenti e girano in sequenza; ognuna e'
nominata per l'operazione, cosi' un fallimento si legge come "GET /receipts/{id}
ha deviato", non "step 7 fallito".

## Effetti su staging

Con la chiave test tutto finisce nello schema sandbox: una company, un
employee, uno scontrino e una delega merchant per esecuzione, attribuiti al
partner "Contract tests (CI)" e cancellati (soft) dove l'API lo consente.
Nessun `webhook_url` e' configurato, quindi niente esce dalla piattaforma.
Le P.IVA sono generate con checksum valido e prefisso `99`, diverse a ogni run.

## Come si lancia

```bash
cd contract-tests
RECIVU_API_KEY=rk_test_... go test ./... -v -count=1
```

`RECIVU_API_BASE_URL` (default `https://staging-api.recivu.it`) punta a un
altro ambiente. Senza `RECIVU_API_KEY` il test **si salta** (skip, non ok):
`TestSpecIsValid` gira comunque, senza rete.

In CI: `.github/workflows/contract-tests.yml`, ogni giorno feriale alle 06:00
UTC, a mano (`workflow_dispatch`) e sulle PR che toccano `openapi.yaml`.

## La chiave

Partner `Contract tests (CI)` su staging, chiave con `environment = 'test'` e
label `api-docs contract-tests (TECH-894)`; il plaintext sta solo nel secret
`STAGING_CONTRACT_TEST_API_KEY` del repo. Per ruotarla: nuova chiave test per
lo stesso partner dal dashboard admin di staging (o `INSERT` in `api_keys`
con `key_hash = sha256(key)` e `key_prefix = key[:12]`, come fa admin-bff),
aggiornare il secret, revocare la vecchia (`revoked_at = now()`).

## Quando e' rosso

Una delle due parti ha torto, e lo decide una persona:

- il servizio ha cambiato comportamento senza PR gemella qui → si apre la PR
  gemella, o si ripristina il comportamento;
- la spec descrive un comportamento che non c'e' mai stato → si corregge la
  spec (voce `Fixed`, documentale);
- l'ambiente e' rotto (es. object storage giu': `POST /receipt` risponde 500
  e le sottoprove che dipendono dallo scontrino si saltano) → non e' drift,
  si rilancia dopo.
