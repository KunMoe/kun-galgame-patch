// GET /api/v1/message — see apps/api/internal/user/model UserMessage.
interface Message {
  id: number
  type: string
  content: string
  status: number
  link: string
  sender_id: number | null
  recipient_id: number | null
  created: string | Date
  updated: string | Date
  sender?: KunUser | null
  // Multilingual name of the referenced patch (favorite / favoriteResource /
  // likeResource), so the game name follows the viewer's 标题优先语言 setting
  // instead of the zh-cn-first name baked into `content`. Absent otherwise.
  galgame_name?: KunLanguage
}
