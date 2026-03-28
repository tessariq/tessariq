# Tessariq Brand Guide

## Name

"Tessariq" originates from "tesseract" (the 4D hypercube), with a secondary connection to "tessera" (Latin for mosaic tile). Both etymologies inform the visual identity.

## Logo

### Primary Mark — Tesseract Mosaic

A 2×2 grid of square tiles inside a rounded-rectangle boundary, with diagonal corner lines connecting the inner grid corners to the outer boundary corners — a tesseract (hypercube) projection.

The four tiles follow a **clockwise spiral opacity fade**, giving each tile a unique brightness:

| Position     | Opacity |
|-------------|---------|
| Top-left     | 1.0     |
| Top-right    | 0.7     |
| Bottom-right | 0.4     |
| Bottom-left  | 0.30    |

The spiral differentiates the mark from uniform 2×2 grids and creates directional energy that maps to the product workflow: run → execute → evidence → recede.

#### Geometry (canonical 160×160 viewBox)

- **Outer boundary:** `rect x=10 y=10 w=140 h=140 rx=4`, stroke only, stroke-width 2, opacity 0.25
- **Tiles (8px gap):**
  - TL: `rect x=38 y=38 w=38 h=38 rx=2`
  - TR: `rect x=84 y=38 w=38 h=38 rx=2`
  - BR: `rect x=84 y=84 w=38 h=38 rx=2`
  - BL: `rect x=38 y=84 w=38 h=38 rx=2`
- **Corner connections (4 diagonal lines):**
  - TL: `(10,10)→(38,38)` / TR: `(150,10)→(122,38)`
  - BL: `(10,150)→(38,122)` / BR: `(150,150)→(122,122)`
  - Stroke width 1.5, opacity 0.2

#### Scaling

| Size    | Context       | Adjustments                                                |
|---------|---------------|------------------------------------------------------------|
| 128px+  | Full icon     | All elements rendered                                      |
| 64px    | GitHub avatar | Corner lines visible, slightly thicker strokes             |
| 40px    | Small icon    | Drop corner lines, increase tile gaps slightly, rx=3       |
| 16px    | Favicon       | Drop corner lines, larger tile gaps, rx=4, boundary stroke thickens to ~8 |

At 16px, the favicon uses optimized geometry for pixel clarity: tiles shift to 42×42 at positions (32,32)/(86,32)/(86,86)/(32,86), boundary rx increases to 14, stroke-width thickens to 8, and opacities compress to 1.0 / 0.65 / 0.35 / 0.25. The favicon is petrol dark only — no theme switching, matching industry convention for consistent browser-tab recognition.

### Secondary Mark — Dimensional Stack

Three filled, overlapping squares receding in depth with wireframe connection lines between layers. Reserved for **large-format contexts only** (100px+).

#### Geometry (canonical 160×160 viewBox)

- **Back layer:** `rect x=46 y=20 w=80 h=80 rx=5`
- **Middle layer:** `rect x=33 y=37 w=80 h=80 rx=5`
- **Front layer:** `rect x=20 y=54 w=80 h=80 rx=5`
- **Connection lines:** 8 lines connecting corresponding corners between layers

#### Layer opacities

| Color mode    | Front | Middle | Back |
|--------------|-------|--------|------|
| Mono on dark  | 1.0   | 0.3    | 0.1  |
| Mono on light | 1.0   | 0.2    | 0.06 |
| Petrol        | 1.0   | 0.45   | 0.2  |

Petrol uses higher opacities because the darker hue needs more contrast to remain visible.

#### When to use

- Landing page hero sections
- Conference slides
- Social preview backgrounds
- Never for icons, avatars, or small contexts (degrades to a generic document-stack shape)

### Lockup Variants

- **Horizontal:** Icon (48px) + wordmark, side by side with 1.5rem gap. For: headers, navigation, docs.
- **Stacked:** Icon (72px) above wordmark. For: social previews, splash screens.

## Typography

### Wordmark — Sora

| Property       | Value                                               |
|---------------|-----------------------------------------------------|
| Font           | Sora                                                |
| Source         | BunnyFonts (OFL-1.1)                                |
| Weight         | 300 (Light)                                         |
| Letter-spacing | 0.18em                                              |
| Text-transform | lowercase                                           |
| URL            | `https://fonts.bunny.net/css?family=sora:300`       |

The name reads as one atomic unit — no split, separator, or camelCase. Sora's squarish proportions and asymmetric 't' crossbar give the wordmark a precise, engineered feel that complements the icon without competing with it.

### UI — Albert Sans

| Property | Value                                                                  |
|----------|------------------------------------------------------------------------|
| Font     | Albert Sans                                                            |
| Source   | BunnyFonts (OFL-1.1)                                                   |
| URL      | `https://fonts.bunny.net/css?family=albert-sans:300,400,500,600`       |

| Context      | Weight |
|-------------|--------|
| Headlines    | 600    |
| UI / buttons | 500    |
| Body text    | 400    |
| Light body   | 300    |
| Code         | System monospace |

Albert Sans provides humanist warmth that balances the geometric precision of both the icon and Sora wordmark. It shares geometric DNA with Sora but is clearly distinct, creating proper visual hierarchy.

## Color

### Monochrome (default)

| Background | Fill color |
|-----------|------------|
| Dark      | `#ffffff`  |
| Light     | `#111111`  |

Tile differentiation is achieved through opacity alone.

### Accent — Petrol

| Theme      | Hex       |
|-----------|-----------|
| Dark mode  | `#0891b2` |
| Light mode | `#155e75` |

Petrol was chosen over standard teal to avoid generic SaaS aesthetics. It reads as industrial and infrastructure-grade, matching the product's positioning.

### Rules

- Monochrome is the default for all contexts.
- Petrol accent is optional — use when color adds value (marketing, website, social).
- Never mix monochrome and petrol in the same mark.
- On light backgrounds, always use `#155e75`, never the dark-mode `#0891b2`.

## Usage Quick Reference

| Context              | Mark      | Color       | Lockup      |
|---------------------|-----------|-------------|-------------|
| GitHub repo avatar   | Primary   | Mono        | Icon only   |
| README header        | Primary   | Mono        | Horizontal  |
| Favicon              | Primary   | Petrol      | Icon only   |
| Website nav          | Primary   | Petrol      | Horizontal  |
| Social preview       | Primary   | Petrol      | Stacked     |
| Landing page hero    | Secondary | Petrol      | Decorative  |
| CLI output           | Primary   | Mono        | Icon only   |
| Docs site            | Primary   | Mono/Petrol | Horizontal  |

## Assets

All production SVGs live in `assets/logo/`. Wordmark lockups use outlined text (no external font dependency).

| File pattern                    | Description                    |
|--------------------------------|--------------------------------|
| `icon-{color}-{theme}.svg`     | Primary mark (4 variants)      |
| `favicon.svg`                  | Simplified primary mark (petrol dark only) |
| `lockup-horizontal-*.svg`      | Icon + wordmark (4 variants)   |
| `lockup-stacked-*.svg`         | Stacked layout (2 variants, dark only) |
| `hero-{color}-{theme}.svg`     | Secondary mark (4 variants)    |
