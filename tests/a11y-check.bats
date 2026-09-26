#!/usr/bin/env bats
# bin/a11y-check.sh with a fake axe: digest format, and the exit codes for a
# missing axe (3) and a URL that does not answer (4).

setup() {
  load helper
  kit_test_home
  export HOME="$FAKE_HOME"
  SCRIPT="$BATS_TEST_DIRNAME/../bin/a11y-check.sh"
  REPO="$BATS_TEST_TMPDIR/repo"
  git init -q -b main "$REPO"
  cd "$REPO"
  printf '<html></html>\n' >page.html
  PAGE="file://$REPO/page.html"
}

# A fake axe in the repo's node_modules/.bin that prints fixture results.
fake_axe() {
  mkdir -p node_modules/.bin
  cat >node_modules/.bin/axe <<'SH'
#!/usr/bin/env bash
cat <<'JSON'
[{"violations":[
  {"id":"color-contrast","impact":"serious","tags":["cat.color","wcag2aa","wcag143"],
   "nodes":[{"target":[".btn-primary"]},{"target":[".link"]}]},
  {"id":"region","impact":"moderate","tags":["cat.keyboard","best-practice"],
   "nodes":[{"target":[["my-card","p.note"]]}]}
]}]
JSON
SH
  chmod +x node_modules/.bin/axe
}

@test "digest: one line per rule with impact, wcag tags, node count, selector" {
  fake_axe
  run "$SCRIPT" "$PAGE"
  [ "$status" -eq 0 ]
  [[ "${lines[0]}" == "a11y-check: 2 violated rule(s) on $PAGE (raw: "*"/.claude/scratch/a11y-runtime-"*".json)" ]]
  [ "${lines[1]}" = "color-contrast  serious  wcag2aa,wcag143  nodes=2  .btn-primary" ]
  [ "${lines[2]}" = "region  moderate  -  nodes=1  my-card p.note" ]
  raw="${lines[0]##*raw: }"
  jq -e '.[0].violations | length == 2' "${raw%)}"
}

@test "missing axe exits 3 with the install command" {
  if command -v axe >/dev/null; then skip "axe is installed globally"; fi
  run "$SCRIPT" "$PAGE"
  [ "$status" -eq 3 ]
  [[ "$output" == *"npm install -D @axe-core/cli"* ]]
}

@test "a URL that does not answer exits 4 and names the start script" {
  fake_axe
  printf '{"scripts":{"dev":"vite","storybook":"storybook dev"}}\n' >package.json
  touch pnpm-lock.yaml
  run "$SCRIPT" http://127.0.0.1:9/
  [ "$status" -eq 4 ]
  [[ "$output" == *"Start it with: pnpm run storybook"* ]]
}
