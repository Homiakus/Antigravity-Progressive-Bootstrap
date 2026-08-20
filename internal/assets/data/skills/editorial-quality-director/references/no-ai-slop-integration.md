# no-ai-slop integration protocol

`no-ai-slop` is a subordinate specialized editor/detector.

## When to load it

Load it when:

- the user asks to remove AI-sounding prose;
- the user asks whether a text "reads like AI";
- a rewrite/polish task contains obvious generic rhetorical scaffolding;
- publication QA would benefit from an anti-slop pass.

Do not load it automatically for:

- a one-word typo;
- pure spellcheck;
- exact legal/technical transformations where stylistic rewriting is prohibited;
- code/config-only tasks.

## Ordering

Preferred order for substantial edits:

1. preserve meaning/facts;
2. fix structure/logic;
3. copyedit;
4. run no-ai-slop;
5. language-specific proofread;
6. final regression QA.

Why: running anti-slop first can optimize sentences that later need structural deletion or movement.

## Conflict policy

If a no-ai-slop recommendation conflicts with:

1. user instruction;
2. factual accuracy;
3. quotation/source fidelity;
4. technical precision;
5. established terminology;
6. deliberate author voice;

reject or adapt that recommendation.

## Diagnose-only behavior

When the user asks whether a text contains slop:

- detect patterns;
- cite concrete excerpts sparingly;
- explain why the pattern weakens the text;
- do not claim AI authorship;
- do not rewrite unless asked.

## Rewrite behavior

When rewriting:

- replace generic abstraction with source-supported specificity;
- remove empty scaffolding;
- vary rhythm naturally;
- keep strong original phrases;
- preserve intentional repetition;
- preserve emotional temperature.

Do not "humanize" by adding fake anecdotes, opinions, personal experiences, uncertainty, slang, or errors.
