# Reading a deprecation

- Under semver, marking an API deprecated is a minor release, and removing it is a major. The major releases between the two versions hold the removals.
- Under `0.y.z` anything may change at any time, so every release in the range gets read, not only the majors.
- Three places name what changes. The changelog or upgrade guide, the warnings the code emits at runtime, and the vendor's codemod list. One that can't be read is an unknown in the report.
- Each API names the version that deprecated it and the version that removes it. Neither known is "removal version unknown", never a guess.
