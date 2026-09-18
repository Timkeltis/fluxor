/**
 * 订阅名校验（与后端 backend/internal/config/name.go 的 ValidateSubscriptionName 保持一致）。
 *
 * 订阅名会被用作节点文件名与 config.yaml 中 proxy-providers 的键，因此限制为
 * 中文、英文、数字、emoji，不允许空格与任何标点。
 *
 * 前后端双端校验：前端负责即时反馈，后端负责最终防线（前端可被绕过）。
 *
 * 实现说明：必须按「码点」而非 UTF-16 码元判断。emoji 与部分中文位于 BMP 之外，
 * 用单个正则字符类匹配时会被代理对拆成两个码元而误判，故这里显式做区间比较。
 */

/** 订阅名长度上限（按字符计，与后端 MaxSubscriptionNameLength 一致） */
export const MAX_SUBSCRIPTION_NAME_LENGTH = 32

/** 判定单个码点是否允许：中文 / 英文 / 数字 / emoji。 */
function isAllowedCodePoint(cp: number): boolean {
  // 英文与数字
  if (cp >= 0x30 && cp <= 0x39) return true
  if (cp >= 0x41 && cp <= 0x5a) return true
  if (cp >= 0x61 && cp <= 0x7a) return true

  // 中文：CJK 统一表意文字（基本区 + 扩展 A）
  if (cp >= 0x4e00 && cp <= 0x9fff) return true
  if (cp >= 0x3400 && cp <= 0x4dbf) return true

  // emoji：与后端 isEmoji 的区段保持一致
  if (cp >= 0x1f300 && cp <= 0x1faff) return true // 图形与表情（🚀 等）
  if (cp >= 0x1f1e6 && cp <= 0x1f1ff) return true // 区域指示符（国旗）
  if (cp >= 0x2600 && cp <= 0x27bf) return true // 杂项符号与装饰符号
  if (cp >= 0x2190 && cp <= 0x21ff) return true // 箭头
  if (cp >= 0x2b00 && cp <= 0x2bff) return true // 杂项符号与箭头
  if (cp === 0xfe0f) return true // 变体选择符（emoji 呈现）
  if (cp === 0x200d) return true // 零宽连接符（组合 emoji）

  return false
}

export type SubscriptionNameError = 'empty' | 'tooLong' | 'invalidChar' | null

/** 校验订阅名，返回错误类型；null 表示通过。 */
export function validateSubscriptionName(name: string): SubscriptionNameError {
  if (!name.trim()) return 'empty'
  // 首尾空白会导致「看起来一样但实际不同」的名称
  if (name !== name.trim()) return 'invalidChar'

  const chars = [...name]
  if (chars.length > MAX_SUBSCRIPTION_NAME_LENGTH) return 'tooLong'

  for (const ch of chars) {
    if (!isAllowedCodePoint(ch.codePointAt(0)!)) return 'invalidChar'
  }
  return null
}

/**
 * 过滤输入，只保留合法字符（用于输入框实时过滤）。
 *
 * 注意：只做「剔除」，不做长度截断——长度问题由校验提示，静默截断会让用户
 * 困惑于「输入了却没生效」。
 */
export function filterSubscriptionNameInput(value: string): string {
  let out = ''
  for (const ch of value) {
    if (isAllowedCodePoint(ch.codePointAt(0)!)) out += ch
  }
  return out
}
