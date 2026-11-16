# Classing a change

- Patch is for backward compatible bug fixes only.
- Minor is for new backward compatible functionality, or any public API marked deprecated.
- Major is for any backward incompatible change to the public API.
- Under `0.y.z` anything may change at any time, so a class there is advice.
- A deprecation ships in a minor release with the docs updated. The removal waits for a major.
- An accidental break is fixed by a new minor that restores compatibility. A released version is never changed.
- The spec counts only the declared public API. A surface the project never declared gets classed as if it were, and the report says so.
