import os
import re
from typing import Literal

import pandas as pd
import seaborn as sns
import matplotlib.pyplot as plt

COMPARISON = 6.5
SIZE: tuple[Literal[5], Literal[1]] = (10, 3.7)
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

<<<<<<< HEAD
sns.set(style="whitegrid")

sns.pointplot(
=======


sns.set(style="whitegrid")

plt.figure(figsize=SIZE)
ax = sns.pointplot(
>>>>>>> ddd8fce0b8a98d5bbdb22fcc8efc8df789a53daf
    data=merged,
    x="num_rules",
    y="startup_delta",
    errorbar="sd",
    join=False,
    color=sns.color_palette("colorblind")[0],
    label="Mean difference ± std",
    capsize=.4
)

<<<<<<< HEAD
plt.ylabel("Startup Overhead (s)")
plt.xlabel("Number of Rules")
plt.show()

sns.pointplot(
=======
ax.yaxis.label.set_fontsize(15 * SIZE_RATION)
ax.xaxis.label.set_fontsize(15 * SIZE_RATION)
ax.tick_params(labelsize=12 * SIZE_RATION)
plt.tight_layout()
plt.ylabel("Deployment\nDifference (s)")
plt.xlabel("Number of Rules")
plt.legend(fontsize=12 * SIZE_RATION)
plt.savefig(f"deployment-{RESULTS}.pdf", bbox_inches='tight', pad_inches=0)
plt.show()

plt.figure(figsize=SIZE)
ax = sns.pointplot(
>>>>>>> ddd8fce0b8a98d5bbdb22fcc8efc8df789a53daf
    data=merged,
    x="num_rules",
    y="finishing_delta",
    errorbar="sd",
    join=False,
    color=sns.color_palette("colorblind")[0],
    label="Mean difference ± std",
    capsize=.4
)

<<<<<<< HEAD
plt.ylabel("Teardown Overhead (s)")
plt.xlabel("Number of Rules")
plt.show()


# Remove first 50 cycles
# df = df[df["test_id"] > discard_first_n]

# CI-based outlier removal
# cleaned = []

# for exp, group in df.groupby("experiment_label"):

#     # Startup filtering
#     mu_s = group["startup"].mean()
#     sd_s = group["startup"].std()
#     low_s = mu_s - z_value * sd_s
#     high_s = mu_s + z_value * sd_s

#     # Finishing filtering
#     mu_f = group["finishing"].mean()
#     sd_f = group["finishing"].std()
#     low_f = mu_f - z_value * sd_f
#     high_f = mu_f + z_value * sd_f

#     # Keep values within CI for BOTH metrics
#     filtered = group[
#         (group["startup"].between(low_s, high_s)) &
#         (group["finishing"].between(low_f, high_f))
#     ]

#     cleaned.append(filtered)

# df_clean = pd.concat(cleaned)

# # Final means & stds
# summary = df_clean.groupby("experiment_label")[["startup", "finishing"]].agg(["mean", "std"])
# print("\n===== FINAL RESULTS AFTER OUTLIER REMOVAL =====\n")
# print(summary)
# print("\n(Mean ± Std) values are in seconds.\n")

# present_labels = sorted(df_clean["experiment_label"].unique(),
#                         key=lambda x: list(label_map.values()).index(x))


# ----- BOXPLOTS -----
# plt.figure(figsize=SIZE)
# ax = sns.boxplot(data=df_clean, x="experiment_label", y="startup", order=present_labels)
# ax.yaxis.label.set_fontsize(15 * SIZE_RATION)
# ax.xaxis.label.set_fontsize(15 * SIZE_RATION)
# ax.tick_params(labelsize=12 * SIZE_RATION)
# plt.ylabel("Time (s)")
# plt.xlabel("")
# plt.tight_layout()
# plt.savefig(f"invocation_startup.pdf", bbox_inches='tight', pad_inches=0)
# plt.show()

# plt.figure(figsize=SIZE)
# ax = sns.boxplot(data=df_clean, x="experiment_label", y="finishing", order=present_labels)
# ax.yaxis.label.set_fontsize(15 * SIZE_RATION)
# ax.xaxis.label.set_fontsize(15 * SIZE_RATION)
# ax.tick_params(labelsize=12 * SIZE_RATION)
# plt.ylabel("Time (s)")
# plt.xlabel("")
# plt.tight_layout()
# plt.savefig(f"invocation_finish.pdf", bbox_inches='tight', pad_inches=0)
# plt.show()
=======
ax.yaxis.label.set_fontsize(15 * SIZE_RATION)
ax.xaxis.label.set_fontsize(15 * SIZE_RATION)
ax.tick_params(labelsize=12 * SIZE_RATION)
plt.tight_layout()
plt.ylabel("Teardown\nDifference (s)")
plt.xlabel("Number of Rules")
plt.legend(fontsize=12 * SIZE_RATION, loc="upper left")
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
>>>>>>> ddd8fce0b8a98d5bbdb22fcc8efc8df789a53daf
