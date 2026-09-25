local addonName, NS = ...
local Codec = NS.Codec
local frame = CreateFrame("Frame", "ForeverDubbedTile", UIParent)
frame:SetFrameStrata("TOOLTIP")
frame:SetFrameLevel(100)
if frame.SetIgnoreParentAlpha then frame:SetIgnoreParentAlpha(true) end
frame:SetClampedToScreen(true)
frame:SetMovable(true)
frame:RegisterForDrag("LeftButton")
frame:Hide()

local outline = {}
local textures, pages, page, elapsed, expires = {}, nil, 1, 0, 0
local session = (time() * 1000 + math.floor(GetTime() * 1000) % 1000) % 4294967296
local sequence, lastBody, lastAt, ready = 0, nil, -1, false
local PAGE_SECONDS = 0.25
local WAVE_SECONDS = 1 / 15
local wavePhase, waveElapsed = 0, 0
local getMetadata = C_AddOns and C_AddOns.GetAddOnMetadata or GetAddOnMetadata
local VERSION = getMetadata and getMetadata(addonName, "Version") or "unknown"
local requestID, lastSpeaker = 0, nil
local controlUntil, deferredDialogue = 0, nil
local drawnColors, pageValues = {}, {}
local drawnFrame, drawnPhase
local colors = {}
for value = 0, Codec.WAVE do
    local rgb = value == Codec.WAVE and Codec.FINDER or Codec.PALETTE[value + 1]
    colors[value] = {rgb[1] / 255, rgb[2] / 255, rgb[3] / 255}
end

local function pixelFactor()
    -- GetEffectiveScale is measured against WoW's 768-unit canvas, not desktop
    -- pixels. This is the same conversion used by Blizzard's PixelUtil.
    if PixelUtil and PixelUtil.GetPixelToUIUnitFactor then
        return PixelUtil.GetPixelToUIUnitFactor()
    end
    local _, height = GetPhysicalScreenSize()
    return 768 / height
end

local function screenHeight()
    return UIParent:GetHeight() * UIParent:GetEffectiveScale() / pixelFactor()
end

local function printStatus(s)
    DEFAULT_CHAT_FRAME:AddMessage("|cff66ddffForeverDubbed:|r " .. s)
end

local function clean(s)
    if issecretvalue and issecretvalue(s) then return "" end
    if type(s) ~= "string" then return "" end
    return (s:gsub("%z", ""):gsub("|c%x%x%x%x%x%x%x%x", ""):gsub("|r", "")
        :gsub("|H.-|h(.-)|h", "%1"):gsub("|T.-|t", ""):gsub("|A.-|a", "")
        :gsub("|n", "\n"):gsub("||", "|"):gsub("^%s+", ""):gsub("%s+$", ""))
end

local function read(fn, ...)
    if type(fn) ~= "function" then return "" end
    local ok, result = pcall(fn, ...)
    if ok then return clean(result) end
    return ""
end

local function place()
    if not ready then return end
    local db = ForeverDubbedDB
    local grid = Codec.GRID
    -- One local unit becomes one physical pixel, independent of UI scale.
    frame:SetScale(pixelFactor() / UIParent:GetEffectiveScale())
    local inset, size = Codec.OUTLINE, grid * db.cell + 2 * Codec.OUTLINE
    frame:SetSize(size, size)
    -- Four strips keep the light-blue finder outside the dark calibration ring.
    local strips = {{0,0,size,inset}, {0,-size+inset,size,inset},
        {0,-inset,inset,size-2*inset}, {size-inset,-inset,inset,size-2*inset}}
    for i, rect in ipairs(strips) do
        local t = outline[i]
        if not t then
            t = frame:CreateTexture(nil, "ARTWORK")
            if t.SetSnapToPixelGrid then t:SetSnapToPixelGrid(false) end
            if t.SetTexelSnappingBias then t:SetTexelSnappingBias(0) end
            outline[i] = t
        end
        t:ClearAllPoints()
        t:SetPoint("TOPLEFT", frame, "TOPLEFT", rect[1], rect[2])
        t:SetSize(rect[3], rect[4])
        t:SetColorTexture(Codec.FINDER[1]/255, Codec.FINDER[2]/255, Codec.FINDER[3]/255, 1)
        t:Show()
    end
    frame:ClearAllPoints()
    frame:SetPoint("TOPLEFT", UIParent, "BOTTOMLEFT", db.x, db.y)
    frame:EnableMouse(not db.locked)
    for i = 1, grid * grid do
        local t = textures[i]
        if not t then
            t = frame:CreateTexture(nil, "ARTWORK")
            if t.SetSnapToPixelGrid then t:SetSnapToPixelGrid(false) end
            if t.SetTexelSnappingBias then t:SetTexelSnappingBias(0) end
            textures[i] = t
        end
        t:ClearAllPoints()
        t:SetPoint("TOPLEFT", frame, "TOPLEFT", inset + ((i - 1) % grid) * db.cell, -inset - math.floor((i - 1) / grid) * db.cell)
        t:SetSize(db.cell, db.cell)
        t:Show()
    end
end

local function draw()
    if not pages then return end
    local current = pages[page]
    if current ~= drawnFrame or wavePhase ~= drawnPhase then
        if current ~= drawnFrame then Codec.Values(current, pageValues) end
        local layout = Codec.Layout(wavePhase)
        for i = 1, #layout do
            local value = pageValues[layout[i]]
            -- Most cells (especially zero padding and the calibration border)
            -- retain their color. Avoid dirtying their textures in WoW's UI.
            if drawnColors[i] ~= value then
                local rgb = colors[value]
                textures[i]:SetColorTexture(rgb[1], rgb[2], rgb[3], 1)
                drawnColors[i] = value
            end
        end
        drawnFrame, drawnPhase = current, wavePhase
    end
    frame:Show()
end

local function publishReady(kind, speaker, title, text, info, objectives)
    if not ready or not ForeverDubbedDB.enabled then return end
    if GetTime() < controlUntil then
        deferredDialogue = {kind, speaker, title, text, info, objectives}
        return
    end
    speaker, title, text = clean(speaker), clean(title), clean(text)
    objectives = clean(objectives)
    if text == "" and objectives == "" then return end
    info = info or {}
    lastSpeaker = info
    local race, gender, npcID = clean(info.apiRace or info.race), clean(info.gender), clean(info.npcID)
    local displayID = info.displayID and tostring(info.displayID) or ""
    local modelID = info.modelID and tostring(info.modelID) or ""
    local raceOverride = clean(info.raceOverride)
    -- Do not let desktop lookups reconstruct a restricted identity.
    if info.restricted then race, npcID, displayID, modelID, raceOverride = "", "", "", "", "" end
    local key = kind .. table.concat({speaker, title, text, race, gender, npcID, displayID, modelID, raceOverride, objectives}, "\0")
    if key == lastBody and GetTime() - lastAt < 0.75 then return end
    lastBody, lastAt = key, GetTime()
    sequence = (sequence + 1) % 4294967296
    local encoded, err = Codec.Encode(session, sequence, kind, speaker, title, text, race, gender, npcID, displayID, modelID, raceOverride, objectives)
    if not encoded then printStatus(err); return end
    pages, page, elapsed = encoded, 1, 0
    -- Keep sending after a dialog closes so slow captures can finish. New text
    -- replaces old text immediately; this is a latest-dialog transport.
    expires = GetTime() + math.max(15, #pages * PAGE_SECONDS * 3)
    draw()
end

-- Commands use the normal checksummed transport and sequence deduplication.
-- Hold them briefly so a new NPC event cannot overwrite them before capture.
function NS.Control(action)
    if not ready or (action ~= "stop" and action ~= "skip") then return end
    local now = GetTime()
    requestID = requestID + 1 -- cancel unresolved older NPC identities
    deferredDialogue = nil
    sequence = (sequence + 1) % 4294967296
    pages = Codec.Encode(session, sequence, action == "stop" and 7 or 8, "", "", "")
    page, elapsed = 1, 0
    controlUntil, expires = now + 1.5, now + 15
    draw()
end

local function publish(kind, speaker, title, text, info, objectives)
    requestID = requestID + 1
    local id = requestID
    if info then
        NS.Speakers.Resolve(info, function(resolved)
            if id == requestID then publishReady(kind, speaker, title, text, resolved, objectives) end
        end)
    else
        publishReady(kind, speaker, title, text, nil, objectives)
    end
end

local function publishNPC(kind, title, text, objectives)
    local info = NS.Speakers.Dialog()
    publish(kind, info.name, title, text, info, objectives)
end
local function quest(kind, getter, objectives)
    publishNPC(kind, read(GetTitleText), read(getter), objectives and read(GetObjectiveText) or "")
end

frame:SetScript("OnDragStart", function(self) self:StartMoving() end)
frame:SetScript("OnDragStop", function(self)
    self:StopMovingOrSizing()
    local scale = self:GetEffectiveScale() / pixelFactor()
    ForeverDubbedDB.x = math.floor(self:GetLeft() * scale + 0.5)
    ForeverDubbedDB.y = math.floor(self:GetTop() * scale + 0.5)
    place()
end)

frame:SetScript("OnUpdate", function(_, dt)
    if deferredDialogue and GetTime() >= controlUntil then
        local dialogue = deferredDialogue
        deferredDialogue = nil
        publishReady(unpack(dialogue, 1, 6))
    end
    if not pages then return end
    if GetTime() > expires and ForeverDubbedDB.locked then frame:Hide(); return end
    elapsed = elapsed + dt
    waveElapsed = waveElapsed + dt
    local redraw = false
    if elapsed >= PAGE_SECONDS then
        local ticks = math.floor(elapsed / PAGE_SECONDS)
        elapsed = elapsed % PAGE_SECONDS
        page = (page - 1 + ticks) % #pages + 1
        redraw = #pages > 1
    end
    if waveElapsed + 1e-9 >= WAVE_SECONDS then
        local ticks = math.floor((waveElapsed + 1e-9) / WAVE_SECONDS)
        waveElapsed = math.max(0, waveElapsed - ticks * WAVE_SECONDS)
        wavePhase = (wavePhase + ticks) % Codec.WAVE_PHASES
        redraw = true
    end
    if redraw then draw() end -- one redraw if page and wave ticks coincide

end)

local events = CreateFrame("Frame")
events:RegisterEvent("ADDON_LOADED")
events:SetScript("OnEvent", function(_, event, ...)
    if event == "ADDON_LOADED" then
        if ... ~= "ForeverDubbed" then return end
        ForeverDubbedDB = ForeverDubbedDB or {}
        local db = ForeverDubbedDB
        -- Previously saved positions used canvas units. Convert them once to
        -- physical pixels so the corrected scale keeps the same screen anchor.
        if db.positionVersion ~= 2 then
            if tonumber(db.x) then db.x = math.floor(tonumber(db.x) / pixelFactor() + 0.5) end
            if tonumber(db.y) then db.y = math.floor(tonumber(db.y) / pixelFactor() + 0.5) end
            db.positionVersion = 2
        end
        -- Upgrade the encoding once while preserving position and other settings.
        if db.encodingVersion ~= 3 and db.encodingVersion ~= 4 and db.encodingVersion ~= 5 then db.cell = 2 end
        db.encodingVersion, db.mode = 5, nil
        db.cell = math.max(2, math.min(8, math.floor(tonumber(db.cell) or 2)))
        db.x = tonumber(db.x) or 16
        db.y = tonumber(db.y) or (screenHeight() - 16)
        if db.enabled == nil then db.enabled = true end
        if db.chat == nil then db.chat = true end
        if db.locked == nil then db.locked = true end
        ready = true
        place()
        NS.Controls.Init(db)
        for _, e in ipairs({"GOSSIP_SHOW", "QUEST_GREETING", "QUEST_DETAIL", "QUEST_PROGRESS", "QUEST_COMPLETE",
            "ITEM_TEXT_READY", "CHAT_MSG_MONSTER_SAY", "CHAT_MSG_MONSTER_YELL", "CHAT_MSG_MONSTER_WHISPER",
            "CHAT_MSG_MONSTER_EMOTE", "CHAT_MSG_RAID_BOSS_EMOTE", "CHAT_MSG_RAID_BOSS_WHISPER",
            "UI_SCALE_CHANGED", "DISPLAY_SIZE_CHANGED"}) do
            pcall(events.RegisterEvent, events, e)
        end
        printStatus(VERSION .. " ready. /fdb test to test, /fdb unlock to drag the square.")
    elseif event == "UI_SCALE_CHANGED" or event == "DISPLAY_SIZE_CHANGED" then
        place()
    elseif event == "GOSSIP_SHOW" then
        publishNPC(1, "", read(C_GossipInfo and C_GossipInfo.GetText or GetGossipText))
    elseif event == "QUEST_GREETING" then
        publishNPC(1, "", read(GetGreetingText))
    elseif event == "QUEST_DETAIL" then
        quest(2, GetQuestText, true)
    elseif event == "QUEST_PROGRESS" then
        quest(3, GetProgressText)
    elseif event == "QUEST_COMPLETE" then
        quest(4, GetRewardText)
    elseif event == "ITEM_TEXT_READY" then
        publish(6, "", read(ItemTextGetItem), read(ItemTextGetText))
    elseif event:match("^CHAT_MSG_") and ForeverDubbedDB.chat then
        local text, speaker = ...
        publish(5, speaker, "", text, NS.Speakers.Chat(select(12, ...)))
    end
end)

SLASH_FOREVERDUBBED1 = "/fdb"
SlashCmdList.FOREVERDUBBED = function(input)
    local cmd, arg = input:match("^%s*(%S*)%s*(.-)%s*$")
    cmd = cmd:lower()
    local db = ForeverDubbedDB
    if not ready then return end
    if cmd == "stop" or cmd == "skip" then
        NS.Control(cmd)
    elseif cmd == "test" or cmd == "unlock" then
        if cmd == "unlock" then db.locked = false; place() end
        publish(0, "ForeverDubbed", "Connection test", "Welcome to ForeverDubbed. Quest and NPC dialogue will be read aloud here.")
        if cmd == "unlock" then printStatus("Drag the square, then /fdb lock. Hovering the pointer over it may interrupt capture.") end
    elseif cmd == "lock" then
        db.locked = true; place(); printStatus("Position locked.")
    elseif cmd == "cell" then
        local size = tonumber(arg)
        if not size or size < 2 or size > 8 or size ~= math.floor(size) then printStatus("Use /fdb cell 2 through 8."); return end
        db.cell = size; place()
        printStatus("Square is " .. (size * Codec.GRID + 2 * Codec.OUTLINE) .. " × " .. (size * Codec.GRID + 2 * Codec.OUTLINE) .. " physical pixels, " .. Codec.PAYLOAD .. " bytes per page.")
    elseif cmd == "on" or cmd == "off" then
        db.enabled = cmd == "on"
        if not db.enabled then requestID = requestID + 1; deferredDialogue = nil; pages = nil; frame:Hide() end
        printStatus("Enabled: " .. tostring(db.enabled))
    elseif cmd == "chat" then
        db.chat = not db.chat; printStatus("NPC chat: " .. tostring(db.chat))
    elseif cmd == "reset" then
        db.x, db.y = 16, screenHeight() - 16
        place(); printStatus("Position reset.")
    elseif cmd == "npc" or cmd == "race" then
        local info = NS.Speakers.Dialog()
        if info.npcID == "" then info = NS.Speakers.ForUnit("target") end
        if cmd == "race" then
            if arg == "" then printStatus("Use /fdb race Skyborne (or another race), or /fdb race clear. Talk to or target the NPC first."); return end
            local race = arg:lower() ~= "clear" and clean(arg) or nil
            if not NS.Speakers.SetRace(info, race) then printStatus("Talk to or target an NPC first."); return end
            printStatus("NPC " .. info.npcID .. " race override: " .. (race or "cleared") .. ". Reopen its dialogue to resend.")
        else
            NS.Speakers.Resolve(info, function(resolved)
                printStatus("NPC: " .. resolved.name .. "; ID: " .. resolved.npcID .. "; race: " .. resolved.race
                    .. "; source: " .. (resolved.raceSource or "unavailable") .. "; gender: " .. resolved.gender
                    .. "; display ID: " .. tostring(resolved.displayID or "unavailable")
                    .. " (" .. (resolved.displayStatus or "not read") .. ")"
                    .. "; model file: " .. tostring(resolved.modelID or "unavailable"))
            end, true)
        end
    elseif cmd == "status" then
        local version, build, _, interface = GetBuildInfo()
        printStatus("Addon " .. VERSION .. "; client " .. tostring(version) .. ", build " .. tostring(build) .. ", interface " .. tostring(interface))
        printStatus("Gossip API: " .. tostring(C_GossipInfo and type(C_GossipInfo.GetText) == "function")
            .. "; quest API: " .. tostring(type(GetQuestText) == "function") .. "; enabled: " .. tostring(db.enabled)
            .. "; colors: 16; cell: " .. db.cell .. "; bytes/page: " .. Codec.PAYLOAD
            .. "; locked: " .. tostring(db.locked))
        printStatus(string.format("Pixel factor: %.5f; effective scale: %.5f; physical cell: %.3f px; square: %.1f px",
            pixelFactor(), frame:GetEffectiveScale(), db.cell * frame:GetEffectiveScale() / pixelFactor(),
            (Codec.GRID * db.cell + 2 * Codec.OUTLINE) * frame:GetEffectiveScale() / pixelFactor()))
        local info = lastSpeaker or NS.Speakers.Dialog()
        printStatus("Last NPC: " .. (info.name or "") .. "; race: " .. (info.race or "") .. "; gender: " .. (info.gender or "") .. "; NPC ID: " .. (info.npcID or "")
            .. "; race source: " .. (info.raceSource or "unavailable"))
    else
        printStatus("/fdb stop | skip | test | unlock | lock | cell 2–8 | chat | on | off | reset | npc | race NAME | status")
    end
end
