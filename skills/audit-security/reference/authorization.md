# Authorization

- Deny by default. A route with no check is open, and a new route needs an explicit rule.
- Every request is checked on the server, not most of them. One unchecked path is the finding.
- Two checks per request: may this caller use this function, and may this caller touch this object.
- An id from the URL, body, or query is attacker-controlled. Load the object scoped to the caller, or compare its owner, before acting.
- Hiding a button or a route in the client isn't a check.
- The finding quotes the line that enforces the check. For middleware or a decorator, quote it and the line applying it to this route.
