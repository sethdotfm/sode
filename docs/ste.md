DISCLOSURE: This was written by an LLM.

# Writing standard

Sode controls a live broadcast router. A misread word can put the wrong signal on air. All Sode text uses this standard, so that each sentence has one meaning for each reader.

This standard is based on ASD-STE100 Simplified Technical English (STE). The rules below are Sode's own words, not the STE text. The word table is Sode's own list. It is not the STE dictionary, and a word that is not in the table is not "STE-approved" because of that. For the full specification, request a copy from ASD (asd-ste100.org).

## Precedence

1. GLOSSARY.md has priority over this standard. Its terms are Sode's technical names. Do not replace a glossary term with a "simpler" word. Take, Route, Plan, Lock, Preset and the other glossary terms stay as they are.
2. Persisted and wire names (docs/persisted-names.md) do not change.
3. This standard has priority over general style guides.

## Scope

These rules apply to:

- UI text in `web/`: labels, buttons, tooltips, dialogs, errors and status text.
- Server text that a person can see: error messages and log text that the UI shows.
- Docs: README.md, RELEASING.md, GLOSSARY.md definitions, AGENTS.md, everything under `docs/`, the summaries and descriptions in `internal/app/openapi.yaml`, new CHANGELOG sections and `changelog.d/` fragments.
- Code comments in Go and JavaScript.

These rules do not apply to:

- Code, identifiers, text in backticks and fenced code blocks.
- Text in double quotation marks, when it quotes an example, a UI string or an outside source. The checker counts a quotation as one word.
- The `_Avoid_` lines and the bold term names in GLOSSARY.md.
- Search aliases in `web/settingsindex.js`. These aliases keep old words so that a search for them finds the setting.
- Past CHANGELOG sections, `docs/adr/`, `docs/research/`, `docs/superpowers/`, `.scratch/` and `web/vendor/`. These are records or quotations.

## Rules for words

1. Use one word for one thing. When the glossary has a term, use that term every time. Do not use a synonym to give variety.
2. Give each word one meaning. If a word has two meanings, use it with one meaning only, or use a different word. For example, "once" can mean "one time" or "after". Use "one time" or "after".
3. Use the plain word from the word table, not the word in the "Do not use" column.
4. Replace a word only with a word of the same meaning. "take" as an ordinary verb has many senses (get, accept, read, need, last). Pick the verb for that sense. Do not use one default replacement such as "use".
5. Use a verb as a verb. Do not make a verb into a noun when the verb can do the work. Write "Sode checks the name", not "Sode performs a check of the name". Glossary terms such as Take and Route are exceptions.
6. Do not use "-ing" words as verbs or adjectives, except in a technical name ("routing table", "Pending"). Write "When you send a take", not "When sending a take".
7. Write the words in full. Do not use contractions ("do not", not "don't"). Do not use Latin abbreviations ("for example", not "e.g.").
8. Spell out an abbreviation the first time that a page uses it. Protocol names (SW-P-08, TCP, PIN), GLOSSARY.md terms and common units (ms, s, KB) are exceptions. In prose, write "about" for "~" and "compared with" for "vs".

## Rules for noun clusters

9. Do not put more than three nouns in a row. Write "the name of the destination on the router", not "router destination name text".
10. Use the articles "the", "a" and "an". Do not leave them out to make a sentence shorter.

## Rules for verbs

11. Use the active voice. Name the actor: "Sode refuses the plan", not "the plan is refused". Use the passive voice only when the actor is unknown or not important.
12. Use simple tenses: the present, the past and the future ("refuses", "refused", "will refuse").
13. Keep ability, permission and prohibition apart. They are different facts.
    - Ability: "can", "cannot" ("the router cannot store more than 1024 names").
    - Permission given: "can" or "is allowed to".
    - Prohibition: "must not" or "is not allowed to". Never write "cannot" for a prohibition: "cannot" says that it is not possible, not that it is not allowed.
    - Requirement: "must".
    - Possibility: state the condition ("if the link drops, ..."), or write "it is possible that". Do not use "may" or "might".
    - Before you replace "may", decide which of these it means. If you cannot tell, keep it and flag it.

## Rules for sentences

14. Write one idea in one sentence.
15. A descriptive sentence has 25 words or fewer.
16. An instruction has 20 words or fewer.
17. A paragraph has six sentences or fewer, and one topic.
18. Put a condition before the instruction: "If the router is offline, Sode keeps the plan."

## Rules for instructions

19. Write an instruction as a command, in the imperative: "Select the destination."
20. Write one instruction in one sentence. If two actions occur at the same time, you can write them in one sentence.
21. Write the steps of a procedure as a numbered list, in the order that a person does them. When the steps describe what the code does, use the present tense, not the imperative.

## Rules for warnings and cautions

22. Start a warning with a short command that tells the person what to do or not to do. Then give the reason or the risk in a different sentence.
23. Do not soften a warning. Do not write "please" or "you might want to".

## Rules for UI text

24. A button or a menu item names the action as a verb, or names the thing it opens: "Send", "Undo", "Settings".
25. An error message tells the person what occurred and what to do next. Do not blame the person.
26. Status text always has words, as patchbay requires. The panel pages are the one exception (docs/panels-design.md).
27. Use the same words for the same action on every page.

## Rules for code comments

28. The comment rules in AGENTS.md also apply. A comment gives context only. It tells why, or it states an invariant, a quirk, a unit, a sentinel value or a link to code that must change with it.
29. Do not make a comment longer to make it obey a rule. If a rule makes a comment longer, write two short sentences.

## Word table

This is Sode's own list. Add a row when review finds a word that has two meanings or a plainer form. `TestSTEWords` in `internal/app/ste_test.go` checks each row that has a scan.

| Do not use | Use |
| --- | --- |
| utilize, leverage | use |
| ensure | make sure |
| prior to | before |
| in order to | to |
| via | through, with, by |
| facilitate | help |
| numerous | many |
| approximately | about |
| subsequently | then, after |
| e.g. | for example |
| i.e. | that is |
| etc. | a full list, or "and other" |
| simply, just (to mean "only" or "easily") | delete, or "only" |
| please (in an instruction) | delete |
| once (to mean "after") | after, when |
| in case (to mean "if") | if |
| whether or not | if |
| may, might | depends on the sense: see rule 13. "may not" (not allowed) becomes "must not", never "cannot" |
| don't, can't, won't, isn't, doesn't and other contractions | do not, cannot, will not, is not, does not |

## Checks

`internal/app/ste_test.go` checks the rules that a program can check:

- The word table, in all text in scope, and in the string literals of Go code that is not a test.
- Sentence length: 25 words for docs and comments, and 20 words for a UI string or a Go message.
- Contractions, with a straight or a curly apostrophe.

A reviewer checks the other rules. The reviewer reads for meaning first. A rewrite that changes a fact about the protocol or the router is a defect, even if it obeys every rule.
