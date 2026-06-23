# RelayCheck Hub Design System

## Positioning
RelayCheck Hub is a local operations console for NewAPI/Sub2API relay management. The UI should feel calm, precise, compact, and trustworthy. It is not a marketing site and should not use decorative layouts that hide operational data.

## Visual Direction
- Style: Control Room SaaS, compact admin, short rounded cards.
- Mood: calm, technical, reliable, low-noise.
- Density: medium-high on desktop, single-column clarity on mobile.
- Primary palette: blue + teal for system/action, green for success, amber for attention, red for destructive/problem.
- Surfaces: soft white cards on a very light gridded background.
- Shadows: low elevation by default; stronger only on hover or active cards.

## Token Source
- V4 `--v4-*` variables are the active design-token source for the current Control Room layer.
- Tailwind `@theme` should bridge to V4 tokens rather than hard-code a parallel palette.
- Token groups now include semantic colors, status backgrounds/borders, input/focus values, skeleton values, type scale, font weights, tracking, spacing, radius, and shadow levels.
- Legacy `--rc-*` / `--linear-*` / early base tokens may still exist in older CSS layers; do not treat those as the desired long-term source. Continue migrating active rules to V4 tokens before deleting historical layers.

## Hierarchy Rules
- Important numbers are large, tabular, and placed near the top-left of cards.
- Operational metric numbers should use `tabular-nums` / `tnum` consistently across Dashboard, Radar, cards, details, sync, and scheduler surfaces.
- Status text must include words, not color alone.
- Long messages should become short cards or clipped summaries with a clear path to details.
- Primary actions are visible; maintenance and destructive actions stay visually secondary or grouped.
- Success information should be present but softer than failures.

## Component Rules
- Local UI primitives live under `frontend/src/components/ui/*`; they are project-owned wrappers, not a generated shadcn install.
- Current primitives: `Button`, `Card`, `Badge`, `Input`, `Select`, `Skeleton`, `Dialog`, `Progress`, `Tooltip`, and `Switch`.
- UI primitives should use `cn()` from `frontend/src/lib/cn.ts`; `cn()` is backed by `clsx + tailwind-merge` so Tailwind conflicts resolve predictably.
- Dashboard cards: short, fixed-width where possible, never stretch just to fill a row.
- Filters/toolbars: content-width on desktop, full-width only on mobile.
- Account cards: compact, domain/backend identity first, then account/checkin/balance, then actions.
- Channel cards: show backend type and check-in capability above secondary metadata.
- Notifications: important-first; routine success/info stays folded or visually quiet.
- Settings: configuration sections should be card-sized, not page-wide banners.

## Accessibility And Interaction
- Keep native `button`, `input`, `select`, `textarea` semantics.
- Focus rings must remain visible.
- Desktop hover can lift cards subtly; mobile must not rely on hover.
- Respect `prefers-reduced-motion`.
- Avoid horizontal scrolling at 390px and common desktop widths.
- On coarse pointer devices, clickable controls must expose at least a 44x44px touch target.
- Dashboard chart and diagnostic grids should use `auto-fit/minmax` instead of fixed column counts.
- On mobile widths, main content grids should collapse to one column; navigation may remain a compact horizontal strip.
- Table-like grid rows must keep children shrinkable with `min-width: 0`; long URLs, file names, and summaries should wrap instead of forcing horizontal overflow.
- Shared `@keyframes` definitions belong in the global motion/keyframes area, not inside component-specific finishing layers.
- Do not use emoji as UI icons. Prefer lightweight line icons plus visible Chinese text; avoid adding icon dependencies unless there is a broader component-system reason.
- Status indicators must pair color with visible text and a non-color cue such as a line icon. Use `StatusLabel` for high-frequency states across channels, accounts, scheduler, audit, sync results, and settings toggles.

## Implementation Constraints
- Keep the current Go + React/Vite + Tailwind v4 CSS import + plain CSS architecture.
- Tailwind v4 is retained as a build-time CSS layer via `@import "tailwindcss"` and `@tailwindcss/vite`.
- Frontend imports may use the `@/*` alias for `frontend/src/*`.
- Do not add Radix/shadcn runtime dependencies unless explicitly approved; local `components/ui/*` primitives are project-owned wrappers, not an installed shadcn component system.
- Use CSS tokens and finishing-layer overrides before restructuring business JSX.
- Do not write real tokens, passwords, cookies, or API keys into docs, source, or temp files.
