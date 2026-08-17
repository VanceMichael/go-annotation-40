# satnet —— 低轨卫星互联网星座测控与频轨资源调度平台

`satnet` 是一个纯 Go 实现的后端与命令行工具，覆盖卫星与地面站登记、
可见弧窗口计算、测控时隙切分、频轨资源分配、遥测帧入库与归档。

项目不依赖任何第三方模块，只使用 Go 标准库。

## 业务背景

### 卫星状态与可测控性

状态取值为 `launched`、`commissioning`、`operational`、`safe-mode`、`deorbited`。
**在轨运行与安全模式都可以安排测控窗口**；已发射、在轨测试与已离轨不安排测控窗口。

### 可见弧与 UTC 日界

可见弧是卫星在某地面站上空的可跟踪时段。按 UTC 自然日归集时，
**跨日界的可见弧要被裁剪成该自然日内的片段，而不是整段归给某一天**：
一段 23:50 延续到次日 00:10 的可见弧，在前一日贡献 10 分钟、在次日也贡献 10 分钟。

由此得到一条守恒关系：**各自然日裁剪后的累计时长之和必须等于全部可见弧的累计时长**，
裁剪既不丢失也不重复计算时长。

可见弧重叠判定采用左闭右开口径：前一段的结束时刻等于后一段的开始时刻时互不冲突。

### 测控时隙分组

可见弧按固定长度切成测控时隙，不足最小时长的尾段被丢弃。
时隙按卫星分组，**每一组都持有独立存储：向任意一组追加时隙都不会影响其他组**，
也不会改动传入的时隙清单。同一分组集合内时隙编号必须互不相同，
且每个时隙都必须出现在与自身卫星编号一致的分组里。

### 频轨资源分配

各频段总带宽为 S 频段 20MHz、Ku 500MHz、Ka 1000MHz、Q/V 2000MHz。
**任何时刻已授权带宽都不得超过该频段总带宽**；剩余带宽不足时返回
`model.ErrBandExhausted` 并计入拒绝数，不改动已授权量。

并发申请下仍须满足：**成功申请的次数等于授权记录条数，
每个频段的已授权带宽等于该频段全部授权记录之和**。

### 遥测入库

逐帧校验，遇到第一帧校验失败时停止入库。
**该失败必须通过返回值上报给调用方**：返回的入库帧数为失败前已入库的数量，
错误非 nil 且错误链上保留 `model.ErrFrameChecksum`。
不得在有帧校验失败的情况下返回 nil 错误。

### 遥测归档

归档段有帧数与字节数两个硬容量。段容量被突破意味着上游切分逻辑已失效，
属于内部不变量破坏，以 panic 上抛，**由归档函数统一转换为
`model.ErrArchiveWrite` 错误返回给调用方**；
不得把归档失败当成「本批没有可归档数据」的空归档静默返回。

正常路径下，归档段内的帧数与字节数必须与入库明细完全一致。

## 目录结构

```
cmd/satctl                命令行入口
internal/model            领域模型与哨兵错误
internal/constellation    卫星与地面站登记、可测控性判定
internal/pass             可见弧裁剪与按自然日归集
internal/window           测控时隙切分与按卫星分组
internal/spectrum         频轨资源并发分配与释放
internal/telemetry        遥测帧校验与入库
internal/archive          遥测归档段写出
internal/report           星座规模、覆盖、时隙与频轨报表
internal/httpapi          HTTP 接口
internal/seed             内置样例数据
internal/cli              satctl 命令实现
```

## 构建与测试

```bash
export GOTOOLCHAIN=local

go build ./...
go test ./...
go test -race ./...      # 频轨并发分配需配合 -race 检查

make build           # 产出 bin/satctl
make selfcheck       # 构建并运行内置自检
```

## 命令行用法

```bash
satctl sat list
satctl sat show --id SAT-2401
satctl station list

satctl pass list
satctl pass list --day 2026-08-17
satctl pass coverage                      # 校验按日裁剪的时长守恒

satctl window plan
satctl window extend --satellite SAT-2401 # 校验追加时隙不影响其他分组

satctl spectrum allocate --workers 64 --per-worker 5 --band ka --mhz 10

satctl telemetry ingest                   # 含损坏帧，应上报失败
satctl telemetry ingest --clean           # 只入库校验通过的帧

satctl archive flush
satctl archive drill --segment-bytes 512  # 校验归档失败被如实上报

satctl report scale|coverage|schedule|spectrum
satctl serve --addr 127.0.0.1:8080
satctl selfcheck
```

`archive drill` 把单段字节上限故意设得小于最大帧长，使段容量不变量必然被突破，
用于校验归档失败会被上报：输出中的 `flush_reported` 与
`error_is_archive_write` 都应为 true。

### 退出码约定

| 退出码 | 含义 |
| --- | --- |
| 0 | 成功 |
| 1 | 用法错误或未归类的内部错误 |
| 2 | 参数非法 |
| 3 | 业务冲突（频段带宽耗尽、测控窗口冲突、卫星不可测控） |
| 4 | 上行通道被取消或超时 |
| 5 | 资源不存在 |
| 6 | 数据一致性问题（帧校验失败、归档写出失败、归档与明细不一致、频段超额分配、时隙分组串扰） |

## HTTP 接口

```
GET /healthz
GET /api/satellites
GET /api/satellites/{id}
GET /api/stations
GET /api/passes[?day=2026-08-16]
GET /api/report/coverage
GET /api/report/scale
GET /api/report/schedule
GET /api/telemetry/ingest
GET /api/archive/flush
```

错误响应统一为：

```json
{ "error": { "code": "archive_write_failed", "message": "..." } }
```

状态码约定：`404` 资源不存在，`409` 业务冲突，`422` 数据一致性问题，
`400` 参数非法，`502` 上行通道不可用，`503` 调用被取消或超时，
`500` 未归类的内部错误。

> 说明：HTTP 接口默认不带鉴权，仅面向内网或本地演练环境。若需暴露到公网，
> 必须在前置网关补充身份认证与访问控制。

## 容器运行

```bash
docker build -t satnet:local .
docker run --rm satnet:local selfcheck
docker run --rm -p 8080:8080 satnet:local serve --addr 0.0.0.0:8080
```

镜像基于 `golang:1.22` 构建、`distroless/static` 运行，同时支持
`linux/amd64` 与 `linux/arm64`：

```bash
docker build --platform linux/amd64 -t satnet:amd64 .
docker build --platform linux/arm64 -t satnet:arm64 .
```

## 数据来源

内置样例数据（`internal/seed`）包含 5 颗卫星（覆盖 5 种状态）、4 个地面站、
7 段可见弧（其中 2 段跨越 UTC 日界）与 12 帧遥测（其中 1 帧损坏、1 帧长度偏大）。
仅用于本地演练，不代表真实测控数据。
