"""Read screenshot pixels to verify straight border segments at display scales."""
import json
import math
import re
import sys
from PIL import Image

image = Image.open(sys.argv[1]).convert('RGB')
metadata = json.load(open(sys.argv[2], encoding='utf-8'))
scale = metadata['scale']
for index, box in enumerate(metadata['boxes']):
    border = tuple(map(int, re.findall(r'\d+', box['border'])[:3]))
    background = tuple(map(int, re.findall(r'\d+', box['background'])[:3]))
    channel = max(range(3), key=lambda c: abs(border[c] - background[c]))
    expected = abs(border[channel] - background[channel])
    assert expected >= 40, ('weak border contrast', index, border, background)
    x, y, w, h = [box[key] * scale for key in ['x', 'y', 'width', 'height']]
    for side in ['top', 'bottom', 'left', 'right']:
        for fraction in [.2, .5, .8]:
            if side in ['top', 'bottom']:
                fixed = round(x + w * fraction)
                edge = y if side == 'top' else y + h
                values = range(math.floor(edge) - 1, math.ceil(edge) + math.ceil(2 * scale) + 1) if side == 'top' else range(math.floor(edge) - math.ceil(2 * scale), math.ceil(edge) + 1)
                pixels = [image.getpixel((fixed, value)) for value in values if 0 <= value < image.height]
            else:
                fixed = round(y + h * fraction)
                edge = x if side == 'left' else x + w
                values = range(math.floor(edge) - 1, math.ceil(edge) + math.ceil(2 * scale) + 1) if side == 'left' else range(math.floor(edge) - math.ceil(2 * scale), math.ceil(edge) + 1)
                pixels = [image.getpixel((value, fixed)) for value in values if 0 <= value < image.width]
            coverage = max(abs(pixel[channel] - background[channel]) / expected for pixel in pixels)
            assert coverage >= .35, ('border segment disappeared', index, side, fraction, coverage)
print('Border pixels verified:', len(metadata['boxes']), 'cards at scale', scale)
