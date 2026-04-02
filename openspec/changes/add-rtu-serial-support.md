---
title: 新增 Modbus RTU Serial 支援
type: feature
status: completed
created: 2026-04-02
---

# 新增 Modbus RTU Serial 支援

## 變更內容
讓 scanner 支援透過 USB-to-RS485 轉接器直連設備，使用 Modbus RTU 協定掃描暫存器。
- ScanRequest 新增 `mode` 欄位（`tcp` / `rtu`），預設 `tcp` 向下相容
- RTU 模式需要 `serial_port`（如 `/dev/cu.usbserial-xxx`）、`baud_rate`、`data_bits`、`stop_bits`、`parity`
- 新增 `/api/serial/ports` API 列出可用 serial port（方便前端選擇）
- scanner.go 抽出 client 建立邏輯，TCP/RTU 共用 `modbus.Client` 介面
- 前端加 RTU 模式切換

## 影響範圍
- `internal/scanner/scanner.go` — ScanRequest 新增欄位、client 建立邏輯
- `internal/scanner/router.go` — 新增 serial ports API、handleRead 支援 RTU
- `web/scanner-ui/src/` — 前端新增 RTU 模式 UI

## 測試計畫
1. TCP 模式掃描不受影響（向下相容）
2. RTU 模式可列出 serial ports
3. 接上 USB-RS485 後可掃描台達 Unolite
