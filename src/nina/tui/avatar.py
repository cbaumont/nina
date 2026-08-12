"""Pixel-art avatar of Nina, rendered with Unicode half-blocks.

Each "pixel" is a Catppuccin Mocha color (or `None` for transparent). Two
pixel rows are packed into one terminal row using a half-block character,
doubling vertical resolution — the same trick terminal image tools like
`chafa`/`viu` use.
"""

from __future__ import annotations

from rich.style import Style
from rich.text import Text

# Catppuccin Mocha colors, named by their role in the artwork.
HAIR = "#cba6f7"  # Mauve
SKIN = "#fab387"  # Peach
FRAME = "#585b70"  # Surface2 - glasses frame
LENS = "#94e2d5"  # Teal - glasses lens tint
MOUTH = "#f38ba8"  # Red
CLOTHES = "#89b4fa"  # Blue - shoulders/collar

Pixel = str | None
PixelGrid = list[list[Pixel]]

_HeadSpec = list[tuple[int, int]]  # (skin_radius, hair_radius) per row, from center outward


def _ring_row(half_width: int, skin_r: int, hair_r: int) -> list[Pixel]:
    row: list[Pixel] = []
    for dx in range(half_width):
        if dx < skin_r:
            row.append(SKIN)
        elif dx < hair_r:
            row.append(HAIR)
        else:
            row.append(None)
    return row


def _cloth_row(half_width: int, cloth_r: int) -> list[Pixel]:
    return [CLOTHES if dx < cloth_r else None for dx in range(half_width)]


def _build(
    half_width: int,
    head_spec: _HeadSpec,
    cloth_spec: list[int],
    glasses_at: list[int],
    glasses_frame_dx: list[int],
    glasses_lens_dx: list[int],
    mouth_y: int,
    mouth_dx: list[int],
) -> PixelGrid:
    rows = [_ring_row(half_width, skin_r, hair_r) for skin_r, hair_r in head_spec]
    for y in glasses_at:
        for dx in glasses_frame_dx:
            rows[y][dx] = FRAME
        for dx in glasses_lens_dx:
            rows[y][dx] = LENS
    for dx in mouth_dx:
        rows[mouth_y][dx] = MOUTH
    rows.extend(_cloth_row(half_width, r) for r in cloth_spec)
    # dx=0 is the innermost (center) column; mirror outward on both sides.
    return [row[::-1] + row for row in rows]


PIXELS: PixelGrid = _build(
    half_width=12,
    head_spec=[
        (0, 0),
        (0, 1),
        (0, 3),
        (0, 5),
        (0, 6),
        (0, 7),
        (4, 7),
        (5, 7),
        (6, 7),
        (6, 7),  # glasses: frame top
        (6, 7),  # glasses: lens
        (6, 7),  # glasses: frame bottom
        (6, 7),
        (6, 7),  # mouth
        (5, 7),
        (4, 7),
        (3, 6),
        (2, 5),
    ],
    cloth_spec=[5, 7, 9, 10, 11, 12],
    glasses_at=[9, 10, 11],
    glasses_frame_dx=[0, 1, 4, 5],
    glasses_lens_dx=[2, 3],
    mouth_y=13,
    mouth_dx=[0, 1],
)

PIXELS_MINI: PixelGrid = _build(
    half_width=6,
    head_spec=[
        (0, 0),
        (0, 2),
        (0, 4),
        (2, 5),
        (3, 5),  # glasses: frame top
        (3, 5),  # glasses: lens
        (3, 5),  # glasses: frame bottom
        (3, 5),  # mouth
        (2, 5),
        (1, 4),
    ],
    cloth_spec=[4, 6],
    glasses_at=[4, 5, 6],
    glasses_frame_dx=[0, 2],
    glasses_lens_dx=[1],
    mouth_y=7,
    mouth_dx=[0],
)


def _cell(fg: Pixel, bg: Pixel) -> tuple[str, Style | None]:
    if fg is None and bg is None:
        return " ", None
    if fg is not None and bg is not None:
        return "▀", Style(color=fg, bgcolor=bg)
    if fg is not None:
        return "▀", Style(color=fg)
    return "▄", Style(color=bg)


def _render(pixels: PixelGrid) -> Text:
    text = Text()
    height = len(pixels)
    width = len(pixels[0]) if pixels else 0
    for y in range(0, height, 2):
        top = pixels[y]
        bottom = pixels[y + 1] if y + 1 < height else [None] * width
        for x in range(width):
            char, style = _cell(top[x], bottom[x])
            text.append(char, style=style)
        if y + 2 < height:
            text.append("\n")
    return text


def render() -> Text:
    """The full startup-banner avatar (24x24 pixels, 12 terminal rows)."""
    return _render(PIXELS)


def render_mini() -> Text:
    """The compact header-icon avatar (12x12 pixels, 6 terminal rows)."""
    return _render(PIXELS_MINI)
