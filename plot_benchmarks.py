#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# dependencies = [
#     "matplotlib>=3.8",
#     "numpy>=1.24",
# ]
# ///
"""Create per-use-case and cross-use-case plots for benchmark JSON files.

The Go benchmark writer stores durations as numeric values plus a ``units``
field.  Every directory below the input directory that contains benchmark JSON
files is treated as one use-case.  Dated ``*_upload.json`` files are combined
into an upload operation while read files retain their filename as the
operation name.
"""

from __future__ import annotations

import argparse
import csv
import json
import logging
import math
import re
import sys
from collections import defaultdict
from dataclasses import dataclass
from datetime import date
from pathlib import Path
from typing import Any, Iterable, Sequence

try:
    import numpy as np
except ModuleNotFoundError as exc:
    raise SystemExit(
        "numpy is required; install the plotting dependencies with "
        "`python -m pip install -r requirements-plotting.txt`"
    ) from exc


LOG = logging.getLogger("benchmark-plots")

UNIT_TO_MS = {
    "ns": 1.0e-6,
    "nanosecond": 1.0e-6,
    "nanoseconds": 1.0e-6,
    "us": 1.0e-3,
    "µs": 1.0e-3,
    "microsecond": 1.0e-3,
    "microseconds": 1.0e-3,
    "ms": 1.0,
    "millisecond": 1.0,
    "milliseconds": 1.0,
    "s": 1.0e3,
    "second": 1.0e3,
    "seconds": 1.0e3,
}

# Okabe-Ito-derived colors remain distinguishable for common color-vision
# deficiencies and reproduce consistently in PNG, SVG, and PDF output.
COLORS = (
    "#0072B2",
    "#E69F00",
    "#009E73",
    "#D55E00",
    "#CC79A7",
    "#56B4E9",
    "#F0E442",
    "#6B6B6B",
)

READ_OPERATION_ORDER = {
    "read_binned": 0,
    "read_unbinned": 1,
    "read_summaries": 2,
}

plt: Any = None


@dataclass(frozen=True)
class Benchmark:
    """One benchmark JSON file, normalized to milliseconds."""

    use_case: str
    operation: str
    kind: str
    path: Path
    timings_ms: np.ndarray
    wallclock_ms: float | None
    run_date: date | None
    metadata: Any

    @property
    def run_label(self) -> str:
        return self.run_date.isoformat() if self.run_date else self.path.stem


def configure_matplotlib() -> None:
    """Load matplotlib lazily so data-only imports have a clear failure mode."""

    global plt
    try:
        import matplotlib

        matplotlib.use("Agg")
        import matplotlib.pyplot as pyplot
    except ModuleNotFoundError as exc:
        raise SystemExit(
            "matplotlib is required; install the plotting dependencies with "
            "`python -m pip install -r requirements-plotting.txt`"
        ) from exc

    plt = pyplot
    plt.rcParams.update(
        {
            "axes.edgecolor": "#4A4A4A",
            "axes.labelcolor": "#222222",
            "axes.spines.top": False,
            "axes.spines.right": False,
            "axes.titlelocation": "left",
            "axes.titlesize": 12,
            "axes.titleweight": "bold",
            "figure.dpi": 110,
            "figure.facecolor": "white",
            "font.family": "DejaVu Sans",
            "font.size": 10,
            "grid.alpha": 0.24,
            "grid.color": "#777777",
            "legend.frameon": False,
            "savefig.facecolor": "white",
            "xtick.color": "#333333",
            "ytick.color": "#333333",
        }
    )


def parse_arguments(argv: Sequence[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Generate diagnostic plots for each benchmark use-case directory "
            "and comparison plots across use-cases."
        )
    )
    parser.add_argument(
        "input",
        nargs="?",
        type=Path,
        default=Path("sample_data"),
        help="root containing one benchmark directory per use-case (default: sample_data)",
    )
    parser.add_argument(
        "-o",
        "--output",
        type=Path,
        default=Path("benchmark_plots"),
        help="directory to create (default: benchmark_plots)",
    )
    parser.add_argument(
        "--formats",
        nargs="+",
        choices=("png", "pdf", "svg"),
        default=("png",),
        help="one or more figure formats (default: png)",
    )
    parser.add_argument(
        "--dpi",
        type=int,
        default=180,
        help="PNG resolution in dots per inch (default: 180)",
    )
    parser.add_argument(
        "--max-points",
        type=int,
        default=4_000,
        help="maximum raw points drawn per sequence; statistics still use all samples",
    )
    parser.add_argument(
        "--baseline",
        help="optional use-case name for p50/p95 relative-latency plots",
    )
    parser.add_argument(
        "--strict",
        action="store_true",
        help="stop at the first invalid JSON file instead of warning and skipping it",
    )
    parser.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        help="print each discovered file and generated figure",
    )
    args = parser.parse_args(argv)
    if args.dpi < 72:
        parser.error("--dpi must be at least 72")
    if args.max_points < 100:
        parser.error("--max-points must be at least 100")
    return args


def classify_filename(path: Path) -> tuple[str, str, date | None]:
    stem = path.stem
    if stem.endswith("_upload"):
        prefix = stem.removesuffix("_upload")
        try:
            run_date = date.fromisoformat(prefix)
        except ValueError:
            run_date = None
        return "upload", "upload", run_date
    if stem.startswith("read_"):
        return stem, "read", None
    return stem, "other", None


def _read_payload(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        payload = json.load(handle)
    if not isinstance(payload, dict):
        raise ValueError("top-level JSON value is not an object")
    return payload


def load_benchmark(path: Path, input_root: Path) -> Benchmark:
    payload = _read_payload(path)
    units = str(payload.get("units", "ns")).strip().lower()
    if units not in UNIT_TO_MS:
        supported = ", ".join(sorted(UNIT_TO_MS))
        raise ValueError(f"unsupported units {units!r}; expected one of {supported}")

    raw_timings = payload.get("timings")
    if not isinstance(raw_timings, list) or not raw_timings:
        raise ValueError("'timings' must be a non-empty JSON array")
    if any(
        isinstance(item, bool) or not isinstance(item, (int, float))
        for item in raw_timings
    ):
        raise ValueError("every timing must be a number")

    timings_ms = np.asarray(raw_timings, dtype=np.float64) * UNIT_TO_MS[units]
    if not np.all(np.isfinite(timings_ms)):
        raise ValueError("timings include a non-finite value")
    if np.any(timings_ms < 0):
        raise ValueError("timings include a negative duration")

    raw_wallclock = payload.get("total_wallclock")
    if raw_wallclock is None:
        wallclock_ms = None
    elif isinstance(raw_wallclock, bool) or not isinstance(raw_wallclock, (int, float)):
        raise ValueError("'total_wallclock' must be numeric when present")
    else:
        wallclock_ms = float(raw_wallclock) * UNIT_TO_MS[units]
        if not math.isfinite(wallclock_ms) or wallclock_ms < 0:
            raise ValueError("'total_wallclock' must be finite and non-negative")

    relative_parent = path.parent.relative_to(input_root)
    use_case = (
        input_root.name if relative_parent == Path(".") else relative_parent.as_posix()
    )
    operation, kind, run_date = classify_filename(path)
    return Benchmark(
        use_case=use_case,
        operation=operation,
        kind=kind,
        path=path,
        timings_ms=timings_ms,
        wallclock_ms=wallclock_ms,
        run_date=run_date,
        metadata=payload.get("metadata", {}),
    )


def discover_benchmarks(input_root: Path, strict: bool) -> list[Benchmark]:
    input_root = input_root.resolve()
    if not input_root.is_dir():
        raise FileNotFoundError(f"input directory does not exist: {input_root}")

    paths = sorted(input_root.rglob("*.json"))
    if not paths:
        raise ValueError(f"no JSON files found below {input_root}")

    records: list[Benchmark] = []
    for path in paths:
        try:
            record = load_benchmark(path, input_root)
        except (OSError, json.JSONDecodeError, ValueError) as exc:
            if strict:
                raise ValueError(f"invalid benchmark file {path}: {exc}") from exc
            LOG.warning("Skipping %s: %s", path, exc)
            continue
        LOG.debug("Loaded %s (%d samples)", path, record.timings_ms.size)
        records.append(record)

    if not records:
        raise ValueError("no valid benchmark JSON files were found")
    return records


def operation_sort_key(operation: str) -> tuple[int, int, str]:
    if operation in READ_OPERATION_ORDER:
        return (0, READ_OPERATION_ORDER[operation], operation)
    if operation == "upload":
        return (1, 0, operation)
    return (2, 0, operation)


def record_sort_key(record: Benchmark) -> tuple[str, str, date, str]:
    return (
        record.use_case,
        record.operation,
        record.run_date or date.min,
        record.path.name,
    )


def pretty_operation(operation: str) -> str:
    known = {
        "read_binned": "Binned read",
        "read_unbinned": "Unbinned read",
        "read_summaries": "Summary read",
        "upload": "Upload batch",
    }
    return known.get(operation, operation.replace("_", " ").strip().title())


def slugify(value: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
    return slug or "use-case"


def unique_case_slugs(names: Iterable[str]) -> dict[str, str]:
    """Return readable output names, adding a digest only for slug collisions."""

    import hashlib

    names = list(names)
    bases = {name: slugify(name) for name in names}
    counts: dict[str, int] = defaultdict(int)
    for base in bases.values():
        counts[base] += 1
    result = {}
    for name in names:
        base = bases[name]
        if counts[base] == 1:
            result[name] = base
        else:
            digest = hashlib.sha1(
                name.encode("utf-8"), usedforsecurity=False
            ).hexdigest()[:7]
            result[name] = f"{base}-{digest}"
    return result


def group_by_use_case(records: Iterable[Benchmark]) -> dict[str, list[Benchmark]]:
    result: dict[str, list[Benchmark]] = defaultdict(list)
    for record in records:
        result[record.use_case].append(record)
    return {
        name: sorted(items, key=record_sort_key)
        for name, items in sorted(result.items())
    }


def group_by_operation(records: Iterable[Benchmark]) -> dict[str, list[Benchmark]]:
    result: dict[str, list[Benchmark]] = defaultdict(list)
    for record in records:
        result[record.operation].append(record)
    return {
        operation: sorted(items, key=record_sort_key)
        for operation, items in sorted(
            result.items(), key=lambda item: operation_sort_key(item[0])
        )
    }


def concatenate(records: Iterable[Benchmark]) -> np.ndarray:
    arrays = [record.timings_ms for record in records]
    return np.concatenate(arrays) if arrays else np.asarray([], dtype=np.float64)


def quantiles(values: np.ndarray) -> dict[str, float]:
    p25, p50, p75, p90, p95, p99 = np.percentile(values, (25, 50, 75, 90, 95, 99))
    statistics = {
        "min_ms": float(np.min(values)),
        "p25_ms": float(p25),
        "median_ms": float(p50),
        "mean_ms": float(np.mean(values)),
        "p75_ms": float(p75),
        "p90_ms": float(p90),
        "p95_ms": float(p95),
        "p99_ms": float(p99),
        "max_ms": float(np.max(values)),
        # ddof=0 matches the population standard deviation computed by Go.
        "std_ms": float(np.std(values, ddof=0)),
    }
    # Milliseconds to six decimal places preserves the source nanosecond
    # precision without exposing binary floating-point artifacts in CSV files.
    return {name: round(value, 6) for name, value in statistics.items()}


SUMMARY_COLUMNS = (
    "use_case",
    "operation",
    "kind",
    "benchmark_file",
    "run_date",
    "benchmark_runs",
    "samples",
    "min_ms",
    "p25_ms",
    "median_ms",
    "mean_ms",
    "p75_ms",
    "p90_ms",
    "p95_ms",
    "p99_ms",
    "max_ms",
    "std_ms",
    "total_wallclock_ms",
    "timing_sum_ms",
    "effective_parallelism",
    "metadata_json",
)


def summary_row(
    *,
    use_case: str,
    operation: str,
    kind: str,
    values: np.ndarray,
    benchmark_file: str,
    run_date_value: str,
    benchmark_runs: int,
    total_wallclock_ms: float | None,
    metadata: Any,
) -> dict[str, Any]:
    timing_sum = float(np.sum(values))
    parallelism = (
        timing_sum / total_wallclock_ms
        if total_wallclock_ms is not None and total_wallclock_ms > 0
        else ""
    )
    return {
        "use_case": use_case,
        "operation": operation,
        "kind": kind,
        "benchmark_file": benchmark_file,
        "run_date": run_date_value,
        "benchmark_runs": benchmark_runs,
        "samples": int(values.size),
        **quantiles(values),
        "total_wallclock_ms": ""
        if total_wallclock_ms is None
        else round(total_wallclock_ms, 6),
        "timing_sum_ms": round(timing_sum, 6),
        "effective_parallelism": "" if parallelism == "" else round(parallelism, 6),
        "metadata_json": json.dumps(metadata, sort_keys=True, separators=(",", ":")),
    }


def write_summaries(records: Sequence[Benchmark], output: Path) -> list[Path]:
    file_rows = [
        summary_row(
            use_case=record.use_case,
            operation=record.operation,
            kind=record.kind,
            values=record.timings_ms,
            benchmark_file=record.path.name,
            run_date_value=record.run_date.isoformat() if record.run_date else "",
            benchmark_runs=1,
            total_wallclock_ms=record.wallclock_ms,
            metadata=record.metadata,
        )
        for record in sorted(records, key=record_sort_key)
    ]

    aggregate_rows: list[dict[str, Any]] = []
    for use_case, case_records in group_by_use_case(records).items():
        for operation, operation_records in group_by_operation(case_records).items():
            wallclocks = [
                record.wallclock_ms
                for record in operation_records
                if record.wallclock_ms is not None
            ]
            aggregate_rows.append(
                summary_row(
                    use_case=use_case,
                    operation=operation,
                    kind=operation_records[0].kind,
                    values=concatenate(operation_records),
                    benchmark_file="",
                    run_date_value="",
                    benchmark_runs=len(operation_records),
                    total_wallclock_ms=float(sum(wallclocks)) if wallclocks else None,
                    metadata={},
                )
            )

    paths = []
    for filename, rows in (
        ("benchmark_file_summary.csv", file_rows),
        ("benchmark_operation_summary.csv", aggregate_rows),
    ):
        path = output / filename
        with path.open("w", newline="", encoding="utf-8") as handle:
            writer = csv.DictWriter(handle, fieldnames=SUMMARY_COLUMNS)
            writer.writeheader()
            writer.writerows(rows)
        paths.append(path)
    return paths


def color_at(index: int) -> str:
    return COLORS[index % len(COLORS)]


def format_ms(value: float) -> str:
    if value == 0:
        return "0"
    if abs(value) < 0.01:
        return f"{value:.3g}"
    if abs(value) < 1:
        return f"{value:.2f}"
    if abs(value) < 100:
        return f"{value:.1f}"
    if abs(value) < 1_000:
        return f"{value:.0f}"
    if abs(value) < 1_000_000:
        return f"{value:,.0f}"
    return f"{value:.2e}"


def format_ms_tick(value: float, _position: int) -> str:
    return format_ms(value)


def positive_dynamic_range(arrays: Iterable[np.ndarray]) -> float:
    values = np.concatenate([array for array in arrays if array.size])
    # Log scales cannot represent zero-duration measurements.
    if np.any(values <= 0):
        return 1.0
    positive = values[values > 0]
    if not positive.size:
        return 1.0
    return float(np.max(positive) / np.min(positive))


def style_latency_axis(
    axis: Any, arrays: Iterable[np.ndarray], orientation: str = "x"
) -> bool:
    from matplotlib.ticker import FuncFormatter

    arrays = list(arrays)
    use_log = positive_dynamic_range(arrays) >= 50
    if use_log:
        if orientation == "x":
            axis.set_xscale("log")
        else:
            axis.set_yscale("log")
        target = axis.xaxis if orientation == "x" else axis.yaxis
        target.set_major_formatter(FuncFormatter(format_ms_tick))
    axis.grid(True, axis=orientation, which="major")
    return use_log


def save_figure(
    figure: Any,
    stem: Path,
    formats: Sequence[str],
    dpi: int,
) -> list[Path]:
    stem.parent.mkdir(parents=True, exist_ok=True)
    generated = []
    for figure_format in formats:
        path = stem.with_suffix(f".{figure_format}")
        kwargs: dict[str, Any] = {
            "bbox_inches": "tight",
            "metadata": {"Creator": "lightcurvefiller plot_benchmarks.py"},
        }
        if figure_format == "png":
            kwargs["dpi"] = dpi
        figure.savefig(path, **kwargs)
        LOG.debug("Wrote %s", path)
        generated.append(path)
    plt.close(figure)
    return generated


def draw_horizontal_boxes(
    axis: Any,
    arrays: Sequence[np.ndarray],
    labels: Sequence[str],
    colors: Sequence[str],
) -> None:
    positions = np.arange(1, len(arrays) + 1)
    import inspect

    orientation = (
        {"orientation": "horizontal"}
        if "orientation" in inspect.signature(axis.boxplot).parameters
        else {"vert": False}
    )
    result = axis.boxplot(
        arrays,
        positions=positions,
        widths=0.58,
        whis=(5, 95),
        showfliers=False,
        patch_artist=True,
        medianprops={"color": "#111111", "linewidth": 1.6},
        whiskerprops={"color": "#555555", "linewidth": 1.0},
        capprops={"color": "#555555", "linewidth": 1.0},
        **orientation,
    )
    for patch, color in zip(result["boxes"], colors, strict=True):
        patch.set_facecolor(color)
        patch.set_edgecolor(color)
        patch.set_alpha(0.72)
    for position, values, color in zip(positions, arrays, colors, strict=True):
        if values.size <= 20:
            offsets = (
                np.linspace(-0.10, 0.10, num=values.size)
                if values.size > 1
                else np.zeros(1)
            )
            axis.scatter(
                values,
                position + offsets,
                s=20,
                facecolors="white",
                edgecolors=color,
                linewidths=1.1,
                zorder=4,
            )
    axis.set_yticks(positions, labels)
    axis.set_ylim(0.4, len(arrays) + 0.6)
    axis.invert_yaxis()


def plot_use_case_overview(
    use_case: str,
    records: Sequence[Benchmark],
    output: Path,
    formats: Sequence[str],
    dpi: int,
) -> list[Path]:
    grouped = group_by_operation(records)
    kind_groups: list[tuple[str, list[str]]] = []
    for kind, title in (
        ("read", "Read latency"),
        ("upload", "Upload latency"),
        ("other", "Other latency"),
    ):
        operations = [
            operation for operation, items in grouped.items() if items[0].kind == kind
        ]
        if operations:
            kind_groups.append((title, operations))

    max_rows = max(len(operations) for _, operations in kind_groups)
    figure, axes = plt.subplots(
        1,
        len(kind_groups),
        figsize=(6.0 * len(kind_groups), max(3.8, 0.68 * max_rows + 2.0)),
        squeeze=False,
        layout="constrained",
    )
    figure.suptitle(
        f"{use_case}: latency overview",
        fontsize=16,
        fontweight="bold",
        x=0.01,
        ha="left",
    )

    for axis, (title, operations) in zip(axes[0], kind_groups, strict=True):
        arrays = [concatenate(grouped[operation]) for operation in operations]
        labels = [
            f"{pretty_operation(op)}  (n={values.size:,})"
            for op, values in zip(operations, arrays)
        ]
        draw_horizontal_boxes(
            axis,
            arrays,
            labels,
            [color_at(index) for index in range(len(arrays))],
        )
        is_log = style_latency_axis(axis, arrays)
        axis.set_xlabel(f"Latency (ms{' · log scale' if is_log else ''})")
        axis.set_title(title)
    figure.text(
        0.01,
        -0.015,
        "Boxes: IQR · whiskers: p5–p95 · medians: black",
        fontsize=9,
        color="#555555",
    )
    return save_figure(figure, output / "latency_overview", formats, dpi)


def plot_use_case_ecdf(
    use_case: str,
    records: Sequence[Benchmark],
    output: Path,
    formats: Sequence[str],
    dpi: int,
) -> list[Path]:
    from matplotlib.ticker import PercentFormatter

    grouped = group_by_operation(records)
    kind_groups = []
    for kind, title in (("read", "Reads"), ("upload", "Uploads"), ("other", "Other")):
        operations = [
            operation for operation, items in grouped.items() if items[0].kind == kind
        ]
        if operations:
            kind_groups.append((title, operations))

    figure, axes = plt.subplots(
        1,
        len(kind_groups),
        figsize=(6.2 * len(kind_groups), 4.5),
        squeeze=False,
        layout="constrained",
    )
    figure.suptitle(
        f"{use_case}: latency distributions",
        fontsize=16,
        fontweight="bold",
        x=0.01,
        ha="left",
    )
    for axis, (title, operations) in zip(axes[0], kind_groups, strict=True):
        arrays = []
        for index, operation in enumerate(operations):
            values = np.sort(concatenate(grouped[operation]))
            arrays.append(values)
            probability = np.arange(1, values.size + 1) / values.size
            axis.step(
                values,
                probability,
                where="post",
                linewidth=2.0,
                color=color_at(index),
                label=f"{pretty_operation(operation)} (n={values.size:,})",
            )
        is_log = style_latency_axis(axis, arrays)
        axis.set_xlabel(f"Latency (ms{' · log scale' if is_log else ''})")
        axis.set_ylabel("Requests completed")
        axis.yaxis.set_major_formatter(PercentFormatter(1.0))
        axis.set_ylim(0, 1.015)
        axis.grid(True, axis="y")
        axis.set_title(title)
        axis.legend(loc="lower right", fontsize=9)
    return save_figure(figure, output / "latency_ecdf", formats, dpi)


def evenly_spaced_indices(size: int, limit: int) -> np.ndarray:
    if size <= limit:
        return np.arange(size)
    return np.unique(np.linspace(0, size - 1, num=limit, dtype=np.int64))


def block_medians(
    values: np.ndarray, target_blocks: int = 180
) -> tuple[np.ndarray, np.ndarray]:
    block_size = max(5, math.ceil(values.size / target_blocks))
    centers = []
    medians = []
    for start in range(0, values.size, block_size):
        stop = min(start + block_size, values.size)
        centers.append((start + stop - 1) / 2)
        medians.append(float(np.median(values[start:stop])))
    return np.asarray(centers), np.asarray(medians)


def plot_use_case_sequence(
    use_case: str,
    records: Sequence[Benchmark],
    output: Path,
    formats: Sequence[str],
    dpi: int,
    max_points: int,
) -> list[Path]:
    grouped = group_by_operation(records)
    figure, axes = plt.subplots(
        len(grouped),
        1,
        figsize=(11, max(3.2, 2.55 * len(grouped))),
        squeeze=False,
        layout="constrained",
    )
    figure.suptitle(
        f"{use_case}: latency by request order",
        fontsize=16,
        fontweight="bold",
        x=0.01,
        ha="left",
    )

    for operation_index, (operation, operation_records) in enumerate(grouped.items()):
        axis = axes[operation_index, 0]
        values = concatenate(operation_records)
        indices = evenly_spaced_indices(values.size, max_points)
        color = color_at(operation_index)
        axis.scatter(
            indices,
            values[indices],
            s=8,
            alpha=0.55 if values.size <= 250 else 0.18,
            linewidths=0,
            color=color,
            rasterized=values.size > max_points,
            label=("Sampled requests" if values.size > max_points else "Requests"),
        )
        if values.size >= 5:
            block_x, block_y = block_medians(values)
            axis.plot(
                block_x, block_y, color=color, linewidth=2.0, label="Block median"
            )

        boundaries = np.cumsum(
            [record.timings_ms.size for record in operation_records[:-1]]
        )
        if len(boundaries) <= 30:
            for boundary in boundaries:
                axis.axvline(boundary - 0.5, color="#999999", linewidth=0.7, alpha=0.35)

        is_log = style_latency_axis(axis, [values], orientation="y")
        axis.set_ylabel(f"Latency (ms{' · log' if is_log else ''})")
        axis.set_title(
            f"{pretty_operation(operation)} · {values.size:,} samples across "
            f"{len(operation_records):,} file{'s' if len(operation_records) != 1 else ''}"
        )
        axis.legend(loc="upper right", fontsize=8, ncols=2)
        axis.margins(x=0.01)
    axes[-1, 0].set_xlabel("Request index (files ordered by date, then filename)")
    return save_figure(figure, output / "latency_sequence", formats, dpi)


def dated_upload_records(records: Iterable[Benchmark]) -> list[Benchmark]:
    return sorted(
        (
            record
            for record in records
            if record.kind == "upload" and record.run_date is not None
        ),
        key=lambda record: (record.run_date, record.path.name),
    )


def set_date_ticks(axis: Any, dates: Iterable[date], max_ticks: int = 8) -> None:
    """Use observed dates as ticks, avoiding misleading sub-day interpolation."""

    unique_dates = sorted(set(dates))
    selected = evenly_spaced_indices(len(unique_dates), max_ticks)
    tick_dates = [unique_dates[index] for index in selected]
    axis.set_xticks(
        tick_dates, [item.isoformat() for item in tick_dates], rotation=30, ha="right"
    )


def plot_use_case_upload_trend(
    use_case: str,
    records: Sequence[Benchmark],
    output: Path,
    formats: Sequence[str],
    dpi: int,
) -> list[Path]:
    uploads = dated_upload_records(records)
    if len(uploads) < 2:
        return []

    dates = [record.run_date for record in uploads]
    p25 = np.asarray([np.percentile(record.timings_ms, 25) for record in uploads])
    p50 = np.asarray([np.percentile(record.timings_ms, 50) for record in uploads])
    p75 = np.asarray([np.percentile(record.timings_ms, 75) for record in uploads])
    p95 = np.asarray([np.percentile(record.timings_ms, 95) for record in uploads])

    has_wallclock = any(record.wallclock_ms is not None for record in uploads)
    rows = 2 if has_wallclock else 1
    figure, axes = plt.subplots(
        rows,
        1,
        figsize=(11, 3.1 * rows),
        squeeze=False,
        sharex=True,
        layout="constrained",
    )
    figure.suptitle(
        f"{use_case}: upload trend", fontsize=16, fontweight="bold", x=0.01, ha="left"
    )
    latency_axis = axes[0, 0]
    latency_axis.fill_between(dates, p25, p75, color=COLORS[0], alpha=0.18, label="IQR")
    latency_axis.plot(
        dates,
        p50,
        marker="o",
        markersize=4,
        color=COLORS[0],
        linewidth=1.8,
        label="p50",
    )
    latency_axis.plot(
        dates, p95, color=COLORS[3], linewidth=1.5, linestyle="--", label="p95"
    )
    is_log = style_latency_axis(latency_axis, [p25, p50, p75, p95], orientation="y")
    latency_axis.set_ylabel(f"Batch latency (ms{' · log' if is_log else ''})")
    latency_axis.set_title("Request latency")
    latency_axis.legend(ncols=3, fontsize=9)

    if has_wallclock:
        wallclock = np.asarray(
            [
                np.nan if record.wallclock_ms is None else record.wallclock_ms
                for record in uploads
            ]
        )
        wallclock_axis = axes[1, 0]
        wallclock_axis.plot(
            dates, wallclock, marker="o", markersize=4, color=COLORS[2], linewidth=1.8
        )
        valid = wallclock[np.isfinite(wallclock)]
        is_log = style_latency_axis(wallclock_axis, [valid], orientation="y")
        wallclock_axis.set_ylabel(f"Wall clock (ms{' · log' if is_log else ''})")
        wallclock_axis.set_title("Whole upload run")

    axes[-1, 0].set_xlabel("Benchmark date")
    set_date_ticks(axes[-1, 0], (item.run_date for item in uploads))
    return save_figure(figure, output / "upload_trend", formats, dpi)


def operations_of_kind(records: Sequence[Benchmark], kind: str) -> list[str]:
    return sorted(
        {record.operation for record in records if record.kind == kind},
        key=operation_sort_key,
    )


def plot_comparison_distributions(
    records: Sequence[Benchmark],
    kind: str,
    output: Path,
    formats: Sequence[str],
    dpi: int,
) -> list[Path]:
    operations = operations_of_kind(records, kind)
    if not operations:
        return []
    cases = list(group_by_use_case(records))
    columns = min(3, len(operations))
    rows = math.ceil(len(operations) / columns)
    figure, axes = plt.subplots(
        rows,
        columns,
        figsize=(6.0 * columns, max(4.1, 0.62 * len(cases) + 2.0) * rows),
        squeeze=False,
        layout="constrained",
    )
    figure.suptitle(
        f"{kind.title()} latency across use-cases",
        fontsize=16,
        fontweight="bold",
        x=0.01,
        ha="left",
    )
    case_records = group_by_use_case(records)
    for axis, operation in zip(axes.flat, operations):
        available = []
        for case_index, use_case in enumerate(cases):
            matching = [
                item for item in case_records[use_case] if item.operation == operation
            ]
            if matching:
                available.append(
                    (use_case, concatenate(matching), color_at(case_index))
                )
        arrays = [item[1] for item in available]
        labels = [f"{item[0]}  (n={item[1].size:,})" for item in available]
        draw_horizontal_boxes(axis, arrays, labels, [item[2] for item in available])
        is_log = style_latency_axis(axis, arrays)
        axis.set_xlabel(f"Latency (ms{' · log scale' if is_log else ''})")
        axis.set_title(pretty_operation(operation))
    for axis in list(axes.flat)[len(operations) :]:
        axis.set_visible(False)
    figure.text(
        0.01,
        -0.012,
        "Boxes: IQR · whiskers: p5–p95 · medians: black",
        fontsize=9,
        color="#555555",
    )
    return save_figure(figure, output / f"comparison_{kind}_latency", formats, dpi)


def plot_upload_wallclock_comparison(
    records: Sequence[Benchmark],
    output: Path,
    formats: Sequence[str],
    dpi: int,
) -> list[Path]:
    cases = group_by_use_case(records)
    available = []
    for case_index, (use_case, case_items) in enumerate(cases.items()):
        values = np.asarray(
            [
                item.wallclock_ms
                for item in case_items
                if item.kind == "upload" and item.wallclock_ms is not None
            ],
            dtype=np.float64,
        )
        if values.size:
            available.append((use_case, values, color_at(case_index)))
    if not available:
        return []

    figure, axis = plt.subplots(
        figsize=(8.5, max(4.0, 0.62 * len(available) + 2.0)),
        layout="constrained",
    )
    figure.suptitle(
        "Upload wall-clock time across use-cases",
        fontsize=16,
        fontweight="bold",
        x=0.01,
        ha="left",
    )
    arrays = [item[1] for item in available]
    draw_horizontal_boxes(
        axis,
        arrays,
        [f"{item[0]}  (runs={item[1].size:,})" for item in available],
        [item[2] for item in available],
    )
    is_log = style_latency_axis(axis, arrays)
    axis.set_xlabel(
        f"Wall-clock time per benchmark file (ms{' · log scale' if is_log else ''})"
    )
    axis.set_title("Whole upload run")
    figure.text(
        0.01,
        -0.012,
        "Boxes: IQR · whiskers: p5–p95 · medians: black",
        fontsize=9,
        color="#555555",
    )
    return save_figure(figure, output / "comparison_upload_wallclock", formats, dpi)


def aggregate_matrix(
    records: Sequence[Benchmark],
    percentile: float,
) -> tuple[list[str], list[str], np.ndarray]:
    cases = list(group_by_use_case(records))
    operations = sorted(
        {record.operation for record in records}, key=operation_sort_key
    )
    matrix = np.full((len(cases), len(operations)), np.nan)
    case_records = group_by_use_case(records)
    for row, use_case in enumerate(cases):
        grouped = group_by_operation(case_records[use_case])
        for column, operation in enumerate(operations):
            if operation in grouped:
                matrix[row, column] = np.percentile(
                    concatenate(grouped[operation]), percentile
                )
    return cases, operations, matrix


def plot_percentile_heatmap(
    records: Sequence[Benchmark],
    output: Path,
    formats: Sequence[str],
    dpi: int,
) -> list[Path]:
    from matplotlib import colors as mpl_colors

    cases, operations, p50 = aggregate_matrix(records, 50)
    _, _, p95 = aggregate_matrix(records, 95)
    finite = np.concatenate((p50[np.isfinite(p50)], p95[np.isfinite(p95)]))
    if not finite.size:
        return []
    minimum = float(np.min(finite))
    maximum = float(np.max(finite))
    if minimum > 0 and maximum / minimum >= 50:
        normalization = mpl_colors.LogNorm(vmin=minimum, vmax=maximum)
    else:
        normalization = mpl_colors.Normalize(vmin=minimum, vmax=maximum or 1.0)

    figure, axes = plt.subplots(
        1,
        2,
        figsize=(
            max(10.5, 2.1 * len(operations) + 5.5),
            max(4.2, 0.58 * len(cases) + 2.0),
        ),
        squeeze=False,
        layout="constrained",
    )
    figure.suptitle(
        "Latency percentile matrix", fontsize=16, fontweight="bold", x=0.01, ha="left"
    )
    images = []
    for axis, matrix, title in zip(axes[0], (p50, p95), ("p50", "p95"), strict=True):
        masked = np.ma.masked_invalid(matrix)
        image = axis.imshow(masked, aspect="auto", cmap="viridis", norm=normalization)
        images.append(image)
        axis.set_xticks(
            np.arange(len(operations)),
            [pretty_operation(item) for item in operations],
            rotation=30,
            ha="right",
        )
        axis.set_yticks(np.arange(len(cases)), cases)
        axis.set_title(title)
        for row in range(len(cases)):
            for column in range(len(operations)):
                value = matrix[row, column]
                if not np.isfinite(value):
                    continue
                normalized = float(normalization(value))
                text_color = (
                    "white" if normalized < 0.2 or normalized > 0.72 else "#111111"
                )
                axis.text(
                    column,
                    row,
                    format_ms(value),
                    ha="center",
                    va="center",
                    color=text_color,
                    fontsize=9,
                )
        axis.set_xticks(np.arange(-0.5, len(operations), 1), minor=True)
        axis.set_yticks(np.arange(-0.5, len(cases), 1), minor=True)
        axis.grid(which="minor", color="white", linewidth=1.2, alpha=0.8)
        axis.tick_params(which="minor", bottom=False, left=False)
    figure.colorbar(images[-1], ax=list(axes[0]), label="Latency (ms)", shrink=0.88)
    return save_figure(figure, output / "comparison_latency_percentiles", formats, dpi)


def plot_upload_trend_comparison(
    records: Sequence[Benchmark],
    output: Path,
    formats: Sequence[str],
    dpi: int,
) -> list[Path]:
    case_records = group_by_use_case(records)
    uploads_by_case = {
        use_case: dated_upload_records(items)
        for use_case, items in case_records.items()
    }
    uploads_by_case = {name: items for name, items in uploads_by_case.items() if items}
    all_dates = {item.run_date for items in uploads_by_case.values() for item in items}
    if len(uploads_by_case) < 2 or len(all_dates) < 2:
        return []

    has_wallclock = any(
        item.wallclock_ms is not None
        for items in uploads_by_case.values()
        for item in items
    )
    metrics = [("p50", 50), ("p95", 95)]
    rows = len(metrics) + int(has_wallclock)
    figure, axes = plt.subplots(
        rows,
        1,
        figsize=(11, 2.8 * rows),
        squeeze=False,
        sharex=True,
        layout="constrained",
    )
    figure.suptitle(
        "Upload trend across use-cases",
        fontsize=16,
        fontweight="bold",
        x=0.01,
        ha="left",
    )
    for metric_index, (label, percentile) in enumerate(metrics):
        axis = axes[metric_index, 0]
        all_values = []
        for case_index, (use_case, uploads) in enumerate(uploads_by_case.items()):
            dates = [item.run_date for item in uploads]
            values = np.asarray(
                [np.percentile(item.timings_ms, percentile) for item in uploads]
            )
            all_values.append(values)
            axis.plot(
                dates,
                values,
                marker="o",
                markersize=3.5,
                linewidth=1.5,
                color=color_at(case_index),
                label=use_case,
            )
        is_log = style_latency_axis(axis, all_values, orientation="y")
        axis.set_ylabel(f"{label} (ms{' · log' if is_log else ''})")
        axis.set_title(f"Batch latency {label}")
        if metric_index == 0:
            axis.legend(
                ncols=min(3, len(uploads_by_case)), fontsize=8, loc="upper right"
            )

    if has_wallclock:
        axis = axes[-1, 0]
        all_values = []
        for case_index, (use_case, uploads) in enumerate(uploads_by_case.items()):
            dates = [item.run_date for item in uploads]
            values = np.asarray(
                [
                    np.nan if item.wallclock_ms is None else item.wallclock_ms
                    for item in uploads
                ]
            )
            all_values.append(values[np.isfinite(values)])
            axis.plot(
                dates,
                values,
                marker="o",
                markersize=3.5,
                linewidth=1.5,
                color=color_at(case_index),
            )
        is_log = style_latency_axis(axis, all_values, orientation="y")
        axis.set_ylabel(f"Wall clock (ms{' · log' if is_log else ''})")
        axis.set_title("Whole upload run")

    axes[-1, 0].set_xlabel("Benchmark date")
    set_date_ticks(axes[-1, 0], all_dates)
    return save_figure(figure, output / "comparison_upload_trend", formats, dpi)


def plot_relative_to_baseline(
    records: Sequence[Benchmark],
    baseline: str,
    output: Path,
    formats: Sequence[str],
    dpi: int,
) -> list[Path]:
    from matplotlib import colors as mpl_colors

    cases, operations, p50 = aggregate_matrix(records, 50)
    _, _, p95 = aggregate_matrix(records, 95)
    if baseline not in cases:
        raise ValueError(
            f"baseline {baseline!r} is not present; choose one of: {', '.join(cases)}"
        )
    baseline_index = cases.index(baseline)
    ratios = []
    for matrix in (p50, p95):
        denominator = matrix[baseline_index, :]
        ratios.append(
            np.divide(
                matrix,
                denominator,
                out=np.full_like(matrix, np.nan),
                where=denominator > 0,
            )
        )

    finite = np.concatenate([matrix[np.isfinite(matrix)] for matrix in ratios])
    distance = max(abs(float(np.min(finite)) - 1), abs(float(np.max(finite)) - 1), 0.05)
    normalization = mpl_colors.TwoSlopeNorm(
        vmin=max(0, 1 - distance), vcenter=1.0, vmax=1 + distance
    )
    figure, axes = plt.subplots(
        1,
        2,
        figsize=(
            max(10.5, 2.1 * len(operations) + 5.5),
            max(4.2, 0.58 * len(cases) + 2.0),
        ),
        squeeze=False,
        layout="constrained",
    )
    figure.suptitle(
        f"Latency relative to {baseline}",
        fontsize=16,
        fontweight="bold",
        x=0.01,
        ha="left",
    )
    images = []
    for axis, matrix, title in zip(
        axes[0], ratios, ("p50 ratio", "p95 ratio"), strict=True
    ):
        image = axis.imshow(
            np.ma.masked_invalid(matrix),
            aspect="auto",
            cmap="RdBu_r",
            norm=normalization,
        )
        images.append(image)
        axis.set_xticks(
            np.arange(len(operations)),
            [pretty_operation(item) for item in operations],
            rotation=30,
            ha="right",
        )
        axis.set_yticks(np.arange(len(cases)), cases)
        axis.set_title(title)
        for row in range(len(cases)):
            for column in range(len(operations)):
                value = matrix[row, column]
                if np.isfinite(value):
                    axis.text(
                        column,
                        row,
                        f"{value:.2f}×",
                        ha="center",
                        va="center",
                        fontsize=9,
                    )
    figure.colorbar(
        images[-1],
        ax=list(axes[0]),
        label="Latency ratio (lower is faster)",
        shrink=0.88,
    )
    return save_figure(
        figure, output / f"comparison_relative_to_{slugify(baseline)}", formats, dpi
    )


def generate_plots(
    records: Sequence[Benchmark],
    output: Path,
    formats: Sequence[str],
    dpi: int,
    max_points: int,
    baseline: str | None,
) -> list[Path]:
    output.mkdir(parents=True, exist_ok=True)
    generated = write_summaries(records, output)
    by_case = group_by_use_case(records)
    case_slugs = unique_case_slugs(by_case)

    for use_case, case_records in by_case.items():
        case_output = output / "use_cases" / case_slugs[use_case]
        generated.extend(
            plot_use_case_overview(use_case, case_records, case_output, formats, dpi)
        )
        generated.extend(
            plot_use_case_ecdf(use_case, case_records, case_output, formats, dpi)
        )
        generated.extend(
            plot_use_case_sequence(
                use_case,
                case_records,
                case_output,
                formats,
                dpi,
                max_points,
            )
        )
        generated.extend(
            plot_use_case_upload_trend(
                use_case, case_records, case_output, formats, dpi
            )
        )

    comparison_output = output / "comparisons"
    for kind in ("read", "upload", "other"):
        generated.extend(
            plot_comparison_distributions(
                records, kind, comparison_output, formats, dpi
            )
        )
    generated.extend(
        plot_upload_wallclock_comparison(records, comparison_output, formats, dpi)
    )
    generated.extend(plot_percentile_heatmap(records, comparison_output, formats, dpi))
    generated.extend(
        plot_upload_trend_comparison(records, comparison_output, formats, dpi)
    )
    if baseline:
        generated.extend(
            plot_relative_to_baseline(
                records, baseline, comparison_output, formats, dpi
            )
        )
    return generated


def main(argv: Sequence[str] | None = None) -> int:
    args = parse_arguments(argv)
    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(levelname)s: %(message)s",
    )
    # Avoid pages of font-manager debug output under --verbose.
    logging.getLogger("matplotlib").setLevel(logging.WARNING)

    try:
        records = discover_benchmarks(args.input, args.strict)
        configure_matplotlib()
        generated = generate_plots(
            records,
            args.output.resolve(),
            tuple(dict.fromkeys(args.formats)),
            args.dpi,
            args.max_points,
            args.baseline,
        )
    except (FileNotFoundError, OSError, ValueError) as exc:
        LOG.error("%s", exc)
        return 2

    use_case_count = len(group_by_use_case(records))
    sample_count = sum(record.timings_ms.size for record in records)
    print(
        f"Generated {len(generated)} files from {len(records)} benchmark files, "
        f"{use_case_count} use-cases, and {sample_count:,} timing samples in "
        f"{args.output.resolve()}"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
