/** 登录页随机标语（与 hashcheckweb 同源，按语言分池） */

export const LOGIN_SLOGANS_ZH = [
  '成功不是终点，奋斗才是永恒的动力。',
  '每一个不曾起舞的日子，都是对生命的辜负。',
  '只有经历过风雨，才能看见最美的彩虹。',
  '坚持梦想，即使路途遥远，也终将到达彼岸。',
  '每一步的努力，都是未来辉煌的基石。',
  '当你无法改变风向时，学会调整自己的风帆。',
  '勇敢面对挑战，才能激发出最强大的潜能。',
  '失败是成功的起点，只要不放弃，你就不会被击倒。',
  '今天的汗水，将成为明天的荣光。',
  '只有相信自己，你才能创造出属于自己的奇迹。',
  '每一个今日的努力，都是明日的成就。',
  '梦想从不嫌你慢，只要你一直在路上。',
  '在逆境中坚持，才能在顺境中收获成功。',
  '你永远不知道自己有多强大，直到强大成为你唯一的选择。',
  '不怕走得慢，只怕停下不前行。',
  '机会只会眷顾那些时刻准备着的人。',
  '相信过程，收获会在不经意间降临。',
  '人生的每一次攀登，都是为了更好地俯瞰风景。',
  '不要等未来，创造现在，你就是未来的掌控者。',
  '即使天空布满乌云，太阳依然在云层之上等待。',
  '不要害怕慢，怕的是从未迈出第一步。',
  '每一次跌倒，都是让你站得更稳的机会。',
  '勇敢面对困境，胜利就在坚持的尽头。',
  '只有不懈的努力，才能把不可能变成可能。',
  '生活的每一份磨砺，都是你未来辉煌的勋章。',
  '愿你脚踏实地，心怀星辰，走向更好的自己。',
  '只要心中有光，再长的黑夜也会迎来黎明。',
  '挫折是成长的阶梯，越过它，视野将更加宽广。',
  '成长的路上没有捷径，唯有坚持和努力才能到达终点。',
  '你有多坚强，决定了你能走多远。'
] as const

export const LOGIN_SLOGANS_EN = [
  'Success is not the destination, but the journey of perseverance.',
  'Every great dream begins with the courage to take the first step.',
  'The harder the battle, the sweeter the victory.',
  'Believe in yourself, and you are halfway to achieving your dreams.',
  'Difficult roads often lead to beautiful destinations.',
  'Every setback is a setup for a greater comeback.',
  'Success comes to those who never give up on their dreams.',
  "Don't watch the clock; do what it does—keep going.",
  'Your only limit is the one you set for yourself.',
  "Strength doesn't come from what you can do, it comes from overcoming what you once thought you couldn't."
] as const

export function pickRandomSlogan (lang: string): string {
  const list = lang.startsWith('zh') ? LOGIN_SLOGANS_ZH : LOGIN_SLOGANS_EN
  return list[Math.floor(Math.random() * list.length)] ?? ''
}
