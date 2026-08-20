# Russian editorial reference

Load this reference only when the dominant prose language is Russian or the user requests Russian editorial normalization.

## Meaning before polish

- Preserve exact technical terms, proper names, part numbers, model names, commands, code, register names, measurements, dates, concentrations, percentages, and quoted wording.
- Do not replace a precise term with a more literary but less exact synonym.
- Do not strengthen a claim simply because a categorical sentence sounds cleaner.

## Grammar and syntax

Check:

- agreement in gender, number, and case;
- government after verbs/prepositions;
- dangling participial constructions;
- ambiguous pronouns;
- overloaded chains of genitives;
- repeated conjunctions;
- accidental syntactic calques;
- sentences whose grammatical subject changes unnoticed.

Prefer a clear verb when a chain of verbal nouns adds bureaucracy but no precision.

Do not mechanically shorten every long sentence. Split only when comprehension improves.

## Канцелярит and bureaucratic density

Reduce phrases such as:

- "осуществлять проведение" -> when "проводить" is equally precise;
- "производить настройку" -> often "настраивать";
- "в целях обеспечения" -> often a direct causal/purpose construction;
- redundant "данный", "вышеуказанный", "имеющийся" when they add nothing.

Keep formal/legal wording if its formality is functional.

## Ё / Е

Do not globally force one policy without a reason.

Default:

- preserve the document's established style;
- use `ё` when it prevents ambiguity or is part of a proper name;
- normalize globally only if the user or house style requires it.

## Вы / вы

Preserve the document's established convention.

Do not automatically capitalize `Вы` in every user-facing text.
Choose based on destination/house style if normalization is requested.

## Quotes

When typography normalization is requested and the document has no conflicting house style:

- first level: «…»
- nested: „…“ or another consistent Russian nested-quote convention

Do not alter quotation characters inside code or exact quoted source material unless requested.

## Dashes and hyphens

Distinguish:

- hyphen inside compound forms;
- dash as punctuation;
- minus sign in mathematical/technical expressions;
- numeric ranges according to the document's technical style.

Do not replace minus/hyphen characters inside commands, flags, identifiers, file names, model numbers, or code.

## Numbers and units

Preserve numeric value exactly unless correction is explicitly verified.

Check:

- consistent decimal separator according to audience/technical convention;
- spacing between number and unit where applicable;
- non-breaking spacing when typography matters;
- consistent percent/degree notation;
- range notation;
- thousands separators.

Never reformat a number in a way that changes machine-readable technical content.

## Common style defects

Look for:

- unnecessary introductory phrases;
- tautology;
- repeated roots in close proximity when distracting;
- weak "является" constructions when a direct predicate is clearer;
- excessive passive/impersonal wording hiding agency;
- vague "это", "данное", "указанное" without a clear antecedent;
- English-style noun stacking translated literally;
- overuse of colons and dash-driven pseudo-headlines;
- repeated "важно отметить", "следует отметить", "необходимо понимать".

Do not remove a repeated term when terminology consistency matters more than stylistic variety.

## Technical Russian

Prefer unambiguous operational phrasing.

Bad:
"После чего рекомендуется произвести осуществление проверки."

Better:
"После этого проверьте …"

But keep normative strength:
- "должен" is not interchangeable with "рекомендуется";
- "запрещено" is not interchangeable with "нежелательно";
- "можно" is not interchangeable with "требуется".

## Final Russian pass

Confirm:

- no accidental Latin/Cyrillic homoglyph substitutions;
- no broken units;
- no altered identifiers;
- quotation and dash style is consistent where normalized;
- prose does not sound like a literal translation from English;
- editorial cleanup did not introduce bureaucratic phrasing.
