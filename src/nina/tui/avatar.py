"""Pixel-art avatar of Nina, rendered with Unicode half-blocks.

The pixel grid below is a 3x block-averaged downscale of the user-supplied
`nina_v4.svg` artwork (a single-color outline silhouette with two square
glasses lenses), from its native 63x50 resolution down to 21x16. Two pixel
rows are packed into one terminal row using a half-block character,
doubling vertical resolution.
"""

from __future__ import annotations

from rich.style import Style
from rich.text import Text

COLOR = "#cba6f7"  # Catppuccin Mocha Mauve

Pixel = str | None
PixelGrid = list[list[Pixel]]

_ROWS = [
    "######.........######",
    "####............#####",
    "###...............###",
    "#..................##",
    "#...................#",
    "#...................#",
    "#...................#",
    "#...................#",
    "#...................#",
    "#.....##....###.....#",
    "#.....##....###.....#",
    "#.....##....###.....#",
    "......##....###......",
    "##.................##",
    "###...............###",
    "###...............###",
]

PIXELS: PixelGrid = [[COLOR if c == "#" else None for c in row] for row in _ROWS]


def _cell(fg: Pixel, bg: Pixel) -> tuple[str, Style | None]:
    if fg is None and bg is None:
        return " ", None
    if fg is not None and bg is not None:
        return "▀", Style(color=fg, bgcolor=bg)
    if fg is not None:
        return "▀", Style(color=fg)
    return "▄", Style(color=bg)


def render() -> Text:
    """The pixel-art avatar (21x16 pixels, 8 terminal rows)."""
    text = Text()
    height = len(PIXELS)
    width = len(PIXELS[0]) if PIXELS else 0
    for y in range(0, height, 2):
        top = PIXELS[y]
        bottom = PIXELS[y + 1] if y + 1 < height else [None] * width
        for x in range(width):
            char, style = _cell(top[x], bottom[x])
            text.append(char, style=style)
        if y + 2 < height:
            text.append("\n")
    return text
