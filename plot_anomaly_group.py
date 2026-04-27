#!/usr/bin/env python3
"""
AIS Anomaly Group Plotter

Fetches an anomaly group by ID and plots AIS positions, base stations, and
lines connecting positions to their receiving base station.

Display modes
-------------
lines     (default)  Lines are colored by signal strength; label sits on the line.
                     View covers all relevant base stations + positions.
positions            Dots are colored by signal strength; label sits on each dot.
                     View is zoomed to the position reports only.

Usage:
    python plot_anomaly_group.py [group_id] [--mode lines|positions]
    python plot_anomaly_group.py            # prompts for ID interactively
"""

import argparse
import io
import math
import signal
import sys

import matplotlib.image as mpimg
import matplotlib.colors as mcolors
import matplotlib.patches as mpatches
import matplotlib.pyplot as plt
import requests


BASE_URL = "http://localhost:3000/api/v1"
TILE_URL = "https://tile.openstreetmap.org/{z}/{x}/{y}.png"
TILE_HEADERS = {"User-Agent": "AIS-Anomaly-Plotter/1.0 (educational project)"}

_tile_cache: dict = {}


def fetch_json(endpoint: str) -> dict:
    response = requests.get(f"{BASE_URL}{endpoint}")
    response.raise_for_status()
    return response.json()


def signal_to_color(strength: float, alpha: float = 1.0) -> tuple:
    """Map signal strength [0–1] to RdYlGn color (red=weak, green=strong)."""
    rgba = list(plt.cm.RdYlGn(max(0.0, min(1.0, strength))))
    rgba[3] = alpha
    return tuple(rgba)


# ---------------------------------------------------------------------------
# OSM tile helpers
# ---------------------------------------------------------------------------

def _lat_lon_to_tile(lat: float, lon: float, zoom: int) -> tuple[int, int]:
    n = 2 ** zoom
    x = int((lon + 180.0) / 360.0 * n)
    lat_rad = math.radians(lat)
    y = int((1.0 - math.asinh(math.tan(lat_rad)) / math.pi) / 2.0 * n)
    return max(0, min(n - 1, x)), max(0, min(n - 1, y))


def _tile_bounds(x: int, y: int, zoom: int) -> tuple[float, float, float, float]:
    """Returns (west_lon, east_lon, south_lat, north_lat) for a tile."""
    n = 2 ** zoom
    west = x / n * 360.0 - 180.0
    east = (x + 1) / n * 360.0 - 180.0
    north = math.degrees(math.atan(math.sinh(math.pi * (1 - 2 * y / n))))
    south = math.degrees(math.atan(math.sinh(math.pi * (1 - 2 * (y + 1) / n))))
    return west, east, south, north


def _fetch_tile(z: int, x: int, y: int):
    key = (z, x, y)
    if key in _tile_cache:
        return _tile_cache[key]
    try:
        r = requests.get(TILE_URL.format(z=z, x=x, y=y), headers=TILE_HEADERS, timeout=10)
        r.raise_for_status()
        img = mpimg.imread(io.BytesIO(r.content))
        _tile_cache[key] = img
        return img
    except Exception as e:
        print(f"  Warning: tile {z}/{x}/{y}: {e}")
        return None


def _add_osm_background(ax, min_lon: float, max_lon: float, min_lat: float, max_lat: float) -> None:
    """Fetch OSM tiles and display them as a map background."""
    extent = max(max_lon - min_lon, max_lat - min_lat)
    if extent > 15:
        zoom = 5
    elif extent > 8:
        zoom = 6
    elif extent > 4:
        zoom = 7
    elif extent > 2:
        zoom = 8
    elif extent > 1:
        zoom = 9
    elif extent > 0.5:
        zoom = 10
    elif extent > 0.2:
        zoom = 11
    else:
        zoom = 12

    pad = max(extent * 0.12, 0.05)
    x0, y0 = _lat_lon_to_tile(max_lat + pad, min_lon - pad, zoom)  # NW → small y
    x1, y1 = _lat_lon_to_tile(min_lat - pad, max_lon + pad, zoom)  # SE → large y

    if (x1 - x0 + 1) * (y1 - y0 + 1) > 100:
        zoom = max(3, zoom - 1)
        x0, y0 = _lat_lon_to_tile(max_lat + pad, min_lon - pad, zoom)
        x1, y1 = _lat_lon_to_tile(min_lat - pad, max_lon + pad, zoom)

    total = (x1 - x0 + 1) * (y1 - y0 + 1)
    print(f"  Fetching {total} OSM tile(s) at zoom {zoom}...")

    for tx in range(x0, x1 + 1):
        for ty in range(y0, y1 + 1):
            img = _fetch_tile(zoom, tx, ty)
            if img is not None:
                west, east, south, north = _tile_bounds(tx, ty, zoom)
                ax.imshow(img, extent=[west, east, south, north],
                          aspect="auto", zorder=0, interpolation="bilinear")


# ---------------------------------------------------------------------------
# Main plot
# ---------------------------------------------------------------------------

def plot_anomaly_group(group_id: int, mode: str = "lines") -> None:
    print(f"Fetching anomaly group {group_id}...")
    group = fetch_json(f"/anomaly-groups/{group_id}")

    print("Fetching base stations...")
    stations_data = fetch_json("/base-stations")

    stations: dict[int, dict] = {}
    for feature in stations_data["features"]:
        sid = feature["properties"]["id"]
        lon, lat = feature["geometry"]["coordinates"]
        stations[sid] = {"name": feature["properties"]["name"], "lat": lat, "lon": lon}

    props = group.get("properties", {})
    anomalies = [f["properties"] for f in group.get("features", [])]

    if not anomalies:
        print("No anomalies found in this group.")
        return

    # --- Compute bounding box ---
    # "positions" mode: zoom to position reports only.
    # "lines" mode: include the connected base stations as well.
    pos_lats, pos_lons = [], []
    station_lats, station_lons = [], []
    for anomaly in anomalies:
        for report in anomaly.get("metadata", {}).get("positionReports", []):
            pos_lats.append(report["latitude"])
            pos_lons.append(report["longitude"])
        sid = anomaly.get("sourceId")
        if sid in stations:
            station_lats.append(stations[sid]["lat"])
            station_lons.append(stations[sid]["lon"])

    if mode == "positions":
        bound_lats, bound_lons = pos_lats, pos_lons
    else:
        bound_lats = pos_lats + station_lats
        bound_lons = pos_lons + station_lons

    lat_range = max(bound_lats) - min(bound_lats)
    lon_range = max(bound_lons) - min(bound_lons)
    if mode == "positions":
        lat_pad = max(lat_range * 0.15, 0.02)
        lon_pad = max(lon_range * 0.15, 0.02)
    else:
        lat_pad = max(lat_range * 0.20, 0.3)
        lon_pad = max(lon_range * 0.20, 0.3)
    view_min_lon = min(bound_lons) - lon_pad
    view_max_lon = max(bound_lons) + lon_pad
    view_min_lat = min(bound_lats) - lat_pad
    view_max_lat = max(bound_lats) + lat_pad

    fig, ax = plt.subplots(figsize=(13, 10))

    # --- OSM background ---
    print("Loading map...")
    _add_osm_background(ax, view_min_lon, view_max_lon, view_min_lat, view_max_lat)

    # --- Lines (lines mode only) ---
    if mode == "lines":
        for anomaly in anomalies:
            source_id = anomaly.get("sourceId")
            strength = anomaly.get("signalStrength", 0.5)

            if source_id not in stations:
                continue

            station = stations[source_id]
            pos_reports = anomaly.get("metadata", {}).get("positionReports", [])
            line_color = signal_to_color(strength, alpha=0.65)

            for report in pos_reports:
                lat, lon = report["latitude"], report["longitude"]

                ax.plot(
                    [lon, station["lon"]],
                    [lat, station["lat"]],
                    color=line_color,
                    linewidth=1.4,
                    zorder=2,
                )

                mid_lon = (lon + station["lon"]) / 2
                mid_lat = (lat + station["lat"]) / 2
                ax.text(
                    mid_lon, mid_lat,
                    f"{strength:.2f}",
                    fontsize=6.5,
                    ha="center",
                    va="center",
                    color="black",
                    zorder=6,
                    bbox=dict(
                        boxstyle="round,pad=0.15",
                        facecolor="white",
                        edgecolor=signal_to_color(strength),
                        alpha=0.85,
                        linewidth=0.8,
                    ),
                )

    # --- Position report dots ---
    for anomaly in anomalies:
        strength = anomaly.get("signalStrength", 0.5)
        dot_color = signal_to_color(strength)

        for report in anomaly.get("metadata", {}).get("positionReports", []):
            dot_size = 10 if mode == "positions" else 7
            ax.plot(
                report["longitude"], report["latitude"],
                "o",
                color=dot_color,
                markersize=dot_size,
                markeredgecolor="black",
                markeredgewidth=0.5,
                zorder=4,
            )

            if mode == "positions":
                ax.text(
                    report["longitude"], report["latitude"],
                    f"{strength:.2f}",
                    fontsize=7,
                    ha="left",
                    va="bottom",
                    color="black",
                    zorder=6,
                    bbox=dict(
                        boxstyle="round,pad=0.15",
                        facecolor="white",
                        edgecolor=dot_color,
                        alpha=0.85,
                        linewidth=0.8,
                    ),
                )

    # --- Base stations ---
    for sid, s in stations.items():
        ax.plot(
            s["lon"], s["lat"],
            marker="^",
            color="#1a4db5",
            markersize=13,
            markeredgecolor="white",
            markeredgewidth=0.8,
            linestyle="None",
            zorder=5,
        )
        ax.annotate(
            s["name"],
            xy=(s["lon"], s["lat"]),
            xytext=(6, 6),
            textcoords="offset points",
            fontsize=8,
            color="#1a4db5",
            fontweight="bold",
        )

    # --- Colorbar ---
    norm = mcolors.Normalize(vmin=0, vmax=1)
    sm = plt.cm.ScalarMappable(cmap=plt.cm.RdYlGn, norm=norm)
    sm.set_array([])
    cbar = plt.colorbar(sm, ax=ax, fraction=0.03, pad=0.02)
    cbar.set_label("Signal Strength", fontsize=10)
    cbar.set_ticks([0.0, 0.25, 0.5, 0.75, 1.0])
    cbar.set_ticklabels(["Weak (0)", "0.25", "0.50", "0.75", "Strong (1)"])

    # --- Final view ---
    ax.set_xlim(view_min_lon, view_max_lon)
    ax.set_ylim(view_min_lat, view_max_lat)
    ax.set_xlabel("Longitude", fontsize=10)
    ax.set_ylabel("Latitude", fontsize=10)

    started = props.get("startedAt", "")[:19].replace("T", " ")
    ax.set_title(
        f"Anomaly Group {group_id}  |  MMSI: {props.get('mmsi', 'N/A')}  "
        f"|  Type: {props.get('type', 'N/A')}  |  Mode: {mode}\n"
        f"Anomalies: {len(anomalies)}  |  Started: {started}",
        fontsize=11,
    )
    ax.grid(True, alpha=0.15, linestyle="--", color="gray")

    legend_handles = [
        mpatches.Patch(color="#1a4db5", label="Base station"),
        mpatches.Patch(color=plt.cm.RdYlGn(1.0), label="Strong signal"),
        mpatches.Patch(color=plt.cm.RdYlGn(0.0), label="Weak signal"),
    ]
    ax.legend(handles=legend_handles, loc="lower right", fontsize=9)

    signal.signal(signal.SIGINT, signal.SIG_DFL)
    plt.tight_layout()
    plt.show()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Plot an AIS anomaly group.")
    parser.add_argument("group_id", nargs="?", type=int, help="Anomaly group ID")
    parser.add_argument(
        "--mode",
        choices=["lines", "positions"],
        default="lines",
        help=(
            "lines (default): signal strength shown on connecting lines. "
            "positions: signal strength shown on position dots, view zoomed to points."
        ),
    )
    args = parser.parse_args()

    gid = args.group_id
    if gid is None:
        gid = int(input("Enter anomaly group ID: ").strip())

    plot_anomaly_group(gid, mode=args.mode)
