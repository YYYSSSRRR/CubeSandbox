# CubeMaster 自定义调度插件示例

一个最小可运行示例：编写 **CubeMaster 自定义调度插件**（`filter.Selector` 与/或
`score.Selector`），并**仅通过配置**启用它——完全不需要改动 CubeMaster 调度器源码。

## 目录内容

| 文件 | 作用 |
|------|------|
| `agentcapacity_filter.go` | 自定义 **filter**：剔除无可用 CPU 配额的节点 |
| `agentbalance_score.go` | 自定义 **score**：按节点剩余配额占比打分 |
| `register.go` | 按名注册两个插件（`filter.Register` / `score.Register`） |
| `demo_test.go` | 接口编译期断言 + 策略装配演示 |

## 三个步骤

### 1. 实现标准接口

**filter** 实现 `filter.Selector`（`Select`、`ID`）；**score** 实现
`score.Selector`（`Select`、`ID`、`Weight`、`Disable`）。见上方两个文件。

### 2. 按名注册

注册使插件可从配置中被寻址：

```go
filter.Register("agent_capacity", func() filter.Selector { return NewAgentCapacityFilter() })
score.Register("agent_balance",  func() score.Selector  { return NewAgentBalanceScore() })
```

`register.go` 同时提供 `init()`：包一旦被导入 CubeMaster 二进制即自动注册。注册发生在
装配期——**不支持热加载**，改动需重启生效。

### 3. 在调度 YAML 中启用、调权、组合

自定义插件与内置插件同名同级，出现在同样的列表中。在 `CubeMaster/conf.yaml` 的
`scheduler:` 下：

```yaml
scheduler:
  # 方式一：直接启用 + 单插件权重/禁用
  filter:
    enable_filters: [cpu, mem, agent_capacity]
  score:
    enable_scorers: [image_score, agent_balance]
    scorers:
      agent_balance: { weight: 0.8 }        # 覆盖插件默认权重

  # 方式二：打包进策略 Profile 并切换激活项
  active_profile: "agents"
  profiles:
    agents:
      filters: [cpu, agent_capacity]
      scorers:
        agent_balance: { weight: 0.8 }
```

## 运行

```bash
cd examples/cubemaster-scheduler-plugin
go test ./...
```

`demo_test.go` 断言：两个插件满足接口、已成功注册、其名字与权重覆盖能流入策略管线
（`ResolvePipeline`）。

## 与“改动 CubeMaster”的区别

- 无需触碰 `pkg/scheduler`、`pkg/selector/filter`、`pkg/selector/score` 内部。
- 插件是你自己模块里的普通 Go 包；本示例用 `go.work` 同时纳入 CubeMaster 以便本地
  编译/测试。
- 若要打进服务二进制：把本包（或 `RegisterPlugins()`）接到某装配点 import，或将文件
  并入 CubeMaster 代码树——两种方式都会按名字从配置被加载。

完整指南见：`docs/guide/cubemaster-scheduler-config.md`（及其中文镜像）。