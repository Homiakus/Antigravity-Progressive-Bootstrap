# Complete original master specification

> This file preserves the full pre-v2 master-prompt body. It is intentionally detailed and should be loaded only when the task needs that depth.

# Premium Editorial / ORGNZM-Inspired UI System

Act as a **Senior Art Director, Digital Product Designer, UI/UX Designer, Motion Designer, and Frontend Architect at Awwwards / CSS Design Awards level**.

Design or redesign `[PROJECT NAME]` so it uses a contemporary premium digital-studio visual language: monumental typography, strict Swiss grid, editorial composition, generous whitespace, minimal decorative UI, controlled asymmetry, and expressive but restrained motion.

Use ORGNZM Studio only as a stylistic reference. **Do not copy its logo, text, imagery, unique page composition, or branded elements. Reproduce principles and art-direction quality, not the site itself.**

Capture project inputs before implementation:
- Product type: website / web app / dashboard / portfolio / SaaS / Obsidian plugin / desktop app.
- Primary task.
- Target audience.
- Main user actions.
- Content and data.
- Technology stack if known.

## 1. Main visual principle

Treat the UI as one art-directed composition, not a collection of prefab components.

Target style:

**premium editorial minimalism + neo-Swiss typography + restrained brutalism + contemporary motion design.**

The primary expressive tools are:
- typography hierarchy;
- scale;
- rhythm;
- grid;
- negative space;
- size contrast;
- image cropping;
- thin rules;
- numbering;
- micro-typography;
- motion.

Every screen should feel deliberately composed rather than emitted by a component library.

## 2. What not to do

Avoid generic AI-generated SaaS aesthetics.

Do not:
- turn the UI into a set of identical 16–24px-radius cards;
- overuse pill buttons;
- add glassmorphism, neon glow, gradients, glowing borders, or blur without functional reason;
- use random purple-blue gradients;
- put every fragment in its own container;
- use excessive shadows;
- make the UI resemble Bootstrap, Material UI, Ant Design, or a default Tailwind dashboard;
- center everything automatically;
- make every section structurally identical;
- compensate for weak composition with decoration.

If an element can be expressed with typography, spacing, or a line, do not add another card.

## 3. Color system

Keep the base palette nearly monochrome.

Use a conceptual system such as:
- `--bg-primary`: very light warm white / bone / paper;
- `--bg-inverse`: near-black charcoal;
- `--text-primary`: near-black;
- `--text-inverse`: light;
- `--text-muted`: neutral gray;
- `--line`: text color at roughly 10–20% opacity;
- `--accent`: at most one project accent color.

Do not use accent everywhere. Roughly 80–95% of the interface should still work in grayscale. Create contrast mainly through scale, density, imagery, and composition.

## 4. Typography

Typography is the main design element.

Use a high-quality neo-grotesk / Swiss grotesk / contemporary sans-serif. If no brand font exists, choose a Helvetica/Neue Haas/Graphik/Suisse/Inter/Manrope/Satoshi-like family with good Cyrillic support. Use at most two families.

Display typography:
- Desktop: `font-size: clamp(64px, 9vw, 160px)` as a direction, not a rigid rule.
- Mobile: `font-size: clamp(42px, 13vw, 72px)` as a direction.
- Line-height about `0.88–1.02`.
- Slightly negative tracking.
- Deliberate line breaks.
- Headings should participate in composition rather than merely sit inside a block.

Section headings: approximately `clamp(36px, 5vw, 80px)`.

Body:
- usually 16–22px;
- keep comfortable readable measure;
- generally 45–70 characters per line for long text.

Micro-typography:
- 10–13px labels such as `ABOUT`, `01`, `LOCAL TIME`, `PROJECT / 2026`, `SCROLL TO EXPLORE`, `SELECTED WORK`, `STATUS`;
- uppercase and slightly increased tracking where appropriate;
- use micro-labels as a coordinate system for the page.

## 5. Grid

Use a strict 12-column desktop grid.

Prefer a wide container around `92–96vw` rather than a generic centered `1200px` container. Use generous external gutters.

Elements do not need to share one vertical start line. The composition may look asymmetric, but it must be grid-aligned.

Mix:
- full-width sections;
- two-column editorial compositions;
- three/four-column arrangements;
- large negative-space areas;
- images that intentionally cross conventional text boundaries.

Use large vertical spacing between major sections, roughly `12–24vh` where appropriate.

## 6. Containers and borders

Use minimal rounding.

Structural blocks may have no radius at all. Prefer:
- 1px borders;
- thin horizontal separators;
- grid division;
- background changes;
- whitespace.

Large media may have a small radius only when art direction calls for it. Do not turn the entire system into rounded cards.

## 7. Hero

For landing/marketing contexts, the hero may occupy about `100svh`.

Use several information layers:
- top: logo/product name, minimal navigation, primary action;
- center: enormous statement, product name, or short idea;
- secondary: concise product description in 1–3 lines;
- bottom: micro-metadata such as status, version, date, local time, category, or scroll cue.

Optionally add one dominant image, object, video, or 3D composition.

Do not overload hero with buttons. Usually one primary CTA and one secondary action is enough.

## 8. Sections

Treat each major section like an editorial chapter.

Combine:
- small index such as `01`;
- micro-label such as `ABOUT`;
- large statement;
- supporting text;
- visual object.

Example pattern:
- `01   SYSTEM`
- `Designed around the way you actually work.`

Change composition from section to section. Do not repeat one template throughout the page.

Vary rhythm:
- large text;
- image;
- dense data;
- whitespace;
- list;
- fullscreen visual;
- small annotation.

## 9. Numbers and metrics

Treat important numbers as independent visual anchors.

Examples:
- `86` / `PROJECTS`;
- `03 / 12` / `ACTIVE MODULES`.

Make the number much larger than its label. Metrics do not need cards; prefer grid + typography + dividing rules.

## 10. Lists and services

For services, functions, modules, or categories, prefer large horizontal rows over grids of identical cards.

Possible row structure:
- index;
- name;
- short description;
- metadata;
- arrow/action.

On hover, a row may alter background, image, arrow position, or show a visual preview.

## 11. Images

Use imagery as major compositional objects.

Prefer:
- art-directed photography;
- architecture;
- strong product renders;
- 3D;
- macro;
- textures;
- abstract objects;
- visualizations of the actual product.

Avoid generic stock photography.

Use intentional cropping with `object-fit: cover` where appropriate. Full-bleed images, very wide frames, vertical imagery, masks, and unusual aspect ratios are valid if compositionally justified.

Images should support the composition, not merely fill space.

## 12. Motion design

Motion should contribute to comprehension and atmosphere without obstructing navigation.

Use a smooth, calm motion language.

Possible patterns:
- staggered hero reveal;
- line-by-line or mask reveal for headings;
- clipping/masking image reveals;
- very light parallax;
- tactile link hovers such as arrow movement, image swap, text shift, or controlled underline.

Typical UI transitions: `180–450ms`.
Large cinematic transitions: `600–1000ms` only where justified.
Suggested easing direction: `cubic-bezier(0.22, 1, 0.36, 1)`.

Avoid bouncing and animating every element. Motion should direct attention.

Support `prefers-reduced-motion`.

## 13. Scroll experience

The page should feel like a sequence of scenes rather than stacked blocks.

Use:
- controlled whitespace;
- changes of scale;
- occasional contrasting backgrounds;
- sticky sections only when they clarify content;
- horizontal scrolling only when it materially improves project/media/timeline viewing.

Do not break familiar navigation models just for spectacle.

## 14. Navigation

Keep header/navigation light.

Avoid a massive navbar card. Use:
- logo;
- a few primary items;
- menu;
- one CTA.

Experimental desktop navigation is acceptable. Mobile navigation must stay obvious and fast.

Current position may be shown through a small index, section name, or subtle progress indicator.

## 15. Buttons

Minimize the number of visual button styles.

CTA patterns may include:
- text + arrow;
- minimal outline button;
- inverted button;
- text link with animated underline.

Do not make every action a bright filled button.

Provide hover/focus/active states and approximately 44px minimum touch target.

## 16. Forms

Avoid generic corporate forms.

Forms should feel integrated into the editorial system. Large choices can be shown as text options/selectable rows, e.g. services or budget ranges.

Fields may use thin dividers instead of heavy boxes. Labels must remain explicit. Active state should be high-clarity and minimal. Errors must be visible and accessible.

## 17. Dashboard / web-app mode

For an application, **do not copy landing-page structure**. Transfer only the visual language.

Use:
- strong type hierarchy;
- strict grid;
- minimal radius;
- thin dividers;
- restrained palette;
- micro-labels;
- large numbers;
- minimal decorative cards.

Group information by meaning, not because every metric “needs a card”. Tables should be clean and appropriately dense. Toolbars should be minimal. Sidebars should not resemble generic admin templates.

Use progressive disclosure. Show the important information first, reveal advanced detail when requested.

For engineering/data-heavy tools, aesthetics must never reduce work speed.

## 18. Mobile

Treat mobile as separate art direction, not a scaled desktop.

Move from a 12-column desktop grid to a 4-column mobile grid.

Preserve:
- large type character;
- whitespace;
- editorial rhythm.

Reduce extreme spacing appropriately. Convert complex split layouts into deliberate sequences. Keep primary actions reachable. Avoid accidental horizontal scroll.

Do not convert everything into identical mobile cards.

Large headings may wrap across 2–5 lines. Prefer `100svh` over relying exclusively on `100vh`.

## 19. Footer

Treat footer as a final scene.

Use a very large CTA or product name plus secondary system information:
- navigation;
- contact;
- version;
- year;
- location;
- status;
- social links;
- `BACK TO TOP`.

Do not use a tiny generic 100px footer. The ending should compositionally balance the hero.

## 20. Design tokens

Before implementation, define CSS variables/design tokens for:
- colors;
- typography scales;
- spacing scale;
- grid;
- container widths;
- borders;
- radii;
- motion durations;
- easing;
- z-index levels.

Use `clamp()` for fluid type and spacing. Avoid large sets of random values. Everything should feel like one system.

## 21. Technical implementation

When code is requested, produce production-quality implementation.

Use semantic HTML. Prefer CSS Grid and Flexbox. Prefer fluid CSS over excessive breakpoints.

Use CSS, Web Animations API, Motion, or GSAP according to actual complexity. Do not bring in a heavy library for one effect. Do not add WebGL/Three.js without visual need.

Use AVIF/WebP, lazy-load below-the-fold images, and avoid lazy-loading critical hero assets when that would hurt LCP. Prevent layout shifts.

Support keyboard navigation, focus states, and adequate contrast.

## 22. Content-first design

Do not use lorem ipsum in final implementation.

Study real content and determine:
- what is most important;
- what should appear first;
- what the primary action is;
- what can be removed;
- which data requires comparison;
- which details need progressive disclosure.

The design must amplify content rather than force content into predetermined components.

## 23. Asymmetry

Use controlled asymmetry.

Examples:
- heading spans 8 columns while description occupies 3 columns on the right;
- small section label is deliberately distant from main text.

Every unusual placement must still be anchored to the grid. Avoid chaos.

## 24. Space

Whitespace is an active design element.

Do not fill every region. Important messages should receive more surrounding space.

A slightly sparse but strongly composed interface is better than an overloaded one, except where professional data-heavy screens genuinely require density.

## 25. UX

Even with experimental visuals, the user must never wonder:
- where to click;
- what is selected;
- how to go back;
- what happened after an action;
- where the main information is.

Separate artistic expressiveness from interaction ambiguity.

## 26. States

Design beyond the happy path. Cover as applicable:
- loading;
- empty;
- error;
- offline;
- disabled;
- hover;
- focus;
- active;
- selected;
- success;
- skeleton;
- long content;
- overflow;
- mobile keyboard;
- slow network.

All states must use one coherent visual system.

## 27. Visual rhythm

Continuously vary visual density.

Example rhythm:

`monumental hero → whitespace → large statistics → immersive image → dense list → editorial statement → projects → quiet text → testimonials → enormous CTA`.

Avoid `card → card → card → card` repetition.

## 28. Final check

Before completion, run a dedicated visual-quality pass.

Ask:
- Can at least 20% of decorative containers be removed?
- Are there too many rounded cards?
- Is typography strong without imagery?
- Does the interface work in grayscale?
- Is there a clear visual rhythm?
- Is the grid perceptible?
- Is asymmetry intentional?
- Does it look unlike a generic SaaS template?
- Does character survive on mobile?
- Does motion ever interfere with task completion?
- Is the main action obvious on every screen?
- Is there at least one memorable compositional moment?

If not, revise.

## 29. Order of work

Do not start by coding components.

1. Study the current interface, content, and user scenarios.
2. Form a concise visual direction.
3. Create design tokens.
4. Define grid and responsive rules.
5. Rework information architecture where needed.
6. Design components and pages.
7. Perform a dedicated mobile UX pass.
8. Run visual consistency and accessibility audits.

## 30. Expected result

The final interface should feel like work from a contemporary European digital design studio.

It should be:
- minimalist but not empty;
- experimental but understandable;
- premium but not decorative;
- expressive but functional;
- typographic but readable;
- animated but calm;
- asymmetric but systematic.

Core formula:

**Typography × Grid × Whitespace × Art Direction × Controlled Motion × Functional Clarity.**

When choosing between another decorative component and stronger composition, choose composition.

When choosing between a spectacular animation and usability, choose usability.

If the existing interface already has working business logic, do not rewrite it merely for design. Preserve functionality and architecture; change visual system, layout, interaction hierarchy, and UX only where justified.

## Existing-codebase execution mode

When applied by a coding agent to an existing project:

1. Audit the current UI first.
2. Identify every place that violates this system.
3. Produce a file/module change map before implementation.
4. Preserve existing behavior unless a UX change explicitly requires otherwise.
5. Implement iteratively and verify each affected flow.
