# /// script
# requires-python = ">=3.10"
# dependencies = [
#     "pillow",
# ]
# ///

import argparse
import string
import math
import sys
import os

# Pillow is imported after the metadata block so uv handles the install first
from PIL import Image, ImageDraw, ImageFont


def get_charset(mode):
    """Returns the character set based on the selected mode."""
    if mode == "ascii":
        # Standard printable ASCII (32-126)
        return string.printable[:95]
    elif mode == "numeric":
        return string.digits
    elif mode == "alphanumeric":
        return string.digits + string.ascii_letters
    elif mode == "extended":
        # ASCII + common latin-1 supplement characters
        return string.printable[:95] + "".join([chr(c) for c in range(161, 256)])
    else:
        return mode  # Treat input as a custom string of characters


def generate_font_texture(
    font_path, font_size, char_mode, output_path, padding, background_color, text_color
):
    try:
        # 1. Load the font
        font = ImageFont.truetype(font_path, font_size)
    except IOError:
        print(f"Error: Could not load font at {font_path}")
        sys.exit(1)
    except Exception as e:
        print(f"An unexpected error occurred: {e}")
        sys.exit(1)

    chars = get_charset(char_mode)
    print(f"Generating texture for {len(chars)} characters...")

    # 2. Calculate Cell Dimensions (Max Width/Height)
    # Track bounding box extremes to handle glyphs that extend above/below/left of the origin
    min_left = 0
    min_top = 0
    max_right = 0
    max_bottom = 0

    # We use a dummy image to measure text accurately
    dummy_draw = ImageDraw.Draw(Image.new("RGBA", (1, 1)))

    for char in chars:
        bbox = dummy_draw.textbbox((0, 0), char, font=font)
        # bbox is (left, top, right, bottom)
        min_left = min(min_left, bbox[0])
        min_top = min(min_top, bbox[1])
        max_right = max(max_right, bbox[2])
        max_bottom = max(max_bottom, bbox[3])

    # Add user-defined padding
    cell_width = (max_right - min_left) + (padding * 2)
    cell_height = (max_bottom - min_top) + (padding * 2)

    # 3. Calculate Texture Size
    num_chars = len(chars)
    cols = math.ceil(math.sqrt(num_chars))
    rows = math.ceil(num_chars / cols)

    image_width = cols * cell_width
    image_height = rows * cell_height

    # 4. Create Image
    # 'RGBA' ensures we support transparency.
    img = Image.new("RGBA", (image_width, image_height), background_color)
    draw = ImageDraw.Draw(img)

    # 5. Render Characters
    for i, char in enumerate(chars):
        row = i // cols
        col = i % cols

        # Offset by min_left/min_top so glyphs that extend in negative space are not clipped
        x = (col * cell_width) + padding - min_left
        y = (row * cell_height) + padding - min_top

        draw.text((x, y), char, font=font, fill=text_color)

    # 6. Save
    img.save(output_path)
    print(f"Success! Texture saved to: {output_path}")
    print(f"Grid Layout: {cols} cols x {rows} rows")
    print(f"Cell Size: {cell_width}px x {cell_height}px")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Export a font to a grid texture.")

    parser.add_argument("font_path", help="Path to the .ttf or .otf font file.")
    parser.add_argument(
        "--size", type=int, default=32, help="Font size in points (default: 32)."
    )
    parser.add_argument(
        "--charset",
        default="ascii",
        help="Character set: 'ascii', 'extended', 'numeric', or a custom string.",
    )
    parser.add_argument(
        "--output",
        default="font_texture.png",
        help="Output filename (default: font_texture.png).",
    )
    parser.add_argument(
        "--padding",
        type=int,
        default=2,
        help="Padding around each character in pixels (default: 2).",
    )

    args = parser.parse_args()

    # Colors: Transparent background and White text
    bg_color = (0, 0, 0, 0)
    txt_color = (255, 255, 255, 255)

    generate_font_texture(
        args.font_path,
        args.size,
        args.charset,
        args.output,
        args.padding,
        bg_color,
        txt_color,
    )
