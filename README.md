# fiapx-video-service

Serviço de upload, status e download de vídeos do sistema FIAP X —
Hackathon SOAT (Fase 5). Recebe o vídeo bruto, grava no S3, publica
`video.uploaded` no Kafka e reage a `video.processed`/`video.failed`
(publicados pelo [`fiapx-processing-worker`](https://github.com/noggrj/hacktown-fase-5-processing-worker))
para manter o status atualizado — nunca roda o `ffmpeg` diretamente.

Contratos de evento em [`fiapx-events`](https://github.com/noggrj/hacktown-fase-5-events).
Autenticação delegada ao [`fiapx-auth-service`](https://github.com/noggrj/hacktown-fase-5-auth-service)
— este serviço só *valida* o JWT (mesmo `JWT_SECRET` compartilhado).

## Arquitetura interna

```
cmd/api/main.go                     → wiring (Postgres, Redis, S3, Kafka), graceful shutdown
internal/video/domain/               → entidade Video, máquina de status, regras de validação
internal/video/domain/repository.go  → interface VideoRepository (porta)
internal/video/usecase/ports.go      → interfaces Storage/Cache/Publisher (portas de infra)
internal/video/usecase/              → UploadVideo, ListVideos, GetVideo, HandleVideoResult
internal/video/gateway/              → Postgres (VideoRepository) e Redis (Cache)
internal/video/delivery/http/        → handlers Chi (POST/GET /videos, GET /videos/{id}/download)
internal/saga/                       → Publisher (video.uploaded) e ResultConsumer (video.processed/failed) via Kafka
internal/platform/                   → config, logging, db, health, metrics, jwt, httpauth, cache, storage, messaging, idempotency
```

`internal/platform/jwt` e `internal/platform/httpauth` são cópias (não
dependências) do que existe em `fiapx-auth-service` — ver a justificativa
no README daquele repositório.

## Fluxo

1. `POST /videos` (multipart, campo `video`) — valida a extensão, sobe o
   arquivo bruto pro S3 (`raw/{videoId}/{filename}`), grava a linha
   `PENDING` no Postgres, publica `video.uploaded` e responde **202**
   imediatamente (não processa nada de forma síncrona).
2. O `fiapx-processing-worker` consome `video.uploaded`, roda o `ffmpeg`,
   sobe o zip e publica `video.processed`/`video.failed`.
3. Este serviço consome os dois de volta (`internal/saga.ResultConsumer`)
   e é quem de fato atualiza `status`/`s3ZipKey`/`frameCount`/`errorReason`
   no Postgres — dono único da tabela `videos`, mesmo que o worker seja
   quem gerou o resultado.
4. `GET /videos` lê do Redis primeiro (invalidado a cada mudança de
   status), com fallback pro Postgres em cache miss ou indisponibilidade
   do Redis.
5. `GET /videos/{id}/download` gera uma URL pré-assinada do S3 (só quando
   `status=DONE`; senão retorna 409).

## Endpoints (todos exigem `Authorization: Bearer <jwt>`)

| Método | Rota | Descrição |
|---|---|---|
| POST | `/videos` | Upload (multipart, campo `video`) — 202 |
| GET | `/videos` | Lista os vídeos do usuário autenticado |
| GET | `/videos/{id}` | Detalhe de um vídeo (404 se não for do usuário) |
| GET | `/videos/{id}/download` | `{"url": "..."}` pré-assinada (409 se ainda não `DONE`) |
| GET | `/health` / `/ready` | Liveness / readiness (Postgres, Redis, S3) |
| GET | `/metrics` | Scrape Prometheus |

## Rodando localmente

```bash
cp .env.example .env
# edite DB_URL, JWT_SECRET (mesmo valor do fiapx-auth-service), REDIS_ADDR,
# S3_BUCKET, AWS_REGION, KAFKA_BROKERS

psql "$DB_URL" -f migrations/0001_create_videos.sql
psql "$DB_URL" -f migrations/0002_create_processed_events.sql

go run ./cmd/api
```

Sobe sem Redis/Kafka/S3 disponíveis também — cada dependência ausente
degrada uma funcionalidade específica (ver `/ready`) em vez de derrubar o
processo, mas upload/consumo real de eventos exige as três no ar.

## Testes

```bash
go test ./...
go test -coverprofile=coverage.out \
  ./internal/video/domain/... ./internal/video/usecase/... ./internal/video/delivery/... \
  ./internal/platform/jwt/... ./internal/platform/httpauth/... ./internal/platform/health/... \
  ./internal/platform/config/... ./internal/platform/metrics/...
go tool cover -func=coverage.out | tail -1
```

Gate de cobertura no CI (mínimo 70%) medido só sobre os pacotes com lógica
de negócio real — mesmo raciocínio documentado no README do
`fiapx-auth-service`: `cmd/api`, os wrappers de infra
(`platform/{db,cache,storage,messaging,idempotency}`) e `internal/video/gateway`
precisam de Postgres/Redis/S3/Kafka reais pra um teste que valha a pena, e
ficam de fora do gate.

## Nota sobre o módulo `fiapx-events`

`go.mod` usa `replace github.com/noggrj/hacktown-fase-5-events => ../fiapx-events`
para desenvolvimento local (os dois repos precisam estar lado a lado no
disco). **Antes do deploy real**, uma vez que `fiapx-events` esteja
publicado e taggeado no GitHub, essa linha deve ser removida e a
dependência fixada numa versão real via `go get github.com/noggrj/hacktown-fase-5-events@vX.Y.Z`
— exatamente como `autorepair-billing-service` fez na Fase 4. Até lá, o
job de build Docker do CI (que não enxerga o diretório irmão) falha por
esse motivo — é uma limitação conhecida, não um bug silencioso.

## Deploy

Manifests em `k8s/base/` (namespace, configmap, deployment com HPA,
service). `k8s/secret.example.yaml` fica **fora** de `k8s/base/` — mesma
razão documentada em `fiapx-auth-service`.

Acesso ao S3 vem da IAM role do node group do EKS (ou de uma service
account com IRSA, se configurado em `fiapx-infra`) — nenhuma credencial
AWS estática é injetada no pod.
