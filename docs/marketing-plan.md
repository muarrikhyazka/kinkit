# Kinkit marketing plan

Updated 21 November 2026. Moved from the Claude doc "Kinkit Marketing Plan" (18 November 2026), with its product claims checked against the code on 21 November 2026. A spreadsheet copy with the same tables is in `marketing-plan.xlsx`.

Launch Kinkit in the US, the UK and Western Europe as the household app families run together: money, chores, reminders and shopping in one place, with no bank login. English first, a privacy-first story for Europe, and a 90-day target of 500 active households, reached through Reddit, Product Hunt, parenting creators and referrals on about US$1,500 a month.

The first open question below asks whether to run a 30-day beta in Indonesia before this, because the product is ready for Indonesian families today.

## Positioning

Kinkit replaces the three or four apps a family juggles today with one shared household. In the US and Europe the gap is clear: budgeting apps are built for one person or a couple, family organizers do not touch money, and kids' allowance apps require a debit card.

**One-line promise:** "Run your whole household, not just its budget." This is the line the landing page, share card and app login already use, so ads and store listings should repeat it word for word.

| What families use today | Examples (from memory, check before publishing) | Where it falls short | Kinkit's answer |
| --- | --- | --- | --- |
| Budgeting apps | YNAB, Monarch Money, Copilot, Goodbudget | Built for one adult or a couple; kids and grandparents are left out | Roles for parent, kid and viewer; everyone logs into one household budget |
| Family organizers | Cozi, FamilyWall | Lists and calendars, but no money | Shopping lists that turn into one expense; reminders for bills |
| Kids' allowance and chore apps | Greenlight, GoHenry, BusyKid | Tied to a paid debit card; separate from the family budget | Chores earn points and allowance inside the family's own budget, no card needed |
| Open-source finance tools | Firefly III, Actual Budget | Powerful but built for one technical user | The whole family app, with a phone app, TV view and chat bots |

**Proof points to repeat everywhere:**

- No bank login, ever: you never give your bank credentials to a third party. Strong for privacy-minded Europeans.
- Your data stays yours: download spending as CSV or Excel, or a full backup as JSON or Excel at any time.
- Works where the family already is: phone, web, the living-room TV, Telegram, Discord, Slack and Matrix.
- Shows money in USD, EUR, GBP and more.
- Kids get a Playful theme and grandparents a high-contrast Senior theme.

The main risk: US users expect automatic bank sync (Plaid-style), and Kinkit relies on manual or chat logging plus CSV import. Turn it into the pitch ("logging together builds the habit, and nobody holds your bank login") and measure whether it costs sign-ups.

## Audience

Start with US and UK parents aged 30 to 45 with kids aged 6 to 14 who want to teach money without buying a kids' debit card. English covers both markets from day one; Germany, the Netherlands and the Nordics follow once the app is translated.

| Segment | Priority | What they need | Where to reach them | Hook feature |
| --- | --- | --- | --- | --- |
| Parents with kids 6 to 14 (US, UK) | Primary | Teach kids money; stop nagging about chores | Parenting subreddits, Instagram and TikTok parenting creators, parent newsletters and podcasts | Chores that earn points and allowance, no debit card |
| Couples sharing household money (US, UK) | Secondary | One shared budget without merging bank accounts | Personal finance subreddits and YouTube, couple finance creators | Shared budget, partner logs from chat |
| Privacy-minded families (Germany, Netherlands, Nordics) | Secondary, after translation | Budgeting without handing bank logins to a fintech | Privacy communities, local tech press | No bank login, EU hosting (planned) |

## Before launch

The product was built Indonesia-first, so a few gaps must close before any promotion in the US and Europe. The two biggest: amounts are stored in Indonesian rupiah, and there is no iPhone app in markets where many families use iPhones.

- [ ] **Household base currency.** Amounts are stored in rupiah today (`accounts.currency` defaults to IDR) and only converted for display. US and European families need USD, EUR or GBP as the real stored currency, without exchange-rate drift.
- [ ] **iPhone app on the App Store.** Needs a US$99 a year Apple developer account and a Mac build (for example GitHub Actions). Launching without it loses a large share of US parents.
- [ ] **Google Play listing.** A one-time US$25 account; nobody in these markets sideloads an APK.
- [ ] **Hosting in the EU and GDPR basics.** A public server in an EU region, terms, a data processing agreement, self-serve account deletion (today only an admin can delete an account) and a cookie-free analytics setup. A factual privacy page went live on 20 November 2026; it still needs a real contact address. The family server on jamal in Indonesia is not suitable.
- [ ] **Bank CSV import for US, UK and EU banks.** Import today knows BCA, Mandiri and a generic format. Add presets for common exports (Chase, Bank of America, Monzo, Revolut, N26) or a column-mapping step.
- [ ] **Pricing in USD and EUR with card payments** (for example Stripe), including EU VAT handling. The placeholder tiers were removed on 20 November 2026 and the landing page now says "Free to sign up"; add pricing back only once plans exist in the app.
- [ ] **US and European defaults:** week starting on Sunday or Monday, date formats, sample categories such as groceries, utilities and childcare instead of Indonesian ones.
- [ ] **Store assets:** screenshots and a 30-second demo video in English. The app icon and Kiki already exist in `brand/`; the current mock screens show rupiah amounts, so versions in dollars, euros and pounds are needed.
- [ ] **Sample household on first login,** so a new family sees a filled Today page before entering any data.
- [ ] **Support:** a help email and a Discord server for beta families (Discord fits US families with teens; Telegram is less common in the US).

## Goals

The headline goal is 500 active households by day 90, where "active" means at least two members used Kinkit in the past 7 days. Count households, not users: a family with only one person logging behaves like a solo budgeting app and churns.

| Metric | Day 30 (private beta) | Day 60 (public launch) | Day 90 | Why it matters |
| --- | --- | --- | --- | --- |
| Households signed up | 80 | 500 | 1,200 | Top of funnel |
| Active households (2+ members, past 7 days) | 50 | 250 | 500 | The north-star metric |
| Second member invited within 7 days | 50% | 55% | 60% | The strongest sign a family will stay |
| Households still active after 30 days | Not measured yet | 40% | 45% | Retention |
| Landing visit to sign-up | 4% | 5% | 6% | Landing page and message fit |
| Trial to paid (if a paid tier exists) | Not measured yet | 8% | 10% | Early revenue signal |
| Households in the EU | 10% | 20% | 30% | Shows the privacy story is landing |

These are starting targets for a bootstrapped launch, not market benchmarks; adjust them after the first 30 days of real data. Measuring them needs the cookie-free analytics from Before launch, since the app and site load no analytics today.

## Channels

Launch through communities where founders can post directly, then build steady reach with content and creators. Kinkit spreads inside a family by design, since every member needs a login, so referrals stay the cheapest channel.

| Channel | Tactic | Effort | Cost per month (US$) | Phase |
| --- | --- | --- | --- | --- |
| Beta families | Recruit 50 to 80 families from friends, Reddit and a waitlist; weekly feedback | High | 0 | Beta |
| Reddit | Honest founder posts and helpful answers in r/Parenting, r/personalfinance, r/UKPersonalFinance, r/FinancialPlanning; follow each subreddit's self-promotion rules | Medium | 0 | Beta onward |
| Hacker News and Product Hunt | "Show HN" for the family app; a Product Hunt launch on a weekday at public launch | Medium, one-off | 0 | Launch week |
| Short-form video (TikTok, Instagram Reels, YouTube Shorts) | 3 videos a week: a kid earning allowance, the partner logging by chat, the TV dashboard | High | 0 to 300 (editing) | Launch onward |
| Parenting and money creators (10k to 100k followers) | Sponsored reviews and newsletter mentions, each with its own referral code | Medium | 600 to 800 | Launch onward |
| Referral program | A referral code per family to see who brought whom; rewards wait until a paid tier exists, since there is nothing to give a free month of yet | Low once built | 0 (build time) | Launch onward |
| App Store and Google Play | Keywords such as "family budget", "chore chart", "kids allowance"; screenshots built around one message each | Low | 0 | Launch onward |
| Content and SEO | Comparison and how-to pages: "chore chart with allowance", "family budget without bank linking", "YNAB alternative for families" | Medium | 0 to 200 | Day 30 onward |
| Paid social (Meta, Reddit ads) | Retarget landing visitors; test only after sign-up conversion reaches 4% | Medium | 300 to 400 | Day 60 onward |

Skip for now: Google search ads (expensive finance keywords), PR agencies and app-install networks. Revisit after day 90.

## Messaging and content

Sell the calmer household, not the features: less nagging, no surprise bills, kids who understand money. In Europe, lead with privacy; in the US, lead with kids and allowance.

| Message | Headline | Best market | Proof to show |
| --- | --- | --- | --- |
| Kids learn money by doing | Chores earn points. Points become allowance. No debit card needed. | US, UK | Kid taps Done with a photo, parent approves, allowance goes up |
| Everyone pitches in | One app for the whole family. Everyone logs, everyone sees. | US, UK | Partner sends "$12 lunch" in Discord; it appears on the dashboard (needs the base-currency fix) |
| Your money data stays yours | No bank login. Export anytime. | EU, privacy-minded families | Export screen (CSV, Excel, JSON) |
| Nothing slips | Bills, meds and school dates remind the right person. | All | Reminder in the family chat with Done, Snooze 1h and Tomorrow buttons |

**Content pillars** (rotate weekly across video and posts):

1. **"How we run our home"**: real family routines, such as the Sunday money check on the TV.
2. **Money lessons for kids**: allowance amounts by age, saving goals, chores that fit each age.
3. **Privacy and ownership**: why you never need to share your bank login and how to take your data out, aimed at Europe.
4. **Build in public**: what shipped this week and what families asked for, on Reddit, X and a short newsletter.

**Sample posts:**

- Reel, 20 seconds: a parent types "eggs, 2 milk, bread x3" in the shopping list; the other parent ticks items off in the store; "Finish shopping" logs one expense. Caption: "Grocery runs without the group chat chaos."
- Carousel: "5 chores for kids aged 6 to 10, and how much allowance each could earn."

## Budget

About US$1,500 a month from the public launch onward, plus about US$125 one-time for the store accounts. The private beta (days 1 to 30) costs little beyond hosting and time.

| Item | Monthly (US$) | Notes |
| --- | --- | --- |
| Creator reviews and newsletter mentions | 700 | 2 to 3 creators a month, each with a referral code |
| Paid social ads | 350 | Only from day 60, retargeting landing visitors |
| Video editing help | 200 | Freelance editor for 12 short videos |
| Content and SEO writing | 150 | 2 comparison or how-to articles a month |
| EU hosting, domain, email, analytics | 100 | Separate from the family server on jamal |
| **Total** | **1,500** | |

One-time: Apple developer account US$99 a year and Google Play US$25. Budget up to US$500 more if you want a lawyer to review the privacy policy and terms for GDPR. Referrals cost nothing until a paid tier exists.

## Roadmap

Two gates stand between the private beta and 500 active households by day 90.

| Phase | Days | Work | Gate to pass before the next phase |
| --- | --- | --- | --- |
| Private beta | 1 to 30 | Onboard 50 to 80 beta families; App Store, Play and EU server live; weekly feedback calls | Day 30: 50 active households, base currency fixed, both stores live |
| Public launch | 31 to 60 | 3 short videos a week; Reddit, Show HN, Product Hunt; referral program live | Day 60: sign-up rate at least 4%; only then start paid ads |
| Grow | 61 to 90 | Creator reviews and SEO pages; paid ads to retarget visitors; test the paid tier | Day 90: 500 active households |

Do not move to the next phase until its gate is met: promotion before the base-currency fix, both app stores and an EU server only wastes reach, and paid ads before sign-ups convert only wastes money.

## Open questions

- [ ] Run a 30-day beta in Indonesia first? It is ready there today: amounts are stored in rupiah, import knows BCA and Mandiri, and Telegram is common. An Indonesian beta could test the two-member activation goal while the blockers above are fixed.
- [ ] Which domain will the public app and landing page use: kinkit.app, or another name?
- [ ] Free only at launch, or a paid tier from day one? If paid, what price per family per month in USD and EUR?
- [ ] US and UK first with Europe in English only, or translate to German or Dutch within the 90 days?
- [ ] Which EU hosting provider and region for the public server?
- [ ] Who records the videos and runs the Discord community: you, or a part-time helper?
- [ ] Is about US$1,500 a month workable, or should the plan run on community channels only?
