---
type: regex
target: { source: file, path: site/landing.html }
pattern: "lorem ipsum|background-clip:\\s*text|backdrop-filter:\\s*blur|#(667eea|764ba2|8b5cf6|7c3aed|a855f7|6366f1)|\\b(elevate|seamless|supercharge|unleash|unlock|revolutioniz|game-chang|next-level)"
flags: i
match: not_contains
---
