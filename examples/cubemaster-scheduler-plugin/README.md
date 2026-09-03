# CubeMaster Custom Scheduler Plugin

A minimal, runnable example of writing a **custom CubeMaster scheduling
plugin** (a `filter.Selector` and/or a `score.Selector`) and enabling it purely
through configuration — without modifying the CubeMaster scheduler source.

## What's in here

| File | Purpose |
|------|---------|
| `agentcapacity_filter.go` | A custom **filter**: drops nodes without schedulable CPU capacity |
| `agentbalance_score.go` | A custom **scorer**: scores each node by its remaining quota ratio |
| `register.go` | Registers both plugins by name (`filter.Register` / `score.Register`) |
| `demo_test.go` | Compile-time interface assertions + a pipeline assembly demo |

## The three steps

### 1. Implement the standard interface

A **filter** implements `filter.Selector` (`Select`, `ID`). A **scorer**
implements `score.Selector` (`Select`, `ID`, `Weight`, `Disable`). See the two
files above.

### 2. Register by name

Registration is what makes the plugin addressable from configuration:

```go
filter.Register("agent_capacity", func() filter.Selector { return NewAgentCapacityFilter() })
score.Register("agent_balance",  func() score.Selector  { return NewAgentBalanceScore() })
```

`register.go` also provides an `init()` that auto-registers the moment the
package is imported into the CubeMaster binary. Registration happens at
assembly time — there is **no hot reload**; changes take effect on restart.

### 3. Enable, weight, and group from the scheduler YAML

Custom plugins are named exactly like built-ins and live in the same lists.
In `CubeMaster/conf.yaml` under `scheduler:`:

```yaml
scheduler:
  # direct enable + per-scorer weight/disable
  filter:
    enable_filters: [cpu, mem, agent_capacity]
  score:
    enable_scorers: [image_score, agent_balance]
    scorers:
      agent_balance: { weight: 0.8 }        # override the plugin's default weight

  # or pack them into a strategy Profile and switch the active one
  active_profile: "agents"
  profiles:
    agents:
      filters: [cpu, agent_capacity]
      scorers:
        agent_balance: { weight: 0.8 }
```

## Run it

```bash
cd examples/cubemaster-scheduler-plugin
go test ./...
```

`demo_test.go` asserts that both plugins satisfy the interfaces, that they are
registered, and that their names + weight override flow through the strategy
pipeline (`ResolvePipeline`).

## How this differs from editing CubeMaster

- You never touch `pkg/scheduler`, `pkg/selector/filter`, or
  `pkg/selector/score` internals.
- Your plugin is a normal Go package in your own module; the example uses a
  `go.work` that also includes CubeMaster for local building/testing.
- To ship it inside the service binary, import this package (or call
  `RegisterPlugins()`) from an assembly point, or vendor the files into the
  CubeMaster tree — either way it is picked up by name from configuration.

See the full guide: `docs/guide/cubemaster-scheduler-config.md`.