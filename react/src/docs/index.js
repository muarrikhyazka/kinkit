import { DKey, DWallet, DHome, DScreen, DChat, DSteps, DGear, DHelp } from './DocsIcons.jsx'

/**
 * Docs registry. Each page is a module in ./pages exporting
 * { slug, title, section, summary, Content }. The sidebar tree and the
 * Previous / Next order come from SECTIONS below: add a page file, then list
 * its slug here.
 */
const modules = import.meta.glob('./pages/*.jsx', { eager: true })
const bySlug = {}
for (const m of Object.values(modules)) bySlug[m.slug] = m

export const SECTIONS = [
  { id: 'getting-started', title: 'Getting started', Icon: DKey, slugs: ['introduction', 'login', 'navigation', 'roles', 'install'] },
  { id: 'money', title: 'Money', Icon: DWallet, slugs: ['add-spending', 'dashboard', 'spending', 'income', 'recurring', 'budget', 'goals', 'assets'] },
  { id: 'family-home', title: 'Family & home', Icon: DHome, slugs: ['today', 'calendar', 'reminders', 'family', 'chores', 'lists', 'meals', 'meal-requests'] },
  { id: 'screens-devices', title: 'Screens & devices', Icon: DScreen, slugs: ['widgets', 'tv-links'] },
  { id: 'chat-bots', title: 'Chat bots', Icon: DChat, slugs: ['bots-overview', 'bots-telegram', 'bots-discord', 'bots-slack', 'bots-matrix', 'bot-commands'] },
  { id: 'points-badges', title: 'Points, levels & badges', Icon: DSteps, slugs: ['points', 'levels', 'badges'] },
  { id: 'settings-data', title: 'Settings & data', Icon: DGear, slugs: ['settings-categories', 'settings-display', 'import-export'] },
  { id: 'faq', title: 'FAQ', Icon: DHelp, slugs: ['faq'] },
].map(s => ({ ...s, pages: s.slugs.map(slug => {
  if (!bySlug[slug] && import.meta.env.DEV) console.warn('docs: missing page module for', slug)
  return bySlug[slug]
}).filter(Boolean) }))

/** Flat reading order (drives Previous / Next). */
export const PAGES = SECTIONS.flatMap(s => s.pages.map(p => ({ ...p, sectionTitle: s.title })))
export const FIRST_SLUG = PAGES[0].slug

export const pagePath = slug => (slug === FIRST_SLUG ? '/docs' : `/docs/${slug}`)
export const getPage = slug => PAGES.find(p => p.slug === (slug || FIRST_SLUG))
export const sectionOf = page => SECTIONS.find(s => s.pages.some(p => p.slug === page.slug))
