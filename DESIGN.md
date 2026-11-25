# Kinkit design direction

The one place to look before any UI work, on every surface (web, landing, phone, TV). Token values live in code (`react/src/index.css`, `landing/styles.css`); the reasoning behind them lives in `react/DESIGN_AUDIT.md`. This file says what the product should feel like and what to check against.

Dial: ENERGY 2 / RHYTHM 2 / MOTION 1

## Who it's for

Families who manage money and home together, used by the whole household at once: a parent logging lunch from the market, a kid checking their allowance, a grandparent reading the TV screen from the sofa. Phones come first. Amounts are shown in the user's own currency, and typed the short way ("25k", "Rp25.000").

## Personality

Warm, practical and calm, like a well-run home. It's on the family's side, not a bank's. Playful in small doses (Kiki, points, badges) and never noisy about money.

## Identity

- **Kiki, the house-shaped mascot**, is the motif. It appears at real moments (welcome, a badge, everything done) and never just as decoration. Its art lives in `brand/`; copy it from there and never redraw it.
- **The house roof** from the logo is the one shape repeated across the brand: the logo tile, Kiki and the favicon.
- **Real household content** is part of the look. Mock screens show "Vegetables & fruit 48k", "Water the plants · Leo", never lorem ipsum or invented statistics. The sample household everywhere (landing mockups, docs screenshots, demo data) is "The Carter family": Sarah (owner), Tom (parent), Leo and Mia (kids), Grandma Ruth (viewer). Amounts stay in IDR and round. Wherever the landing shows made-up numbers, one visible caption says so.

## Palette

- Neutrals: `--bg`, `--card`/`--surface`, `--border`, `--text`, `--muted`.
- One brand colour: indigo `--primary` #4F46E5. It's used for primary actions, the chrome accent and one key accent per screen.
- Chrome: the web sidebar, mobile top bar and game HUD are white (near-black #151517 in dark) with a purple accent. The active item is a tint, bold text and a 3px purple bar on its left, and the bar survives in the collapsed rail. The phone app bar, the landing top bar, the TV brand bar and the login hero stay flat indigo.
- Semantic colours carry meaning only: green `--success` is income or on track, amber `--warning` is a budget past 80%, red `--danger` is over budget or destructive.
- Light and dark both ship and both must pass WCAG AA. Playful (kids) and Senior (high contrast) themes ship too.
- Dark tokens, shared by web and phone: surface #151517 on page #0B0B0D, text and focus primary #818CF8, hover #A5B4FC, success #4ADE80, danger #F87171, warning #FBBF24. Filled controls keep #4F46E5 (danger #DC2626, success #15803D) with white text, so a primary fill is never white on #818CF8 (2.98:1).
- TV: the viewers use navy surfaces (#0B0E1A page, #151A2E cards) because they mirror `TVView.jsx`, not the web dark theme (the landing dark page is navy too). They share the dark primary #818CF8 for text and fills of bars. The focused control is #4F46E5 with white text and a white ring (6.29:1).

## Type

- Inter where the device has it, falling back to system UI (no web fonts are downloaded). It was chosen for clear numerals at small sizes and good Indonesian text rendering.
- Money uses tabular figures.
- Scale: 12/14/16/20/24/32, plus a 48px display size on the landing hero.
- Weights: 500/600/700.
- Sentence case everywhere. No uppercase labels with wide tracking.

## Composition

- Spacing: 4/8/12/16/24/32 (48/64/96 between landing sections).
- Radii: 6/10/14, plus pill for chips and progress bars only.
- Shadows: two. `shadow-1` is for resting cards, and `shadow-2` is only for floating layers (modal, popover, toast) and the landing's device mockups.
- Lists and rows are preferred over card grids. Each landing section is laid out for its content (editorial feature groups, a timeline, device row, download list) rather than repeating one template.

## Motion

- Hover and press states, plus one-off reveals.
- Kiki's loops end within 5 seconds.
- The login scene is the one long animation, and it has a pause button.
- Everything decorative stops under `prefers-reduced-motion`.

## Never

- Decorative gradients, glows or radial orbs. The chrome accent and the login hero are flat.
- Invented numbers, testimonials, logos or prices. Pricing appears only once real plans exist in code.
- Em dashes in any text people read.
- Generic CTAs ("Get started", "Learn more"). Name the action instead ("Start free", "Create your household").
- Claims on the landing page that the code doesn't back up. See the claims table in `landing/README.md`.

## Checking work

Run the antislop rules (`~/.claude/skills/antislop`) as a filter on top of this file. The latest audit is in `anti-slop/`.
