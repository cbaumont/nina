"""Pixel-art avatar of Nina, rendered with Unicode half-blocks.

The pixel grid below is a 3x block-averaged downscale of the user-supplied
`nina_v4.svg` artwork (a single-color outline silhouette with two square
glasses lenses), from its native 63x50 resolution down to 21x16. Two pixel
rows are packed into one terminal row using a half-block character,
doubling vertical resolution.
"""

from __future__ import annotations

import textwrap

from rich.style import Style
from rich.text import Text

COLOR = "#cba6f7"  # Catppuccin Mocha Mauve

Pixel = str | None
PixelGrid = list[list[Pixel]]

_ROWS = [
    "######.........######",
    "####.............####",
    "###...............###",
    "#...................#",
    "#...................#",
    "#...................#",
    "#...................#",
    "#...................#",
    "#...................#",
    "#.....##.....##.....#",
    "#.....##.....##.....#",
    "#.....##.....##.....#",
    "......##.....##......",
    "##.................##",
    "###...............###",
    "###...............###",
]

# Filled ('#') cells are transparent and empty cells are colored, i.e. the
# artwork's negative — a solid silhouette with the outline/lenses cut out.
PIXELS: PixelGrid = [[None if c == "#" else COLOR for c in row] for row in _ROWS]

_TEXT_WIDTH = 44  # wrap width for the welcome copy beside the avatar


def _cell(fg: Pixel, bg: Pixel) -> tuple[str, Style | None]:
    if fg is None and bg is None:
        return " ", None
    if fg is not None and bg is not None:
        return "▀", Style(color=fg, bgcolor=bg)
    if fg is not None:
        return "▀", Style(color=fg)
    return "▄", Style(color=bg)


def _rows() -> list[Text]:
    """One Text per terminal row of the avatar (21 wide, 8 rows)."""
    height = len(PIXELS)
    width = len(PIXELS[0]) if PIXELS else 0
    lines = []
    for y in range(0, height, 2):
        top = PIXELS[y]
        bottom = PIXELS[y + 1] if y + 1 < height else [None] * width
        line = Text()
        for x in range(width):
            char, style = _cell(top[x], bottom[x])
            line.append(char, style=style)
        lines.append(line)
    return lines


def render() -> Text:
    """The pixel-art avatar (21x16 pixels, 8 terminal rows)."""
    text = Text()
    for i, line in enumerate(_rows()):
        if i:
            text.append("\n")
        text.append_text(line)
    return text


def welcome_panel(intro: str, cred_note: str | None = None) -> Text:
    """The startup banner: avatar on the left, welcome copy on the right,
    both boxed together — echoing Claude Code's CLI splash layout.

    Built by hand (rather than a Rich Panel/Table) because RichLog mangles
    box-drawing borders coming from those — see git history for the
    artifacts a Panel-based version produced.
    """
    avatar_lines = _rows()
    avatar_width = len(PIXELS[0]) if PIXELS else 0

    text_lines: list[Text] = [Text("Welcome to Nina", style=f"bold {COLOR}")]
    if intro:
        text_lines.append(Text(""))
        text_lines.extend(Text(line) for line in textwrap.wrap(intro, _TEXT_WIDTH))
    if cred_note:
        text_lines.append(Text(""))
        text_lines.extend(
            Text(line, style="dim italic") for line in textwrap.wrap(cred_note, _TEXT_WIDTH)
        )

    pad = 2  # spacing on either side of the avatar/text divider
    divider_width = pad * 2 + 1
    content_height = max(len(avatar_lines), len(text_lines))
    inner_width = avatar_width + divider_width + _TEXT_WIDTH

    panel = Text()
    panel.append("┌" + "─" * (inner_width + 2) + "┐\n", style=COLOR)
    for i in range(content_height):
        row = Text()
        row.append("│ ", style=COLOR)
        if i < len(avatar_lines):
            row.append_text(avatar_lines[i])
        else:
            row.append(" " * avatar_width)
        row.append(" " * pad)
        row.append("│", style=COLOR)
        row.append(" " * pad)
        if i < len(text_lines):
            line = text_lines[i]
            row.append_text(line)
            row.append(" " * (_TEXT_WIDTH - line.cell_len))
        else:
            row.append(" " * _TEXT_WIDTH)
        row.append(" │\n", style=COLOR)
        panel.append_text(row)
    panel.append("└" + "─" * (inner_width + 2) + "┘", style=COLOR)
    return panel
