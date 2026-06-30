// Types for the Galgame release calendar (发售月表). Backed by the wiki calendar
// API (docs/galgame_wiki/01-galgame.md §发售月历), surfaced via moyu's
// /galgame/calendar[/pending|/tba] endpoints. Ambient (no import/export) to match
// the rest of app/shared/types.

// release_precision marks how release_date should be read (release_date is
// normalized, so the two MUST be read together).
type GalgameReleasePrecision = 'day' | 'month' | 'year' | 'tba' | 'unknown'

// A calendar entry is an enriched GalgameCard plus has_patch: whether moyu holds
// a local patch row for this galgame (drives the card's link — moyu /patch/:id
// when true, the wiki entry page otherwise). release_date / release_precision live
// on the nested `galgame` object.
interface CalendarItem extends GalgameCard {
  has_patch: boolean
  // Whether the logged-in viewer has favorited this game (false for anonymous).
  // Drives the inline 收藏 toggle's initial state on the calendar card.
  is_favorite: boolean
}

// GET /galgame/calendar?month=YYYY-MM
interface CalendarMonthResponse {
  month: string
  today: string
  items: CalendarItem[]
  meta: {
    prev_month: string
    next_month: string
    has_prev: boolean
    has_next: boolean
    min_month: string
    max_month: string
    count: number
  }
}

// GET /galgame/calendar/pending?year=YYYY  and  GET /galgame/calendar/tba
interface CalendarBucketResponse {
  year?: string
  items: CalendarItem[]
}

// One month segment of the 3-month window.
interface CalendarMonthSection {
  month: string
  items: CalendarItem[]
}

// GET /galgame/calendar/window?month=YYYY-MM — a [prev, focus, next] window so a
// sparse focus month still shows neighbouring releases (rendered as a centered
// scroll "wheel"). `month` is the resolved focus; `meta` is the focus month's.
interface CalendarWindowResponse {
  month: string
  today: string
  meta: CalendarMonthResponse['meta']
  months: CalendarMonthSection[]
}
