# task231-sarsidelobe · 合成孔径雷达旁瓣污染诊断服务

基于 Go 实现的 SAR 成像旁瓣污染诊断后端服务。遥感成像工程师登记成像批次、
天线参数与局部峰值摘要，服务基于 sinc 方位向响应模型估计理论旁瓣位置、
比较峰值间距与幅度比、判定污染来源（旁瓣 / 姿态误差 / 真实强散射），
经人工复核后发布不可变诊断快照。

## 标准命令

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/sarsidelobe --smoke-test        # 全闭环自检（含持久化恢复验证）
go run ./cmd/sarsidelobe --addr :8080        # 启动长驻服务
```

## 核心算法

- 方位分辨率 `ρ_a = λR/(2L)`；第一旁瓣间距 `= 1.5·ρ_a`。
- 点目标方位向响应按 `|sinc|²` 建模，第一旁瓣衰减 ≈ 13.26 dB。
- 配对判定：峰值间距落入 `n·1.5·ρ_a ± 容差` 且幅度比在校准带内 → 旁瓣候选；
  间距不匹配但姿态误差超阈值 → 姿态候选；均不满足 → 真实强散射。
- 响应相似度 = `1 − |幅度比 − 理论旁瓣衰减| / 10`（0..1）。

## API 入口（/api 前缀，31 个操作）

批次 `GET/POST /api/batches`、`POST /api/batches/{id}/submit|review|confirm|archive`；
成像参数 `PUT/GET /api/batches/{id}/imaging-params`；
校准 `GET/POST /api/calibrations`、`POST /api/calibrations/{id}/activate`；
峰值 `POST/GET /api/batches/{id}/peaks`、`POST /api/peaks/{id}/scatter|exclude`；
分析 `POST /api/batches/{id}/analyze`；候选 `GET /api/batches/{id}/candidates`、
`POST /api/candidates/{id}/evidence|insufficient|confirm|reject`；
快照 `POST/GET /api/batches/{id}/snapshots`、`POST /api/snapshots/{id}/supersede`；
自检 `POST /api/selftest`、`GET /api/health`。

## 持久化

SQLite（modernc.org/sqlite，纯 Go 驱动）。表：batches / imaging_params /
calibration_versions / peak_regions / candidates / evidence / snapshots /
analysis_runs。区域按内容哈希幂等，封存批次只读，快照版本化不可变。
