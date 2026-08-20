# Technical editorial reference

Use for software, engineering, scientific, laboratory, regulatory, manufacturing, SOP, API, hardware, and systems text.

## Immutable technical tokens

Treat these as protected unless the task explicitly includes correcting them:

- code;
- CLI commands;
- command flags;
- environment variables;
- paths;
- filenames;
- API symbols;
- type/function/class names;
- register names/addresses;
- GPIO/pin names;
- protocol fields;
- part/model numbers;
- firmware/software versions;
- URLs;
- hashes;
- IP addresses;
- ports;
- units and numeric values;
- reagent names/concentrations;
- tolerances;
- standards identifiers.

## Instruction quality

Each procedure should make clear:

1. prerequisite/state;
2. action;
3. target;
4. expected result;
5. validation;
6. failure/recovery path when relevant.

Avoid narrative filler between operational steps.

## Requirements language

Keep normative strength stable.

Map consistently:

- mandatory: MUST / SHALL / требуется / должен;
- recommendation: SHOULD / рекомендуется;
- permission/option: MAY / можно;
- prohibition: MUST NOT / запрещено.

Never weaken a safety or mandatory requirement during stylistic editing.

## Terminology

Create an internal term map for long documents.

Detect:

- one concept using multiple accidental names;
- one name used for multiple concepts;
- abbreviations expanded inconsistently;
- translated/non-translated variants drifting.

Prefer consistent repetition of an exact technical term over decorative synonym variation.

## Numbers and units

Run an explicit numeric integrity pass.

Compare source vs edited text for:

- all numbers;
- signs;
- decimals;
- percentages;
- units;
- ranges;
- tolerances;
- time values;
- version numbers.

A smoother sentence is not acceptable if a numeric token changed.

## Code-adjacent prose

Do not introduce "smart" typography into code/commands:

- curly quotes;
- Unicode dash replacing `-`;
- ellipsis replacing `...`;
- non-breaking spaces;
- localized decimal separators.

Keep prose typography and machine-readable syntax separate.

## Safety and uncertainty

Do not turn:

"may cause"
into:
"causes"

Do not turn:
"typically"
into:
"always"

Do not delete warnings because they feel repetitive.

When a statement appears safety-critical and ambiguous, preserve the ambiguity or flag it rather than inventing a resolution.

## Tables and structured data

Do not rewrite table labels into inconsistent terminology.

Check:

- units in headers;
- column alignment semantics;
- repeated enum/status values;
- numeric formatting;
- row/column references from surrounding prose.

## Final technical gate

- procedure remains executable;
- all referenced objects still exist by the same names;
- values/units match source;
- normative language preserved;
- warnings preserved;
- definitions are consistent;
- cross-references remain correct;
- examples are clearly examples, not requirements.
