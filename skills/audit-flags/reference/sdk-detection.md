# Finding flag references

- OpenFeature's evaluation methods take a flag key and a required default, one per type (boolean, number, string, structure). TypeScript and Java name them `getBooleanValue`, Python `get_boolean_value`, Go `BooleanValue`.
- A vendor SDK's evaluation calls come from its own docs, loaded once the manifest names it. The first argument is the key.
- A literal string is a key. Anything else is a dynamic reference, listed apart, and its flags can't be called unused.
- Code scanners match keys exactly as the service spells them. A key built at runtime is invisible to them, and to this audit.
- A homegrown helper is the function the project's flag reads funnel through. The report names it.
- Before calling a flag unread, check every service and app that might read it. A flag read only by a mobile app or another repo isn't orphaned.
