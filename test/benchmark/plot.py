#!/usr/bin/env python3
"""
mydb-go 并发基准测试结果可视化
用法：python3 plot.py [results.csv]
输出：benchmark_tps.png 和 benchmark_latency.png
"""

import sys
import csv
import os
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.ticker as ticker

# 设置中文字体
plt.rcParams["font.family"] = ["PingFang HK", "STHeiti", "Hiragino Sans GB", "DejaVu Sans"]
plt.rcParams["axes.unicode_minus"] = False

CSV_FILE = sys.argv[1] if len(sys.argv) > 1 else "results.csv"

if not os.path.exists(CSV_FILE):
    print(f"找不到文件: {CSV_FILE}")
    print("请先运行: go test -v -timeout 300s ./test/benchmark/")
    sys.exit(1)

# 读取数据
workers, tps_list, lat_list, errors = [], [], [], []
with open(CSV_FILE) as f:
    reader = csv.DictReader(f)
    for row in reader:
        workers.append(int(row["workers"]))
        tps_list.append(float(row["tps"]))
        lat_list.append(float(row["avg_latency_ms"]))
        errors.append(int(row["errors"]))

# ── 图1：TPS 曲线 ──────────────────────────────────────────────────────────────
fig, ax = plt.subplots(figsize=(8, 5))

ax.plot(workers, tps_list, "o-", color="#2563EB", linewidth=2, markersize=6, label="TPS")
ax.fill_between(workers, tps_list, alpha=0.08, color="#2563EB")

# 标注峰值
peak_idx = tps_list.index(max(tps_list))
ax.annotate(
    f"峰值 {tps_list[peak_idx]:.0f} TPS\n(workers={workers[peak_idx]})",
    xy=(workers[peak_idx], tps_list[peak_idx]),
    xytext=(workers[peak_idx] + max(workers) * 0.05, tps_list[peak_idx] * 0.92),
    arrowprops=dict(arrowstyle="->", color="#1E40AF"),
    fontsize=9, color="#1E40AF",
)

ax.set_xlabel("并发客户端数 (workers)", fontsize=11)
ax.set_ylabel("吞吐量 (ops/s)", fontsize=11)
ax.set_title("mydb-go 并发吞吐量 (TPS)", fontsize=13, fontweight="bold")
ax.xaxis.set_major_locator(ticker.FixedLocator(workers))
ax.grid(axis="y", linestyle="--", alpha=0.4)
ax.legend(fontsize=10)
plt.tight_layout()
plt.savefig("benchmark_tps.png", dpi=150)
print("已生成: benchmark_tps.png")
plt.close()

# ── 图2：平均延迟曲线 ──────────────────────────────────────────────────────────
fig, ax = plt.subplots(figsize=(8, 5))

ax.plot(workers, lat_list, "s-", color="#DC2626", linewidth=2, markersize=6, label="平均延迟")
ax.fill_between(workers, lat_list, alpha=0.08, color="#DC2626")

ax.set_xlabel("并发客户端数 (workers)", fontsize=11)
ax.set_ylabel("平均延迟 (ms)", fontsize=11)
ax.set_title("mydb-go 并发平均延迟", fontsize=13, fontweight="bold")
ax.xaxis.set_major_locator(ticker.FixedLocator(workers))
ax.grid(axis="y", linestyle="--", alpha=0.4)
ax.legend(fontsize=10)
plt.tight_layout()
plt.savefig("benchmark_latency.png", dpi=150)
print("已生成: benchmark_latency.png")
plt.close()

# ── 图3：TPS + 延迟双轴合并图（论文用）──────────────────────────────────────
fig, ax1 = plt.subplots(figsize=(9, 5.5))

color_tps = "#2563EB"
color_lat = "#DC2626"

ax1.set_xlabel("并发客户端数 (workers)", fontsize=11)
ax1.set_ylabel("吞吐量 (ops/s)", color=color_tps, fontsize=11)
l1, = ax1.plot(workers, tps_list, "o-", color=color_tps, linewidth=2, markersize=6, label="TPS")
ax1.tick_params(axis="y", labelcolor=color_tps)
ax1.xaxis.set_major_locator(ticker.FixedLocator(workers))

ax2 = ax1.twinx()
ax2.set_ylabel("平均延迟 (ms)", color=color_lat, fontsize=11)
l2, = ax2.plot(workers, lat_list, "s--", color=color_lat, linewidth=2, markersize=6, label="平均延迟")
ax2.tick_params(axis="y", labelcolor=color_lat)

ax1.grid(axis="x", linestyle="--", alpha=0.3)
ax1.set_title("mydb-go 并发性能：吞吐量与延迟", fontsize=13, fontweight="bold")
fig.legend(handles=[l1, l2], loc="upper left", bbox_to_anchor=(0.1, 0.88), fontsize=10)
plt.tight_layout()
plt.savefig("benchmark_combined.png", dpi=150)
print("已生成: benchmark_combined.png")
plt.close()

# ── 打印文字摘要 ───────────────────────────────────────────────────────────────
print()
print("=== 测试摘要 ===")
print(f"{'Workers':>8} {'TPS':>10} {'AvgLat(ms)':>12} {'Errors':>8}")
print("-" * 44)
for w, t, l, e in zip(workers, tps_list, lat_list, errors):
    print(f"{w:>8} {t:>10.1f} {l:>12.3f} {e:>8}")
print(f"\n峰值 TPS: {max(tps_list):.1f}  (workers={workers[tps_list.index(max(tps_list))]})")
print(f"最低延迟: {min(lat_list):.3f} ms  (workers={workers[lat_list.index(min(lat_list))]})")
