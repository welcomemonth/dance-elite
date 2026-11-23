# Design Language: Payments, tax & subscriptions for software companies • Lemon Squeezy

> Extracted from `https://lemonsqueezy.com` on May 28, 2026
> 1224 elements analyzed

This document describes the complete design language of the website. It is structured for AI/LLM consumption — use it to faithfully recreate the visual design in any framework.

## Color Palette

### Neutral Colors

| Hex | HSL | Usage Count |
|-----|-----|-------------|
| `#000000` | hsl(0, 0%, 0%) | 2430 |
| `#121217` | hsl(240, 12%, 8%) | 14 |
| `#6c6c89` | hsl(240, 12%, 48%) | 4 |

### Text Colors

Text color palette: `#000000`, `#121217`, `#6c6c89`

### Full Color Inventory

| Hex | Contexts | Count |
|-----|----------|-------|
| `#000000` | text, border | 2430 |
| `#121217` | text, border | 14 |
| `#6c6c89` | text, border | 4 |

## Typography

### Font Families

- **Times** — used for all (1224 elements)

### Type Scale

| Size (px) | Size (rem) | Weight | Line Height | Letter Spacing | Used On |
|-----------|------------|--------|-------------|----------------|---------|
| 32px | 2rem | 700 | normal | normal | h1 |
| 24px | 1.5rem | 700 | normal | normal | h2 |
| 18.72px | 1.17rem | 700 | normal | normal | h3 |
| 16px | 1rem | 400 | normal | normal | html, head, meta, link |

### Heading Scale

```css
h1 { font-size: 32px; font-weight: 700; line-height: normal; }
h2 { font-size: 24px; font-weight: 700; line-height: normal; }
h3 { font-size: 18.72px; font-weight: 700; line-height: normal; }
```

### Body Text

```css
body { font-size: 16px; font-weight: 400; line-height: normal; }
```

### Font Weights in Use

`400` (1144x), `700` (80x)

## Spacing

**Base unit:** 4px

| Token | Value | Rem |
|-------|-------|-----|
| spacing-8 | 8px | 0.5rem |
| spacing-16 | 16px | 1rem |
| spacing-40 | 40px | 2.5rem |

## CSS Custom Properties

### Colors

```css
--ls-color-yellow: #ffc233;
--ls-color-yellow-lighter: #ffd266;
--bullet-point-color: #6c6c89;
```

### Other

```css
--ls-purple: #5423e7;
```

### Semantic

```css
success: [object Object];
warning: [object Object];
error: [object Object];
info: [object Object];
```

## Breakpoints

| Name | Value | Type |
|------|-------|------|
| 390px | 390px | max-width |
| sm | 479px | max-width |
| sm | 480px | max-width |
| md | 768px | max-width |
| lg | 990px | max-width |
| lg | 991px | max-width |
| xl | 1280px | max-width |

## Transitions & Animations

### Common Transitions

```css
transition: all;
```

## Component Patterns

Detected UI component patterns and their most common styles:

### Buttons (16 instances)

```css
.button {
  color: rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
  padding-top: 0px;
  padding-right: 0px;
  border-radius: 0px;
}
```

### Cards (95 instances)

```css
.card {
  border-radius: 0px;
  padding-top: 0px;
  padding-right: 0px;
}
```

### Links (113 instances)

```css
.link {
  color: rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
}
```

### Navigation (193 instances)

```css
.navigatio {
  color: rgb(0, 0, 0);
  padding-top: 0px;
  padding-bottom: 0px;
  padding-left: 0px;
  padding-right: 0px;
  position: static;
}
```

### Footer (91 instances)

```css
.foote {
  color: rgb(0, 0, 0);
  padding-top: 0px;
  padding-bottom: 0px;
  font-size: 16px;
}
```

### Modals (7 instances)

```css
.modal {
  border-radius: 0px;
  padding-top: 0px;
  padding-right: 0px;
}
```

### Dropdowns (48 instances)

```css
.dropdown {
  border-radius: 0px;
  border-color: rgb(0, 0, 0);
  padding-top: 0px;
}
```

### Tabs (20 instances)

```css
.tab {
  color: rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
  padding-top: 0px;
  padding-right: 0px;
  border-color: rgb(0, 0, 0);
  border-radius: 0px;
}
```

### Switches (2 instances)

```css
.switche {
  border-radius: 0px;
  border-color: rgb(0, 0, 0);
}
```

## Component Clusters

Reusable component instances grouped by DOM structure and style similarity:

### Button — 6 instances, 1 variant

**Variant 1** (6 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Button — 1 instance, 1 variant

**Variant 1** (1 instance)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Button — 2 instances, 1 variant

**Variant 1** (2 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Button — 1 instance, 1 variant

**Variant 1** (1 instance)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Button — 8 instances, 1 variant

**Variant 1** (8 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Button — 1 instance, 1 variant

**Variant 1** (1 instance)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Button — 7 instances, 1 variant

**Variant 1** (7 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Button — 7 instances, 1 variant

**Variant 1** (7 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Button — 12 instances, 1 variant

**Variant 1** (12 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Card — 1 instance, 1 variant

**Variant 1** (1 instance)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Card — 4 instances, 1 variant

**Variant 1** (4 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Card — 11 instances, 1 variant

**Variant 1** (11 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Link — 18 instances, 1 variant

**Variant 1** (18 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Card — 18 instances, 1 variant

**Variant 1** (18 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Card — 18 instances, 1 variant

**Variant 1** (18 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Button — 9 instances, 1 variant

**Variant 1** (9 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Card — 1 instance, 1 variant

**Variant 1** (1 instance)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Card — 6 instances, 1 variant

**Variant 1** (6 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Card — 6 instances, 1 variant

**Variant 1** (6 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Card — 6 instances, 1 variant

**Variant 1** (6 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Button — 2 instances, 1 variant

**Variant 1** (2 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Button — 4 instances, 1 variant

**Variant 1** (4 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Button — 1 instance, 1 variant

**Variant 1** (1 instance)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

### Button — 5 instances, 1 variant

**Variant 1** (5 instances)

```css
  background: rgba(0, 0, 0, 0);
  color: rgb(0, 0, 0);
  padding: 0px 0px 0px 0px;
  border-radius: 0px;
  border: 0px none rgb(0, 0, 0);
  font-size: 16px;
  font-weight: 400;
```

## Layout System

**0 grid containers** and **0 flex containers** detected.

## Accessibility (WCAG 2.1)

**Overall Score: 100%** — 0 passing, 0 failing color pairs

## Design System Score

**Overall: 90/100 (Grade: A)**

| Category | Score |
|----------|-------|
| Color Discipline | 85/100 |
| Typography Consistency | 100/100 |
| Spacing System | 100/100 |
| Shadow Consistency | 85/100 |
| Border Radius Consistency | 100/100 |
| Accessibility | 100/100 |
| CSS Tokenization | 50/100 |

**Strengths:** Tight, disciplined color palette, Consistent typography system, Well-defined spacing scale, Clean elevation system, Consistent border radii, Strong accessibility compliance

**Issues:**
- No clear primary brand color detected
- 8 !important rules — prefer specificity over overrides
- 82% of CSS is unused — consider purging
- 12 duplicate CSS declarations

## SVG Icons

**3 unique SVG icons** detected. Dominant style: **filled**.

| Size Class | Count |
|------------|-------|
| xs | 2 |
| md | 1 |

**Icon colors:** `currentColor`

## Image Style Patterns

| Pattern | Count | Key Styles |
|---------|-------|------------|
| thumbnail | 3 | objectFit: fill, borderRadius: 0px, shape: square |

**Aspect ratios:** 1:1 (2x), 8.44:1 (1x)

## Component Anatomy

### card — 71 instances

**Slots:** heading

### button — 66 instances

**Slots:** label
**Variants:** primary · secondary
**Sizes:** small

| variant | count | sample label |
|---|---|---|
| primary | 29 | Get started for free |
| default | 27 | Read more |
| secondary | 10 | Download |

### link — 18 instances

**Variants:** link

| variant | count | sample label |
|---|---|---|
| link | 10 | e/1
Accept online payments

Don't worry  |
| default | 8 | M/1
Email marketing made easy

Combining |

## Brand Voice

**Tone:** neutral · **Pronoun:** we→you · **Headings:** Sentence case (balanced)

### Top CTA Verbs

- **get** (15)
- **read** (13)
- **download** (5)
- **r** (5)
- **e** (4)
- **explore** (4)
- **m** (3)
- **book** (2)

### Button Copy Patterns

- "get started for free" (8×)
- "read the case study" (8×)
- "explore the api" (4×)
- "read the docs" (4×)
- "download" (4×)
- "get on the list" (4×)
- "get started" (3×)
- "book a product tour →" (2×)
- "read more" (1×)
- "sign in
get started" (1×)

### Sample Headings

> Payments, tax & subscriptions for software companies
> trusted by thousands of companies globally
> Lemon Squeezy is the all-in-one platform for running your SaaS business. Payments, subscriptions, global tax compliance, fraud prevention, multi-currency support, failed payment recovery, PayPal integration and more. We make running your software business easy peasy.
> Global tax compliance
> Borderless SaaS payments
> Payments, tax & subscriptions for software companies
> trusted by thousands of companies globally
> Lemon Squeezy is the all-in-one platform for running your SaaS business. Payments, subscriptions, global tax compliance, fraud prevention, multi-currency support, failed payment recovery, PayPal integration and more. We make running your software business easy peasy.
> Global tax compliance
> Borderless SaaS payments

## Page Intent

**Type:** `landing` (confidence 0.31)
**Description:** Sell digital products and SaaS software the easy peasy way with Lemon Squeezy. As your merchant of record, we handle the tax compliance burden so you can focus on more revenue and less headache.

Alternates: blog-post (0.35)

## Section Roles

Reading order (top→bottom): nav → nav → nav → nav → nav → pricing-table → cta → content → content → testimonials → testimonials → content → pricing-table → testimonials → content → content → testimonial → testimonial → content → testimonials → pricing-table → content → content → cta → pricing

| # | Role | Heading | Confidence |
|---|------|---------|------------|
| 0 | nav | — | 0.9 |
| 1 | nav | — | 0.9 |
| 2 | nav | — | 0.9 |
| 3 | nav | — | 0.9 |
| 4 | nav | — | 0.9 |
| 5 | pricing-table | Payments, tax & subscriptions for software companies | 0.9 |
| 6 | cta | Payments, tax & subscriptions for software companies | 0.75 |
| 7 | content | trusted by thousands of companies globally | 0.3 |
| 8 | content | Lemon Squeezy is the all-in-one platform for running your SaaS business. Payment | 0.3 |
| 9 | testimonials | Global tax compliance | 0.4 |
| 10 | testimonials | Create the perfect buying experience for your customers | 0.4 |
| 11 | content | — | 0.3 |
| 12 | pricing-table | Accept online payments | 0.9 |
| 13 | testimonials | Grow your business with integrated marketing tools | 0.4 |
| 14 | content | — | 0.3 |
| 15 | content | — | 0.3 |
| 16 | testimonial | Email marketing made easy | 0.8 |
| 17 | testimonial | Understand your business and customers | 0.8 |
| 18 | content | — | 0.3 |
| 19 | testimonials | Understand your revenue | 0.4 |

## Material Language

**Label:** `flat` (confidence 0.55)

| Metric | Value |
|--------|-------|
| Avg saturation | 0.143 |
| Shadow profile | none |
| Avg shadow blur | 0px |
| Max radius | 0px |
| backdrop-filter in use | no |
| Gradients | 0 |

## Imagery Style

**Label:** `flat-illustration` (confidence 0.55)
**Counts:** total 3, svg 3, icon 2, screenshot-like 0, photo-like 0
**Dominant aspect:** square-ish
**Radius profile on images:** square

## Quick Start

To recreate this design in a new project:

1. **Install fonts:** Add `Times` from Google Fonts or your font provider
2. **Import CSS variables:** Copy `variables.css` into your project
3. **Tailwind users:** Use the generated `tailwind.config.js` to extend your theme
4. **Design tokens:** Import `design-tokens.json` for tooling integration
