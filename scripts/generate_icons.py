#!/usr/bin/env python3
"""Generate the SynoDemo package icons without external libraries."""
import struct
import zlib
from pathlib import Path
root = Path(__file__).resolve().parents[1]
def chunk(kind, data):
    return struct.pack('>I', len(data)) + kind + data + struct.pack('>I', zlib.crc32(kind + data))
def icon(size):
    pixels = bytearray()
    for y in range(size):
        pixels.append(0)
        for x in range(size):
            points = [pixel((x+(sx+.5)/3)*64/size, (y+(sy+.5)/3)*64/size) for sy in range(3) for sx in range(3)]
            pixels.extend(round(sum(p[c] for p in points)/9) for c in range(4))
    header = struct.pack('>IIBBBBB', size, size, 8, 6, 0, 0, 0)
    return b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', header) + chunk(b'IDAT', zlib.compress(bytes(pixels), 9)) + chunk(b'IEND', b'')
def pixel(x, y):
    cx, cy = min(max(x, 16), 48), min(max(y, 16), 48)
    if not (3 <= x <= 61 and 3 <= y <= 61 and (x-cx)**2 + (y-cy)**2 <= 169): return (0,0,0,0)
    if (17 <= x <= 23 and 17 <= y <= 47) or (23 < x <= 34 and (17 <= y <= 22 or 42 <= y <= 47)) or (34 < x <= 42 and 20 <= y <= 44 and ((x-30)**2+(y-32)**2 <= 145 and (x-30)**2+(y-32)**2 >= 65)):
        return (255,255,255,255)
    return (26, 132, 225, 255)
for size in (16,24,32,48,64,72,128,256):
    data = icon(size)
    (root/'package'/'ui'/'images'/f'icon_{size}.png').write_bytes(data)
    if size in (64,256): (root/'package'/('PACKAGE_ICON.PNG' if size == 64 else 'PACKAGE_ICON_256.PNG')).write_bytes(data)
