
// Minimal inline SVG icons (no emoji), stroke-based, 20x20, inherits currentColor.
const base = { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 1.8, strokeLinecap: 'round', strokeLinejoin: 'round' }

export const IconDashboard = (p) => (
  <svg {...base} {...p}><rect x="3" y="3" width="7" height="9" rx="1" /><rect x="14" y="3" width="7" height="5" rx="1" /><rect x="14" y="12" width="7" height="9" rx="1" /><rect x="3" y="16" width="7" height="5" rx="1" /></svg>
)
export const IconBudget = (p) => (
  <svg {...base} {...p}><rect x="3" y="5" width="18" height="14" rx="2" /><path d="M3 10h18" /><circle cx="7" cy="14.5" r="1" /></svg>
)
export const IconSpending = (p) => (
  <svg {...base} {...p}>
    <path d="M6 3h12v18l-2.5-1.5L13 21l-2.5-1.5L8 21l-2-1.5V3z" />
    <path d="M9 8h6M9 12h6M9 16h3" />
  </svg>
)
export const IconSummary = (p) => (
  <svg {...base} {...p}><path d="M4 19V9M10 19V5M16 19v-7M20 19H4" /></svg>
)
export const IconAssets = (p) => (
  <svg {...base} {...p}><path d="M3 7l9-4 9 4-9 4-9-4z" /><path d="M3 12l9 4 9-4M3 17l9 4 9-4" /></svg>
)
export const IconSetup = (p) => (
  <svg {...base} {...p}><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.7 1.7 0 00.34 1.87l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.7 1.7 0 00-1.87-.34 1.7 1.7 0 00-1 1.55V21a2 2 0 11-4 0v-.09a1.7 1.7 0 00-1-1.55 1.7 1.7 0 00-1.87.34l-.06.06a2 2 0 11-2.83-2.83l.06-.06a1.7 1.7 0 00.34-1.87 1.7 1.7 0 00-1.55-1H3a2 2 0 110-4h.09a1.7 1.7 0 001.55-1 1.7 1.7 0 00-.34-1.87l-.06-.06a2 2 0 112.83-2.83l.06.06a1.7 1.7 0 001.87.34H9a1.7 1.7 0 001-1.55V3a2 2 0 114 0v.09a1.7 1.7 0 001 1.55 1.7 1.7 0 001.87-.34l.06-.06a2 2 0 112.83 2.83l-.06.06a1.7 1.7 0 00-.34 1.87V9a1.7 1.7 0 001.55 1H21a2 2 0 110 4h-.09a1.7 1.7 0 00-1.55 1z" /></svg>
)
export const IconLogout = (p) => (
  <svg {...base} {...p}><path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4" /><path d="M16 17l5-5-5-5" /><path d="M21 12H9" /></svg>
)
export const IconCollapse = (p) => (
  <svg {...base} {...p}><path d="M11 17l-5-5 5-5M18 17l-5-5 5-5" /></svg>
)
export const IconExpand = (p) => (
  <svg {...base} {...p}><path d="M13 17l5-5-5-5M6 17l5-5-5-5" /></svg>
)
export const IconClose = (p) => (
  <svg {...base} strokeWidth={2.4} {...p}><path d="M18 6L6 18M6 6l12 12" /></svg>
)
export const IconEdit = (p) => (
  <svg {...base} {...p}><path d="M12 20h9" /><path d="M16.5 3.5a2.1 2.1 0 013 3L7 19l-4 1 1-4 12.5-12.5z" /></svg>
)
export const IconSun = (p) => (
  <svg {...base} {...p}><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></svg>
)
export const IconMoon = (p) => (
  <svg {...base} {...p}><path d="M21 12.8A9 9 0 1111.2 3a7 7 0 009.8 9.8z" /></svg>
)
export const IconPlus = (p) => (
  <svg {...base} strokeWidth={2.2} {...p}><path d="M12 5v14M5 12h14" /></svg>
)
export const IconGuide = (p) => (
  <svg {...base} {...p}><path d="M4 19.5A2.5 2.5 0 016.5 17H20" /><path d="M6.5 2H20v20H6.5A2.5 2.5 0 014 19.5v-15A2.5 2.5 0 016.5 2z" /><path d="M9 7h8M9 11h8" /></svg>
)
export const IconLink = (p) => (
  <svg {...base} {...p}><path d="M10 13a5 5 0 007.07 0l2.83-2.83a5 5 0 00-7.07-7.07L11.5 4.5" /><path d="M14 11a5 5 0 00-7.07 0L4.1 13.83a5 5 0 007.07 7.07L12.5 19.5" /></svg>
)
export const IconTelegram = (p) => (
  <svg {...base} fill="currentColor" stroke="none" {...p}><path d="M21.5 4.5L2.7 12.1c-1.1.5-1.1 1.2-.2 1.5l4.6 1.4 1.8 5.5c.2.6.4.9.8.9.3 0 .5-.1.7-.4l2.6-2.5 5 3.6c.7.4 1.2.2 1.4-.6l3.1-14.5c.3-1-.3-1.5-1-1z"/></svg>
)
export const IconDiscord = (p) => (
  <svg {...base} fill="currentColor" stroke="none" {...p}><path d="M20 5.5A17 17 0 0015.5 4l-.4.8a13 13 0 017 4-6.5-3-11.2 0-6.5 3a13 13 0 017-4L4.5 4A17 17 0 000 5.5C-1 9 -1.3 12.5 -.9 16a16 16 0 004.9 2.5l1-1.6a9 9 0 01-1.9-.9l.5-.4a13 13 0 0016.8 0l.5.4a9 9 0 01-1.9.9l1 1.6A16 16 0 0024.9 16c.4-3.9-.2-7.4-4.9-10.5zM8.5 13.5c-.9 0-1.6-.8-1.6-1.8s.7-1.8 1.6-1.8 1.6.8 1.6 1.8-.7 1.8-1.6 1.8zm7 0c-.9 0-1.6-.8-1.6-1.8s.7-1.8 1.6-1.8 1.6.8 1.6 1.8-.7 1.8-1.6 1.8z"/></svg>
)
export const IconCopy = (p) => (
  <svg {...base} {...p}><rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>
)
export const IconCheck = (p) => (
  <svg {...base} strokeWidth={2.4} {...p}><path d="M20 6L9 17l-5-5"/></svg>
)
export const IconSortUp = (p) => (
  <svg {...base} strokeWidth={2.2} {...p}><path d="M12 19V5M6 11l6-6 6 6"/></svg>
)
export const IconSortDown = (p) => (
  <svg {...base} strokeWidth={2.2} {...p}><path d="M12 5v14M6 13l6 6 6-6"/></svg>
)
export const IconShield = (p) => (
  <svg {...base} {...p}><path d="M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6l8-3z" /><path d="M9 12l2 2 4-4" /></svg>
)
export const IconMore = (p) => (
  <svg {...base} {...p}><circle cx="5" cy="12" r="1.3" /><circle cx="12" cy="12" r="1.3" /><circle cx="19" cy="12" r="1.3" /></svg>
)
export const IconChevronLeft = (p) => (
  <svg {...base} strokeWidth={2.2} {...p}><path d="M15 18l-6-6 6-6" /></svg>
)
export const IconChevronRight = (p) => (
  <svg {...base} strokeWidth={2.2} {...p}><path d="M9 18l6-6-6-6" /></svg>
)
export const IconChevronDown = (p) => (
  <svg {...base} strokeWidth={2.2} {...p}><path d="M6 9l6 6 6-6" /></svg>
)
export const IconAlert = (p) => (
  <svg {...base} strokeWidth={2.2} {...p}><circle cx="12" cy="12" r="9" /><path d="M12 7v6M12 16.5v.01" /></svg>
)
export const IconTv = (p) => (
  <svg {...base} {...p}><rect x="3" y="5" width="18" height="12" rx="2" /><path d="M8 21h8M12 17v4" /></svg>
)
export const IconWidgets = (p) => (
  <svg {...base} {...p}><rect x="3" y="3" width="8" height="8" rx="1.5" /><rect x="13" y="3" width="8" height="5" rx="1.5" /><rect x="13" y="11" width="8" height="10" rx="1.5" /><rect x="3" y="14" width="8" height="7" rx="1.5" /></svg>
)
// --- home ---
export const IconToday = (p) => (
  <svg {...base} {...p}><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></svg>
)
export const IconHome = (p) => (
  <svg {...base} {...p}><path d="M3 10.5L12 3l9 7.5" /><path d="M5 9.5V21h14V9.5" /><path d="M10 21v-6h4v6" /></svg>
)
export const IconBell = (p) => (
  <svg {...base} {...p}><path d="M18 8a6 6 0 10-12 0c0 7-3 9-3 9h18s-3-2-3-9" /><path d="M13.7 21a2 2 0 01-3.4 0" /></svg>
)
export const IconChores = (p) => (
  <svg {...base} {...p}><rect x="4" y="3" width="16" height="18" rx="2" /><path d="M8 8l1.5 1.5L12 7M8 14l1.5 1.5L12 13M15 8.5h2M15 14.5h2" /></svg>
)
export const IconList = (p) => (
  <svg {...base} {...p}><path d="M9 6h11M9 12h11M9 18h11" /><circle cx="4.5" cy="6" r="1" /><circle cx="4.5" cy="12" r="1" /><circle cx="4.5" cy="18" r="1" /></svg>
)
export const IconMeals = (p) => (
  <svg {...base} {...p}><path d="M7 3v7a2 2 0 002 2v9M7 3v5M11 3v5M11 8a2 2 0 01-2 2" /><path d="M17 21V3c-2.2 1.2-3.5 3.8-3.5 7 0 1.7.9 2.8 2 3.2V21" /></svg>
)
export const IconMenu = (p) => (
  <svg {...base} {...p}><path d="M4 6h16M4 12h16M4 18h16" /></svg>
)
// --- end home ---
export const IconStar = (p) => (
  <svg {...base} {...p}><path d="M12 3l2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17l-5.4 2.9 1-6.1-4.4-4.3 6.1-.9L12 3z" /></svg>
)
export const IconFlame = (p) => (
  <svg {...base} {...p}><path d="M12 21c-3.9 0-7-2.8-7-6.6 0-3.1 2-5.3 3.6-7 .4 1.8 1.4 3 2.6 3.6C11 7.6 12.4 5 14.5 3c.6 3 4.5 5.8 4.5 11.4 0 3.8-3.1 6.6-7 6.6z" /><path d="M12 21c-1.7 0-3-1.2-3-2.9 0-1.9 1.6-2.8 3-4.6 1.4 1.8 3 2.7 3 4.6 0 1.7-1.3 2.9-3 2.9z" /></svg>
)
export const IconCamera = (p) => (
  <svg {...base} {...p}><path d="M4 8h3l2-3h6l2 3h3a1 1 0 011 1v10a1 1 0 01-1 1H4a1 1 0 01-1-1V9a1 1 0 011-1z" /><circle cx="12" cy="13.5" r="3.5" /></svg>
)
export const IconBroom = (p) => (
  <svg {...base} {...p}><path d="M19 3l-6.5 6.5" /><path d="M10.5 9.5l4 4" /><path d="M10.5 9.5C7 10 4.5 12.5 4 21c4-.5 8-2 10.5-7.5" /><path d="M7.5 16.5L6 19" /></svg>
)
export const IconCart = (p) => (
  <svg {...base} {...p}><path d="M3 4h2l2.4 11h10.2L20 8H6.2" /><circle cx="9" cy="19.5" r="1.3" /><circle cx="17" cy="19.5" r="1.3" /></svg>
)
export const IconCalendar = (p) => (
  <svg {...base} {...p}><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M3 10h18M8 3v4M16 3v4" /></svg>
)
export const IconClock = (p) => (
  <svg {...base} {...p}><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg>
)
export const IconTrophy = (p) => (
  <svg {...base} {...p}><path d="M7 4h10v5a5 5 0 01-10 0V4z" /><path d="M7 6H4v1.5A3.5 3.5 0 007.5 11M17 6h3v1.5a3.5 3.5 0 01-3.5 3.5" /><path d="M12 14v4M8 21h8M9.5 18h5" /></svg>
)
export const IconTarget = (p) => (
  <svg {...base} {...p}><circle cx="12" cy="12" r="9" /><circle cx="12" cy="12" r="5" /><circle cx="12" cy="12" r="1.2" /></svg>
)
export const IconPiggy = (p) => (
  <svg {...base} {...p}><path d="M5 11a7 6 0 0113.3-2.4L21 8v4l-1.6.6A7 6 0 0116 16.6V20h-3v-2.6a8 8 0 01-2 0V20H8v-3.4A6 6 0 015 11z" /><path d="M10 7.5h3" /><circle cx="16" cy="10.5" r=".6" /></svg>
)
export const IconPill = (p) => (
  <svg {...base} {...p}><rect x="2.5" y="8.5" width="19" height="7" rx="3.5" transform="rotate(-45 12 12)" /><path d="M8.5 8.5l7 7" /></svg>
)
export const IconBook = (p) => (
  <svg {...base} {...p}><path d="M2 5h6a4 4 0 014 4v11a3 3 0 00-3-3H2V5z" /><path d="M22 5h-6a4 4 0 00-4 4v11a3 3 0 013-3h7V5z" /></svg>
)
export const IconGift = (p) => (
  <svg {...base} {...p}><rect x="3" y="8" width="18" height="4" rx="1" /><path d="M5 12v9h14v-9M12 8v13" /><path d="M12 8C10.5 4 7 4 7 6s2.5 2 5 2c2.5 0 5 0 5-2s-3.5-2-5 2z" /></svg>
)
export const IconBulb = (p) => (
  <svg {...base} {...p}><path d="M9 18h6M10 21h4" /><path d="M12 3a6 6 0 00-3.5 10.9c.6.5 1 1.2 1 2.1h5c0-.9.4-1.6 1-2.1A6 6 0 0012 3z" /></svg>
)
export const IconSparkle = (p) => (
  <svg {...base} {...p}><path d="M12 3l1.9 5.6L19.5 10.5l-5.6 1.9L12 18l-1.9-5.6L4.5 10.5l5.6-1.9L12 3z" /><path d="M19 17v4M17 19h4" /></svg>
)
export const IconMedal = (p) => (
  <svg {...base} {...p}><path d="M8 3l2.5 6M16 3l-2.5 6" /><circle cx="12" cy="15" r="6" /><path d="M12 12.5v5" /></svg>
)

// Achievement / challenge icons, keyed by the badge `key` or challenge `id` from /api/gamification.
// Everything the map references must be defined above it (const arrow fns are not hoisted).
const IconGlasses = (p) => (
  <svg {...base} {...p}><circle cx="6.5" cy="14.5" r="3.5" /><circle cx="17.5" cy="14.5" r="3.5" /><path d="M10 14.5h4M3.2 13.5L5 6M20.8 13.5L19 6" /></svg>
)
const IconCalculator = (p) => (
  <svg {...base} {...p}><rect x="5" y="3" width="14" height="18" rx="2" /><rect x="8" y="6" width="8" height="3" rx=".5" /><path d="M8.5 13h.01M12 13h.01M15.5 13h.01M8.5 17h.01M12 17h.01M15.5 17h.01" /></svg>
)
const IconFlag = (p) => (
  <svg {...base} {...p}><path d="M5 21V4" /><path d="M5 4h12l-2.5 4L17 12H5" /></svg>
)
const IconBolt = (p) => (
  <svg {...base} {...p}><path d="M13 2L4.5 13.5H11L10 22l9-12h-6.5L13 2z" /></svg>
)
const IconCoinUp = (p) => (
  <svg {...base} {...p}><circle cx="12" cy="12" r="9" /><path d="M12 16.5V8M8.5 11.5L12 8l3.5 3.5" /></svg>
)
const IconSprout = (p) => (
  <svg {...base} {...p}><path d="M12 21v-9" /><path d="M12 12C12 8 9 6 4.5 6.5 4.5 10.5 7 12.5 12 12z" /><path d="M12 14c0-3.2 2.4-5 6.5-4.5 0 3.4-2.2 5.2-6.5 4.5z" /></svg>
)
const IconFlameBig = (p) => (
  <svg {...base} {...p}><path d="M12 22c-4.4 0-7.5-3-7.5-7.2 0-3.4 2.2-5.8 4-7.8.4 2 1.5 3.3 2.9 4C11.5 8.2 13 5.2 15.3 3c.7 3.4 4.2 6.2 4.2 11.8 0 4.2-3.1 7.2-7.5 7.2z" /><path d="M12 22c-1.9 0-3.3-1.3-3.3-3.2 0-2 1.8-3 3.3-5 1.5 2 3.3 3 3.3 5 0 1.9-1.4 3.2-3.3 3.2z" /><path d="M3 4.5l1 1M21 4.5l-1 1M12 1v1.5" /></svg>
)
const IconBellCheck = (p) => (
  <svg {...base} {...p}><path d="M6 16V11a6 6 0 1112 0v5l1.5 2h-15L6 16z" /><path d="M10 21h4" /><path d="M9.5 11.5l2 2 3-3.5" /></svg>
)
const IconCalendarCheck = (p) => (
  <svg {...base} {...p}><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M3 10h18M8 3v4M16 3v4" /><path d="M9 15.5l2.2 2.2 3.8-4" /></svg>
)
const IconMenuCard = (p) => (
  <svg {...base} {...p}><path d="M7 3v7a2 2 0 002 2v9" /><path d="M4.5 3v5.5A2.5 2.5 0 007 11M9.5 3v5.5A2.5 2.5 0 017 11" /><path d="M17 3c-2.2 1.2-3 3.8-3 7h3v11" /></svg>
)
const IconChefHat = (p) => (
  <svg {...base} {...p}><path d="M7 14.5V20h10v-5.5" /><path d="M7 14.5A4 4 0 015.8 6.8 5 5 0 0112 4a5 5 0 016.2 2.8A4 4 0 0117 14.5" /><path d="M7 17.5h10" /></svg>
)
const IconRank = (p) => (
  <svg {...base} {...p}><path d="M12 3l8 3v6c0 4.8-3.4 7.8-8 9-4.6-1.2-8-4.2-8-9V6l8-3z" /><path d="M8.5 13l3.5-3.2 3.5 3.2M8.5 16.5l3.5-3.2 3.5 3.2" /></svg>
)
const IconCrown = (p) => (
  <svg {...base} {...p}><path d="M3 8l4.5 4L12 5l4.5 7L21 8l-2 11H5L3 8z" /><path d="M5.5 21h13" /></svg>
)

const BADGE_ICONS = {
  // money
  first_log: IconSpending,
  financial_geek: IconGlasses,
  number_cruncher: IconCalculator,
  first_budget: IconBudget,
  under_budget: IconTarget,
  saver: IconPiggy,
  goal_setter: IconFlag,
  goal_crusher: IconBolt,
  asset_tracker: IconAssets,
  income_insider: IconCoinUp,
  // habits
  streak_3: IconSprout,
  streak_7: IconFlame,
  streak_30: IconFlameBig,
  never_forget: IconBellCheck,
  planner: IconCalendarCheck,
  // home
  chores_10: IconBroom,
  chores_50: IconTrophy,
  menu_maker: IconMenuCard,
  chefs_choice: IconChefHat,
  // levels
  level_5: IconRank,
  level_10: IconCrown,
  // challenges / celebration
  log_every_day: IconCalendar,
  no_spend_weekend: IconPiggy,
  wants_under: IconTarget,
  all_chores: IconBroom,
  level_up: IconStar,
  chore_approved: IconCheck,
}
export const BadgeIcon = ({ id, ...p }) => {
  const Icon = BADGE_ICONS[id] || IconMedal
  return <Icon {...p} />
}

/** Hexagon level emblem with the level number inside. tier colours the rim (bronze < 10, silver < 25, gold). */
export const levelTier = (n) => (n >= 25 ? 'gold' : n >= 10 ? 'silver' : 'bronze')
export const LevelBadge = ({ level, size = 40, className = '', ...p }) => {
  const n = Math.max(1, Number(level) || 1)
  const digits = String(n).length
  return (
    <svg viewBox="0 0 48 48" width={size} height={size} role="img" aria-label={`Level ${n}`}
      className={`level-badge tier-${levelTier(n)} ${className}`} {...p}>
      <path className="lb-hex" d="M24 3l18 10.5v21L24 45 6 34.5v-21L24 3z" strokeWidth="3" strokeLinejoin="round" />
      <text className="lb-num" x="24" y="25" textAnchor="middle" dominantBaseline="central"
        fontSize={digits > 1 ? 18 : 22} fontWeight="800">{n}</text>
    </svg>
  )
}

// --- household reminder categories ---
export const IconSpray = (p) => (
  <svg {...base} {...p}><path d="M9 8h5v3l2 2v8H7v-8l2-2V8z" /><path d="M10 8V5h4l2 2" /><path d="M19 4.5h.01M21 6.5h.01M19 8.5h.01" /></svg>
)
export const IconShirt = (p) => (
  <svg {...base} {...p}><path d="M8 3l-5 3 2 4 2-1v12h10V9l2 1 2-4-5-3c-.5 1.5-2 2.5-4 2.5S8.5 4.5 8 3z" /></svg>
)
export const IconPot = (p) => (
  <svg {...base} {...p}><path d="M4 10h16v6a4 4 0 01-4 4H8a4 4 0 01-4-4v-6z" /><path d="M2 10h2M20 10h2" /><path d="M9 6c0-1 1-1 1-2M14 6c0-1 1-1 1-2" /></svg>
)
export const IconCar = (p) => (
  <svg {...base} {...p}><path d="M5 16V11l2-5h10l2 5v5" /><path d="M3 16h18v2H3z" /><circle cx="7.5" cy="13" r=".8" /><circle cx="16.5" cy="13" r=".8" /><path d="M6 18v2M18 18v2" /></svg>
)
export const IconWrench = (p) => (
  <svg {...base} {...p}><path d="M14.7 6.3a4 4 0 00-5.4 5.1L3 17.7 6.3 21l6.3-6.3a4 4 0 005.1-5.4l-2.6 2.6-2.4-.6-.6-2.4 2.6-2.6z" /></svg>
)
export const IconHeart = (p) => (
  <svg {...base} {...p}><path d="M12 20s-7-4.4-7-10a4 4 0 017-2.6A4 4 0 0119 10c0 5.6-7 10-7 10z" /></svg>
)
export const IconRepeat = (p) => (
  <svg {...base} {...p}><path d="M17 2l3 3-3 3" /><path d="M4 11V9a4 4 0 014-4h12" /><path d="M7 22l-3-3 3-3" /><path d="M20 13v2a4 4 0 01-4 4H4" /></svg>
)
export const IconPause = (p) => (
  <svg {...base} {...p}><path d="M9 5v14M15 5v14" /></svg>
)
export const IconPlay = (p) => (
  <svg {...base} {...p}><path d="M7 4l13 8-13 8V4z" /></svg>
)
export const IconChat = (p) => (
  <svg {...base} {...p}><path d="M21 12a8 8 0 01-11.6 7.1L4 20.5l1.4-4.6A8 8 0 1121 12z" /><path d="M8.5 12h.01M12 12h.01M15.5 12h.01" /></svg>
)
export const IconUser = (p) => (
  <svg {...base} {...p}><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0116 0" /></svg>
)

// Reminder category icons (ids match the backend's reminderCategoryIDs).
const CATEGORY_ICONS = {
  cleaning: IconSpray, laundry: IconShirt, cooking: IconPot, groceries: IconCart, vehicle: IconCar,
  home_maintenance: IconWrench, self_care: IconHeart,
  bill: IconBulb, medicine: IconPill, school: IconBook, birthday: IconGift, chores: IconBroom, other: IconBell,
}
export const CategoryIcon = ({ id, ...p }) => {
  const Icon = CATEGORY_ICONS[id] || IconBell
  return <Icon {...p} />
}

// Kinkit app logo (matches brand/kinkit-logo.svg): white tile (hairline #E0E7FF border), purple #4F46E5 house + dot.
export const KinkitLogo = ({ size = 32, ...p }) => (
  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width={size} height={size} {...p}>
    <rect x="0.75" y="0.75" width="62.5" height="62.5" rx="15.25" fill="#FFFFFF" stroke="#E0E7FF" strokeWidth="1.5" />
    <path d="M14 31 32 16l18 15" fill="none" stroke="#4F46E5" strokeWidth="5" strokeLinecap="round" strokeLinejoin="round" />
    <path d="M19 28v18a3 3 0 0 0 3 3h20a3 3 0 0 0 3-3V28" fill="none" stroke="#4F46E5" strokeWidth="5" strokeLinejoin="round" />
    <circle cx="32" cy="39" r="5.5" fill="#4F46E5" />
  </svg>
)
