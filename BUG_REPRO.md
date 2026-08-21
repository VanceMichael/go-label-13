# Bug Reproduction

## 包的性质

当前 test_model_fix 保存的是被测模型修复后的结果源码，不是初始含 Bug 源码。要复现原始缺陷，必须检出下面固定的 parent SHA；不要在当前修复结果源码上期待重新出现修复前失败。生成系统使用的可信验证补丁和完整验证日志仅在本地留存，不提交到结果分支。

## 问题现象

调度员给同一票货物在原航段上增加重量，系统会先腾出旧舱位再建立替代预留。最近审计存储短暂故障时，改配接口虽然报失败，原来的 60 公斤预留却变成失效，新建的 80 公斤预留反而继续占着容量；后续重试也找不到可用的原分配。请修复改配失败后的状态交接：任何下游确认失败都要恢复旧预留并撤销替代预留，而正常确认仍只保留新分配，容量统计必须与最终结果一致。

## 含 Bug 版本

- 仓库：VanceMichael/go-label-13
- 仓库地址：https://github.com/VanceMichael/go-label-13.git
- parent SHA：a4f458039b19fb69f826e961e80dddc48c35b470

## 复现步骤

```bash
git clone -- https://github.com/VanceMichael/go-label-13.git bug-repro
cd bug-repro
git checkout --detach a4f458039b19fb69f826e961e80dddc48c35b470
go test ./internal/booking -run ^TestFailedSameLegRebookingRestoresOriginalAllocation$ -count=1
```

## 双架构完整错误信息

### linux/amd64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/booking -run ^TestFailedSameLegRebookingRestoresOriginalAllocation$ -count=1
--- FAIL: TestFailedSameLegRebookingRestoresOriginalAllocation (0.00s)
    change_test.go:58: original allocation after failure = {ID:shipment-13-1787302358305831801 Request:{TenantID:cargo-east ShipmentID:shipment-13 LegID:PVG-FRA WeightKg:60 Priority:0 RequestedAt:2026-08-21 04:00:00 +0000 UTC} Accepted:false Reason:rebooking CreatedAt:2026-08-21 08:52:38.306387551 +0000 UTC}, want restored
FAIL
FAIL	github.com/VanceMichael/go-base-airbridge/internal/booking	0.031s
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
$ go test ./internal/booking -run ^TestFailedSameLegRebookingRestoresOriginalAllocation$ -count=1
--- FAIL: TestFailedSameLegRebookingRestoresOriginalAllocation (0.00s)
    change_test.go:58: original allocation after failure = {ID:shipment-13-1787302404597092878 Request:{TenantID:cargo-east ShipmentID:shipment-13 LegID:PVG-FRA WeightKg:60 Priority:0 RequestedAt:2026-08-21 04:00:00 +0000 UTC} Accepted:false Reason:rebooking CreatedAt:2026-08-21 08:53:24.597102044 +0000 UTC}, want restored
FAIL
FAIL	github.com/VanceMichael/go-base-airbridge/internal/booking	0.002s
FAIL

```

stderr：

```text
(empty)
```

## 通过条件

同一航段的加重改配必须能够临时让出原舱位并建立替代预留；审计确认失败时，原 60 公斤分配恢复为有效且原因清空，80 公斤替代记录不再占用容量，航段可用量回到 40。随后下游确认正常时，改配应成功保留 80 公斤新分配并把可用量更新为 20。TestFailedSameLegRebookingRestoresOriginalAllocation 需要由红转绿，booking 回归、全仓 go test ./... 与 go build ./... 均通过，不得交换步骤以破坏加重成功路径，也不得删改状态或容量断言。
