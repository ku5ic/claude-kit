#!/usr/bin/env bats
# Command-line parsing matrix for hooks/guard-bash.sh: every way a command
# can hide inside another (heredocs, substitutions, quotes, grouping,
# wrappers) must still reach its check, and data must not look like
# commands. One test, so a failure lists every mismatch at once.

load helper

setup() {
  HOOK="$BATS_TEST_DIRNAME/../hooks/guard-bash.sh"
  mismatches=()
}

# probe <block|ask|allow|pass> <command>
probe() {
  local out rc verdict=pass
  out="$(hook_payload Bash "$2" "" "$BATS_TEST_TMPDIR" | "$HOOK" 2>&1)" && rc=0 || rc=$?
  ((rc == 2)) && verdict=block
  ((rc == 0)) && [[ "$out" == *'"ask"'* ]] && verdict=ask
  ((rc == 0)) && [[ "$out" == *'"allow"'* ]] && verdict=allow
  ((rc != 0 && rc != 2)) && verdict="error-$rc"
  if [[ "$verdict" != "$1" ]]; then
    mismatches+=("got $verdict want $1: ${2//$'\n'/\\n}")
  fi
}

@test "parsing matrix: hidden commands are checked, data isn't" {
  local R='rm -rf ~'
  # Heredocs: bodies are data, unless fed to a shell or never terminated.
  probe block $'cat <<EOF > "$(scratch-dir.sh)/n.txt"\nit\'s done\nEOF\ngit push --force origin main'
  probe pass $'git commit -F - <<\'EOF\'\nfix(x): map a -> b\nEOF'
  probe block $'bash <<EOF\n'"$R"$'\nEOF'
  probe block $'cat <<EOF | sh\n'"$R"$'\nEOF'
  probe block $'cat <<EOF > "$(scratch-dir.sh)/x"\nbody\nEOF\n'"$R"
  probe block $'cat <<-EOF\n\tx\n\tEOF\n'"$R"
  probe block $'cat <<-EOF\n  x\n  EOF\n'"$R"
  probe pass $'cat <<-EOF > "$(scratch-dir.sh)/x"\n\trm -rf ~ is text\n\tEOF'
  probe block $'cat <<EOF\n'"$R"
  probe block $'cat <<EOF\nx\nEOFX\n'"$R"
  probe block $'cat <<A <<B\na\nA\nb\nB\n'"$R"
  probe pass $'cat <<A <<B > "$(scratch-dir.sh)/x"\na\nA\nb\nB'
  probe pass $'cat <<"EOF" > "$(scratch-dir.sh)/x"\n$(rm -rf ~)\nEOF'
  probe pass $'cat <<\\EOF > "$(scratch-dir.sh)/x"\n$(rm -rf ~)\nEOF'
  probe block $'cat <<"EOF"\nx\nEOF\n'"$R"
  probe block $'x=$(cat <<EOF\nit\'s\nEOF\n)\n'"$R"
  probe block $'echo "$(cat <<EOF\nhi\nEOF\n)"; '"$R"
  # Here-strings and shifts aren't heredocs.
  probe block $'cat <<<"foo"\n'"$R"
  probe block $'cat <<< foo\n'"$R"
  probe block $'cat <<<foo; '"$R"
  probe block $'(( x = 1 << 2 ))\n'"$R"
  probe block $'x=$((1<<2))\n'"$R"
  probe block $'let "x = 1 << 2"\n'"$R"
  # Substitutions, quoted or not, including process substitution.
  probe block 'echo $(true && git push --force origin main)'
  probe block 'git status && echo "$(git push --force origin main)"'
  probe block 'echo `git push --force origin main`'
  probe block 'echo "$(echo "$(rm -rf ~)")"'
  probe block 'echo $( (rm -rf ~) )'
  probe block 'x=`echo \`rm -rf ~\``'
  probe pass "echo '\$(rm -rf ~)'"
  probe pass "echo '\`rm -rf ~\`'"
  probe block 'echo ")"; rm -rf ~'
  probe block 'echo "(" && rm -rf ~'
  probe block 'echo $(echo ")"); rm -rf ~'
  probe block 'cat <(rm -rf ~)'
  probe block 'diff <(git push --force origin main) x'
  probe block 'tee >(rm -rf ~) < x'
  # Comments.
  probe pass 'echo hi # rm -rf ~ in a comment'
  probe block 'echo "a#b"; rm -rf ~'
  probe block $'echo a # c\n'"$R"
  probe block 'echo $#; rm -rf ~'
  probe block 'echo ${#x}; rm -rf ~'
  probe pass '# rm -rf ~'
  probe block $'# only a comment\n'"$R"
  # Separators outside quotes only.
  probe block $'echo a\n'"$R"
  probe block 'echo "a;b" ; rm -rf ~'
  probe block "echo 'a&&b' && rm -rf ~"
  probe pass 'git commit -m "a && rm -rf ~ in the message"'
  probe block 'curl "https://x.test/f?a=1&b=2" -O'
  probe block 'true&x&rm -rf ~'
  probe block 'ls & rm -rf ~'
  probe block 'sleep 1 & git push --force origin main'
  probe block $'ls \\\n; rm -rf ~'
  probe pass 'ls 2>&1 | head'
  probe pass 'ls &>/dev/null'
  probe pass 'ls |& head'
  probe pass 'echo "unterminated; rm -rf ~'
  probe block "echo \$'a\\'b'; rm -rf ~"
  probe pass "echo \$'it\\'s fine'"
  # Grouping, keywords, functions, wrappers, eval.
  probe block '(rm -rf ~)'
  probe block '{ rm -rf ~; }'
  probe block 'if true; then rm -rf ~; fi'
  probe block 'for f in a; do rm -rf ~; done'
  probe block '! rm -rf ~'
  probe block 'time rm -rf ~'
  probe block '( cd /tmp && git push --force origin main )'
  probe pass '(ls)'
  probe pass 'if true; then echo ok; fi'
  probe block 'f() { rm -rf ~; }; f'
  probe block 'function f { rm -rf ~; }; f'
  probe block 'exec rm -rf ~'
  probe block 'nohup rm -rf ~'
  probe block 'nice rm -rf ~'
  probe block 'eval "rm -rf ~"'
  # Command names written other ways.
  probe block '/bin/rm -rf ~'
  probe block '"rm" -rf ~'
  probe block "r''m -rf ~"
  probe block '\rm -rf ~'
  probe block '/usr/bin/git push --force origin main'
  # Unwrapping ( ) and { } keeps ${HOME} and $(...) arguments whole.
  probe block 'rm -rf ${HOME}'
  probe block 'rm -rf "${HOME}"'
  probe block 'rm -rf $HOME'
  probe block 'rm -rf ~/'
  probe block '(rm -rf ${HOME})'
  probe block '{ rm -rf ${HOME}; }'
  probe block 'chmod -R +x ${HOME}'
  probe block 'chmod -R 777 ~'
  probe block 'rm -rf "$(echo ~)"; rm -rf ${HOME}'
  # Overlay writes by any command, not only >, sed -i, and sd.
  local O="$BATS_TEST_TMPDIR/home/.claude/claude-kit.local.yml"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME/.claude"
  touch "$O"
  probe ask 'tee -a ~/.claude/claude-kit.local.yml'
  probe ask 'echo x | tee ~/.claude/claude-kit.local.yml'
  probe ask 'cp /tmp/x ~/.claude/claude-kit.local.yml'
  probe ask 'mv /tmp/x "$HOME/.claude/claude-kit.local.yml"'
  probe block 'dd if=/tmp/x of=~/.claude/claude-kit.local.yml'
  probe ask 'rsync /tmp/x ~/.claude/claude-kit.local.yml'
  probe ask 'truncate -s 0 ~/.claude/claude-kit.local.yml'
  probe ask 'cat /tmp/x | /usr/bin/tee ~/.claude/claude-kit.local.yml'
  probe block 'chmod -R +x "${HOME}"'
  probe block "rm -rf '~'"
  probe block 'rm -rf "/"'
  probe ask 'install -m 644 /tmp/x ~/.claude/claude-kit.local.yml'
  probe ask 'ln -sf /tmp/x ~/.claude/claude-kit.local.yml'
  probe pass 'cat ~/.claude/claude-kit.local.yml'
  probe pass 'yq . ~/.claude/claude-kit.local.yml'
  probe pass 'rg disabled ~/.claude/claude-kit.local.yml'
  # The commit-message heredoc inside $( ): quotes and parens in it are data.
  probe block $'git commit -m "$(cat <<\'EOF\'\nIt\'s fine\nEOF\n)" && rm -rf ~'
  probe block $'git commit -m "$(cat <<\'EOF\'\nfix: x (y\nEOF\n)" && rm -rf ~'
  probe pass $'git commit -m "$(cat <<\'EOF\'\nIt\'s fine (really)\nEOF\n)"'
  probe block $'x=$(cat <<EOF\n# it\'s a comment\nEOF\n); rm -rf ~'
  probe block $'x=$(echo a # it\'s\n); rm -rf ~'
  # Overlay: yq -i and git checkout write it; plain reads don't ask.
  probe ask "yq -i '.disabled_rules += [\"rm-recursive\"]' ~/.claude/claude-kit.local.yml"
  probe ask 'git checkout HEAD -- ~/.claude/claude-kit.local.yml'
  probe ask 'perl -pi -e s/a/b/ ~/.claude/claude-kit.local.yml'
  probe pass 'git log -- ~/.claude/claude-kit.local.yml'
  # Downloads: .. never escapes scratch, even behind $(scratch-dir.sh).
  probe block 'curl -o "$(scratch-dir.sh)/../../x" https://x.example/a'
  probe block 'wget -P "$(scratch-dir.sh)/.." https://x.example/a'
  # Wrappers with their own options.
  probe block 'nice -n 10 rm -rf ~'
  probe block 'nice -10 rm -rf ~'
  probe block 'command -p rm -rf ~'
  probe block 'env -i rm -rf ~'
  probe block 'env -u PATH rm -rf ~'
  probe block 'env FOO=1 rm -rf ~'
  probe block 'env -i FOO=1 nice -n 5 rm -rf ~'
  probe block 'timeout 5 rm -rf ~'
  probe block 'timeout -s KILL 5 rm -rf ~'
  probe block 'stdbuf -o0 rm -rf ~'
  probe block 'exec -a x rm -rf ~'
  probe pass 'env'
  probe pass 'command -v rm'
  # git-base.sh: an explicit ask, so a settings allow rule can't approve it.
  probe allow 'git-base.sh --diff'
  probe ask 'git-base.sh --diff --output=/tmp/x'
  probe ask 'git-base.sh --log --ext-diff'
  probe block 'git-base.sh --diff --output=/tmp/x; rm -rf ~'
  # Ordinary commands stay quiet.
  probe pass 'git status'
  probe pass "jq '.a > 1' f.json"
  probe pass "rg '<div>' src"
  probe pass 'curl -s https://x.example | jq .'

  if ((${#mismatches[@]})); then
    printf '%s\n' "${mismatches[@]}"
    return 1
  fi
}
