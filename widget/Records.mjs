export const maxRecordBytes = 2 * 1024 * 1024

export function recordId(filename) {
  const match = /^([a-zA-Z0-9][a-zA-Z0-9_.-]{0,63})\.json$/.exec(filename)
  return match && match[0] === filename ? match[1] : null
}

function object(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value)
}

function label(value, maxLength = 160) {
  return typeof value === "string" ? value.trim().slice(0, maxLength) : ""
}

function number(value) {
  return typeof value === "number" && isFinite(value) && value >= 0 ? value : null
}

function time(value) {
  const parsed = typeof value === "string" ? Date.parse(value) : value
  return number(parsed) !== null && parsed > 0 && parsed <= 253402300799999 ? parsed : null
}

function entries(list, limit, normalize) {
  return Array.isArray(list) ? list.slice(0, limit).filter(object).map(normalize).filter(Boolean) : []
}

function limits(list) {
  return entries(list, 64, item => {
    const percent = number(item.percent)
    if (percent === null || percent > 10000) return null
    return { label: label(item.title) || label(item.label), percent, reset: time(item.resetsAt) }
  })
}

function balance(value) {
  if (!object(value) || number(value.remaining) === null) return null
  return {
    remaining: value.remaining,
    funded: number(value.funded),
    spent: number(value.spent),
    currency: label(value.currency, 12) || "USD",
    estimated: value.estimated === true
  }
}

function details(item, plan, stale, fetched) {
  return {
    plan: label(plan),
    limits: limits(item.limits),
    balance: balance(item.balance),
    resetCredits: object(item.resetCredits) ? number(item.resetCredits.available) : null,
    status: label(item.usageStatusText, 280),
    help: label(item.authHelpText, 500),
    stale: stale === true,
    fetched: time(fetched)
  }
}

export function parse(text, id) {
  if (text.length > maxRecordBytes) return null
  let item
  try {
    item = JSON.parse(text)
  } catch (error) {
    return null
  }
  if (!object(item) || item.schemaVersion !== 1 || item.id !== id) return null

  const record = details(item, item.tierLabel, item.limitsStale === true || item.stale === true, item.limitsFetchedAt)
  record.id = id
  record.name = label(item.name) || id
  record.updated = time(item.updatedAt)
  record.hasActivity = item.hasLocalStats === true || number(item.activeDays) > 0
    || number(item.totalSessions) > 0 || number(item.totalPrompts) > 0
  record.todayTokens = number(item.todayTotalTokens)
  record.todayPrompts = item.hasPromptStats === false ? null : number(item.todayPrompts)
  record.todaySessions = item.hasPromptStats === false ? null : number(item.todaySessions)
  record.days = Object.create(null)
  for (const day of entries(item.recentDays, 31, day => /^\d{4}-\d{2}-\d{2}$/.test(day.date)
    && number(day.messageCount) !== null ? day : null)) {
    // The version 1 contract calls this messageCount, but the unit is tokens.
    record.days[day.date] = day.messageCount
    record.hasActivity = record.hasActivity || day.messageCount > 0
  }
  record.hasActivity = record.hasActivity || record.todayTokens > 0 || record.todayPrompts > 0 || record.todaySessions > 0
  record.accounts = entries(item.accounts, 32, account => {
    const name = label(account.label) || label(account.id)
    if (!name) return null
    const result = details(account, account.plan, account.stale, account.fetchedAt)
    result.name = name
    result.email = label(account.email, 254)
    result.active = account.active === true
    return result
  })
  const available = item.ready === true || record.hasActivity || record.limits.length || record.balance
    || record.accounts.length || record.plan || record.status || record.help || record.stale || record.resetCredits > 0
  return { record: available ? record : null }
}

function dayKey(date) {
  return String(date.getFullYear()).padStart(4, "0") + "-" + String(date.getMonth() + 1).padStart(2, "0")
    + "-" + String(date.getDate()).padStart(2, "0")
}

export function history(record, now) {
  const today = new Date(now)
  const includeToday = record.updated !== null && dayKey(new Date(record.updated)) === dayKey(today)
  const days = []
  for (let offset = 6; offset >= 0; offset--) {
    const date = new Date(today.getFullYear(), today.getMonth(), today.getDate() - offset)
    const tokens = record.days[dayKey(date)] ?? null
    days.push({ date, tokens: offset === 0 && includeToday ? record.todayTokens ?? tokens : tokens })
  }
  return days
}

export function countdown(reset, now) {
  if (reset === null) return null
  const seconds = Math.max(0, Math.ceil((reset - now) / 1000))
  return {
    seconds,
    minutes: Math.floor(seconds / 60) % 60,
    hours: Math.floor(seconds / 3600) % 24,
    days: Math.floor(seconds / 86400)
  }
}
