export const useKunCopy = (originText: string) => {
  const text = decodeIfEncoded(originText)

  navigator.clipboard
    .writeText(text)
    .then(() => {
      useKunMessage(`${text} 复制成功`, 'success')
    })
    .catch(() => {
      useKunMessage(`${text} 复制失败! 请更换更现代的浏览器!`, 'error')
    })
}
