"""Unit checks for synthetic fixture/format helpers; no network or credentials."""
import importlib.util
from pathlib import Path
import struct
import sys
import unittest
import zlib

sys.path.insert(0, str(Path(__file__).parent))
spec = importlib.util.spec_from_file_location('image_memory_smoke', Path(__file__).with_name('image-memory-smoke.py'))
smoke = importlib.util.module_from_spec(spec)
spec.loader.exec_module(smoke)


class ImageMemoryHelpersTest(unittest.TestCase):
    def test_only_explicit_loopback_origin(self):
        for origin in ('http://127.0.0.1:8788', 'http://localhost:8788/', 'http://[::1]:8788'):
            self.assertEqual(smoke.validate_origin(origin), origin.rstrip('/'))
        for origin in ('https://example.com', 'file:///tmp', 'http://127.0.0.1.evil',
                       'http://user:secret@127.0.0.1', 'http://localhost/data', 'http://localhost/?token=secret'):
            with self.subTest(origin=origin), self.assertRaises(ValueError):
                smoke.validate_origin(origin)

    def test_rgba16_png_and_adam7_rows_crc(self):
        pixel = b'\x80\0\x20\0\xc0\0\x80\0'
        for adam7 in (False, True):
            data = smoke.rgba16_png(2, 2, adam7)
            self.assertEqual(data[:8], b'\x89PNG\r\n\x1a\n')
            chunks = {}
            offset = 8
            while offset < len(data):
                length = struct.unpack_from('>I', data, offset)[0]
                kind, body = data[offset + 4:offset + 8], data[offset + 8:offset + 8 + length]
                crc = struct.unpack_from('>I', data, offset + 8 + length)[0]
                self.assertEqual(crc, zlib.crc32(kind + body) & 0xffffffff)
                chunks[kind] = body
                offset += length + 12
            self.assertEqual(offset, len(data))
            self.assertEqual(struct.unpack('>IIBBBBB', chunks[b'IHDR']), (2, 2, 16, 6, 0, 0, int(adam7)))
            raw = zlib.decompress(chunks[b'IDAT'])
            expected = b'\0' + pixel + b'\0' + pixel + b'\0' + pixel * 2 if adam7 else (b'\0' + pixel * 2) * 2
            self.assertEqual(raw, expected)
            self.assertEqual(chunks[b'IEND'], b'')

    def test_jpeg_dimensions_and_malformed_segments(self):
        # A minimal SOF header: this tests metadata parsing, not JPEG decoding.
        sof = b'\xff\xd8\xff\xc0' + struct.pack('>HBHHB', 8, 8, 1067, 1600, 0) + b'\xff\xd9'
        self.assertEqual(smoke.jpeg_dimensions(sof), (1600, 1067))
        for data in (b'not-jpeg', b'\xff\xd8\xff\xd9', sof[:-2], b'\xff\xd8\xff\xc0\xff\xff\xff\xd9'):
            with self.assertRaises(AssertionError):
                smoke.jpeg_dimensions(data)


if __name__ == '__main__':
    unittest.main()
