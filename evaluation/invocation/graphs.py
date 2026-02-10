import os
import re
from typing import Literal

import pandas as pd
import seaborn as sns
import matplotlib.pyplot as plt

COMPARISON = 6.5/2
SIZE: tuple[Literal[5], Literal[1]] = (10/2, 3.7)
SIZE_RATION = SIZE[0] / COMPARISON
RESULTS = "automatic-gc"           # automatic-gc/manual-gc
BASE_DIR = f"results/{RESULTS}"

# Regex for function extraction: pod_{name}-{5digits}-deployment...
FUNC_RE = re.compile(r"pod_([a-zA-Z0-9\-]+)-\d{5}-deployment")

def extract_function_name(filename):
    m = FUNC_RE.search(filename)
    if not m:
        return None
    return m.group(1)

df = pd.DataFrame()

for root, dirs, files in os.walk(BASE_DIR):
    for file in files:
        if not file.endswith("requests_trace.txt"):
            continue

        filepath = os.path.join(root, file)
        folder = os.path.basename(root)

        # Extract mode
        if "test-baseline" in folder:
            mode = "baseline"
        elif "test-enforce" in folder:
            mode = "enforce"
        else:
            continue

        # Extract application
        if "_namespace-sample-app" in folder:
            app = "sample-app"
        else:
            continue

        # Extract number of rules
        num_rules = int(folder.split("_")[1].split("-")[1])

        # Read trace file
        tmp_df = pd.read_csv(
            filepath,
            header=None,
            names=[
                "experiment",
                "test_id",
                "start_start",
                "start_end",
                "finish_start",
                "finish_end",
            ]
        )

        # Add metadata columns
        tmp_df["num_rules"] = num_rules
        tmp_df["mode"] = mode
        tmp_df["app"] = app

        # Append to main dataframe
        df = pd.concat([df, tmp_df], ignore_index=True)

# Compute durations in seconds
df["startup"] = (df["start_end"] - df["start_start"]) / 1000.0
df["finishing"] = (df["finish_end"] - df["finish_start"]) / 1000.0

baseline_ref = (
    df[
        (df["mode"] == "baseline") &
        (df["num_rules"] == 0)
    ][
        ["test_id", "app", "startup", "finishing"]
    ]
    .rename(columns={
        "startup": "startup_baseline",
        "finishing": "finishing_baseline",
    })
)

enforce_df = df[df["mode"] == "enforce"]

merged = enforce_df.merge(
    baseline_ref,
    on=["test_id", "app"],
    how="inner"
)

merged["startup_delta"] = (
    merged["startup"] - merged["startup_baseline"]
)

merged["finishing_delta"] = (
    merged["finishing"] - merged["finishing_baseline"]
)



sns.set(style="whitegrid")

plt.figure(figsize=SIZE)
ax = sns.pointplot(
    data=merged,
    x="num_rules",
    y="startup_delta",
    errorbar="sd",
    join=False,
    color=sns.color_palette("colorblind")[0],
    label="Mean difference ± std",
    capsize=.4
)

ax.yaxis.label.set_fontsize(15 * SIZE_RATION)
ax.xaxis.label.set_fontsize(15 * SIZE_RATION)
ax.tick_params(labelsize=12 * SIZE_RATION)
plt.xticks(rotation=45)
plt.ylabel("Time (s)")
plt.xlabel("Number of Rules")
plt.tight_layout()
plt.legend(fontsize=12 * SIZE_RATION, loc="upper left", bbox_to_anchor=(-0.03, 1.055))
plt.savefig(f"deployment-{RESULTS}.pdf", bbox_inches='tight', pad_inches=0)
plt.show()


plt.figure(figsize=SIZE)
ax = sns.pointplot(
    data=merged,
    x="num_rules",
    y="finishing_delta",
    errorbar="sd",
    join=False,
    color=sns.color_palette("colorblind")[0],
    label="Mean difference ± std",
    capsize=.4
)

ax.yaxis.label.set_fontsize(15 * SIZE_RATION)
ax.xaxis.label.set_fontsize(15 * SIZE_RATION)
ax.tick_params(labelsize=12 * SIZE_RATION)
plt.xticks(rotation=45)
plt.ylabel("Time (s)", labelpad=-10)
plt.xlabel("Number of Rules")
plt.tight_layout()
plt.legend(fontsize=12 * SIZE_RATION, loc="upper left", bbox_to_anchor=(-0.03, 1.055))
plt.savefig(f"teardown-{RESULTS}.pdf", bbox_inches='tight', pad_inches=0)
plt.show()

summary = (
    merged
    .groupby("num_rules")
    .agg(
        startup_mean=("startup_delta", "mean"),
        startup_std=("startup_delta", "std"),
        teardown_mean=("finishing_delta", "mean"),
        teardown_std=("finishing_delta", "std"),
    )
    .reset_index()
)

print(summary)
