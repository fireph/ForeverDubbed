local _, NS = ...
-- FileDataIDs from https://github.com/wowdev/wow-listfile/releases/latest
-- Character model appearance is a fallback, not an authoritative lore identity.
NS.ModelRaces = {
    [116921] = {race="BloodElf", gender="female"}, -- character/bloodelf/female/bloodelffemale.m2
    [117170] = {race="BloodElf", gender="male"}, -- character/bloodelf/male/bloodelfmale.m2
    [117437] = {race="Draenei", gender="female"}, -- character/draenei/female/draeneifemale.m2
    [117721] = {race="Draenei", gender="male"}, -- character/draenei/male/draeneimale.m2
    [118135] = {race="Dwarf", gender="female"}, -- character/dwarf/female/dwarffemale.m2
    [118355] = {race="Dwarf", gender="male"}, -- character/dwarf/male/dwarfmale.m2
    [119063] = {race="Gnome", gender="female"}, -- character/gnome/female/gnomefemale.m2
    [119159] = {race="Gnome", gender="male"}, -- character/gnome/male/gnomemale.m2
    [119369] = {race="Goblin", gender="female"}, -- character/goblin/female/goblinfemale.m2
    [119376] = {race="Goblin", gender="male"}, -- character/goblin/male/goblinmale.m2
    [119563] = {race="Human", gender="female"}, -- character/human/female/humanfemale.m2
    [119940] = {race="Human", gender="male"}, -- character/human/male/humanmale.m2
    [120590] = {race="NightElf", gender="female"}, -- character/nightelf/female/nightelffemale.m2
    [120791] = {race="NightElf", gender="male"}, -- character/nightelf/male/nightelfmale.m2
    [121087] = {race="Orc", gender="female"}, -- character/orc/female/orcfemale.m2
    [121287] = {race="Orc", gender="male"}, -- character/orc/male/orcmale.m2
    [121608] = {race="Scourge", gender="female"}, -- character/scourge/female/scourgefemale.m2
    [121768] = {race="Scourge", gender="male"}, -- character/scourge/male/scourgemale.m2
    [121961] = {race="Tauren", gender="female"}, -- character/tauren/female/taurenfemale.m2
    [122055] = {race="Tauren", gender="male"}, -- character/tauren/male/taurenmale.m2
    [122414] = {race="Troll", gender="female"}, -- character/troll/female/trollfemale.m2
    [122560] = {race="Troll", gender="male"}, -- character/troll/male/trollmale.m2
    [878772] = {race="Dwarf", gender="male"}, -- character/dwarf/male/dwarfmale_hd.m2
    [900914] = {race="Gnome", gender="male"}, -- character/gnome/male/gnomemale_hd.m2
    [917116] = {race="Orc", gender="male"}, -- character/orc/male/orcmale_hd.m2
    [921844] = {race="NightElf", gender="female"}, -- character/nightelf/female/nightelffemale_hd.m2
    [940356] = {race="Gnome", gender="female"}, -- character/gnome/female/gnomefemale_hd.m2
    [949470] = {race="Orc", gender="female"}, -- character/orc/female/orcfemale_hd.m2
    [950080] = {race="Dwarf", gender="female"}, -- character/dwarf/female/dwarffemale_hd.m2
    [959310] = {race="Scourge", gender="male"}, -- character/scourge/male/scourgemale_hd.m2
    [968705] = {race="Tauren", gender="male"}, -- character/tauren/male/taurenmale_hd.m2
    [974343] = {race="NightElf", gender="male"}, -- character/nightelf/male/nightelfmale_hd.m2
    [986648] = {race="Tauren", gender="female"}, -- character/tauren/female/taurenfemale_hd.m2
    [997378] = {race="Scourge", gender="female"}, -- character/scourge/female/scourgefemale_hd.m2
    [1000764] = {race="Human", gender="female"}, -- character/human/female/humanfemale_hd.m2
    [1005887] = {race="Draenei", gender="male"}, -- character/draenei/male/draeneimale_hd.m2
    [1011653] = {race="Human", gender="male"}, -- character/human/male/humanmale_hd.m2
    [1018060] = {race="Troll", gender="female"}, -- character/troll/female/trollfemale_hd.m2
    [1022598] = {race="Draenei", gender="female"}, -- character/draenei/female/draeneifemale_hd.m2
    [1022938] = {race="Troll", gender="male"}, -- character/troll/male/trollmale_hd.m2
    [1100087] = {race="BloodElf", gender="male"}, -- character/bloodelf/male/bloodelfmale_hd.m2
    [1100258] = {race="BloodElf", gender="female"}, -- character/bloodelf/female/bloodelffemale_hd.m2
}

-- Confirmed in the Forever beta; shared models cannot distinguish this race.
NS.NPCRaces = { ["254100"] = "Skyborne" } -- Zephras Citizen
