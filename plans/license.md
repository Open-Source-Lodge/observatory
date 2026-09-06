# Plan: AGPLv3 license

## Current state

`LICENSE` is the GNU Affero General Public License, version 3, copied from
forestry. Nothing else in the repository names the license.

## Steps

1. Keep `LICENSE` as it is. The copyright holder is Stephan Nordnes Eriksen, because Open Source Lodge is not a registered organization yet. Write the holder and the year 2026 in the README, not in the license text: the text ends with a template, `Copyright (C) <year> <name of author>`, and that section is instructions, not a notice.
2. README: a "License" section: "Copyright (C) 2026 Stephan Nordnes Eriksen. Observatory is free software under the GNU AGPLv3. See `LICENSE`."
3. Check the dependencies: the Anthropic SDK (MIT), Bubble Tea, Bubbles, Lipgloss (MIT). All are compatible with the AGPLv3.
4. The GitHub Action distributes the binary to the runners of other people. The AGPLv3 asks for the source to be available; the repository is public, so this is met.
5. No SPDX headers in the source files. Forestry has none, and the license file is enough.

## Later

- When Open Source Lodge becomes a registered organization, change the holder in the README, or add the organization as a second holder.
