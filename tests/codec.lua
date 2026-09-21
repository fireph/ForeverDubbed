local ns = {}
assert(loadfile("addon/ForeverDubbed/Codec.lua"))("ForeverDubbed", ns)
local frames = assert(ns.Codec.Encode(123456789, 42, 2, "Thrall", "A new beginning",
    "Hello, champion! " .. string.rep("Café — 世界. ", 90), "Orc", "male", "4949"))
local palette = {}
for _, rgb in ipairs(ns.Codec.PALETTE) do
    palette[#palette+1] = string.format("%02x%02x%02x", rgb[1], rgb[2], rgb[3])
end
print(table.concat(palette))
for _, frame in ipairs(frames) do
    print((frame:gsub(".", function(c) return string.format("%02x", c:byte()) end)))
    local cells = ns.Codec.Cells(frame)
    for i, v in ipairs(cells) do cells[i] = string.format("%02x", v) end
    print(table.concat(cells))
end
