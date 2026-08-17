# Bug Reproduction

## 包的性质

当前 test_model_fix 保存的是被测模型修复后的结果源码，不是初始含 Bug 源码。要复现原始缺陷，必须检出下面固定的 parent SHA；不要在当前修复结果源码上期待重新出现修复前失败。生成系统使用的可信验证补丁和完整验证日志仅在本地留存，不提交到结果分支。

## 问题现象

遥测归档失败了却一声不响，上游把「没有可归档数据」当成正常结果，那批遥测就这么没了。

```
$ ./satctl archive drill --segment-bytes 512
{
  "segment_bytes_limit": 512,
  "ingested_frames": 11,
  "oversized_frames": 1,
  "segments": 0,
  "archived_frames": 0,
  "flush_reported": false,
  "error_is_archive_write": false,
  "message": "",
  "ok": false
}
错误: model: 遥测归档与明细不一致: 存在 1 帧超过单段字节上限, 但归档未上报任何失败（返回 0 个段）
$ echo $?
6
```

这条演练命令把单段字节上限压到 512，而样例里有一帧是 4096 字节，段容量不变量必然被突破。按 README，这种情况归档函数要把失败转换成归档写出失败上报出来，不能把它当成「本批没有可归档数据」。实际 `message` 是空的、`flush_reported` 是 false，归档返回了 0 个段和 nil 错误。

入库那 11 帧是实打实进去了的（`ingested_frames` 是 11），归档却交出 0 个段还说一切正常。

`GET /api/archive/flush` 在正常上限下没问题，但一旦触发这个路径同样静默返回。

对照现象：

- `archive flush`（默认单段 8192 字节，放得下最大的那一帧）完全正常，能产出归档段，且归档帧数与入库明细一致。
- `archive flush --segment-frames 0` 这种参数非法的情况能正常报错，返回归档写出失败。

所以「参数校验」这条失败路径是通的，唯独段容量不变量被突破这条路径失败了却报成功。

请先不要修改代码。先帮我定位根因，讲清楚为什么这条失败路径会变成「0 个段 + nil 错误」、为什么参数非法那条失败路径反而能正常报错，并给出实际执行过的复现命令与观察到的输出作为证据。结论确认后再讨论怎么改。

## 含 Bug 版本

- 仓库：VanceMichael/go-annotation-40
- 仓库地址：https://github.com/VanceMichael/go-annotation-40.git
- parent SHA：1ceb0a71478a5d7bb826cfdea460d6d5b3c4034e

## 复现步骤

```bash
git clone -- https://github.com/VanceMichael/go-annotation-40.git bug-repro
cd bug-repro
git checkout --detach 1ceb0a71478a5d7bb826cfdea460d6d5b3c4034e
go test ./internal/archive/ ./internal/cli/ -run "TestFlushReportsSegmentInvariantBreach|TestFlushDoesNotReturnEmptyArchiveOnFailure|TestCLIArchiveDrillReportsFailure" -count=1
```

## 双架构完整错误信息

### linux/amd64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/archive/ ./internal/cli/ -run "TestFlushReportsSegmentInvariantBreach|TestFlushDoesNotReturnEmptyArchiveOnFailure|TestCLIArchiveDrillReportsFailure" -count=1
--- FAIL: TestFlushReportsSegmentInvariantBreach (0.00s)
    archive_test.go:66: 段容量不变量被突破时必须上报失败，实际返回 nil
--- FAIL: TestFlushDoesNotReturnEmptyArchiveOnFailure (0.00s)
    archive_test.go:78: 归档失败时必须上报错误，实际返回 0 个段且 err 为 nil
FAIL
FAIL	satnet/internal/archive	0.077s
--- FAIL: TestCLIArchiveDrillReportsFailure (0.01s)
    app_test.go:128: 归档演练退出码应为 0, 实际 6
        错误: model: 遥测归档与明细不一致: 存在 1 帧超过单段字节上限, 但归档未上报任何失败（返回 0 个段）
FAIL
FAIL	satnet/internal/cli	0.055s
FAIL

```

stderr：

```text
(empty)
```

### linux/arm64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/archive/ ./internal/cli/ -run "TestFlushReportsSegmentInvariantBreach|TestFlushDoesNotReturnEmptyArchiveOnFailure|TestCLIArchiveDrillReportsFailure" -count=1
--- FAIL: TestFlushReportsSegmentInvariantBreach (0.00s)
    archive_test.go:66: 段容量不变量被突破时必须上报失败，实际返回 nil
--- FAIL: TestFlushDoesNotReturnEmptyArchiveOnFailure (0.00s)
    archive_test.go:78: 归档失败时必须上报错误，实际返回 0 个段且 err 为 nil
FAIL
FAIL	satnet/internal/archive	0.015s
--- FAIL: TestCLIArchiveDrillReportsFailure (0.00s)
    app_test.go:128: 归档演练退出码应为 0, 实际 6
        错误: model: 遥测归档与明细不一致: 存在 1 帧超过单段字节上限, 但归档未上报任何失败（返回 0 个段）
FAIL
FAIL	satnet/internal/cli	0.003s
FAIL

```

stderr：

```text
(empty)
```

## 通过条件

目标仓库零改动（git status 干净，无新增、修改或删除文件）。
准确指出出问题的 Go 文件与具体符号。
说明段容量不变量被破坏时归档失败为什么没有被转换成错误返回，而是变成了「零个归档段 + nil 错误」，并解释这如何让调用方把失败当成「本批没有可归档数据」、使已入库的 11 帧被静默丢弃。
解释为什么参数非法（段容量上限为 0）这条失败路径仍能正常返回 model.ErrArchiveWrite，从而说明两条失败路径的上报方式并不一致、症状只出现在其中一条上。
给出实际执行过的复现命令与观察到的输出作为证据，而非仅凭阅读代码推断。
