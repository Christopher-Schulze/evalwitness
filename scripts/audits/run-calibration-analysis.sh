#!/usr/bin/env bash
# Reliability of the pairwise signal, from published --details artifacts.
#
# Three mechanisms read win_probability: the K=1 stop, escalation, and
# reverse-order evaluation. This reports whether that number tracks correctness
# at all, and whether its magnitude means what it claims.
#
# Usage: run-calibration-analysis.sh [--format text|json] <details.json> [<details.json> ...]
#
# Needs no provider and no network. Artifacts produced before the calibration
# block existed are recomputed from their pair decisions instead of skipped.

set -euo pipefail

format="text"
if [[ ${1:-} == "--format" ]]; then
    [[ $# -ge 3 ]] || { echo "usage: $(basename "$0") [--format text|json] <details.json> [<details.json> ...]" >&2; exit 2; }
    format=$2
    shift 2
fi
if [[ $format != "text" && $format != "json" ]]; then
    echo "unsupported format: $format (want text or json)" >&2
    exit 2
fi
if [[ $# -lt 1 ]]; then
    echo "usage: $(basename "$0") [--format text|json] <details.json> [<details.json> ...]" >&2
    exit 2
fi

for f in "$@"; do
    [[ -f $f ]] || { echo "not a file: $f" >&2; exit 2; }
done

python3 - "$format" "$@" <<'PY'
import hashlib
import json
import math
import pathlib
import sys

SCHEMA = "logprobe.reliability.v1"
BOOTSTRAP_REPLICATES = 2000


def observations(doc):
    """Pairs whose two trajectories differ in reward, so a correct answer
    exists. Equal-reward pairs carry no information about the signal and are
    excluded rather than labelled."""
    out = []
    extraction_mode = (doc.get("usage") or {}).get("extraction_mode", "unknown")
    plan = doc.get("plan") or {}
    pair_matches = plan.get("pair_matches") or 0
    worst_calls = (plan.get("calls") or {}).get("worst") or 0
    pair_call_limit = math.ceil(worst_calls / pair_matches) if pair_matches else None
    for row_index, row in enumerate(doc.get("details") or []):
        task_id = row.get("task_name") or row.get("instance_id") or f"row:{row_index:08d}"
        rewards = row.get("rewards") or []
        for decision_index, d in enumerate((row.get("selection") or {}).get("pair_decisions") or []):
            i, j = d["pair"]
            if i < 0 or j < 0 or i >= len(rewards) or j >= len(rewards) or rewards[i] == rewards[j]:
                continue
            out.append({
                "source_row_id": f"{task_id}:pair:{i}:{j}:{decision_index}",
                "task_id": task_id,
                "pair": [i, j],
                "extraction_mode": extraction_mode,
                "predicted": d["win_probability"],
                "won": rewards[i] > rewards[j],
                "outcome_id": "left_wins" if rewards[i] > rewards[j] else "right_wins",
                "difference": d.get("mean_difference", 0.0),
                "score_mass": d.get("score_mass"),
                "calls": d.get("calls"),
                "pair_call_limit": pair_call_limit,
                "order_policy": d.get("order_policy", "not-recorded-in-legacy-artifact"),
                "first_order": d.get("first_order", "not-recorded-in-legacy-artifact"),
                "inconsistent": d.get("inconsistent", False),
            })
    return out


def absolute_observations(doc):
    if (doc.get("plan") or {}).get("strategy") != "absolute":
        return []
    extraction_mode = (doc.get("usage") or {}).get("extraction_mode", "unknown")
    out = []
    for row_index, row in enumerate(doc.get("details") or []):
        task_id = row.get("task_name") or row.get("instance_id") or f"row:{row_index:08d}"
        rewards = row.get("rewards") or []
        scores = (row.get("selection") or {}).get("scores") or []
        if len(scores) != len(rewards) or len(set(rewards)) < 2:
            continue
        for trajectory, (predicted, actual) in enumerate(zip(scores, rewards)):
            out.append({
                "source_row_id": f"{task_id}:trajectory:{trajectory}",
                "task_id": task_id,
                "trajectory": trajectory,
                "extraction_mode": extraction_mode,
                "predicted": predicted,
                "actual": actual,
                "outcome_id": f"{task_id}:reward:{trajectory}={actual}",
            })
    return out


def mid_ranks(values):
    order = sorted(range(len(values)), key=lambda index: values[index])
    ranks = [0.0] * len(values)
    i = 0
    while i < len(order):
        j = i
        while j < len(order) and values[order[j]] == values[order[i]]:
            j += 1
        rank = (i + j + 1) / 2
        for index in order[i:j]:
            ranks[index] = rank
        i = j
    return ranks


def spearman(rows):
    if len(rows) < 2:
        return 0.0
    predicted = mid_ranks([row["predicted"] for row in rows])
    actual = mid_ranks([row["actual"] for row in rows])
    predicted_mean = sum(predicted) / len(predicted)
    actual_mean = sum(actual) / len(actual)
    covariance = sum((p - predicted_mean) * (a - actual_mean) for p, a in zip(predicted, actual))
    predicted_variance = sum((p - predicted_mean) ** 2 for p in predicted)
    actual_variance = sum((a - actual_mean) ** 2 for a in actual)
    if predicted_variance == 0 or actual_variance == 0:
        return 0.0
    return covariance / math.sqrt(predicted_variance * actual_variance)


def bootstrap_absolute(rows):
    groups = {}
    for row in rows:
        groups.setdefault(row["task_id"], []).append(row)
    keys = sorted(groups)
    values = []
    for replicate in range(BOOTSTRAP_REPLICATES):
        sample = []
        for draw in range(len(keys)):
            key = keys[deterministic_cluster_index(replicate, draw, len(keys))]
            sample.extend(groups[key])
        values.append(spearman(sample))
    return interval(values, len(keys)), len(keys)


def analyze(obs):
    bins = [[0, 0.0, 0.0] for _ in range(10)]
    brier = correct = 0.0
    for observation in obs:
        p = observation["predicted"]
        won = observation["won"]
        p = min(1.0, max(0.0, p))
        k = min(int(p * 10), 9)
        y = 1.0 if won else 0.0
        bins[k][0] += 1
        bins[k][1] += p
        bins[k][2] += y
        brier += (p - y) ** 2
        correct += 0.5 if p == 0.5 else float((p > 0.5) == won)

    n = len(obs)
    ece = mce = 0.0
    rows = []
    for k, (count, psum, ysum) in enumerate(bins):
        if not count:
            continue
        mp, mo = psum / count, ysum / count
        gap = abs(mo - mp)
        ece += count / n * gap
        mce = max(mce, gap)
        rows.append((k / 10, (k + 1) / 10, count, mp, mo))

    pos = sum(1 for observation in obs if observation["won"])
    neg = n - pos
    if pos and neg:
        ordered = sorted(obs, key=lambda o: o["predicted"])
        ranks, i = [0.0] * n, 0
        while i < n:
            j = i
            while j < n and ordered[j]["predicted"] == ordered[i]["predicted"]:
                j += 1
            mid = (i + j + 1) / 2
            for k in range(i, j):
                ranks[k] = mid
            i = j
        s = sum(r for r, observation in zip(ranks, ordered) if observation["won"])
        auc = (s - pos * (pos + 1) / 2) / (pos * neg)
    else:
        auc = 0.5

    monotone = all(rows[i][4] >= rows[i - 1][4] for i in range(1, len(rows)))
    return rows, ece, mce, brier / n, auc, correct / n, monotone


def error_shares(obs):
    counts = [0, 0, 0]
    for observation in obs:
        p = min(1.0, max(0.0, observation["predicted"]))
        if p == 0.5 or ((p > 0.5) == observation["won"]):
            continue
        difference = abs(observation["difference"])
        if difference <= 0.05:
            counts[0] += 1
        elif difference <= 0.20:
            counts[1] += 1
        else:
            counts[2] += 1
    total = sum(counts)
    if not total:
        return counts, None
    return counts, [count / total for count in counts]


def error_band(difference):
    difference = abs(difference)
    if difference <= 0.05:
        return "near_zero"
    if difference <= 0.20:
        return "moderate"
    return "large"


def directional_error_rows(obs):
    out = []
    for observation in obs:
        p = min(1.0, max(0.0, observation["predicted"]))
        if p == 0.5 or ((p > 0.5) == observation["won"]):
            continue
        row = dict(observation)
        row["error_band"] = error_band(observation["difference"])
        out.append(row)
    return out


def deterministic_cluster_index(replicate, draw, clusters):
    digest = hashlib.sha256(f"{SCHEMA}:{replicate}:{draw}".encode()).digest()
    return int.from_bytes(digest[:8], "big") % clusters


def interval(values, clusters):
    if not values:
        return None
    ordered = sorted(values)
    lower = math.floor(0.025 * (len(ordered) - 1))
    upper = math.ceil(0.975 * (len(ordered) - 1))
    return ordered[lower], ordered[upper], len(ordered), clusters


def bootstrap(obs):
    groups = {}
    for observation in obs:
        groups.setdefault(observation["task_id"], []).append(observation)
    keys = sorted(groups)
    values = {name: [] for name in ("ece", "mce", "brier", "auc", "accuracy")}
    shares = [[], [], []]
    for replicate in range(BOOTSTRAP_REPLICATES):
        sample = []
        for draw in range(len(keys)):
            key = keys[deterministic_cluster_index(replicate, draw, len(keys))]
            sample.extend(groups[key])
        _, ece, mce, brier, auc, accuracy, _ = analyze(sample)
        for name, value in zip(values, (ece, mce, brier, auc, accuracy)):
            values[name].append(value)
        _, sample_shares = error_shares(sample)
        if sample_shares is not None:
            for index, value in enumerate(sample_shares):
                shares[index].append(value)
    return (
        {name: interval(metric_values, len(keys)) for name, metric_values in values.items()},
        [interval(values, len(keys)) for values in shares],
        len(keys),
    )


def interval_text(bounds, percent=False):
    if bounds is None:
        return "not estimable"
    lower, upper, effective, clusters = bounds
    if percent:
        return f"95% [{lower:.1%}, {upper:.1%}], tasks={clusters}, effective={effective}"
    return f"95% [{lower:.3f}, {upper:.3f}], tasks={clusters}, effective={effective}"


def interval_object(bounds):
    if bounds is None:
        return None
    lower, upper, effective, clusters = bounds
    return {
        "level": 0.95,
        "lower": lower,
        "upper": upper,
        "cluster_count": clusters,
        "effective_replicates": effective,
    }


output_format = sys.argv[1]
reports = []
for path_text in sys.argv[2:]:
    path = pathlib.Path(path_text)
    with path.open(encoding="utf-8") as fh:
        doc = json.load(fh)
    obs = observations(doc)
    absolute_rows = absolute_observations(doc)
    mode = (doc.get("usage") or {}).get("extraction_mode", "?")
    name = path.name
    if not obs and absolute_rows:
        absolute_value = spearman(absolute_rows)
        absolute_interval, absolute_clusters = bootstrap_absolute(absolute_rows)
        reports.append({
            "artifact": path.as_posix(),
            "artifact_sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
            "extraction_mode": mode,
            "absolute": {
                "spearman": {
                    "point": absolute_value,
                    "count": len(absolute_rows),
                    "low_sample": len(absolute_rows) < 30,
                    "interval": interval_object(absolute_interval),
                },
                "source_rows": absolute_rows,
            },
            "run_context": {
                "trajectory_set": doc.get("trajectory_set"),
                "strategy": "absolute",
                "evidence_tokens_per_trajectory": (doc.get("plan") or {}).get("evidence_tokens_per_trajectory"),
                "bundled_criteria": (doc.get("plan") or {}).get("bundled_criteria"),
                "call_budget": ((doc.get("plan") or {}).get("limits") or {}).get("max_calls"),
            },
        })
        if output_format == "text":
            print(f"{name}   schema={SCHEMA}   role=development   mode={mode}   absolute_n={len(absolute_rows)}   tasks={absolute_clusters}")
            print(f"  Spearman {absolute_value:.3f}  {interval_text(absolute_interval)}\n")
        continue
    if not obs:
        if output_format == "text":
            print(f"{name}: no decidable pair decisions (absolute selection or parity run)\n")
        continue

    rows, ece, mce, brier, auc, acc, mono = analyze(obs)
    intervals, error_intervals, clusters = bootstrap(obs)
    error_counts, _ = error_shares(obs)
    error_rows = directional_error_rows(obs)
    metric_values = {"ece": ece, "mce": mce, "brier": brier, "auc": auc, "accuracy": acc}
    reports.append({
        "artifact": path.as_posix(),
        "artifact_sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
        "extraction_mode": mode,
        "observation_count": len(obs),
        "task_cluster_count": clusters,
        "metrics": {
            metric_name: {
                "point": value,
                "interval": interval_object(intervals[metric_name]),
                "low_sample": len(obs) < 30,
            }
            for metric_name, value in metric_values.items()
        },
        "monotone_in_development_sample": mono,
        "interpretation": "raw gate invalid; held-out recalibration remains untested" if not mono else "raw development bins are monotone; held-out validity remains untested",
        "error_decomposition": {
            "directional_error_count": len(error_rows),
            "bands": [
                {
                    "id": band_id,
                    "range": band_range,
                    "count": count,
                    "share_interval": interval_object(bounds),
                }
                for band_id, band_range, count, bounds in zip(
                    ("near_zero", "moderate", "large"),
                    ("[0,0.05]", "(0.05,0.20]", "(0.20,1]"),
                    error_counts,
                    error_intervals,
                )
            ],
            "source_rows": error_rows,
        },
        "run_context": {
            "trajectory_set": doc.get("trajectory_set"),
            "strategy": (doc.get("plan") or {}).get("strategy"),
            "evidence_tokens_per_trajectory": (doc.get("plan") or {}).get("evidence_tokens_per_trajectory"),
            "bundled_criteria": (doc.get("plan") or {}).get("bundled_criteria"),
            "call_budget": ((doc.get("plan") or {}).get("limits") or {}).get("max_calls"),
        },
    })
    if output_format == "json":
        continue
    flag = "  [low sample]" if len(obs) < 30 else ""
    print(f"{name}   schema={SCHEMA}   role=development   mode={mode}   n={len(obs)}   tasks={clusters}{flag}")
    for metric_name, value in (("ECE", ece), ("MCE", mce), ("Brier", brier), ("AUC", auc)):
        print(f"  {metric_name:<8}{value:.3f}  {interval_text(intervals[metric_name.lower()])}")
    print(f"  accuracy {acc:.1%}  {interval_text(intervals['accuracy'], percent=True)}")
    print(f"  monotone in this development sample: {'yes' if mono else 'no; raw gate invalid, held-out recalibration remains untested'}")
    print(f"  directional errors: {sum(error_counts)}")
    for label, count, bounds in zip(("near_zero [0,.05]", "moderate (.05,.20]", "large (.20,1]"), error_counts, error_intervals):
        print(f"    {label:<23}{count:>4}  {interval_text(bounds, percent=True)}")
    print(f"\n  {'bin':<12}{'n':>5}{'predicted':>12}{'observed':>11}{'gap':>9}")
    for lo, hi, count, mp, mo in rows:
        print(f"  {lo:.1f}-{hi:.1f}{'':<4}{count:>5}{mp:>12.2f}{mo:>11.2f}{mo - mp:>+9.2f}")
    print()

if output_format == "json":
    print(json.dumps({
        "schema_version": SCHEMA,
        "data_role": "development",
        "cluster_unit": "task_id",
        "bootstrap": {
            "method": "deterministic task-cluster percentile",
            "replicates": BOOTSTRAP_REPLICATES,
            "interval_level": 0.95,
        },
        "error_bands": [
            {"id": "near_zero", "range": "[0,0.05]"},
            {"id": "moderate", "range": "(0.05,0.20]"},
            {"id": "large", "range": "(0.20,1]"},
        ],
        "reports": reports,
    }, indent=2, sort_keys=True))
PY
