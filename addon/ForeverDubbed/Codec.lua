local _, NS = ...
local Codec = {}
NS.Codec = Codec
Codec.GRID, Codec.PAYLOAD = 50, 1056
-- Chromaglyph OKLab pack: h 264, L 0.22, C 0.045; minimum RGB distance sqrt(37).
-- Palette order is part of the wire format; all sixteen references are sent.
Codec.OUTLINE = 2 -- physical pixels, independent of cell size
Codec.FINDER = {128, 192, 240} -- light blue #80c0f0
Codec.PALETTE = {
    {16,26,47},
    {0,28,49},
    {30,25,46},
    {14,27,59},
    {18,24,35},
    {5,26,38},
    {24,27,57},
    {5,28,58},
    {8,27,48},
    {26,24,38},
    {23,26,47},
    {19,19,45},
    {15,33,50},
    {9,20,46},
    {15,27,53},
    {17,25,41},
}

-- One hard-coded loop with a two-cell perpendicular stroke width.
-- Each column contains the inclusive zero-based data rows below. Animation
-- only rotates these columns; it never recalculates or changes the shape.
Codec.WAVE = 16 -- drawing-only marker, never a data nibble
Codec.WAVE_PHASES = 48
local waveColumns = {
    {22,25}, {24,26}, {25,28}, {27,29}, {28,30}, {29,31}, {30,32}, {31,33},
    {32,34}, {33,34}, {34,35}, {34,35}, {34,35}, {34,35}, {34,35}, {33,34},
    {32,34}, {31,33}, {30,32}, {29,31}, {28,30}, {27,29}, {25,28}, {24,26},
    {22,25}, {21,23}, {19,22}, {18,20}, {17,19}, {16,18}, {15,17}, {14,16},
    {13,15}, {13,14}, {12,13}, {12,13}, {12,13}, {12,13}, {12,13}, {13,14},
    {13,15}, {14,16}, {15,17}, {16,18}, {17,19}, {18,20}, {19,22}, {21,23},
}
function Codec.IsWave(x, y, phase)
    if x < 1 or x > 48 or y < 1 or y > 48 then return false end
    local rows = waveColumns[(x-1 + (phase or 0)) % 48 + 1]
    return y-1 >= rows[1] and y-1 <= rows[2]
end

local function uint(n, width)
    local s = ""
    for _ = 1, width do
        s = string.char(n % 256) .. s
        n = math.floor(n / 256)
    end
    return s
end

function Codec.Adler(s)
    local a, b = 1, 0
    for i = 1, #s do
        a = (a + s:byte(i)) % 65521
        b = (b + a) % 65521
    end
    return b * 65536 + a
end

function Codec.Border(x, y)
    if y == 0 then return (x * 5 + 1) % 8 end
    if y == Codec.GRID - 1 then
        if x >= 1 and x <= 16 then return x - 1 end
        return (x * 3 + 6) % 8
    end
    if x == 0 then return (y * 3 + 2) % 8 end
    return (y * 5 + 4) % 8
end

function Codec.Encode(session, sequence, kind, speaker, title, text, race, gender, npcID)
    local body = speaker .. "\0" .. title .. "\0" .. text
    race, gender, npcID = race or "", gender or "", npcID or ""
    local flags = 0
    if race ~= "" or gender ~= "" or npcID ~= "" then
        body = body .. "\0" .. race .. "\0" .. gender .. "\0" .. npcID
        flags = 1
    end
    local count = math.ceil(#body / Codec.PAYLOAD)
    if count > 256 then return nil, "Text exceeds the 270,336-byte transport limit." end
    local frames, checksum = {}, Codec.Adler(body)
    for i = 0, count - 1 do
        local payload = body:sub(i * Codec.PAYLOAD + 1, (i + 1) * Codec.PAYLOAD)
        local frame = "FDB5" .. uint(session, 4) .. uint(sequence, 4) .. uint(checksum, 4)
            .. uint(i, 2) .. uint(count, 2) .. uint(#payload, 2) .. string.char(kind, flags)
            .. payload .. string.rep("\0", Codec.PAYLOAD - #payload)
        frames[#frames + 1] = frame .. uint(Codec.Adler(frame), 4)
    end
    return frames
end

-- A layout depends only on the wave phase, not on the message. Positive
-- entries address data nibbles; negative entries address fixed palette colors
-- (including the wave). Cache at most 48 layouts across all messages/pages.
local layouts = {}
function Codec.Layout(phase)
    phase = (phase or 0) % Codec.WAVE_PHASES
    if layouts[phase] then return layouts[phase] end
    local layout, nibble = {}, 0
    for y = 0, Codec.GRID - 1 do
        for x = 0, Codec.GRID - 1 do
            local source
            if x > 0 and x < Codec.GRID - 1 and y > 0 and y < Codec.GRID - 1 then
                if Codec.IsWave(x, y, phase) then
                    source = -Codec.WAVE - 1
                else
                    nibble = nibble + 1
                    source = nibble
                end
            else
                source = -Codec.Border(x, y) - 1
            end
            layout[#layout + 1] = source
        end
    end
    layouts[phase] = layout
    return layout
end

-- Decode once per page change, rather than extracting every nibble again on
-- every animation tick. The optional buffer keeps page cycling allocation-free.
function Codec.Values(frame, values)
    values = values or {}
    for value = 0, Codec.WAVE do values[-value - 1] = value end
    for i = 1, #frame do
        local byte = frame:byte(i)
        values[i * 2 - 1], values[i * 2] = math.floor(byte / 16), byte % 16
    end
    return values
end

function Codec.Cells(frame, phase)
    local cells, values, layout = {}, Codec.Values(frame), Codec.Layout(phase)
    for i = 1, #layout do cells[i] = values[layout[i]] end
    return cells
end
