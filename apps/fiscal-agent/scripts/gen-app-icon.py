#!/usr/bin/env python3
"""Generate Farvoo Agent + 开票 app icons (shared palette, distinct marks).

Writes:
  assets/agent_app_icon.ico     — F + orange dot (Agent EXE / installer)
  assets/fiscal_app_icon.ico    — F + folded doc badge A1 (开票 EXE / WebView / Client)
  assets/icon-previews/*        — PNG previews

Legacy alias:
  assets/app_icon.ico           — copy of agent_app_icon.ico (do not use in new paths)

Run from anywhere:
  python3 apps/fiscal-agent/scripts/gen-app-icon.py
"""
from __future__ import annotations

import math
import shutil
import struct
import zlib
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ASSETS = ROOT / "assets"
PREVIEWS = ASSETS / "icon-previews"
SIZES = (16, 32, 48, 256)

GREEN = (11, 61, 46)  # #0b3d2e — matches Admin --green
PAPER = (243, 239, 230)  # #f3efe6
ACCENT = (196, 92, 38)  # #c45c26
LINE_MUTED = (140, 120, 100)


def clamp(v: float) -> int:
    return max(0, min(255, int(v)))


def aa_cover(dist: float, half_px: float = 0.65) -> float:
    return max(0.0, min(1.0, 0.5 - dist / (2 * half_px)))


def blend(dst_rgb: tuple[int, int, int], src: tuple[int, int, int], a: float) -> tuple[int, int, int, int]:
    if a <= 0:
        return (*dst_rgb[:3], 255)
    if a >= 1:
        return (*src, 255)
    return (
        clamp(dst_rgb[0] * (1 - a) + src[0] * a),
        clamp(dst_rgb[1] * (1 - a) + src[1] * a),
        clamp(dst_rgb[2] * (1 - a) + src[2] * a),
        255,
    )


def rounded_rect_dist(x: float, y: float, size: int, radius: float, pad: float) -> float:
    cx = (size - 1) / 2
    cy = (size - 1) / 2
    hw = size / 2 - pad - radius
    hh = size / 2 - pad - radius
    dx = abs(x - cx) - hw
    dy = abs(y - cy) - hh
    outside = math.hypot(max(dx, 0), max(dy, 0))
    inside = min(max(dx, dy), 0)
    return outside + inside - radius


def blank(size: int) -> list[list[tuple[int, int, int, int]]]:
    return [[(0, 0, 0, 0) for _ in range(size)] for _ in range(size)]


def fill_rounded(px: list[list[tuple[int, int, int, int]]], size: int, rgb: tuple[int, int, int]) -> None:
    pad = size * 0.06
    radius = size * 0.22
    for y in range(size):
        for x in range(size):
            d = rounded_rect_dist(x + 0.5, y + 0.5, size, radius, pad)
            a = aa_cover(d)
            if a > 0:
                px[y][x] = (*rgb, clamp(a * 255))


def rect(
    px: list[list[tuple[int, int, int, int]]],
    size: int,
    x0: float,
    y0: float,
    x1: float,
    y1: float,
    color: tuple[int, int, int],
    soft: float = 0.55,
) -> None:
    for y in range(size):
        for x in range(size):
            d = max(max(x0 - x, x - x1), max(y0 - y, y - y1))
            a = aa_cover(d, soft)
            if a > 0:
                r, g, b, _ = px[y][x]
                px[y][x] = blend((r, g, b), color, a)


def draw_serif_f(
    px: list[list[tuple[int, int, int, int]]],
    size: int,
    color: tuple[int, int, int],
    *,
    scale: float = 1.0,
    ox: float = 0.0,
    oy: float = 0.0,
) -> None:
    s = float(size)
    soft = 0.55 if size >= 32 else 0.45

    def R(x0: float, y0: float, x1: float, y1: float) -> None:
        rect(
            px,
            size,
            x0 * scale + ox * s,
            y0 * scale + oy * s,
            x1 * scale + ox * s,
            y1 * scale + oy * s,
            color,
            soft,
        )

    stem_x0, stem_x1 = s * 0.28, s * 0.42
    R(stem_x0, s * 0.22, stem_x1, s * 0.78)
    R(stem_x0, s * 0.22, s * 0.72, s * 0.34)
    R(stem_x0, s * 0.46, s * 0.62, s * 0.56)
    R(s * 0.22, s * 0.22, stem_x0 + 1, s * 0.30)
    R(s * 0.22, s * 0.70, s * 0.48, s * 0.78)


def draw_accent_dot(px: list[list[tuple[int, int, int, int]]], size: int) -> None:
    cx, cy = size * 0.78, size * 0.78
    r = max(1.15, size * 0.10)
    for y in range(size):
        for x in range(size):
            dist = math.hypot(x + 0.5 - cx, y + 0.5 - cy) - r
            a = aa_cover(dist, 0.55)
            if a > 0:
                pr, pg, pb, pa = px[y][x]
                if pa == 0:
                    continue
                blended = blend((pr, pg, pb), ACCENT, a)
                px[y][x] = (*blended[:3], pa)


def draw_doc_badge(px: list[list[tuple[int, int, int, int]]], size: int) -> None:
    """A1: folded invoice plate, bottom-right (开票 mark)."""
    s = float(size)
    x0, y0, x1, y1 = s * 0.58, s * 0.55, s * 0.88, s * 0.88
    cx, cy = (x0 + x1) / 2, (y0 + y1) / 2
    hw, hh = (x1 - x0) / 2 - s * 0.02, (y1 - y0) / 2 - s * 0.02
    rad = s * 0.04
    for y in range(size):
        for x in range(size):
            dx = abs(x + 0.5 - cx) - hw + rad
            dy = abs(y + 0.5 - cy) - hh + rad
            d = math.hypot(max(dx, 0), max(dy, 0)) + min(max(dx, dy), 0) - rad
            a = aa_cover(d, 0.55)
            if a > 0:
                pr, pg, pb, pa = px[y][x]
                if pa == 0:
                    continue
                blended = blend((pr, pg, pb), PAPER, a)
                px[y][x] = (*blended[:3], pa)
    # dog-ear
    darker = (220, 214, 200)
    for y in range(size):
        for x in range(size):
            if x + 0.5 >= s * 0.74 and y + 0.5 <= s * 0.68 and (x + 0.5 - s * 0.74) + (s * 0.68 - (y + 0.5)) <= s * 0.14:
                pr, pg, pb, pa = px[y][x]
                if pa == 0:
                    continue
                px[y][x] = (*blend((pr, pg, pb), darker, 0.85)[:3], pa)
    soft = 0.4 if size >= 32 else 0.35
    for i, yy in enumerate((0.64, 0.72, 0.80)):
        w = 0.18 if i < 2 else 0.12
        color = ACCENT if i == 0 else LINE_MUTED
        rect(px, size, s * 0.64, s * yy, s * (0.64 + w), s * (yy + 0.035), color, soft)


def make_agent_icon(size: int) -> list[list[tuple[int, int, int, int]]]:
    px = blank(size)
    fill_rounded(px, size, GREEN)
    draw_serif_f(px, size, PAPER)
    draw_accent_dot(px, size)
    return px


def make_fiscal_icon(size: int) -> list[list[tuple[int, int, int, int]]]:
    """A1 开票：F + folded doc badge."""
    px = blank(size)
    fill_rounded(px, size, GREEN)
    draw_serif_f(px, size, PAPER, scale=0.92, ox=-0.02, oy=-0.02)
    draw_doc_badge(px, size)
    return px


def png_bytes(px: list[list[tuple[int, int, int, int]]]) -> bytes:
    h, w = len(px), len(px[0])

    def chunk(tag: bytes, payload: bytes) -> bytes:
        return struct.pack(">I", len(payload)) + tag + payload + struct.pack(">I", zlib.crc32(tag + payload) & 0xFFFFFFFF)

    raw = b"".join(b"\x00" + bytes(c for p in row for c in p) for row in px)
    ihdr = struct.pack(">IIBBBBB", w, h, 8, 6, 0, 0, 0)
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", ihdr) + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b"")


def dib_bgra(px: list[list[tuple[int, int, int, int]]]) -> bytes:
    """BMP DIB for ICO: BITMAPINFOHEADER + BGRA bottom-up XOR + AND mask."""
    size = len(px)
    xor = bytearray()
    for y in range(size - 1, -1, -1):
        for x in range(size):
            r, g, b, a = px[y][x]
            xor.extend((b, g, r, a))
    row_and = ((size + 31) // 32) * 4
    and_rows = []
    for y in range(size - 1, -1, -1):
        bits = 0
        row = bytearray()
        count = 0
        for x in range(size):
            bits = (bits << 1) | (0 if px[y][x][3] > 0 else 1)
            count += 1
            if count == 8:
                row.append(bits)
                bits = 0
                count = 0
        if count:
            bits <<= 8 - count
            row.append(bits)
        while len(row) < row_and:
            row.append(0)
        and_rows.append(bytes(row))
    and_mask = b"".join(and_rows)
    header = struct.pack(
        "<IIIHHIIIIII",
        40,
        size,
        size * 2,
        1,
        32,
        0,
        len(xor) + len(and_mask),
        0,
        0,
        0,
        0,
    )
    return header + bytes(xor) + and_mask


def write_ico(path: Path, make, sizes: tuple[int, ...] = SIZES) -> None:
    images = []
    for s in sizes:
        px = make(s)
        if s >= 256:
            payload = png_bytes(px)
        else:
            payload = dib_bgra(px)
        images.append((s, payload))

    count = len(images)
    offset = 6 + 16 * count
    entries = b""
    blobs = b""
    for s, payload in images:
        w = 0 if s >= 256 else s
        h = 0 if s >= 256 else s
        entries += struct.pack("<BBBBHHII", w, h, 0, 0, 1, 32, len(payload), offset)
        blobs += payload
        offset += len(payload)
    path.write_bytes(struct.pack("<HHH", 0, 1, count) + entries + blobs)


def write_previews(prefix: str, make) -> None:
    for s in SIZES:
        out = PREVIEWS / f"{prefix}_{s}.png"
        out.write_bytes(png_bytes(make(s)))
        print("wrote", out)


def main() -> None:
    ASSETS.mkdir(parents=True, exist_ok=True)
    PREVIEWS.mkdir(parents=True, exist_ok=True)

    agent_ico = ASSETS / "agent_app_icon.ico"
    fiscal_ico = ASSETS / "fiscal_app_icon.ico"
    write_ico(agent_ico, make_agent_icon)
    write_ico(fiscal_ico, make_fiscal_icon)
    print("wrote", agent_ico, agent_ico.stat().st_size, "bytes")
    print("wrote", fiscal_ico, fiscal_ico.stat().st_size, "bytes")

    # Legacy path kept as agent alias so old docs/scripts do not 404 mid-migrate.
    legacy = ASSETS / "app_icon.ico"
    shutil.copyfile(agent_ico, legacy)
    print("wrote", legacy, "(alias of agent_app_icon.ico)")

    write_previews("agent_app_icon", make_agent_icon)
    write_previews("fiscal_app_icon", make_fiscal_icon)
    # Keep old preview names pointing at agent for continuity.
    write_previews("app_icon", make_agent_icon)


if __name__ == "__main__":
    main()
