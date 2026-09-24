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
    if type(fn) ~= "function" then return nil, "API unavailable" end
    local ok, value = pcall(fn, ...)
    if not ok then return nil, "API call failed" end
    if not public(value) then return nil, "restricted value" end
    if value == nil then return nil, "API returned nil" end
    if type(value) ~= "number" then return nil, "API returned non-number" end
    if value == 0 then return nil, "API returned 0" end
    if value > 0 then return value, "available" end
    return nil, "API returned invalid number"
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
    local old = cache[info.guid]
    if old then
        info.modelID, info.displayID, info.displayStatus = old.modelID, old.displayID, old.displayStatus
        if info.race == "" then
            info.race = old.apiRace or ""
            if info.race ~= "" then info.raceSource = "cached UnitRace" end
        end
        if info.gender == "" then info.gender, info.genderSource = old.gender, old.genderSource end
    end
    info.apiRace = info.race
    local overrides = ForeverDubbedDB and ForeverDubbedDB.npcRaces or {}
    info.raceOverride = text(overrides[info.npcID])
    if info.raceOverride ~= "" then
        info.race, info.raceSource = info.raceOverride, "saved NPC override"
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
            if info.gender ~= "" then info.genderSource = "UnitSex" end
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
    if info.restricted or info.guid == "" or not info.unit then callback(info); return end
    info.modelID = nil
    if inspect then info.displayID, info.displayStatus = nil, nil end
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
    -- The beta returns zero after SetUnit. Normal dialogue must not probe or
    -- wait for this optional API; explicit inspection/marking can still try it.
    local hasDisplay = inspect and type(probe.GetDisplayInfo) == "function"
    if type(probe.SetUnit) ~= "function" or
        (not hasDisplay and type(probe.GetModelFileID) ~= "function") then callback(info); return end
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
        local displayID, displayStatus = info.displayID, info.displayStatus or "not requested"
        if inspect then displayID, displayStatus = number(probe.GetDisplayInfo, probe) end
        local modelID = number(probe.GetModelFileID, probe)
        info.displayID, info.modelID = displayID, modelID
        info.displayStatus = displayStatus
        -- Only collect observations here; all database lookups run on desktop.
        local canWait = attempts < 12 and C_Timer and type(C_Timer.After) == "function"
        if (displayID and (modelID or type(probe.GetModelFileID) ~= "function")) or
            (modelID and (not hasDisplay or displayStatus == "API returned 0")) then
            finish()
        elseif canWait then
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
    -- Finish any old probe before invalidating its cached identity.
    if activeStop then activeStop() end
    ForeverDubbedDB.npcRaces = ForeverDubbedDB.npcRaces or {}
    ForeverDubbedDB.npcRaces[info.npcID] = race
    ForeverDubbedDB.npcRaceEvidence = ForeverDubbedDB.npcRaceEvidence or {}
    ForeverDubbedDB.npcRaceEvidence[info.npcID] = nil
    for guid, old in pairs(cache) do
        if old.npcID == info.npcID then cache[guid] = nil end
    end
    -- Rebuild order as invalidation removed entries from the cache.
    local kept = {}
    for _, guid in ipairs(order) do if cache[guid] then kept[#kept+1] = guid end end
    order = kept
    if race then
        local evidence = {name=info.name, race=race, gender=info.gender}
        ForeverDubbedDB.npcRaceEvidence[info.npcID] = evidence
        local observed = {}
        for k, v in pairs(info) do observed[k] = v end
        observed.race, observed.raceSource = race, "saved NPC override"
        -- Cached appearance is not evidence of this particular observation.
        observed.displayID, observed.modelID = nil, nil
        observed.displayStatus = nil
        Speakers.Resolve(observed, function(resolved)
            if ForeverDubbedDB.npcRaceEvidence[info.npcID] ~= evidence then return end
            evidence.gender = resolved.gender
            evidence.displayID, evidence.modelID = resolved.displayID, resolved.modelID
            evidence.displayStatus = resolved.displayStatus or "not read"
        end, true)
    end
    return true
end
