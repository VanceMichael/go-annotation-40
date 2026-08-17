# BENZHI_README

## 项目说明

- 项目：VanceMichael/go-annotation-40
- 项目用途：satnet 是一个纯 Go 实现的后端与命令行工具，覆盖卫星与地面站登记、 可见弧窗口计算、测控时隙切分、频轨资源分配、遥测帧入库与归档。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令

进入容器后执行：

```bash
# 编译
cd '/app' && GOTOOLCHAIN=local go build ./...

# 启动
cd '/app' && GOTOOLCHAIN=local go run ./cmd/satctl

# 测试
cd '/app' && GOTOOLCHAIN=local go test ./...
```

## Docker 构建和进入容器

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-task-40-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-task-40-arm64 linux/arm64
docker run -it benzhi-task-40-amd64:latest
docker run -it --platform linux/arm64 benzhi-task-40-arm64:latest
```

## 题目验证命令

1. 预期退出码 1：`go test ./internal/archive/ ./internal/cli/ -run "TestFlushReportsSegmentInvariantBreach|TestFlushDoesNotReturnEmptyArchiveOnFailure|TestCLIArchiveDrillReportsFailure" -count=1`

## Bug 复现

Bug 现象、触发步骤和完整错误信息见 `BUG_REPRO.md`。
