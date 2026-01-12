
import os
import re
from typing import Literal
import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns

COMPARISON = 6.5
SIZE: tuple[Literal[5], Literal[1]] = (10, 4.1)
# SIZE: tuple[Literal[5], Literal[1]] = (10, 5)
SIZE_RATION = SIZE[0] / COMPARISON

BASE_DIR = "results"

# Regex for function extraction: pod_{name}-{5digits}-deployment...
FUNC_RE = re.compile(r"pod_([a-zA-Z0-9\-]+)-\d{5}-deployment")

# Regex to extract latency: "latency": "0.053133726s"
LAT_RE = re.compile(r'"latency":\s*"([0-9.]+)s"')

EXPECTED_ENTRIES = 550


def extract_function_name(filename):
    m = FUNC_RE.search(filename)
    if not m:
        return None
    return m.group(1)


def extract_latencies_from_file(filepath):
    latencies = []
    with open(filepath, "r") as f:
        for line in f:
            match = LAT_RE.search(line)
            if match:
                latencies.append(float(match.group(1)) * 1000)  # milliseconds
    return latencies


records = []

###############################################
# WALK ALL DIRECTORIES IN results/*
###############################################
for root, dirs, files in os.walk(BASE_DIR):
    for file in files:
        if not file.endswith("queue_proxy_logs.txt"):
            continue

        filepath = os.path.join(root, file)

        folder = os.path.basename(root)

        # Extract "baseline" or "enforce"
        if "test-baseline" in folder:
            mode = "baseline"
        elif "test-enforce" in folder:
            mode = "enforce"
        else:
            continue  # unknown folder, skip

        # Extract application
        if "_namespace-sample-app" in folder:
            app = "sample-app"
        else:
            continue
        
        num_rules = int(folder.split("_")[1].split("-")[1])

        # Extract function name
        func = extract_function_name(file)

        # Extract latencies
        latencies = extract_latencies_from_file(filepath)

        # Validate count
        if len(latencies) != EXPECTED_ENTRIES:
            raise ValueError(
                f"File {filepath} has {len(latencies)} entries, expected {EXPECTED_ENTRIES}"
            )

        # Store record
        records.append({
            "app": app,
            "function": func,
            "mode": mode,
            "latencies": latencies,
            "num_rules": num_rules,
            "mean": pd.Series(latencies).mean(),
            "std": pd.Series(latencies).std()
        })

df = pd.DataFrame(records)
sns.set(style="whitegrid")

###############################################
# difference POINT-PLOTS WITH LINEAR FIT
# for sample-app
###############################################
for app in ["sample-app"]:
    df_app = df[df["app"] == app]

    df_with_samples = df_app.copy()

    df_with_samples["sample_id"] = df_with_samples["latencies"].apply(
        lambda x: list(range(len(x)))
    )

    df_exploded = (
        df_with_samples
        .explode(["latencies", "sample_id"])
        .rename(columns={"latencies": "latency"})
    )

    df_exploded["latency"] = df_exploded["latency"].astype(float)

    # --------------------------------------------------
    # 2. Split baseline and enforce
    # --------------------------------------------------

    baseline_df = df_exploded[df_exploded["mode"] == "baseline"]
    enforce_df  = df_exploded[df_exploded["mode"] == "enforce"]

    # --------------------------------------------------
    # 3. Pair enforce with baseline (ignore num_rules)
    # --------------------------------------------------

    paired_df = (
        enforce_df
        .merge(
            baseline_df,
            on=["app", "function", "sample_id"],
            suffixes=("_enforce", "_baseline"),
            how="inner"
        )
    )

    # --------------------------------------------------
    # 4. Per-sample latency difference
    # --------------------------------------------------

    paired_df["difference"] = (
        paired_df["latency_enforce"] - paired_df["latency_baseline"]
    )

    # --------------------------------------------------
    # 5. Aggregate mean ± std per enforce rule count
    # --------------------------------------------------

    difference_df = (
        paired_df
        .groupby("num_rules_enforce")["difference"]
        .agg(["mean", "std"])
        .reset_index()
        .rename(columns={"num_rules_enforce": "num_rules"})
    )

    # --------------------------------------------------
    # 6. Plot
    # --------------------------------------------------
    plt.figure(figsize=SIZE)
    ax = sns.pointplot(
        data=paired_df,
        x="num_rules_enforce",
        y="difference",
        errorbar="sd",
        join=False,
        color=sns.color_palette("colorblind")[0],
        label="Mean difference ± std",
        capsize=.4
    )

    # ---- ADD BEST-FIT LINE ----
    # Compute mean per num_rules for fitting
    # Extract arrays for fitting
    num_rules_f = set(difference_df["num_rules"])

    x_arr = pd.array(list(range(len(num_rules_f))))
    y_arr = difference_df["mean"].values

    # Best-fit linear regression (y = ax + b)
    a, b = np.polyfit(x_arr, y_arr, 1)
    x_fit = np.linspace(x_arr.min(), x_arr.max(), 200)
    y_fit = a * x_fit + b

    sns.lineplot(
        x=x_fit,
        y=y_fit,
        color=sns.color_palette("colorblind")[1],
        linewidth=2,
        label=f"y = {a:.4f}x + {b:.4f}"
    )
    # Fix axis limits
    plt.xlim(-0.5, len(num_rules_f) - 0.5)
    plt.ylim(0, )
    plt.xticks([i for i in x_arr if not (i)%5])

    plt.xticks(rotation=45)
    plt.xlabel("Number of Rules")
    plt.ylabel("Latency Difference\n(ms)")

    ax.yaxis.label.set_fontsize(15 * SIZE_RATION)
    ax.xaxis.label.set_fontsize(15 * SIZE_RATION)
    ax.tick_params(labelsize=12 * SIZE_RATION)
    ax.yaxis.set_label_coords(-.07, 0.43)

    plt.tight_layout()
    plt.legend(fontsize=12 * SIZE_RATION)
    plt.savefig(f"{app}.pdf", bbox_inches='tight')
    plt.show()

    print(f"\n=== difference Summary (difference) for {app} ===\n")
    print(difference_df)
