# cube-sched-bench

A reproducible, offline **CubeMaster scheduler simulator**. It replays a
workload against one or more scheduling strategies on a synthetic node pool and
prints the same metric families CubeMaster exposes for scheduling, so different
strategies (the legacy `default` pipeline vs. the built-in agent profiles) can
be compared under identical conditions.

This is a **model**, not the real scheduler: node quota, template caching and
create latency are simulated, and every run is deterministic for a given
`-seed`. Use it to explore strategy trade-offs, not to predict absolute
numbers.

## Run

```bash
go test ./...          # determinism + strategy sanity checks

# One workload x one set of strategies (default = all four)
go run . -workload burst -ticks 200 -nodes 8 -seed 42
go run . -workload template-heavy -ticks 60 -nodes 6 -seed 7
go run . -workload mixed -ticks 200 -nodes 8 -seed 42

# Any combination
go run . -workload burst -strategies default,agent-burst -seed 7 -nodes 6 -ticks 60

# Template cache "drift" (nodes lose cached images periodically) is on by
# default so locality effects stay visible; tune or disable it with:
#   -drift-every 0          (disable)
#   -drift-every 8 -drift-prob 0.5
```

## Workloads (the three exam papers)

| `-workload` | Scenario | What matters |
|---|---|---|
| `burst` | high-concurrency, short-lived sandboxes | spread load, low P95, success |
| `template-heavy` | repeated creation of a few templates | local image hits |
| `mixed` | mixed specs, some long-lived | packing vs. affinity stability |

## Strategies

- `default` — the legacy pipeline modelled as a load-balancing scorer.
- `agent-burst`, `agent-template-heavy`, `agent-mixed` — the built-in profiles;
  their weights are read from `CubeMaster/pkg/base/config` 
  (`DefaultAgentProfiles`), the single source of truth shared with the
  scheduler's configuration defaults.

## Columns

| Column | Meaning |
|---|---|
| succ% | share of requests that found a node |
| pack% | average CPU quota utilisation across ticks |
| balance stddev | stddev of per-node final CPU utilisation (lower = more even) |
| hit% | share of templated creations on a node that already had the template |
| p50 / p95 (ms) | create-latency percentiles **for templated creations only** (model: base 120 ms, +500 ms remote pull on a miss) |

`vs default` lines show the delta relative to the `default` strategy.

## Reading the results honestly

- Template caches are periodically invalidated ("drift": nodes lose their
  cached images on a schedule) so locality effects do not saturate. Drift is
  deterministic and identical across strategies — only a strategy's placements
  respond to it differently.
- The image-weighted strategy (`agent-template-heavy`) concentrates copies on
  still-cached nodes, so expect **higher hits and a lower P95** (fewer remote
  pulls) **but a higher balance stddev** (hotter nodes) than the spreading
  `default`. That trade-off is the point of the comparison.
- Under pressure (`mixed` at high packing) the spreading `default` can show the
  best success rate because concentrated strategies occasionally exhaust a hot
  node.