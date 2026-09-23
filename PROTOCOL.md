# ForeverDubbed optical protocol FDB5

The sole supported format uses 16 dark navy colors and four bits per cell. A tile has a 50 × 50 cell grid including a one-cell calibration ring; its inner 48 × 48 cells reserve 136 cells for an animated sine wave. The remaining 2,168 cells carry 8,672 bits (1,084 bytes). A separate light-blue outline surrounds the calibration ring, exactly **2 physical pixels** thick on every side. Cells are integer 2–8 physical pixels wide, default 2. The total square is `50 × cell size + 4` pixels wide: **104 × 104** by default, or **154 × 154** with 3px cells. Coordinates start at the outer outline's top-left; x increases rightward and y downward.

FDB5 requires companion and addon version 0.5.0 or newer. Update both together: the static FDB4 format and previous optical formats are not accepted. Header fields are unchanged, but the frame magic and payload capacity have changed. Existing tile position, lock state, and FDB3/FDB4 cell-size settings are preserved.

The addon uses `(768 / physical screen height) / UIParent:GetEffectiveScale()` for its local scale, so cell sizes and outline thickness remain physical pixels regardless of UI scale. Grid origin is two pixels right and down from the outer tile origin.

## Palette

Chromaglyph OKLab pack: center hue 264, lightness 0.22, chroma 0.045. The closest source colors are approximately 6.08 RGB units apart (squared distance 37). Palette order is part of the protocol.

| Index (hex) | R | G | B |
| --- | --- | --- | --- |
| 0 | 16 | 26 | 47 |
| 1 | 0 | 28 | 49 |
| 2 | 30 | 25 | 46 |
| 3 | 14 | 27 | 59 |
| 4 | 18 | 24 | 35 |
| 5 | 5 | 26 | 38 |
| 6 | 24 | 27 | 57 |
| 7 | 5 | 28 | 58 |
| 8 | 8 | 27 | 48 |
| 9 | 26 | 24 | 38 |
| A | 23 | 26 | 47 |
| B | 19 | 19 | 45 |
| C | 15 | 33 | 50 |
| D | 9 | 20 | 46 |
| E | 15 | 27 | 53 |
| F | 17 | 25 | 41 |

All cells are opaque. Lua supplies channels divided by 255 to `SetColorTexture`.

## Border, calibration, and discovery

Evaluate these rules in order, for coordinates 0..49:

| Position | Palette index |
| --- | --- |
| y = 0 | (5x + 1) mod 8 |
| y = 49 and 1 <= x <= 16 | x − 1 (all sixteen reference swatches) |
| y = 49 otherwise | (3x + 6) mod 8 |
| x = 0 | (3y + 2) mod 8 |
| x = 49 | (5y + 4) mod 8 |

The outer outline is RGB **(128, 192, 240)** (`#80c0f0`), rendered opaque. Discovery scans for a light-blue top-left inner corner, then checks both pixels of all four outline sides at each supported cell size. The broad discovery gate requires R >=25, G >=70, B >=100, G−R >=8 and B−G >=8. Outline pixels must lie within 12 RGB units of the captured top-left color. A blue box alone is never sufficient: calibration, ring pattern, magic, dimensions, and frame checksum must also pass.

The reader samples cell center pixels. Sixteen bottom-row reference swatches define the palette **as captured**, on every page. Each pair must be at least 3 units apart in Euclidean RGB distance; transforms that merge colors are rejected. Both ring and data cells use the nearest observed reference, within 40% of that reference's distance to its nearest competing reference. No binary RGB thresholds are used for the dark cells.

This deliberately subtle palette trades noise tolerance for appearance. Mild consistent gamma/tint changes can be calibrated, but strong dark-level compression, spatially varying filters, blur, lossy screenshots, or HDR transforms can make it unreadable. Ambiguous samples are rejected; checksums catch corrupted packets rather than attempting error correction. Use original, lossless captures at native resolution.

Subsequent reads capture the complete discovered rectangle including the blue outline. Rotation, fractional rescaling, perspective, and arbitrary external image resizing are unsupported.

## Animated wave and cell reservation

The wave is one **hard-coded 48-column cell pattern** approximating a two-cell-wide stroke measured perpendicular to a sine curve. Each column lists its inclusive zero-based data row range. The pattern spans data rows 12–35, never the calibration ring, and always contains exactly **136 blue cells**. The same integer pattern is stored in Lua and Go.

Animation only cyclically shifts that pattern left by one whole column per frame. It never interpolates, recalculates a sine curve, or changes the stroke's shape. Cells wrapping off the left edge reappear at the right. At **15 fps**, the 48-frame loop takes **3.2 seconds**. All cells remain opaque and unaliased. Tests compare every captured mask against an exact cyclic translation of the original pattern.

Page rotation remains independent at 250 ms per page; coincident page and animation updates share one redraw. The addon re-packs the current frame's nibbles around the translated mask, including single-page messages. This changes only visual placement, not the packet sequence or checksum. Delayed updates advance by the elapsed number of ticks.

The encoder skips every wave cell, consuming no data nibble there. The decoder recognizes wave cells by comparing them with the captured outline color (within 12 RGB units) and skips them too. It requires one complete cyclic shift of the known pattern, exactly 136 reserved cells, and exactly 2,168 data nibbles. The receiver infers the shift from the screenshot; no clock synchronization or phase metadata is needed. Extra/missing blue cells, malformed masks, and checksum failures invalidate a frame. Captures spanning a redraw can be rejected and retried on the next poll.

## Frame bytes

Interior cells are read row by row, skipping the wave mask. Each palette index encodes a four-bit nibble; consecutive cells supply a byte's high nibble, then low nibble. All integer fields are unsigned big-endian.

| Byte offset | Size | Field |
| --- | --- | --- |
| 0 | 4 | ASCII `FDB5` |
| 4 | 4 | Session ID, changes when addon reloads |
| 8 | 4 | Sequence number, increments for each new message |
| 12 | 4 | Adler-32 of the complete unpadded message |
| 16 | 2 | Zero-based page index |
| 18 | 2 | Total page count, 1..256 |
| 20 | 2 | Payload length, 1..1056 |
| 22 | 1 | Message kind |
| 23 | 1 | Flags: 0 = text only, 1 = speaker metadata (0.3.0+) |
| 24 | 1056 | Payload followed by zero padding |
| 1080 | 4 | Adler-32 of bytes 0..1079, including padding |

Adler-32 uses initial a=1, b=0, modulus 65521, result b × 65536 + a. All pages except the final one contain exactly 1,056 payload bytes. Maximum message size is 270,336 bytes; oversized messages are rejected visibly in the addon rather than truncated.

With flags 0, the complete message is UTF-8 `speaker + NUL + title + NUL + text`. With flags 1, it is `speaker + NUL + title + NUL + text + NUL + race + NUL + gender + NUL + npcID`. Race is an English race key obtained from the API, an NPC-ID lookup, a model mapping, or a saved user assignment (for example `Orc` or `Skyborne`), gender is `male`, `female`, or empty, and NPC ID is decimal text or empty. Empty metadata fields mean unknown. All other flag values are rejected, and flags must agree across every page. FDB5 readers accept both layouts; older optical formats remain unsupported. Empty speaker/title fields are allowed; embedded NULs in fields are not. Byte chunks may split UTF-8 characters. Decode text only after concatenating and validating every page. WoW formatting escapes are removed before encoding.

Kinds: 0=test, 1=gossip/greeting, 2=quest offer, 3=quest progress, 4=quest completion, 5=NPC chat, 6=item/book text. Pocket TTS speaks title and text using the metadata-selected voice. The SAPI fallback joins nonempty speaker, title, and text fields. Metadata is never spoken.

## Delivery

The addon rotates pages every 250 ms and transmits for at least 15 seconds or three page cycles, whichever is longer. A newer message replaces the current one. The receiver holds at most one incomplete message, accepts pages in any order, deduplicates repeated pages, and emits once on complete validation. Incomplete pages expire after two minutes without accepted activity. No acknowledgment, compression, or error correction is used.

For a matching session, newer sequence numbers are determined with signed 32-bit serial arithmetic, supporting wraparound. Recent retired session IDs are ignored. Session IDs are a best-effort time-based reload discriminator, not cryptographic randomness. The transport and checksums are intended for one addon in a local screen-capture environment, not untrusted senders.
