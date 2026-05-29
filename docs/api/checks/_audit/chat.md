# Domain: chat

## Summary
11 endpoints audited (all REST, group middleware `auth`; D9: no WebSocket). FE↔BE shapes match the `chat.d.ts` contract field-by-field (verified live: `RoomSummaryView`, `RoomDetail`, enriched `ChatMessage` with `content_html`/`reaction`/`quote_message`). Two real defects found: one HIGH security IDOR (ToggleReaction has no room-membership check — proven live: a non-member reacted to a message inside a PRIVATE room they don't belong to), and one LOW latent validation bug (the `ids` fetch mode 400s without an explicit `limit` because `validate:"min=1"` runs before the handler's `Limit==0` default; FE masks it by always sending `&limit=100`). `PUT /chat/room/:link/seen` is correct but has no FE caller (dead endpoint; `chat_message_seen` table is empty). No shape mismatches. Dropped a suspected room-detail IDOR as false positive: admin user id=2 IS member #183 of room "kun" (verified in DB + via `GET /chat/room`), so membership authz is actually enforced.

## Endpoints

### GET /chat/room — ChatHandler.ListRooms
- verdict: ok
- tested: `curl GET /chat/room` (admin cookie) → `{"code":0,...,"data":[{"id":1,"link":"kun","type":"GROUP","name":"鲲的 Galgame 大汤锅","avatar":"...","last_message":"咕咕咕1","last_message_time":"2026-05-16T01:00:26.209Z","created":...,"updated":...}]}`. Matches `ChatRoomSummary` (id, link, type, name, avatar, last_message, last_message_time, created, updated) exactly. PRIVATE peer override + last-message preview logic verified in handler.go:197-222. Empty-room filtering via EXISTS subquery (repository.go:44-48) confirmed (only the 1 room with live messages returned).

### POST /chat/room — ChatHandler.CreateRoom
- verdict: ok
- tested: Not executed (mutating; creates a group room). Admin-only gate verified statically: handler.go:263 `if !middleware.HasRole(c, "admin") { return ...ErrForbidden() }`. No FE caller exists (grep of app/ finds only the GET). Returns bare `ChatRoom` (model.go:18) which carries `link`; acceptable since nothing consumes the body. Ready curl for human: `curl -X POST -H "$C" -H 'Content-Type: application/json' -d '{"name":"测试群","avatar":""}' http://127.0.0.1:5214/api/v1/chat/room`
- issues:
  - [low][shape] CreateRoom returns the raw `ChatRoom` model, not `RoomSummaryView` like ListRooms. No consumer today so harmless, but inconsistent. EVIDENCE: handler.go:274 — `return response.OK(c, room)`. FIX: none required; if a create-group UI is added, prefer returning a RoomSummaryView for shape parity.

### POST /chat/room/join — ChatHandler.JoinRoom
- verdict: ok
- tested: Not executed (mutating; idempotent AddMember). FE caller `index.vue:28` posts `{ link: 'kun' }`, checks only `res.code===0`. DTO `JoinRoomRequest.Link` (dto.go:13) matches. `AddMember` uses `OnConflict{DoNothing}` (repository.go:86) so re-join is safe/idempotent — matches the FE comment that the button is safe to re-click. Ready curl: `curl -X POST -H "$C" -H 'Content-Type: application/json' -d '{"link":"kun"}' http://127.0.0.1:5214/api/v1/chat/room/join`

### POST /chat/room/private — ChatHandler.StartPrivate
- verdict: ok
- tested: Not executed (mutating; creates a private room row). FE caller `pages/user/[id].vue:88` posts `{ peer_uid: user.value.id }` and reads `res.data.link`. DTO `StartPrivateChatRequest.PeerUID json:"peer_uid"` (dto.go:22) matches. Self-chat blocked twice (handler.go:317 + repository.go:101). Link normalized to `<low>-<high>` so both directions converge (repository.go:104-108); unique index + race fallback (repository.go:135-142) verified. Returns bare `ChatRoom` carrying `link` — FE only needs `.link`. Ready curl: `curl -X POST -H "$C" -H 'Content-Type: application/json' -d '{"peer_uid":30}' http://127.0.0.1:5214/api/v1/chat/room/private`

### GET /chat/room/:link — ChatHandler.GetRoomDetail
- verdict: ok
- tested: `curl GET /chat/room/kun` → `{"code":0,...,"data":{"id":1,"name":...,"link":"kun","avatar":...,"type":"GROUP","last_message_time":...,"created":...,"updated":...,"member":[{"id":1,"role":"OWNER","user_id":30,"chat_room_id":1,"created":...,"updated":...,"user":{"id":30,"name":"雪雪","avatar":...,"avatar_image_hash":"","roles":["admin"]}},...]}}`. Matches `ChatRoomDetail` = `ChatRoomSummary` + `member: ChatRoomMember[]`, and `member[].user` matches `KunUser` ({id,name,avatar,avatar_image_hash?,roles?}) — `PatchUser` json tags (model.go:136-142) line up. Members with no roles correctly omit `roles` (omitempty). Membership enforced via `resolveRoomForMember` (service.go:68-69→180-192): verified non-member gets `房间不存在`/`您不是该房间的成员` (tested below). Note: detail response does NOT include `last_message` — `chat.d.ts:14` documents it as optional/absent on detail, correct.
  Adversarial check: admin user id=2 read room "kun" successfully — NOT an IDOR; DB confirms user 2 is member #183 of room 1 (`GET /chat/room` returns "kun" for user 2; member uids list ends in `...,2`).

### GET /chat/room/:link/message — ChatHandler.ListMessages
- verdict: fix
- tested: Live, all four modes on room "kun":
  - latest `?limit=3` → ascending ids, each msg has `content`, `content_html` (rendered+escaped markdown), `file_url`, `status`, `deleted_at`, `deleted_by_id`, `reply_to_id`, `created`, `updated`, `sender` (KunUser), `reaction` (array or `null`), `quote_message` (present when reply_to_id set, with `{id,sender_name,content(HTML)}`). Matches `ChatMessageItem` field-by-field.
  - `?before=1752&limit=3` → `[1749,1750,1751]` (older page, ascending). Correct.
  - `?after=1748&limit=5` → `[1749,1750,1751,1752]` (newer, ascending). Correct.
  - `?ids=1750,1752&limit=100` → `[1750,1752]` (exact-set refresh, room-scoped). Correct.
  - membership reject on private room user2 isn't in (`/chat/room/30-18635/message`) → `{"code":40000,"message":"房间不存在"}` (resolveRoomForMember). Correct.
- issues:
  - [low][bug] The `ids` fetch mode 400s unless an explicit `limit` is sent. `?ids=1750,1752` (no limit) → `{"code":40000,"message":"Limit length must not be less than 1"}`. Cause: `ParseQueryAndValidate` runs `validate:"min=1,max=100"` on `Limit` (dto.go:41) against the parsed 0 and rejects BEFORE the handler default. EVIDENCE: dto.go:41 — `Limit int query:"limit" validate:"min=1,max=100"`; handler.go:338 — `if q.Limit == 0 { q.Limit = 30 }` (dead code — never reached for 0). The FE always appends `&limit=100` (`[link].vue:178` refreshLoaded, :98/:131/:153) so it works in practice; only a direct/3rd-party `ids`-only call fails. FIX (BE): change the validate tag to `validate:"omitempty,min=1,max=100"` (or `min=0,max=100`) so the handler's `Limit==0` default becomes reachable. Low severity — FE unaffected.
  - [low][shape] `reaction` serializes as JSON `null` (not `[]`) for messages with no reactions (handler only assigns `msgs[i].Reaction` inside the `len(reactions)>0` branch, handler.go:101). `chat.d.ts:63` types it `reaction: ChatMessageReactionItem[]` (non-null) but the FE guards `m.reaction && m.reaction.length` ([link].vue:481), so no runtime break. EVIDENCE: live latest-page response shows `"reaction":null` for id 1751. FIX (optional, FE-honesty): either init `Reaction = []` in enrichMessages or change the type to `ChatMessageReactionItem[] | null`. Not a defect; documenting for contract accuracy.

### POST /chat/room/:link/message — ChatHandler.CreateMessage
- verdict: ok
- tested: Not executed (mutating; would post a real message). FE caller `[link].vue:222` posts `{ content, reply_to_id? }`; sticker path posts `content="![sticker](url)"` (:251). DTO `CreateMessageRequest` (dto.go:45-49): `content` (max 2000), `file_url` (optional), `reply_to_id *int omitempty`. Empty content+empty file rejected in service (service.go:115). Returns a single enriched `ChatMessage` via `attachOneSender`+`enrichMessages` (handler.go:393-396) — same shape the FE appends to its list. `last_message_time` updated in same tx (repository.go:244-245). Ready curl: `curl -X POST -H "$C" -H 'Content-Type: application/json' -d '{"content":"audit test"}' http://127.0.0.1:5214/api/v1/chat/room/kun/message`

### PUT /chat/message/:id — ChatHandler.UpdateMessage
- verdict: ok
- tested: Not executed (mutating; edits a message). Authz verified static: only sender may edit (service.go:138 `if m.SenderID != userID`), deleted messages can't be edited (service.go:141). Edit history row written in same tx before content update (repository.go:257-269); confirmed `chat_message_edit_history` has 25 rows in DB. Returns `OKMessage("消息已编辑")` — FE `saveEdit` ([link].vue:308) only checks `res.code===0` then `refreshLoaded()`. No IDOR: a different user editing another's message gets `仅发送者可以编辑消息`. Ready curl (as message owner): `curl -X PUT -H "$C" -H 'Content-Type: application/json' -d '{"content":"edited"}' http://127.0.0.1:5214/api/v1/chat/message/<your-own-msg-id>`

### DELETE /chat/message/:id — ChatHandler.DeleteMessage
- verdict: ok
- tested: Not executed (irreversible soft-delete). Authz: sender OR admin/moderator (handler.go:425 `HasAnyRole("admin","moderator")` → service.go:153). Soft-delete sets status=DELETED, deleted_at, deleted_by_id (repository.go:273-279). `reply_to_id`/`deleted_by_id` FK ON DELETE SET NULL documented (model.go:64-79). Privileged-user deletion of any message (incl. rooms they aren't in) is intentional moderation, not an IDOR. FE `confirmDelete` ([link].vue:324) checks code===0 → refreshLoaded; tombstone rendered from status==='DELETED'. Ready curl: `curl -X DELETE -H "$C" http://127.0.0.1:5214/api/v1/chat/message/<msg-id>`

### POST /chat/message/:id/reaction — ChatHandler.ToggleReaction
- verdict: fix
- tested: Live self-reversing toggle on msg 1750 (a room user2 belongs to): ADD → `{"code":0,"data":{"added":true}}`, REMOVE → `{"added":false}`. Toggle logic correct (repository.go:284-301, unique index idx_user_msg_emoji). Then PROVED the IDOR below. DB recheck: 0 leftover test reactions (cleaned up).
- issues:
  - [high][security/IDOR] ToggleReaction does NOT verify the caller is a member of the message's room — any authenticated user can add/remove reactions on ANY message, including messages inside PRIVATE rooms they are not part of. PROVEN LIVE: msg 523 lives in PRIVATE room 117 (link "21659-21666"), which user id=2 is NOT a member of (verified via SQL `chat_member`); `POST /chat/message/523/reaction {"emoji":"🧪"}` returned `{"code":0,"data":{"added":true}}`, then a second call returned `added:false` (cleaned up). Every other message-scoped path resolves the room and checks membership; this one skips it. EVIDENCE: service.go:161-166 — `func (s *ChatService) ToggleReaction(...) { if _, err := s.repo.GetMessage(messageID); err != nil {...}; return s.repo.ToggleReaction(...) }` (no membership check); contrast service.go:180-192 `resolveRoomForMember` used by every other message op. FIX (BE): in `ToggleReaction`, after `GetMessage`, look up the message's room and call the existing membership check, e.g. `ok, _ := s.repo.IsMember(userID, m.ChatRoomID); if !ok { return false, fmt.Errorf("您不是该房间的成员") }` before toggling. (Privileged moderation bypass, if desired, can be added explicitly — but reactions need no mod path.)

### PUT /chat/room/:link/seen — ChatHandler.MarkSeen
- verdict: ok
- tested: Not executed (mutating, but harmless). DTO `SeenRequest.MessageIDs json:"message_ids"` (dto.go:63) requires 1..200 positive ids. Membership enforced (service.go:169-173 resolveRoomForMember). Repo filters ids to those actually in the room before insert (repository.go:333-339) — prevents cross-room seen markers; OnConflict DoNothing makes it idempotent. NO FE CALLER: grep of app/ finds no `/seen` post and no `message_ids` usage; `chat_message_seen` table has 0 rows. Endpoint is correct but currently dead/unused. Ready curl: `curl -X PUT -H "$C" -H 'Content-Type: application/json' -d '{"message_ids":[1750,1751]}' http://127.0.0.1:5214/api/v1/chat/room/kun/seen`
- issues:
  - [low][deadcode] Endpoint has no frontend consumer and no read path surfaces seen state, so writing seen markers is a no-op feature. EVIDENCE: `grep -rn "/seen\|message_ids" apps/web/app` → no chat caller; `SELECT count(*) FROM chat_message_seen` → 0. FIX: none required; either wire up read receipts in FE or leave as a documented future hook. Not a defect.

## Cross-cutting
- [info] Reaction enrichment, reply-quote loading, and sender attachment are all batched via `userclient.BriefMapByInt` (handler.go:101-152) — no N+1; verified by the single multi-message response carrying all senders/reactions/quotes. ok.
- [info] DELETED messages get empty `content_html` (handler.go:85) but the FE renders a tombstone keyed on `status==='DELETED'` ([link].vue:411) before reaching the markdown branch — consistent. Quote of a deleted message is replaced with "该消息已删除" (handler.go:138-140). ok.
- [low][consistency] Two endpoints (`POST /chat/room/private`, `POST /chat/room/join`, `POST /chat/room`) return the raw `ChatRoom` model while `GET /chat/room` returns the enriched `RoomSummaryView`. The model lacks `last_message`; harmless today (FE only reads `.link` from create/join responses) but worth noting if those bodies start being consumed.
