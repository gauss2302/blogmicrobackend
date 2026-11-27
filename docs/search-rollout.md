# Search Service Rollout and Backfill Procedure

This document describes the phased rollout and backfill strategy for the search service and discovery features.

## Prerequisites

- Follow schema deployed (user-service migrations applied).
- OpenSearch and Kafka available in the target environment (see `docker-compose.yml` and env configuration).

## Phase 1: Deploy follow schema and user-service

1. Apply user-service migrations so the `follows` table exists.
2. Deploy user-service with follow APIs (Follow, Unfollow, GetFollowers, GetFollowing, AreFollowed).
3. Verify follow endpoints via gateway (e.g. `POST/GET/DELETE /api/v1/users/:id/follow`, followers/following lists).
4. No frontend or search changes in this phase.

## Phase 2: Deploy search infra and search-service

1. Start OpenSearch and Kafka (e.g. `make infra-up` or equivalent).
2. Deploy search-service with:
   - `OPENSEARCH_URL`, `KAFKA_BROKERS`, `USER_SERVICE_GRPC_ADDR` and related env set.
   - Index bootstrap will create `users_index` and `posts_index` on first run.
3. Ensure search-service health check passes and Kafka consumer group is registered.
   - search-service joins the group only once the broker reports both `search.users` and `search.posts` (joining earlier would leave it with no partitions); with topic auto-creation on, its own startup metadata request creates them. With auto-creation off it logs `kafka topics … not ready` (the pause grows to 30s) until someone creates them, then starts consuming without a restart.
4. Do **not** enable the gateway search route or frontend discovery yet.

## Phase 3: Enable event publishing (backfill feed)

1. **User-service** and **post-service** publish to Kafka topics `search.users` and `search.posts` with the agreed JSON contract (`entity_type`, `event_type`, `entity_id`, `payload`, `timestamp`, `message_id`). The user payload is `id`, `name`, `picture`, `bio` — not email; the post payload is `id`, `user_id`, `title`, `slug`, `content`, `published`.
   - **Transactional outbox.** Triggers on `users` and `posts` add a row to the service's `search_outbox` table in the same transaction as every write that changes what search sees (users: `name`, `picture`, `bio`, `is_active`; posts: `user_id`, `title`, `slug`, `content`, `published`). A relay in each service sends the *current* state of those rows and deletes the outbox rows only once Kafka has accepted them: `updated` for a published post or an active user, `deleted` for a draft, a deleted post or a deleted user. Drafts never reach OpenSearch.
   - Writes never wait for Kafka. During a Kafka outage or a restart the changes wait in `search_outbox` and go out when Kafka is back, without a restart; the service log shows `search outbox: … N pending, retrying in …` meanwhile. Without `KAFKA_BROKERS` no relay runs and the outbox keeps growing until one does.
2. **Post-service** keeps RabbitMQ for notifications; search does not use it.
   - gRPC `UnpublishUserPosts`, meant for account deletion, hides all of a user's posts (rows are kept, `published` becomes false). Their removal from search is queued in the same transaction, so success means it will reach the index. Nothing goes to RabbitMQ; the call is idempotent.
3. **Rebuild** the index from the database: start user-service and/or post-service once with `SEARCH_BACKFILL_ON_START=true`.
   - It queues every row of `users` / `posts` in `search_outbox` in one statement (one snapshot, no paging, so no row is skipped), and the relay sends them like any other change: a failed send stays queued and is retried, it is never counted as done. Drafts and deleted users go out as `deleted`, which also removes drafts that older versions stored whole.
   - The log says `Search rebuild: N posts queued`; the rebuild has reached Kafka when `SELECT count(*) FROM search_outbox` is 0 and the log shows no `pending` errors.
   - **After upgrading to the outbox version, run the rebuild once**: the index may already disagree with the database.
   - The rebuild cannot remove documents whose rows are gone (posts are deleted from the database, so a `deleted` lost before the outbox left an orphan). To make the index equal the database: delete the users and posts indices in OpenSearch (`OPENSEARCH_USERS_INDEX`, `OPENSEARCH_POSTS_INDEX`), restart search-service (it creates them again, see Phase 2), then run the rebuild. Search returns partial results until it drains.
4. Wait for consumer lag to drain (e.g. monitor Kafka consumer group lag). Target freshness: 1–3 minutes for backfill completion.

## Phase 4: Enable gateway search route and frontend

1. Configure api-gateway with `SEARCH_SERVICE_GRPC_ADDR` pointing at search-service.
2. Enable the protected route `GET /api/v1/search` (already wired; ensure gateway is restarted/redeployed with config).
3. Deploy frontend with BFF search proxy (`/api/bff/search`) and discovery UI (Discover tab, cursor-based results, Subscribe/Unsubscribe).
4. Smoke-test: authenticated user runs a search and sees grouped users and posts; load more and follow/unfollow work.

## Phase 5: Deprecate or proxy legacy search (optional)

- If legacy `/public/users/search` and `/public/posts/search` exist, either:
  - Deprecate and redirect to the new combined search with a message, or
  - Proxy them to the new search endpoint and map responses until clients migrate.

## Rollback

- **Frontend/gateway**: Disable Discover tab and/or search route; revert to previous frontend/gateway version.
- **Search-service**: Stop search-service; gateway search will fail (return 503 or equivalent). Optional: feature-flag the search route to turn it off without redeploy.
- **Events**: Stopping Kafka publishing from user/post services does not require rollback of search-service; index will simply stop updating until publishing is restored.

## Health and SLO

- **Search-service**: Health check should reflect OpenSearch and (optionally) Kafka connectivity. If OpenSearch is down, search returns partial results (users_partial / posts_partial) and no top-level error.
- **Indexing failures**: the consumer handles one message at a time and commits it only after it is indexed or parked in the `search.dlq` topic (an event with an unknown `event_type` is skipped and committed). A message OpenSearch rejects (4xx) or that cannot be parsed is parked. Anything transient (network, timeout, 5xx, 429) is retried on the same message with a pause that grows to 30s, without a limit: the partition stalls until OpenSearch is back rather than losing events, so an outage shows up as consumer lag. Nothing replays the DLQ automatically.
- **Freshness**: Target 1–3s from user/post change to searchable. Monitor consumer lag; alert if lag exceeds threshold (e.g. > 1000 messages or > 30s behind).
- **Demotion**: AreFollowed is called per search request; ensure user-service latency and timeouts (e.g. 3s) are acceptable; consider batched lookup if needed.

## Testing checklist before rollout

- [ ] Unit: search cursor encode/decode, build body, demote order (search-service).
- [ ] Unit: follow invariants (no self-follow, idempotent follow/unfollow) (user-service).
- [ ] Unit: gateway search handler (auth required, missing query 400, happy path with mock client).
- [ ] Failure: OpenSearch unavailable returns partial branches and no top-level error (search-service).
- [ ] Integration (optional): Gateway → search-service with real services; Kafka event → OpenSearch document; demotion via user-service.
