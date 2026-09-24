package handler

import (
	"kun-galgame-patch-api/pkg/communityclient"
	"kun-galgame-patch-api/pkg/errors"
	"kun-galgame-patch-api/pkg/response"
	"kun-galgame-patch-api/pkg/upstream"

	"github.com/gofiber/fiber/v3"
)

type refusalWording struct {
	notFound string
	conflict string
}

var (
	onRead   = refusalWording{}
	onCreate = refusalWording{notFound: "回复的评论不存在或已被删除", conflict: "该评论区已关闭"}
	onEdit   = refusalWording{notFound: "评论不存在或已被删除", conflict: "该评论已被删除或正在审核，暂时无法编辑"}
	onPost   = refusalWording{notFound: "评论不存在或已被删除"}
)

func (w refusalWording) of(err error) string {
	switch upstream.KindOf(err) {
	case upstream.NotFound:
		return w.notFound
	case upstream.Conflict:
		if communityclient.RefusalOf(err) == communityclient.RefusalKeyReused {
			return "这次提交的内容与重试前不一致，请重新发布"
		}
		return w.conflict
	case upstream.Rejected:
		switch communityclient.RefusalOf(err) {
		case communityclient.RefusalNotAuthor:
			return "只能编辑或删除自己的评论"
		case communityclient.RefusalContentBlocked:
			return "评论包含违禁词，请修改后再发布"
		case communityclient.RefusalSandbox:
			return "新用户的评论中链接、图片或提及过多，请减少后再发布"
		}
		return "当前账号暂时无法进行此操作"
	}
	return ""
}

func fail(c fiber.Ctx, err error, w refusalWording) error {
	if appErr, ok := err.(*errors.AppError); ok {
		return response.Error(c, appErr)
	}
	return response.Upstream(c, err, w.of(err))
}
