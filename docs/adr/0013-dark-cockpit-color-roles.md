# Dark cockpit color roles

Each color in Sode has one job, and a normal, healthy state shows no color. This is the "dark cockpit" rule: when nothing needs attention, the page is quiet, so a color that shows is a signal. The table of roles is in [docs/color-roles.md](../color-roles.md).

The rules:

1. A normal, healthy state shows no color.
2. Color shows only a state that needs attention or a change in progress.
3. Each color has one job.
4. Every colored state also has text, an icon or a shape.

Identity colors (source, destination, level, tie line, the page border) say what a thing is and can always show. State colors say what is happening and follow the rules on every page. Red is failure only. Amber is caution: a warning, unsaved work and every Take. Green is routed only: the current source on a destination and a Take that just succeeded. The accent is selection, focus and the main action (`sode-btn--primary`). Panel pages keep their colors, since a panel is an input device and its fills say what each cell is.

Before this, green had four jobs (ok status, the current source, the main action of a form and steady healthy states) and red had three (failure, the armed Take and delete buttons). A person could not tell from a color alone what it meant, and a healthy page was full of green.

Take is amber, not red, because red is now only for failure. A Take is a change that needs care, not a failure. The armed pulse and the key text ("Take (T)") keep Take apart from a Save with unsaved work, which is also amber.

The fourth rule is not optional. Patchbay's red and green differ in OKLCH lightness by only 0.04 (light) and 0.10 (dark), so for a color-blind user the text, icon or shape is the only difference.

Considered: a setting for a quieter or a louder mode. Rejected: two modes double what a person must learn and what a check must cover. Also considered: a red Take, kept apart from failure by its shape. Rejected: a failed Take next to a red Take button reads as one thing.
