---
version: alpha
name: K-Mainstay Midnight Blue
description: A dark-only, calm workspace interface with precise typography, restrained blue interaction, and almost no decoration.
colors:
  primary: "#6EA0FF"
  canvas: "#090B10"
  surface: "#0F131A"
  surface-raised: "#151A23"
  surface-hover: "#1A202B"
  border: "#252D3A"
  text-primary: "#F4F7FB"
  text-secondary: "#AAB4C3"
  text-muted: "#758196"
  accent-hover: "#85AFFF"
  accent-pressed: "#578CEF"
  accent-ink: "#07101D"
  danger: "#B84F5A"
  danger-hover: "#C9606A"
  success: "#52B788"
  warning: "#D7A84A"
typography:
  display:
    fontFamily: Geist Sans
    fontSize: 2rem
    fontWeight: 600
    lineHeight: 1.15
    letterSpacing: "-0.03em"
  heading-lg:
    fontFamily: Geist Sans
    fontSize: 1.5rem
    fontWeight: 600
    lineHeight: 1.25
    letterSpacing: "-0.02em"
  heading-md:
    fontFamily: Geist Sans
    fontSize: 1.125rem
    fontWeight: 600
    lineHeight: 1.35
    letterSpacing: "-0.01em"
  body:
    fontFamily: Geist Sans
    fontSize: 0.9375rem
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "0em"
  body-strong:
    fontFamily: Geist Sans
    fontSize: 0.9375rem
    fontWeight: 500
    lineHeight: 1.5
    letterSpacing: "0em"
  body-small:
    fontFamily: Geist Sans
    fontSize: 0.8125rem
    fontWeight: 400
    lineHeight: 1.45
    letterSpacing: "0em"
  label:
    fontFamily: Geist Sans
    fontSize: 0.8125rem
    fontWeight: 500
    lineHeight: 1.25
    letterSpacing: "0em"
  code:
    fontFamily: Geist Mono
    fontSize: 0.8125rem
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "0em"
spacing:
  1: 4px
  2: 8px
  3: 12px
  4: 16px
  5: 24px
  6: 32px
  7: 48px
rounded:
  control: 6px
  panel: 8px
  dialog: 12px
  full: 9999px
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.accent-ink}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: 10px
    height: 40px
  button-primary-hover:
    backgroundColor: "{colors.accent-hover}"
    textColor: "{colors.accent-ink}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: 10px
    height: 40px
  button-primary-pressed:
    backgroundColor: "{colors.accent-pressed}"
    textColor: "{colors.accent-ink}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: 10px
    height: 40px
  button-secondary:
    backgroundColor: "{colors.surface-raised}"
    textColor: "{colors.text-primary}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: 10px
    height: 40px
  button-danger:
    backgroundColor: "{colors.danger}"
    textColor: "{colors.text-primary}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: 10px
    height: 40px
  button-danger-hover:
    backgroundColor: "{colors.danger-hover}"
    textColor: "{colors.accent-ink}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: 10px
    height: 40px
  input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.text-primary}"
    typography: "{typography.body}"
    rounded: "{rounded.control}"
    padding: 12px
    height: 40px
  sidebar-item:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.text-secondary}"
    typography: "{typography.body}"
    rounded: "{rounded.control}"
    padding: 8px
    height: 40px
  sidebar-item-hover:
    backgroundColor: "{colors.surface-hover}"
    textColor: "{colors.text-primary}"
    typography: "{typography.body}"
    rounded: "{rounded.control}"
    padding: 8px
    height: 40px
  sidebar-item-active:
    backgroundColor: "{colors.surface-raised}"
    textColor: "{colors.text-primary}"
    typography: "{typography.body-strong}"
    rounded: "{rounded.control}"
    padding: 8px
    height: 40px
  metadata:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.text-muted}"
    typography: "{typography.body-small}"
  divider:
    backgroundColor: "{colors.border}"
    height: 1px
  status-success:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.success}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: 8px
  status-warning:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.warning}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: 8px
  dialog:
    backgroundColor: "{colors.surface-raised}"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.dialog}"
    padding: 24px
---

## Overview

K-Mainstay is a dark-only workspace product. It should feel calm, fast and exact: the confidence and strong contrast of Wise, expressed through blue rather than green; Linear's disciplined hierarchy; Slack's understandable workspace structure; and Superhuman's restraint.

This is not a marketing aesthetic. The chat workspace is primarily an **Operate** surface. Organisation settings is a **Configure** surface. Comprehension and action always beat decoration.

The interface has one visual idea: near-black space, clear white information, quiet grey context and restrained blue interaction. It must never look busy, ornamental, futuristic or like a generic AI product.

### Governing principles

1. **Default dark means dark only.** Do not add a light theme, theme switcher or parallel token set without a demonstrated user need.
2. **Remove before styling.** Delete unnecessary containers, labels, icons, dividers and controls before improving their appearance.
3. **Hierarchy before decoration.** Establish importance with position, spacing, type weight and contrast. Do not use extra boxes or colours as a substitute.
4. **One obvious next action.** A view may have many available actions, but only one should normally receive primary blue emphasis.
5. **Density without crowding.** Related controls stay close; separate concepts receive more space. Avoid both giant empty areas and cramped card grids.
6. **Consistency beats novelty.** Reuse the defined primitives. Do not invent a new radius, colour, shadow or control treatment for one screen.
7. **Quiet by default.** The interface should recede behind conversations and work.

## Colors

### Core roles

- **Canvas (`#090B10`)** is the permanent application background.
- **Surface (`#0F131A`)** separates major working regions such as the composer or settings form without appearing as a card.
- **Raised surface (`#151A23`)** is reserved for dialogs, menus and selected or genuinely elevated areas.
- **Primary text (`#F4F7FB`)** carries names, messages, headings and values. It is softer than pure white.
- **Secondary text (`#AAB4C3`)** carries supporting descriptions and metadata.
- **Muted text (`#758196`)** is only for timestamps, placeholders and low-priority labels. Never use it for essential instructions.
- **Primary (`#6EA0FF`)** communicates interaction: primary actions, active focus, links and selected indicators. Blue is not decorative body copy.
- **Semantic colours** communicate status only. They never become section themes.

### Colour restraint

- Most screens should use canvas, one surface, primary text, secondary text and occasional accent.
- Blue wording must mean clickable, selected or focused. Do not make headings blue merely for style.
- Never use gradients, coloured glows, rainbow accents, translucent glass or decorative blur.
- Never place large areas of saturated blue behind content.
- Borders should be quiet. Prefer spacing over a divider when the relationship remains clear without one.

### Contrast requirements

The core palette is chosen to exceed WCAG AA for normal text:

- Primary text on canvas: 18.32:1.
- Secondary text on canvas: 9.40:1.
- Muted text on canvas: 5.00:1.
- Accent on canvas: 7.62:1.
- Dark accent ink on accent: 7.39:1.
- Primary text on danger: 4.54:1.

Do not reduce opacity on text. Use the defined solid text colours instead.

## Typography

Use **Geist Sans** for all interface and message text. Use **Geist Mono** only for API keys, identifiers and code. Self-host the required WOFF2 files when implementing; do not depend on a third-party font request at runtime.

Only ship weights 400, 500 and 600. More weights create noise without improving hierarchy.

### Rules

- Default application text is 15px with a 1.5 line height.
- Navigation, buttons and labels use 13px or 15px. Do not use tiny uppercase labels as decoration.
- Use 600 only for headings and strong emphasis. Most interface emphasis uses 500.
- Headings use modest negative tracking. Body and labels use normal tracking.
- Avoid italics except when they carry meaning in user-authored content.
- Keep line lengths readable. Long-form message content should not exceed roughly 75 characters per line when the viewport permits.
- User-authored Markdown controls content hierarchy, but cannot introduce arbitrary application fonts or colours.

## Layout

Use the 4px-based spacing scale defined above. Prefer the sequence 8, 12, 16, 24 and 32px. Use 4px only for tightly related micro-elements and 48px only for major page separation.

### Workspace composition

- Use one persistent sidebar for organisation identity and conversations on desktop.
- Keep the conversation header, message history and composer visually aligned.
- Let message content occupy the space. Do not wrap each message in a card or speech bubble.
- Keep settings in one readable column. Group by clear headings and spacing rather than a dashboard of equal-weight cards.
- Use a maximum readable content width where appropriate, but do not centre the entire operational application in a floating panel.

### Responsive behaviour

- Desktop is the primary workspace, but all controls must remain usable on narrow screens.
- Collapse structure rather than shrinking typography below the defined minimums.
- Interactive targets must be at least 40px on desktop and 44px on touch-oriented layouts.
- Preserve one primary task per narrow-screen view. Do not squeeze the sidebar and conversation into unusable columns.

## Elevation & Depth

The application is nearly flat.

1. **Base:** canvas with no shadow.
2. **Contained:** a subtle border or surface change, not both unless necessary for contrast.
3. **Overlay:** dialogs and menus may use a raised surface, border and one soft shadow.

No ordinary card, message, sidebar item or settings section receives a drop shadow. Do not use inset highlights, neon glows or stacked decorative shadows. A modal backdrop may use black at approximately 70% opacity; it must not use backdrop blur.

## Shapes

- Controls use a 6px radius.
- Panels use an 8px radius only when they require visible containment.
- Dialogs use a 12px radius.
- Full pills are reserved for genuine compact statuses, never ordinary buttons or navigation.
- Avatars may be circular.
- Do not create large rounded rectangles merely to fill space.

## Components

### Buttons

- One primary button per local decision area.
- Primary buttons use blue with dark text for strong, comfortable contrast.
- Secondary buttons use a raised neutral surface and a visible border.
- Tertiary actions are quiet text or icon buttons; they gain a surface only on hover or focus.
- Destructive buttons remain neutral until the user reaches a clear confirmation step. The final destructive action uses danger red.
- Buttons do not scale, glow or move on hover. Use a subtle colour change over 120ms.
- Icon-only buttons require an accessible name and at least a 40px target.

### Inputs and composer

- Inputs sit directly in the layout unless a surface is needed to separate an editing region.
- Use one border, one background and one strong focus ring. Do not use inset shadows.
- Placeholder text cannot replace a persistent label when the field's meaning would become ambiguous.
- The message composer should feel anchored and quiet, not like a floating glass capsule.

### Navigation

- Active navigation is communicated by stronger text and a restrained surface change.
- Blue may appear as a small active indicator, but not as a full saturated navigation background.
- Keep icons subordinate to labels. Do not add icons when text scans more clearly.
- Conversation names are the primary signal. Metadata must not compete with them.

### Messages

- Messages are a continuous reading stream, not a collection of cards.
- Group author, bot marker and timestamp on one quiet metadata line.
- Bot markers are subtle and textual. They identify; they do not advertise.
- Use whitespace to distinguish authors and message groups.
- Code blocks may use the surface colour and Geist Mono. Avoid syntax colours unless they materially improve comprehension.

### Settings

- Settings use a Configure composition: page title, short explanation, clear sections, direct controls.
- Prefer rows and dividers over independent floating cards.
- Explain irreversible consequences beside the relevant action, not in a separate callout box.
- Progressive disclosure is allowed for uncommon destructive or credential actions. Do not hide common tasks behind menus.

### Dialogs

- Use dialogs only when the user must complete or confirm a bounded task before continuing.
- Keep one title, optional short explanation, content and actions. Do not put a card inside a dialog.
- Place the primary or final action consistently. Cancellation must remain obvious.
- API keys use Geist Mono, selectable text and a copy action. Copy-once consequences must be explicit.

### Feedback and states

- Loading states preserve layout and avoid decorative animation.
- Empty states state what is absent and provide the next useful action when one exists.
- Errors appear near the failed action and remain until understood or superseded.
- Success feedback is brief and does not block continued work.
- Focus indicators use a 2px accent ring with sufficient separation from the control edge.

## Do's and Don'ts

### Do

- Start every screen by naming its primary task.
- Use typography and spacing before introducing a container.
- Keep actions close to the object they affect.
- Use familiar labels such as “Add bot”, “Rotate key” and “Delete conversation”.
- Preserve keyboard navigation, visible focus and meaningful source order.
- Check every new colour pair for WCAG AA contrast.
- Review the interface at desktop and narrow widths before accepting it.

### Don't

- Do not add a light mode.
- Do not use gradients, glassmorphism, blur, glows or decorative animation.
- Do not use serif typography, all-caps eyebrows or oversized display text in the application.
- Do not turn every section into a card.
- Do not use pills for ordinary buttons.
- Do not add icon grids, fake statistics, decorative illustrations or generic AI imagery.
- Do not use blue as decoration; it must signal interaction or state.
- Do not create abstractions or variants without a current screen that needs them.
- Do not introduce a component framework solely to reproduce these simple primitives.

## Implementation Contract

This document is the visual source of truth for K-Mainstay. Once approved:

1. Put frontend source under one clearly named top-level `frontend/` directory.
2. Keep the Vue application small and explicit. Split by meaningful product surface, not by every HTML element.
3. Implement tokens as CSS custom properties derived from this file.
4. Build only the primitives required by current screens.
5. Keep behavioural composables separate from visual components.
6. Add visual or browser tests for critical states, but do not create a full design-system application.
7. Treat additions to the palette, typography scale, spacing scale or component variants as changes to this contract.

Approval of this document does not approve the frontend refactor. That remains a separate implementation step.

## Sources and Inspiration

This is an original K-Mainstay system, not a reproduction of another product.

- Refactoring UI: hierarchy, spacing, colour restraint and practical developer-led visual decisions — https://www.refactoringui.com/
- Linear: dark-surface precision, restrained borders and operational density.
- Slack: understandable workspace and conversation structure.
- Superhuman: confident typography, minimal decoration and focused interaction.
- Wise: strong contrast and one unmistakable accent, translated from green to blue.
- Nielsen Norman Group usability heuristics: visibility, consistency, error prevention and recognition over recall — https://www.nngroup.com/articles/ten-usability-heuristics/
