local _, NS = ...
local Codec = {}
NS.Codec = Codec
Codec.GRID, Codec.PAYLOAD = 50, 1124
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
    if count > 256 then return nil, "Text exceeds the 287,744-byte transport limit." end
    local frames, checksum = {}, Codec.Adler(body)
    for i = 0, count - 1 do
        local payload = body:sub(i * Codec.PAYLOAD + 1, (i + 1) * Codec.PAYLOAD)
        local frame = "FDB4" .. uint(session, 4) .. uint(sequence, 4) .. uint(checksum, 4)
            .. uint(i, 2) .. uint(count, 2) .. uint(#payload, 2) .. string.char(kind, flags)
            .. payload .. string.rep("\0", Codec.PAYLOAD - #payload)
        frames[#frames + 1] = frame .. uint(Codec.Adler(frame), 4)
    end
    return frames
end

function Codec.Cells(frame)
    local cells, nibble = {}, 0
    for y = 0, Codec.GRID - 1 do
        for x = 0, Codec.GRID - 1 do
            local value = Codec.Border(x, y)
            if x > 0 and x < Codec.GRID - 1 and y > 0 and y < Codec.GRID - 1 then
                local byte = frame:byte(math.floor(nibble / 2) + 1)
                value = math.floor(byte / 16 ^ (1 - nibble % 2)) % 16
                nibble = nibble + 1
            end
            cells[#cells + 1] = value
        end
    end
    return cells
end
