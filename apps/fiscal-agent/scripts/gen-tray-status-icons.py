#!/usr/bin/env python3
"""Generate 16x16 tray status ICOs (green / yellow / red). Run from apps/print-agent."""
import struct
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SIZE = 16


def bgra_pixels(rgb):
    r, g, b = rgb
    out = []
    cx, cy = 7.5, 7.5
    for y in range(SIZE):
        for x in range(SIZE):
            if (x - cx) ** 2 + (y - cy) ** 2 <= 7.5**2:
                out.extend([b, g, r, 255])
            else:
                out.extend([0, 0, 0, 0])
    return bytes(out)


def write_ico(path: Path, rgb):
    xor = bgra_pixels(rgb)
    and_mask = bytes([0xFF] * (SIZE * ((SIZE + 31) // 32) * 4))
    dib_header = struct.pack(
        "<IIIHHIIIIII",
        40,
        SIZE,
        SIZE * 2,
        1,
        32,
        0,
        len(xor) + len(and_mask),
        0,
        0,
        0,
        0,
    )
    image = dib_header + xor + and_mask
    entry = struct.pack("<BBBBHHII", SIZE, SIZE, 0, 0, 1, 32, len(image), 22)
    header = struct.pack("<HHH", 0, 1, 1)
    path.write_bytes(header + entry + image)


def main():
    colors = {
        "tray_icon_ok.ico": (34, 160, 70),
        "tray_icon_warn.ico": (220, 170, 30),
        "tray_icon_err.ico": (210, 55, 45),
    }
    for name, rgb in colors.items():
        out = ROOT / name
        write_ico(out, rgb)
        print("wrote", out, len(out.read_bytes()), "bytes")


if __name__ == "__main__":
    main()
