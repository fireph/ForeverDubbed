local _, NS = ...
local Speakers = {}
NS.Speakers = Speakers
local cache, order = {}, {}
local probe, activeStop

local function public(value)
    return not (issecretvalue and issecretvalue(value))
end
local function text(value)
    if not public(value) or type(value) ~= "string" then return "" end
    return value
end
local function read(fn, ...)
    if type(fn) ~= "function" then return "" end
    local ok, value = pcall(fn, ...)
    return ok and text(value) or ""
end
local function number(fn, ...)
    if type(fn) ~= "function" then return nil end
    local ok, value = pcall(fn, ...)
    if ok and public(value) and type(value) == "number" and value > 0 then return value end
end
local function npcID(guid)
    if not guid:match("^Creature%-") and not guid:match("^Vehicle%-") then return "" end
    return guid:match("^[^-]+%-[^-]+%-[^-]+%-[^-]+%-[^-]+%-(%d+)%-") or ""
end
local function remember(info)
    if info.guid == "" or info.restricted then return end
    if not cache[info.guid] then order[#order+1] = info.guid end
    local saved = {}
    for k, v in pairs(info) do saved[k] = v end
    saved.unit = nil -- A unit token can later refer to a different creature.
    cache[info.guid] = saved
    if #order > 256 then cache[table.remove(order, 1)] = nil end
end
local function enrich(info)
    if info.restricted then return info end
    local overrides = ForeverDubbedDB and ForeverDubbedDB.npcRaces or {}
    local override = text(overrides[info.npcID])
    if override ~= "" then
        info.race, info.raceSource = override, "saved NPC override"
    elseif info.race == "" then
        local known = NS.NPCRaces and NS.NPCRaces[info.npcID]
        local old = cache[info.guid]
        if known then
            info.race, info.raceSource = known, "NPC ID lookup"
        elseif old then
            info.race, info.raceSource = old.race, old.raceSource
            info.modelID = old.modelID
            if info.gender == "" then info.gender = old.gender end
        end
    end
    return info
end
function Speakers.ForUnit(unit)
    local info = {unit=unit, name=read(UnitName, unit), guid=read(UnitGUID, unit),
        race="", gender="", npcID="", raceSource="unavailable"}
    info.npcID = npcID(info.guid)
    if type(UnitRace) == "function" then
        local ok, localized, english = pcall(UnitRace, unit)
        if ok then
            info.restricted = not public(localized) or not public(english)
            info.race = text(english)
            if info.race ~= "" then info.raceSource = "UnitRace" end
        end
    end
    if type(UnitSex) == "function" then
        local ok, sex = pcall(UnitSex, unit)
        if ok and public(sex) then
            if sex == 2 then info.gender = "male" elseif sex == 3 then info.gender = "female" end
        end
    end
    enrich(info)
    remember(info)
    return info
end
function Speakers.Dialog()
    local info = Speakers.ForUnit("npc")
    if info.name == "" then info = Speakers.ForUnit("questnpc") end
    return info
end
function Speakers.Chat(guid)
    guid = text(guid)
    if guid == "" then return {name="", guid="", race="", gender="", npcID="", raceSource="unavailable"} end
    for _, unit in ipairs({"npc", "questnpc", "target", "mouseover"}) do
        if read(UnitGUID, unit) == guid then return Speakers.ForUnit(unit) end
    end
    if C_NamePlate and type(C_NamePlate.GetNamePlates) == "function" then
        local ok, plates = pcall(C_NamePlate.GetNamePlates)
        if ok and public(plates) and type(plates) == "table" then
            for _, plate in ipairs(plates) do
                local unit = text(plate.namePlateUnitToken)
                if unit ~= "" and read(UnitGUID, unit) == guid then return Speakers.ForUnit(unit) end
            end
        end
    end
    return enrich({name="", guid=guid, race="", gender="", npcID=npcID(guid), raceSource="unavailable"})
end

-- Complete before publishing, so a late model load never restarts spoken text.
-- New requests retire the previous probe; callers discard superseded dialogue.
function Speakers.Resolve(info, callback, inspect)
    if activeStop then activeStop() end
    if info.restricted or info.guid == "" or not info.unit or
        (info.race ~= "" and not inspect) then callback(info); return end
    if not probe then
        local ok, frame = pcall(CreateFrame, "PlayerModel", nil, UIParent)
        if not ok then callback(info); return end
        probe = frame
        probe:SetSize(1, 1)
        probe:SetPoint("TOPLEFT", UIParent, "TOPLEFT", 0, 0)
        probe:SetAlpha(0)
        probe:EnableMouse(false)
        probe:Hide()
    end
    if type(probe.SetUnit) ~= "function" or type(probe.GetModelFileID) ~= "function" then callback(info); return end
    local done, attempts = false, 0
    local function finish()
        if done then return end
        done = true
        activeStop = nil
        probe:Hide()
        remember(info)
        callback(info)
    end
    activeStop = finish
    -- Clear the old model before binding, including when SetUnit subsequently fails.
    if probe.ClearModel then pcall(probe.ClearModel, probe) end
    probe:Show()
    local ok, success = pcall(probe.SetUnit, probe, info.unit, false, true)
    if not ok or (public(success) and success == false) or not public(success) then finish(); return end
    local function poll()
        if done then return end
        -- Never attribute a model to a new occupant of target/npc/nameplate tokens.
        if read(UnitGUID, info.unit) ~= info.guid then finish(); return end
        local id = number(probe.GetModelFileID, probe)
        if id then
            info.modelID = id
            local identity = NS.ModelRaces and NS.ModelRaces[id]
            if identity then
                if info.race == "" then info.race, info.raceSource = identity.race, "model appearance" end
                if info.gender == "" then info.gender = identity.gender end
            end
            finish()
        elseif attempts < 12 and C_Timer and type(C_Timer.After) == "function" then
            attempts = attempts + 1
            C_Timer.After(0.05, poll)
        else
            finish()
        end
    end
    poll()
end

function Speakers.SetRace(info, race)
    if info.npcID == "" then return false end
    ForeverDubbedDB.npcRaces = ForeverDubbedDB.npcRaces or {}
    ForeverDubbedDB.npcRaces[info.npcID] = race
    for guid, old in pairs(cache) do
        if old.npcID == info.npcID then cache[guid] = nil end
    end
    -- Rebuild order as invalidation removed entries from the cache.
    local kept = {}
    for _, guid in ipairs(order) do if cache[guid] then kept[#kept+1] = guid end end
    order = kept
    return true
end
