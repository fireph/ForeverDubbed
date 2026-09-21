# ForeverDubbed optical protocol FDB3

The sole supported format uses 16 colors and four bits per cell. A tile is 50 × 50 cells including a one-cell border; its inner 48 × 48 cells carry 9,216 bits (1,152 bytes). Cells are integer 2–8 physical desktop pixels wide, default 2, making the default square 100 × 100 pixels. Coordinates start at the top-left; x increases rightward and y downward.

FDB3 requires companion/addon version 0.2.0 or newer. Previous 8-color formats are not accepted. Both components must be updated together.

Use addon 0.2.1 or newer for correct physical pixel sizing. Its local scale is `(768 / physical screen height) / UIParent:GetEffectiveScale()`, following Blizzard's [PixelUtil conversion](https://github.com/Gethe/wow-ui-source/blob/forever/Interface/AddOns/Blizzard_SharedXML/PixelUtil.lua). Version 0.2.1 retains the 0.2.0 wire format; 0.3.0 adds the optional metadata layout described below without changing the tile dimensions or palette.

## Palette

Eight RGB cube corners plus eight edge midpoints provide 16 distinct symbols. The closest pair is 127 RGB units apart in the source palette. Indices 0–7 retain binary RGB ordering for discovery.

| Index (hex) | R | G | B |
| --- | --- | --- | --- |
| 0 | 0 | 0 | 0 |
| 1 | 0 | 0 | 255 |
| 2 | 0 | 255 | 0 |
| 3 | 0 | 255 | 255 |
| 4 | 255 | 0 | 0 |
| 5 | 255 | 0 | 255 |
| 6 | 255 | 255 | 0 |
| 7 | 255 | 255 | 255 |
| 8 | 128 | 0 | 0 |
| 9 | 0 | 128 | 255 |
| A | 128 | 255 | 0 |
| B | 128 | 0 | 255 |
| C | 255 | 128 | 0 |
| D | 255 | 0 | 128 |
| E | 0 | 255 | 128 |
| F | 128 | 255 | 255 |

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

The reader samples cell center pixels. It searches for the top-row pattern at every supported cell size, then checks the remaining border, palette, frame magic, dimensions, and checksum. All non-calibration border colors use binary thresholds: channel <=72 means 0, >=183 means 1; intermediate values invalidate the candidate. The first eight calibration swatches must also match their binary corner indices.

The sixteen bottom-row reference swatches define the palette **as captured**, on every page. Each pair must be at least 32 units apart in Euclidean RGB distance. Data cells are classified by nearest observed reference; the distance to that reference must be at most 40% of its distance to its nearest competing reference. Cells outside that radius invalidate the page. This follows consistent gamma/tint changes, while rejecting ambiguous values rather than guessing bits. It does not correct spatially varying filters, blur, or transforms that collapse colors together.

Subsequent reads capture only the discovered rectangle. Candidate origins can differ from the exact visual edge by a pixel while still sampling cell interiors correctly. Rotation, fractional rescaling, perspective, and arbitrary external image resizing are unsupported.

## Frame bytes

Interior cells are read row by row. Each palette index encodes a four-bit nibble; consecutive cells supply a byte's high nibble, then low nibble. All integer fields are unsigned big-endian.

| Byte offset | Size | Field |
| --- | --- | --- |
| 0 | 4 | ASCII `FDB3` |
| 4 | 4 | Session ID, changes when addon reloads |
| 8 | 4 | Sequence number, increments for each new message |
| 12 | 4 | Adler-32 of the complete unpadded message |
| 16 | 2 | Zero-based page index |
| 18 | 2 | Total page count, 1..256 |
| 20 | 2 | Payload length, 1..1124 |
| 22 | 1 | Message kind |
| 23 | 1 | Flags: 0 = text only, 1 = speaker metadata (0.3.0+) |
| 24 | 1124 | Payload followed by zero padding |
| 1148 | 4 | Adler-32 of bytes 0..1147, including padding |

Adler-32 uses initial a=1, b=0, modulus 65521, result b × 65536 + a. All pages except the final one contain exactly 1,124 payload bytes. Maximum message size is 287,744 bytes; oversized messages are rejected visibly in the addon rather than truncated.

With flags 0, the complete message is UTF-8 `speaker + NUL + title + NUL + text`. With flags 1, it is `speaker + NUL + title + NUL + text + NUL + race + NUL + gender + NUL + npcID`. Race is an English race key obtained from the API, an NPC-ID lookup, a model mapping, or a saved user assignment (for example `Orc` or `Skyborne`), gender is `male`, `female`, or empty, and NPC ID is decimal text or empty. Empty metadata fields mean unknown. All other flag values are rejected, and flags must agree across every page. 0.3.0 readers accept both layouts; older readers reject metadata pages, so update both components. Empty speaker/title fields are allowed; embedded NULs in fields are not. Byte chunks may split UTF-8 characters. Decode text only after concatenating and validating every page. WoW formatting escapes are removed before encoding.

Kinds: 0=test, 1=gossip/greeting, 2=quest offer, 3=quest progress, 4=quest completion, 5=NPC chat, 6=item/book text. Pocket TTS speaks title and text using the metadata-selected voice. The SAPI fallback joins nonempty speaker, title, and text fields. Metadata is never spoken.

## Delivery

The addon rotates pages every 250 ms and transmits for at least 15 seconds or three page cycles, whichever is longer. A newer message replaces the current one. The receiver holds at most one incomplete message, accepts pages in any order, deduplicates repeated pages, and emits once on complete validation. Incomplete pages expire after two minutes without accepted activity. No acknowledgment, compression, or error correction is used.

For a matching session, newer sequence numbers are determined with signed 32-bit serial arithmetic, supporting wraparound. Recent retired session IDs are ignored. Session IDs are a best-effort time-based reload discriminator, not cryptographic randomness. The transport and checksums are intended for one addon in a local screen-capture environment, not untrusted senders.
