
import os
import re
from typing import Literal
import numpy as np
import pandas as pd
import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.lines import Line2D
import seaborn as sns

COMPARISON = 6.5
SIZE: tuple[Literal[5], Literal[1]] = (10, 3.5)
# SIZE: tuple[Literal[5], Literal[1]] = (10, 5)
SIZE_RATION = SIZE[0] / COMPARISON

BASE_DIR = "results"

# Regex for function extraction: pod_{name}-{5digits}-deployment...
FUNC_RE = re.compile(r"pod_([a-zA-Z0-9\-]+)-\d{5}-deployment")

# Regex to extract latency: "latency": "0.053133726s"
LAT_RE = re.compile(r'"latency":\s*"([0-9.]+)s"')

# Regex to extract jq processing time: "Total jq processing time: 533.396µs"
JQ_RE = re.compile(r'Total jq processing time:\s*([0-9.]+)(µ|m)s')

EXPECTED_ENTRIES = 550


def extract_function_name(filename):
    m = FUNC_RE.search(filename)
    if not m:
        return None
    return m.group(1)


def extract_latencies_from_file(filepath):
    latencies = []
    jq_times = []
    with open(filepath, "r") as f:
        for line in f:
            match = LAT_RE.search(line)
            if match:
                latencies.append(float(match.group(1)) * 1000)  # milliseconds
            
            # Extract jq processing times
            jq_match = JQ_RE.search(line)
            if jq_match:
                value = float(jq_match.group(1))
                unit = jq_match.group(2)
                # Convert to milliseconds
                if unit == 'µ':  # microseconds
                    value = value / 1000
                # else: already in milliseconds
                jq_times.append(value)
    return latencies, jq_times


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

        # Extract latencies and jq times
        latencies, jq_times = extract_latencies_from_file(filepath)

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
            "jq_times": jq_times,
            "num_rules": num_rules,
            "mean": pd.Series(latencies).mean(),
            "std": pd.Series(latencies).std(),
            "jq_mean": pd.Series(jq_times).mean() if jq_times else 0,
            "jq_std": pd.Series(jq_times).std() if jq_times else 0
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

    # # --------------------------------------------------
    # # 6. Plot
    # # --------------------------------------------------

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

    # Plot difference on primary y-axis
    plt.figure(figsize=SIZE)
    ax = sns.pointplot(
        data=paired_df,
        x="num_rules_enforce",
        y="difference",
        errorbar="sd",
        join=False,
        color=sns.color_palette("colorblind")[0],
        label="Mean latency difference ± std",
        capsize=.4,
        legend=False,
    )
    
    # ---- ADD BEST-FIT LINE FOR DIFFERENCE ----
    a, b = np.polyfit(x_arr, y_arr, 1)
    x_fit = np.linspace(x_arr.min(), x_arr.max(), 200)
    y_fit = a * x_fit + b
    
    diff_fit_line = sns.lineplot(
        x=x_fit,
        y=y_fit,
        color=sns.color_palette("colorblind")[1],
        linewidth=2,
        label=f"y = {a/10:.4f}x + {b/10:.4f}",
        legend=False
    )

    ax = plt.gca()

    # -------- Legend 1: latency difference --------
    diff_handles = [
        Line2D([0], [0],
            marker='o',
            linestyle='None',
            color=sns.color_palette("colorblind")[0],
            label="Mean latency difference ± std"),
        Line2D([0], [0],
            linestyle='-',
            linewidth=2,
            color=sns.color_palette("colorblind")[1],
            label=f"y = {a/10:.4f}x + {b/10:.4f}")
    ]

    legend_diff = ax.legend(
        handles=diff_handles,
        loc="upper left",
        fontsize=12 * SIZE_RATION,
        bbox_to_anchor=(-0.015, 1.055),
    )

    # Filter enforce mode records with jq times
    enforce_jq_df = df[(df["app"] == app) & (df["mode"] == "enforce") & (df["jq_times"].apply(len) > 0)]
    
    jq_agg_df = None
    if not enforce_jq_df.empty:
        # Create DataFrame for jq processing times
        jq_records = []
        for _, row in enforce_jq_df.iterrows():
            for jq_time in row["jq_times"]:
                jq_records.append({
                    "num_rules": row["num_rules"],
                    "jq_time": jq_time
                })
        
        jq_df = pd.DataFrame(jq_records)
        
        # Plot JQ processing times on secondary y-axis
        jq_point = sns.pointplot(
            data=jq_df,
            x="num_rules",
            y="jq_time",
            errorbar="sd",
            join=False,
            color=sns.color_palette("colorblind")[2],
            label="Mean gojq time ± std",
            capsize=.4,
            legend=False,
        )
        
        # Aggregate mean ± std per rule count for best-fit line
        
        jq_fit_line = jq_agg_df = (
            jq_df
            .groupby("num_rules")["jq_time"]
            .agg(["mean", "std"])
            .reset_index()
        )
        
        # Add best-fit line for jq times
        num_rules_jq = sorted(jq_agg_df["num_rules"].unique())
        x_arr_jq = pd.array(list(range(len(num_rules_jq)))) + 1
        y_arr_jq = jq_agg_df.set_index("num_rules").loc[num_rules_jq, "mean"].values
        
        a_jq, b_jq = np.polyfit(x_arr_jq, y_arr_jq, 1)
        x_fit_jq = np.linspace(x_arr_jq.min(), x_arr_jq.max(), 200)
        y_fit_jq = a_jq * x_fit_jq + b_jq
        
        sns.lineplot(
            x=x_fit_jq,
            y=y_fit_jq,
            color=sns.color_palette("colorblind")[3],
            linewidth=2,
            label=f"y = {a_jq/10:.4f}x + {b_jq/10:.4f}",
            legend=False,
        )

        # -------- Legend 2: gojq --------
        jq_handles = [
            Line2D([0], [0],
                marker='o',
                linestyle='None',
                color=sns.color_palette("colorblind")[2],
                label="Mean gojq time ± std"),
            Line2D([0], [0],
                linestyle='-',
                linewidth=2,
                color=sns.color_palette("colorblind")[3],
                label=f"y = {a_jq/10:.4f}x + {b_jq/10:.4f}")
        ]

        legend_jq = ax.legend(
            handles=jq_handles,
            loc="lower right",
            fontsize=12 * SIZE_RATION,
            bbox_to_anchor=(1.015, -.05),
        )

    # Keep both legends
    ax.add_artist(legend_diff)


    # Fix axis limits
    plt.xlim(-0.5, len(num_rules_f) - 0.5)
    plt.ylim(0, )
    plt.xticks([i for i in x_arr if not (i)%5])
    plt.yticks([i for i in range(0, 31, 10)])
    
    plt.xticks(rotation=45)
    plt.xlabel("Number of Rules")
    plt.ylabel("Time (ms)")

    ax.yaxis.label.set_fontsize(15 * SIZE_RATION)
    ax.xaxis.label.set_fontsize(15 * SIZE_RATION)
    ax.tick_params(labelsize=12 * SIZE_RATION)

    plt.tight_layout()
    # plt.legend(fontsize=12 * SIZE_RATION)
    plt.savefig(f"{app}.pdf", bbox_inches='tight')
    plt.show()

    print(f"\n=== difference Summary (difference) for {app} ===\n")
    print(difference_df)
    
    if not enforce_jq_df.empty and 'jq_agg_df' in locals():
        print(f"\n=== JQ Processing Time Summary for {app} ===\n")
        print(jq_agg_df)
