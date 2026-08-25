基于 Go 实现的合成孔径雷达旁瓣污染诊断 Web 项目，一款后端服务，估计强散射目标的旁瓣位置、比较方位向响应并发布带证据链的不可变诊断快照。

# BENZHI 评测说明 — task231-sarsidelobe

## 评测构建命令

```bash
bash build_benzhi_docker.sh docker-baseline-env linux/amd64
bash build_benzhi_docker.sh docker-baseline-env linux/arm64
```

## 双架构 smoke 契约

容器 ENTRYPOINT 固定为 `/out/sarsidelobe`，默认 CMD 为 `--smoke-test`。
评测仅传 flag、不追加路径参数：

```bash
docker run --rm --platform linux/amd64  docker-baseline-env:amd64  --smoke-test
docker run --rm --platform linux/arm64  docker-baseline-env:arm64  --smoke-test
```

`--smoke-test` 执行完整闭环：创建成像批次 → 登记 X 波段成像参数（ρ_a=3 m）
→ 激活 sinc 校准 → 注册峰值区域 → 并行旁瓣分析 → 候选自动复核（confirmed）
→ 发布快照 → 关闭并重开同一 SQLite 数据库验证持久化与重启恢复，全部成功以
退出码 0 结束并输出 `smoke-test: OK`。

## API 入口

所有路由以 `/api` 前缀暴露（健康检查 `GET /api/health`、批次 `GET/POST /api/batches`、
成像参数 `PUT /api/batches/{id}/imaging-params`、峰值 `POST /api/batches/{id}/peaks`、
分析 `POST /api/batches/{id}/analyze`、候选复核 `POST /api/candidates/{id}/confirm`、
快照发布 `POST /api/batches/{id}/snapshots` 等，共 31 个操作）。

## 持久化

SQLite（`modernc.org/sqlite` 纯 Go 驱动，CGO 无关），建表：batches /
imaging_params / calibration_versions / peak_regions / candidates / evidence /
snapshots / analysis_runs。相同批次 + 区域内容哈希幂等，封存批次与已发布
快照不可修改，快照版本化并可被新版本替代。
