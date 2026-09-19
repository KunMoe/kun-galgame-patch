package common

import (
	"bytes"
	"encoding/json"

	"github.com/gofiber/fiber/v3"
)

const hikariRetiredMessage = "此接口已下线。请到 https://developer.nextmoe.dev 申请 API Key，" +
	"在服务端改用 GET https://api.nextmoe.dev/v2/moyu/patches?refs=vndb:<vndb_id>&nsfw=true&include=resources ，" +
	"接入文档：https://developer.nextmoe.dev/docs/moyu-patches 。" +
	"This endpoint is retired: get an API key at https://developer.nextmoe.dev and call /v2/moyu from your server instead."

var hikariRetiredBody = func() []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(fiber.Map{"success": false, "message": hikariRetiredMessage, "data": nil})
	return buf.Bytes()
}()

// Partners built https://www.moyu.moe/patch/<patch_id>/resource from this
// answer. After migration 037 patch_id is the catalog work id while /patch/<n>
// still resolves the pre-renumber page number, so on 2026-09-19 8,486 of the
// 11,096 pages it could answer linked to a different game. /v2/moyu hands out
// web_url instead; do not bring the lookup back.
func (h *CommonHandler) HikariRetired(c fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	return c.Status(fiber.StatusGone).Send(hikariRetiredBody)
}
