#!/usr/bin/env python3
"""Generates report-quality figures from existing evidence files only.

Reads:
  evidence/latency/results.json
  evidence/stress/summary.csv
  evidence/final-verification/10-recovery-time-measurements.log

Writes PNGs to docs/figures/evaluation/. No data is invented: every point
plotted comes directly from one of the files above.
"""
import csv
import json
import re
import sys
from pathlib import Path

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "docs" / "figures" / "evaluation"
OUT.mkdir(parents=True, exist_ok=True)

plt.rcParams.update({
    "figure.dpi": 150,
    "font.size": 10,
    "axes.grid": True,
    "grid.alpha": 0.3,
})


def fig_latency_distribution():
    data = json.loads((ROOT / "evidence/latency/results.json").read_text())
    dec = data["decision_latency_ms"]["samples_ms"]
    act = data["actuation_confirmation_latency_ms"]["samples_ms"]

    fig, ax = plt.subplots(figsize=(6.4, 4.0))
    ax.hist(dec, bins=30, alpha=0.7, label="Access decision latency", color="#2b6cb0")
    ax.hist(act, bins=30, alpha=0.7, label="Actuation-confirmation latency", color="#c05621")
    ax.set_xlabel("Latency (ms, wall-clock)")
    ax.set_ylabel("Count (of 200 samples)")
    ax.set_title("Decision and actuation-confirmation latency distribution\n(n=200, loopback, FakeBuildSimClient)")
    ax.legend()
    fig.tight_layout()
    fig.savefig(OUT / "latency-distribution.png")
    plt.close(fig)


def fig_latency_percentiles():
    data = json.loads((ROOT / "evidence/latency/results.json").read_text())
    dec = data["decision_latency_ms"]
    act = data["actuation_confirmation_latency_ms"]

    labels = ["p50", "p95", "max"]
    dec_vals = [dec["p50"], dec["p95"], dec["max"]]
    act_vals = [act["p50"], act["p95"], act["max"]]

    x = range(len(labels))
    width = 0.35
    fig, ax = plt.subplots(figsize=(6.0, 4.0))
    ax.bar([i - width / 2 for i in x], dec_vals, width, label="Decision latency", color="#2b6cb0")
    ax.bar([i + width / 2 for i in x], act_vals, width, label="Actuation-confirmation latency", color="#c05621")
    ax.axhline(10, color="red", linestyle="--", linewidth=1, label="REQ-PERF-001 p95 threshold (10 ms)")
    ax.set_xticks(list(x))
    ax.set_xticklabels(labels)
    ax.set_ylabel("Latency (ms)")
    ax.set_title("Latency percentiles vs. REQ-PERF-001 threshold (n=200)")
    ax.legend(fontsize=8)
    fig.tight_layout()
    fig.savefig(OUT / "latency-percentiles.png")
    plt.close(fig)


def load_stress_rows():
    rows = []
    with open(ROOT / "evidence/stress/summary.csv") as f:
        for row in csv.DictReader(f):
            rows.append({k: float(v) for k, v in row.items()})
    rows.sort(key=lambda r: r["concurrency"])
    return rows


def fig_stress_throughput():
    rows = load_stress_rows()
    conc = [r["concurrency"] for r in rows]
    throughput = [r["throughput_rps"] for r in rows]

    fig, ax = plt.subplots(figsize=(6.4, 4.0))
    ax.plot(conc, throughput, marker="o", color="#2b6cb0")
    ax.set_xscale("log")
    ax.set_xlabel("Concurrency (concurrent senders, log scale)")
    ax.set_ylabel("Throughput (matched decisions / s)")
    ax.set_title("Stress test: throughput vs. concurrency\n(cmd/stressbench, 2s per level)")
    ax.axvspan(100, 1500, color="red", alpha=0.08, label="Breaking point (0 throughput)")
    ax.legend(fontsize=8)
    fig.tight_layout()
    fig.savefig(OUT / "stress-throughput.png")
    plt.close(fig)


def fig_stress_p95():
    rows = load_stress_rows()
    conc = [r["concurrency"] for r in rows]
    p95 = [r["p95_ms"] for r in rows]

    fig, ax = plt.subplots(figsize=(6.4, 4.0))
    ax.plot(conc, p95, marker="o", color="#c05621")
    ax.set_xscale("log")
    ax.set_yscale("symlog", linthresh=1)
    ax.set_xlabel("Concurrency (concurrent senders, log scale)")
    ax.set_ylabel("p95 decision latency (ms, symlog scale)")
    ax.set_title("Stress test: p95 decision latency vs. concurrency\n(0 plotted where throughput collapsed to zero)")
    fig.tight_layout()
    fig.savefig(OUT / "stress-p95-latency.png")
    plt.close(fig)


def fig_stress_error_rate():
    rows = load_stress_rows()
    conc = [r["concurrency"] for r in rows]
    err = [r["error_rate_pct"] for r in rows]

    fig, ax = plt.subplots(figsize=(6.4, 4.0))
    ax.bar([str(int(c)) for c in conc], err, color="#9b2c2c")
    ax.set_xlabel("Concurrency (concurrent senders)")
    ax.set_ylabel("Error rate (%)")
    ax.set_title("Stress test: error rate vs. concurrency")
    fig.tight_layout()
    fig.savefig(OUT / "stress-error-rate.png")
    plt.close(fig)


def fig_recovery_time():
    log_path = ROOT / "evidence/final-verification/10-recovery-time-measurements.log"
    text = log_path.read_text()
    values_ms = [float(m) for m in re.findall(r"RECOVERY TIME.*?: ([\d.]+)ms", text)]
    if not values_ms:
        print("no recovery-time samples found, skipping recovery-time figure", file=sys.stderr)
        return

    fig, ax = plt.subplots(figsize=(6.0, 4.0))
    runs = list(range(1, len(values_ms) + 1))
    ax.bar(runs, values_ms, color="#2f855a")
    ax.axhline(sum(values_ms) / len(values_ms), color="black", linestyle="--", linewidth=1,
               label=f"mean = {sum(values_ms)/len(values_ms):.1f} ms")
    ax.set_xlabel("Run #")
    ax.set_ylabel("Recovery time (ms, wall-clock)")
    ax.set_title("Decision Service restart recovery time\n(restart issued -> first correct ALLOW/UNLOCK, n=%d runs)" % len(values_ms))
    ax.set_xticks(runs)
    ax.legend(fontsize=8)
    fig.tight_layout()
    fig.savefig(OUT / "recovery-time.png")
    plt.close(fig)


if __name__ == "__main__":
    fig_latency_distribution()
    fig_latency_percentiles()
    fig_stress_throughput()
    fig_stress_p95()
    fig_stress_error_rate()
    fig_recovery_time()
    print("wrote figures to", OUT)
