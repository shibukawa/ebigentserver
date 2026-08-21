# tinybind — configbind `enum` feedback

Downstream report from building the `ebigent` configuration layer on configbind.

- Module: `github.com/shibukawa/tinybind-go` (v0.5.17 at time of writing)
- Surfaces: `docs/configbind.md`, `configbind/codegen`, `generator/configbind.go`
- Reported from: `github.com/shibukawa/ebigentserver`, package `config/`
- Status: **feedback** — one documentation fix, two cheap wins, one design question, one TBD answered

---

## 1. Summary

`rule:enum-value-validation` already records the state accurately:

> not implemented for config. configbind reads the tag, but only to check the values a
> `decision:dependon-value-condition` names against it; no source value is rejected yet

Nothing below contradicts that. The report is about the consequences that the state marker
does not cover, in rough order of cost to fix:

| # | Finding | Kind |
|---|---|---|
| 1 | `docs/configbind.md` presents `enum` as an allowlist with no note that config does not enforce it | documentation |
| 2 | `cli_and_scaffold` of `rule:enum-value-validation` is also unimplemented — neither help nor scaffold lists the choices | cheap win |
| 3 | The same tag *is* enforced in request binding, so the two binders disagree on one tag | expectation |
| 4 | The array-of-tables element ban groups `enum` with `dependon` and `falsy`, but only the other two need what the ban cites | design |
| 5 | `TBD_policy` for `[]string` — a concrete use case argues for per-element matching | answers a TBD |

---

## 2. What the code does today

| Claim | Evidence |
|---|---|
| The tag is read at generation | `generator/configbind.go:386`, the comment naming `rule:enum-value-validation` as unimplemented |
| It is used only for `dependon` values | `configbind/codegen/generate_test.go:523` — "value outside the parent's enum" |
| It reaches no generated artifact | `internal/configbindfixture/configbind_gen.go` contains no `enumOK`; `grep -r Enum configbind/*.go` is empty |
| It reaches neither flags nor scaffold | generated `FlagMetas` and `Scaffold` entries carry `Key`, `Kind`, `Default`, `Opt`, `Help` — no allowlist |
| Request binding does enforce it | `internal/checkfixture/tinybind_gen.go:212` emits `enumOK` and `must be one of: asc, desc` |

Reproduction, against a field tagged `enum:"standalone,listen,dedicated,p2p"`:

```toml
[run]
topology = "peer2peer"
```

`configbind.Load` returns no error and the field binds to `peer2peer`. Pinned downstream as
`TestLoadItselfDoesNotEnforceTheEnumTag`.

---

## 3. Finding 1 — the user-facing doc promises enforcement

`docs/configbind.md:191` and `docs/configbind.ja.md:191`:

```
| `enum:"a,b,c"` | Allowlist of accepted values | `enum:"oidc_only,jwt_only"` |
```

Every other row of that table describes behavior that exists. Nothing on the page says this
one is inert for config, and `## What is automated` lists `enum` beside `default`, `key`, and
`help` without qualification. A reader who tags a field and moves on ships a silent typo.

The knowledge catalog is honest about the state; the documentation a user actually reads is
not. Suggested minimum: a note on the row, or in `## Struct tags`, saying the config load does
not yet reject unlisted values and that callers should validate after `Load` until it does.

## 4. Finding 2 — help and scaffold could list the choices now

`rule:enum-value-validation` already allows for this:

```yaml
cli_and_scaffold:
  - usage/help may list allowed values
  - scaffold comments may list allowed values
```

Neither happens. A generated scaffold renders:

```toml
# execution topology of this process
topology = "standalone"
```

where it could render the choices it already knows at generation time:

```toml
# execution topology of this process (one of: standalone, listen, dedicated, p2p)
topology = "standalone"
```

This is worth separating from the load-time check because it is much cheaper — the allowlist
only has to reach `ScaffoldField` and `cliparser.FieldMeta` — and because it recovers most of
the practical value on its own. A developer reading a scaffold or `--help` sees the choices,
which is where the typo would otherwise be made.

Two further generation-time checks are cheap in the same pass, and neither needs the runtime:

- a `default` value outside its own `enum` — `rule:enum-value-validation` requires config
  defaults to be listed, so this is a generation error waiting to be written;
- a `falsy` value outside its own `enum`, as in `enum:"off,otlp,jaeger" falsy:"off"`, where a
  typo silently disables the emptiness test that `dependon` rides on.

## 5. Finding 3 — the two binders disagree on one tag

`enum:"asc,desc"` on a query field produces a generated check and a `must be one of` field
error. The identical tag on a config field produces nothing. Same tag, same generator, opposite
behavior, and nothing at the call site distinguishes them.

This is the part that surprised us rather than the missing check itself. Whatever the
implementation order ends up being, one sentence in the doc saying which binders enforce the
tag would have saved the investigation.

## 6. Finding 4 — the element-field ban cites a reason `enum` does not share

`configbind/codegen/generate.go:739` rejects `enum` on a field of an array-of-tables element:

```
fields of an array-of-tables element have no provenance key for dependon, falsy, or enum
```

The reason is sound for the other two tags. `dependon` names a key to be referenced; `falsy`
fills a key in and is compared against by other keys. Both need a stable configuration key, and
an element field has none.

`rule:enum-value-validation` needs no key. It needs the value in hand at apply time, which the
element overlay has. The grouping looks like it came from the tag being introduced for
`dependon`'s sake — the test comment says as much:

> The generator only started reading the tag for the sake of a `dependon` condition, so its
> placement has to be decided rather than silently dropped.

The same comment argues "an enum tag names a value, which neither a struct nor an array element
has". That holds for the array itself, and for a nested struct. It does not hold for a *field*
of an element, which is a scalar with a value like any other.

We hit this directly. `[[build.target]]` has a `kind` that must be one of four, and
`[[run.slot]]` has a controller `kind` that must be one of six. Both are exactly the shape
`enum` is for, and both had to be hand-validated. Suggested outcome: when the load-time check
lands, allow `enum` on element fields and keep the ban for `dependon` and `falsy`, with the
error message split so it stops naming a reason that applies to only two of the three.

## 7. Finding 5 — `[]string` policy

`rule:enum-value-validation` leaves this open:

```yaml
for []string with enum TBD_policy: each element must match or whole field must match one value
```

Our one instance argues for per-element:

```go
Enable []string `enum:"websocket,webtransport,webrtc" help:"listeners this process may open"`
```

The field is a set drawn from a fixed vocabulary — the natural reading of an allowlist on a
list. "Whole field matches one value" would mean a list-valued field that may hold exactly one
of several fixed *lists*, which we cannot construct a use for; a field like that is a mode key,
and would be written as a scalar with its own `enum`.

Per-element also degrades better: an unlisted element names itself in the error, where a
whole-field match can only report that the list as a whole is wrong.

---

## 8. What downstream does meanwhile

Recorded here so the cost of the gap is visible, not as a complaint — the workaround is fine.

Every allowlist is restated in Go beside the struct that carries the tag:

```go
// They are duplicated here on purpose. In tinybind-go v0.5.17 the enum
// tag reaches neither the generated code nor the loader ...
var (
	topologies    = []string{"standalone", "listen", "dedicated", "p2p"}
	baselineModes = []string{"speculative", "confirmed_only", "bounded_speculation"}
	// ...
)
```

`Validate` walks them, and our `Load` wrapper runs `Validate` before returning so an unlisted
value fails the load rather than binding. The duplication is the whole cost: two lists that can
drift, with nothing to catch the drift, because the tag is invisible at runtime and the scaffold
does not echo it. Finding 2 alone would remove most of that — a scaffold that prints the choices
gives the duplication something to be checked against.

We also pin the current behavior in a test that will fail when the check lands, which is the
signal to delete the workaround:

```go
func TestLoadItselfDoesNotEnforceTheEnumTag(t *testing.T)
```

---

## 9. Suggested order

1. Doc note on `docs/configbind.md:191` and the `.ja` mirror — minutes, removes the trap.
2. Carry the allowlist into `ScaffoldField` and `cliparser.FieldMeta`; render it in scaffold
   comments and option help. Add the generation-time `default` and `falsy` membership checks in
   the same pass.
3. Implement `rule:enum-value-validation` at load, per-element for `[]string`.
4. With the check in place, allow `enum` on array-of-tables element fields and split the
   error message at `configbind/codegen/generate.go:739`.
