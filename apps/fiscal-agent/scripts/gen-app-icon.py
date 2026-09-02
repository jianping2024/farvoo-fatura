#!/usr/bin/env python3
"""Generate Farvoo 开票 app icon (F + accent dot on forest green).

Writes:
  assets/app_icon.ico          — multi-size (16/32/48/256) for EXE + Inno
  assets/icon-previews/*       — PNG previews

Run from anywhere:
  python3 apps/fiscal-agent/scripts/gen-app-icon.py
"""
from __future__ import annotations

import math
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


def clamp(v: float) -> int:
    return max(0, min(255, int(v)))


def aa_cover(dist: float, half_px: float = 0.65) -> float:
    return max(0.0, min(1.0, 0.5 - dist / (2 * half_px)))


def blend(dst_rgb: tuple[int, int, int], src: tuple[int, int, int], a: float) -> tuple[int, int, int, int]:
    if a <= 0:
        return (*dst_rgb, 255) if len(dst_rgb) == 3 else (*dst_rgb[:3], 255)
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


def draw_serif_f(px: list[list[tuple[int, int, int, int]]], size: int, color: tuple[int, int, int]) -> None:
    s = float(size)
    soft = 0.55 if size >= 32 else 0.45

    def rect(x0: float, y0: float, x1: float, y1: float) -> None:
        for y in range(size):
            for x in range(size):
                d = max(max(x0 - x, x - x1), max(y0 - y, y - y1))
                a = aa_cover(d, soft)
                if a > 0:
                    r, g, b, _ = px[y][x]
                    px[y][x] = blend((r, g, b), color, a)

    stem_x0, stem_x1 = s * 0.28, s * 0.42
    rect(stem_x0, s * 0.22, stem_x1, s * 0.78)
    rect(stem_x0, s * 0.22, s * 0.72, s * 0.34)
    rect(stem_x0, s * 0.46, s * 0.62, s * 0.56)
    rect(s * 0.22, s * 0.22, stem_x0 + 1, s * 0.30)
    rect(s * 0.22, s * 0.70, s * 0.48, s * 0.78)


def make_icon(size: int) -> list[list[tuple[int, int, int, int]]]:
    px = [[(0, 0, 0, 0) for _ in range(size)] for _ in range(size)]
    pad = size * 0.06
    radius = size * 0.22
    for y in range(size):
        for x in range(size):
            d = rounded_rect_dist(x + 0.5, y + 0.5, size, radius, pad)
            a = aa_cover(d)
            if a > 0:
                px[y][x] = (*GREEN, clamp(a * 255))
    draw_serif_f(px, size, PAPER)
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
    # AND mask: 1 bit/pixel (0 = opaque), rows padded to 32-bit, bottom-up
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


def write_ico(path: Path, sizes: tuple[int, ...] = SIZES) -> None:
    images = []
    for s in sizes:
        px = make_icon(s)
        # Prefer PNG-compressed entries for 256; DIB for classic small sizes (broader Win shell support).
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


def main() -> None:
    ASSETS.mkdir(parents=True, exist_ok=True)
    PREVIEWS.mkdir(parents=True, exist_ok=True)
    ico = ASSETS / "app_icon.ico"
    write_ico(ico)
    print("wrote", ico, ico.stat().st_size, "bytes")
    for s in SIZES:
        out = PREVIEWS / f"app_icon_{s}.png"
        out.write_bytes(png_bytes(make_icon(s)))
        print("wrote", out)


if __name__ == "__main__":
    main()
