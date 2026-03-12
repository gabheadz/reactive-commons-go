# Contract: Wire Format Reference

**Feature**: 001-go-core-messaging
**Date**: 2026-03-11

Exact wire format shared between `reactive-commons-go` and `reactive-commons-java`.
Any deviation from this contract breaks Java↔Go interoperability.

---

## AMQP Exchange Declarations

| Exchange Name    | Type   | Durable | Auto-Delete | Arguments |
|------------------|--------|---------|-------------|-----------|
| `domainEvents`   | topic  | true    | false       | none      |
| `directMessages` | direct | true    | false       | none      |
| `globalReply`    | direct | false   | false       | none      |

---

## AMQP Queue Declarations

| Queue                           | Durable | Exclusive | Auto-Delete | x-queue-type | DLQ Exchange               |
|---------------------------------|---------|-----------|-------------|--------------|----------------------------|
| `{appName}.subsEvents`          | true    | false     | false       | configurable | `domainEvents.DLQ` (opt.)  |
| `{appName}` (commands)          | true    | false     | false       | configurable | `directMessages.DLQ` (opt.)|
| `{appName}.query`               | true    | false     | false       | configurable | none                       |
| `{appName}.notification.{uuid}` | false   | true      | true        | classic      | none                       |
| `{appName}.replies.{uuid}`      | false   | true      | true        | classic      | none                       |

`{uuid}` = URL-safe base64 of a random UUID generated once per application startup.

---

## Queue Bindings

| Queue                           | Exchange          | Routing Key                     |
|---------------------------------|-------------------|---------------------------------|
| `{appName}.subsEvents`          | `domainEvents`    | each registered event name      |
| `{appName}` (commands)          | `directMessages`  | `{appName}`                     |
| `{appName}.query`               | `directMessages`  | `{appName}.query`               |
| `{appName}.notification.{uuid}` | `domainEvents`    | each registered notification name |
| `{appName}.replies.{uuid}`      | `globalReply`     | `{appName}.replies.{uuid}`      |

---

## AMQP Basic Properties (per message type)

| Property         | Events | Commands | Query Req | Query Reply | Notifications |
|------------------|--------|----------|-----------|-------------|---------------|
| `content-type`   | `application/json` | same | same | same | same |
| `delivery-mode`  | 2      | 2        | 1         | 1           | 1             |
| `app-id`         | source app name | same | same | same | same |
| `message-id`     | UUID (no dashes) | same | same | same | same |
| `timestamp`      | Unix ms | same | same | same | same |

---

## AMQP Headers

### All messages

| Header Key          | Value                 |
|---------------------|-----------------------|
| `sourceApplication` | publishing app name   |

### Query request (additional headers)

| Header Key                | Value                                              |
|---------------------------|----------------------------------------------------|
| `x-reply_id`              | reply queue name (e.g., `order-svc.replies.abc123`) |
| `x-correlation-id`        | UUID (no dashes) unique per request               |
| `x-serveQuery-id`         | query resource name (e.g., `get-product`)         |
| `x-reply-timeout-millis`  | timeout in milliseconds as string (e.g., `"15000"`) |

### Query reply (additional headers)

| Header Key         | Value                                  |
|--------------------|----------------------------------------|
| `x-correlation-id` | same UUID from the request             |
| `x-empty-completion` | `"true"` only when response is nil  |

---

## Message Body JSON

### Domain Event

```
POST to exchange: domainEvents
Routing key:      <event-name>

Body:
{
  "name":    "<event-name>",
  "eventId": "<uuid>",
  "data":    <application JSON payload>
}
```

### Command

```
POST to exchange: directMessages
Routing key:      <targetService>

Body:
{
  "name":      "<command-name>",
  "commandId": "<uuid>",
  "data":      <application JSON payload>
}
```

### Query Request

```
POST to exchange: directMessages
Routing key:      <targetService>.query

Body:
{
  "resource":  "<query-name>",
  "queryData": <application JSON payload>
}
```

### Query Reply

```
POST to exchange: globalReply
Routing key:      <value of x-reply_id from request>

Body: <raw JSON response>  OR  null (when x-empty-completion=true)
```

### Notification

```
POST to exchange: domainEvents
Routing key:      <notification-name>

Body:
{
  "name":    "<notification-name>",
  "eventId": "<uuid>",
  "data":    <application JSON payload>
}
```

---

## Interoperability Fixture Test Data

The following JSON bytes are valid Java-produced messages. Integration tests MUST assert
that the Go consumer deserializes each correctly.

### Java Domain Event fixture
```json
{"name":"order.created","eventId":"d290f1ee-6c54-4b01-90e6-d701748f0851","data":{"orderId":"42","amount":100}}
```

### Java Command fixture
```json
{"name":"send-invoice","commandId":"a3bb189e-8bf9-3888-9912-ace4e6543002","data":{"invoiceId":"INV-001"}}
```

### Java Query fixture
```json
{"resource":"get-product","queryData":{"productId":"SKU-999"}}
```
