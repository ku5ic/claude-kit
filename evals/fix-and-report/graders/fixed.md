---
type: regex
target: { source: file, path: calc.sh }
pattern: 'i = 1; i <= \$#|for [a-z_]+ in "\$@"|for [a-z_]+; do'
---
