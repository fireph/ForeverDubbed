-- Minimal UI/API doubles exercise event routing, configuration and page lifetime.
local now, objects, messages = 100, {}, {}
local colorWrites = 0
local physicalHeight, uiScale = 1440, 0.75
local methods = {}
function methods:SetScript(name, fn) self.scripts[name] = fn end
function methods:RegisterEvent(name) self.events[name] = true end
function methods:CreateTexture()
    local t = setmetatable({}, {__index = methods})
    self.textures = rawget(self, "textures") or {}
    self.textures[#self.textures+1] = t
    return t
end
methods.CreateMaskTexture = methods.CreateTexture
function methods:SetTexture(texture) self.texture = texture end
function methods:SetPoint(...) self.point={...} end
function methods:ClearAllPoints() self.point=nil end
function methods:SetSize(w,h) self.width, self.height = w,h end
function methods:SetScale(scale) self.scale=scale end
function methods:SetColorTexture(r,g,b,a)
    assert(r>=0 and r<=1 and g>=0 and g<=1 and b>=0 and b<=1 and a==1)
    colorWrites = colorWrites + 1
    self.color={r,g,b,a}
end
function methods:Show() self.visible = true end
function methods:Hide() self.visible = false end
function methods:GetEffectiveScale()
    if self == UIParent then return uiScale end
    local parent=rawget(self,"parent")
    return (rawget(self,"scale") or 1) * (parent and parent:GetEffectiveScale() or 1)
end
function methods:GetHeight() if self == Minimap then return 160 end; return 768/uiScale end
function methods:GetWidth() return 160 end
function methods:GetFrameLevel() return 1 end
function methods:GetCenter() return 100, 100 end
function methods:GetLeft() return 20 end
function methods:GetTop() return 900 end
setmetatable(methods, {__index = function() return function() end end})
function CreateFrame(_, name, parent)
    local f = setmetatable({scripts={}, events={}, parent=parent, menu=false}, {__index=methods})
    objects[#objects+1] = f
    if name then _G[name] = f end
    return f
end
UIParent = CreateFrame("Frame")
Minimap = CreateFrame("Frame", nil, UIParent)
GameTooltip = setmetatable({}, {__index=methods})
local shiftDown = false
function IsShiftKeyDown() return shiftDown end
local menuItems
local modernMenu = {CreateContextMenu=function(owner, generate)
    menuItems = {}
    generate(owner, {
        CreateTitle=function(_, title) assert(title == "ForeverDubbed") end,
        CreateButton=function(_, text, callback)
            local item = {text=text, func=callback, disabled=false}
            function item:SetEnabled(enabled) self.disabled = not enabled end
            menuItems[#menuItems+1] = item
            return item
        end,
    })
end}
MenuUtil = modernMenu
function UIDropDownMenu_Initialize(menu, init, style)
    assert(style == "MENU")
    menu.init = init
end
function UIDropDownMenu_CreateInfo() return {} end
function UIDropDownMenu_AddButton(info) menuItems[#menuItems+1] = info end
function ToggleDropDownMenu(_, _, menu)
    menuItems = {}
    menu.init()
end
local cursorX, cursorY = 200, 100
function GetCursorPosition() return cursorX, cursorY end
DEFAULT_CHAT_FRAME = {AddMessage=function(_, text) messages[#messages+1]=text end}
function time() return 12345 end
function GetTime() return now end
local metadataVersion = "9.8.7"
C_AddOns = {GetAddOnMetadata=function(name, field)
    assert(name == "ForeverDubbed" and field == "Version")
    return metadataVersion
end}
function GetBuildInfo() return "1.60.1", "test", "date", 16001 end
function GetPhysicalScreenSize() return physicalHeight*16/9,physicalHeight end
function UnitName() return "Thrall" end
function UnitGUID() return "Creature-0-1-2-3-4949-12345" end
function UnitRace() return "Localized Orc", "Orc" end
function UnitSex() return 2 end
function GetTitleText() return "Quest" end
function GetQuestText() return "Quest body" end
function GetObjectiveText() return "Quest objectives" end
function GetProgressText() return "Progress body" end
function GetRewardText() return "Reward body" end
function GetGreetingText() return "Greeting" end
C_GossipInfo = {GetText=function() return "Hello, champion" end}
SlashCmdList = {}
local ns = {}
assert(loadfile("addon/ForeverDubbed/Codec.lua"))("ForeverDubbed", ns)
assert(loadfile("addon/ForeverDubbed/Speakers.lua"))("ForeverDubbed", ns)
assert(loadfile("addon/ForeverDubbed/Controls.lua"))("ForeverDubbed", ns)
local actual, calls = ns.Codec.Encode, {}
ns.Codec.Encode = function(...)
    calls[#calls+1] = {...}
    return actual(...)
end
assert(loadfile("addon/ForeverDubbed/ForeverDubbed.lua"))("ForeverDubbed", ns)
local events, tile = objects[#objects], ForeverDubbedTile
ForeverDubbedDB = {cell=3, x=20, y=900, chat=false}
events.scripts.OnEvent(events, "ADDON_LOADED", "ForeverDubbed")
assert(calls[#calls][3] == 9 and calls[#calls][14] == "v" .. metadataVersion)
local startupCalls = #calls
now = now + 16
tile.scripts.OnUpdate(tile, 0.1)
assert(not tile.visible and #calls == startupCalls, "startup version must expire without an idle replacement")
assert(ForeverDubbedDB.cell==2 and ForeverDubbedDB.locked)
assert(ForeverDubbedDB.x==38 and ForeverDubbedDB.y==1688 and not ForeverDubbedDB.chat)
assert(ForeverDubbedDB.positionVersion==2)
assert(ForeverDubbedDB.encodingVersion==5 and ForeverDubbedDB.mode==nil)
assert(tile.width==104 and tile.height==104 and #tile.textures==2504)
ForeverDubbedDB.chat=true
for _, event in ipairs({"GOSSIP_SHOW","QUEST_GREETING","QUEST_DETAIL","QUEST_PROGRESS","QUEST_COMPLETE"}) do
    now = now + 1
    events.scripts.OnEvent(events,event)
    assert(tile.visible)
    local sent = calls[#calls]
    if event == "QUEST_DETAIL" then
        assert(sent[5] == "Quest" and sent[6] == "Quest body" and sent[13] == "Quest objectives",
            "quest title, dialogue and objectives must remain separate")
    elseif event == "QUEST_PROGRESS" or event == "QUEST_COMPLETE" then
        assert(sent[5] == "Quest" and sent[13] == "", "stale objectives in follow-up quest dialogue")
    end
end
-- The rendered cells use the exact ordered palette, after the four outline strips.
local expected = ns.Codec.Cells(actual(unpack(calls[#calls]))[1])
for i, value in ipairs(expected) do
    local rgb, t = value==ns.Codec.WAVE and ns.Codec.FINDER or ns.Codec.PALETTE[value+1], tile.textures[i+4]
    assert(t.color[1]==rgb[1]/255 and t.color[2]==rgb[2]/255 and t.color[3]==rgb[3]/255)
    assert(t.point[4]==2+((i-1)%50)*2 and t.point[5]==-2-math.floor((i-1)/50)*2)
end
-- A single page redraws around every wave phase without creating new messages.
local waveFrame, sent = actual(unpack(calls[#calls]))[1], #calls
for phase=1,ns.Codec.WAVE_PHASES do
    local beforeWrites = colorWrites
    tile.scripts.OnUpdate(tile,1/30)
    assert(colorWrites==beforeWrites, "redrew between animation ticks")
    local previous = ns.Codec.Cells(waveFrame, phase-1)
    for i,value in ipairs(previous) do
        local rgb = value==ns.Codec.WAVE and ns.Codec.FINDER or ns.Codec.PALETTE[value+1]
        local t = tile.textures[i+4]
        assert(t.color[1]==rgb[1]/255 and t.color[2]==rgb[2]/255 and t.color[3]==rgb[3]/255)
    end
    local cells = ns.Codec.Cells(waveFrame, phase)
    local changed = 0
    for i,value in ipairs(cells) do
        if value~=previous[i] then changed=changed+1 end
    end
    tile.scripts.OnUpdate(tile,1/30)
    assert(colorWrites-beforeWrites==changed, "must update exactly the changed cells")
    for i,value in ipairs(cells) do
        local rgb = value==ns.Codec.WAVE and ns.Codec.FINDER or ns.Codec.PALETTE[value+1]
        local t = tile.textures[i+4]
        assert(t.color[1]==rgb[1]/255 and t.color[2]==rgb[2]/255 and t.color[3]==rgb[3]/255)
    end
    assert(#calls==sent)
end
-- A stalled frame can skip a whole wave cycle; the displayed image is identical.
local beforeWrites = colorWrites
tile.scripts.OnUpdate(tile, ns.Codec.WAVE_PHASES / 15)
assert(colorWrites==beforeWrites, "redrew an identical page and wave phase")
assert(calls[4][7]=="Orc" and calls[4][8]=="male" and calls[4][9]=="4949")
assert(calls[4][6] == "Quest body" and calls[4][13] == "Quest objectives")
events.scripts.OnEvent(events,"CHAT_MSG_MONSTER_SAY","|cffffffffHello|r |Hitem:1|hfriend|h |Ticon:16|t","NPC")
assert(calls[#calls][6] == "Hello friend")
assert(calls[#calls][5] == "NPC says")
now = now + 1
events.scripts.OnEvent(events, "CHAT_MSG_MONSTER_SAY", "hello there", "Thrall",
    nil, nil, nil, nil, nil, nil, nil, nil, nil, "Creature-0-1-2-3-4949-12345")
assert(calls[#calls][5] == "Thrall says" and calls[#calls][6] == "hello there")
assert(calls[#calls][7] == "Orc" and calls[#calls][8] == "male" and calls[#calls][9] == "4949",
    "world chat must preserve NPC identity for the dialogue voice")
for _, case in ipairs({
    {"CHAT_MSG_MONSTER_SAY", "hello there", "Thrall", "Thrall says", "hello there"},
    {"CHAT_MSG_MONSTER_YELL", "Run!", "Thrall", "Thrall yells", "Run!"},
    {"CHAT_MSG_MONSTER_WHISPER", "Quiet.", "Thrall", "Thrall whispers", "Quiet."},
    {"CHAT_MSG_RAID_BOSS_WHISPER", "Beware.", "Boss", "Boss whispers", "Beware."},
    {"CHAT_MSG_MONSTER_EMOTE", "%s attempts to run away in fear", "Al'aketh Stormcaller", "", "Al'aketh Stormcaller attempts to run away in fear"},
    {"CHAT_MSG_RAID_BOSS_EMOTE", "%s roars at 50% health!", "Boss", "", "Boss roars at 50% health!"},
    {"CHAT_MSG_MONSTER_EMOTE", "Thrall waves.", "Thrall", "", "Thrall waves."},
    {"CHAT_MSG_MONSTER_EMOTE", "%s cheers.", "100% NPC", "", "100% NPC cheers."},
}) do
    now = now + 1
    events.scripts.OnEvent(events, case[1], case[2], case[3])
    local sent = calls[#calls]
    assert(sent[3] == 5 and sent[4] == case[3] and sent[5] == case[4] and sent[6] == case[5], case[1])
end
SlashCmdList.FOREVERDUBBED("cell 2")
assert(ForeverDubbedDB.cell == 2)
SlashCmdList.FOREVERDUBBED("cell 1")
assert(ForeverDubbedDB.cell == 2)
SlashCmdList.FOREVERDUBBED("unlock")
assert(not ForeverDubbedDB.locked)
tile.scripts.OnDragStop(tile)
assert(ForeverDubbedDB.x==20 and ForeverDubbedDB.y==900)
now = now + 100
tile.scripts.OnUpdate(tile,0.3)
assert(tile.visible)
SlashCmdList.FOREVERDUBBED("lock")
tile.scripts.OnUpdate(tile,0.3)
assert(not tile.visible, "locked square must hide after the dialogue expires")
SlashCmdList.FOREVERDUBBED("off")
local before = #calls
events.scripts.OnEvent(events,"QUEST_DETAIL")
assert(#calls==before and not tile.visible)
SlashCmdList.FOREVERDUBBED("on")
SlashCmdList.FOREVERDUBBED("test")
assert(tile.visible)
local statusStart = #messages
SlashCmdList.FOREVERDUBBED("status")
assert(messages[statusStart+1]:find("Addon " .. metadataVersion, 1, true), "status must use TOC metadata version")
-- Older clients expose metadata through the global API instead.
GetAddOnMetadata = C_AddOns.GetAddOnMetadata
C_AddOns = nil
metadataVersion = "9.8.8"
-- A custom cell size survives reload after the one-time encoding upgrade.
ForeverDubbedDB.encodingVersion=3 -- upgrade preserves a valid custom cell size
SlashCmdList.FOREVERDUBBED("cell 3")
assert(tile.width==154 and tile.height==154)
assert(loadfile("addon/ForeverDubbed/ForeverDubbed.lua"))("ForeverDubbed", ns)
events, tile = objects[#objects], ForeverDubbedTile
events.scripts.OnEvent(events, "ADDON_LOADED", "ForeverDubbed")
assert(ForeverDubbedDB.cell==3 and ForeverDubbedDB.encodingVersion==5)
statusStart = #messages
SlashCmdList.FOREVERDUBBED("status")
assert(messages[statusStart+1]:find("Addon " .. metadataVersion, 1, true), "legacy metadata API must provide the version")
assert(tile.width==154 and tile.height==154)
now = now + 1
GetQuestText = function() return string.rep("A long quest. ", 200) end
events.scripts.OnEvent(events, "QUEST_DETAIL")
local animatedPages = actual(unpack(calls[#calls]))
assert(#animatedPages>1)
local function checkPage(index,phase)
    for i,value in ipairs(ns.Codec.Cells(animatedPages[index],phase)) do
        local rgb=value==ns.Codec.WAVE and ns.Codec.FINDER or ns.Codec.PALETTE[value+1]
        local t=tile.textures[i+4]
        assert(t.color[1]==rgb[1]/255 and t.color[2]==rgb[2]/255 and t.color[3]==rgb[3]/255)
    end
end
local function advancePage(dt,index,phase)
    local changed, before = 0, colorWrites
    for i,value in ipairs(ns.Codec.Cells(animatedPages[index],phase)) do
        local rgb=value==ns.Codec.WAVE and ns.Codec.FINDER or ns.Codec.PALETTE[value+1]
        local color=tile.textures[i+4].color
        if color[1]~=rgb[1]/255 or color[2]~=rgb[2]/255 or color[3]~=rgb[3]/255 then
            changed=changed+1
        end
    end
    tile.scripts.OnUpdate(tile,dt)
    checkPage(index,phase)
    assert(colorWrites-before==changed, "page change must update exactly the changed cells")
end
advancePage(0.2,1,3) -- three animation ticks, still the first page
advancePage(0.05,2,3) -- page changes at 250ms, independently of the next wave tick
advancePage(0.05,2,4)
-- Reuse the page buffer through several page wraps and coinciding wave ticks.
local pageIndex = 2
for tick=1,#animatedPages*4 do
    pageIndex=pageIndex%#animatedPages+1
    local phase=math.floor((0.3+tick*0.25)*15+1e-9)%ns.Codec.WAVE_PHASES
    advancePage(0.25,pageIndex,phase)
end
assert(tile.visible)
-- GetEffectiveScale alone is not a physical-pixel conversion. Exercise both
-- PixelUtil and its fallback across resolutions and user-selected UI scales.
for _, config in ipairs({{768,0.75},{1080,0.8},{1440,0.64},{2160,0.9}}) do
    physicalHeight,uiScale=config[1],config[2]
    for _, usePixelUtil in ipairs({false,true}) do
        PixelUtil=usePixelUtil and {GetPixelToUIUnitFactor=function() return 768/physicalHeight end} or nil
        events.scripts.OnEvent(events,"UI_SCALE_CHANGED")
        local pixelsPerUnit=tile:GetEffectiveScale()/(768/physicalHeight)
        assert(math.abs(pixelsPerUnit-1)<0.000001)
        assert(math.abs(tile.width*pixelsPerUnit-154)<0.000001)
        for i=1,4 do
            local t=tile.textures[i]
            assert(math.abs(math.min(t.width,t.height)*pixelsPerUnit-2)<0.000001)
            assert(t.color[1]==128/255 and t.color[2]==192/255 and t.color[3]==240/255)
        end
        assert(tile.textures[5].point[4]==2 and tile.textures[5].point[5]==-2)
        assert(tile.textures[5].width==3 and tile.textures[5].height==3)
        tile.scripts.OnDragStop(tile)
        assert(ForeverDubbedDB.x==20 and ForeverDubbedDB.y==900)
        SlashCmdList.FOREVERDUBBED("reset")
        assert(ForeverDubbedDB.x==16 and math.abs(ForeverDubbedDB.y-(physicalHeight-16))<0.000001)
    end
end
-- Asynchronous identities must not publish old text over a newer message.
local resolve, pending = ns.Speakers.Resolve, {}
ns.Speakers.Resolve = function(info, cb) pending[#pending+1] = function() cb(info) end end
now=now+1
C_GossipInfo.GetText=function() return "Older dialogue" end
events.scripts.OnEvent(events,"GOSSIP_SHOW")
C_GossipInfo.GetText=function() return "Newer dialogue" end
events.scripts.OnEvent(events,"GOSSIP_SHOW")
local count=#calls
pending[2]();pending[1]()
assert(#calls==count+1 and calls[#calls][6]=="Newer dialogue")
pending={}
events.scripts.OnEvent(events,"GOSSIP_SHOW")
SlashCmdList.FOREVERDUBBED("test")
count=#calls
pending[1]()
assert(#calls==count and calls[#calls][4]=="ForeverDubbed")
ns.Speakers.Resolve=resolve
-- Manual assignment persists by NPC ID; it does not alter every shared model.
function methods:SetUnit() return true end
function methods:GetDisplayInfo() return 176 end
function methods:GetModelFileID() return 7478487 end
SlashCmdList.FOREVERDUBBED("race Skyborne")
assert(ForeverDubbedDB.npcRaces["4949"]=="Skyborne")
assert(ForeverDubbedDB.npcRaceEvidence["4949"].name=="Thrall")
assert(ForeverDubbedDB.npcRaceEvidence["4949"].race=="Skyborne")
now=now+1
events.scripts.OnEvent(events,"GOSSIP_SHOW")
local marked=calls[#calls]
assert(marked[7]=="Orc" and marked[9]=="4949")
assert(marked[10]=="176" and marked[11]=="7478487" and marked[12]=="Skyborne")
SlashCmdList.FOREVERDUBBED("race clear")
assert(ForeverDubbedDB.npcRaces["4949"]==nil)
assert(ForeverDubbedDB.npcRaceEvidence["4949"]==nil)
now=now+1
events.scripts.OnEvent(events,"GOSSIP_SHOW")
assert(calls[#calls][7]=="Orc" and calls[#calls][12]=="")
-- GUID identity is required for chat; do not infer race from a shared name.
assert(ns.Speakers.Chat("Creature-0-1-2-3-9999-54321").race=="")
assert(ns.Speakers.Chat("").race=="")
local cachedGUID=UnitGUID()
ns.Speakers.ForUnit("npc")
UnitGUID=function() return nil end
assert(ns.Speakers.Chat(cachedGUID).race=="Orc")
issecretvalue=function(v) return v=="Orc" or v==2 end
local secret=ns.Speakers.ForUnit("npc")
assert(secret.race=="" and secret.gender=="")
-- Minimap and keybindings send control packets even with automatic dialogue off.
local button = ForeverDubbedMinimapButton
assert(button.visible and ForeverDubbedDB.minimapAngle == 225)
assert(BINDING_HEADER_FOREVERDUBBED == "ForeverDubbed")
SlashCmdList.FOREVERDUBBED("off")
now = now + 1
local before = #calls
button.scripts.OnClick(button, "LeftButton")
assert(#calls == before+1 and calls[#calls][3] == 8 and calls[#calls][6] == "")
assert(tile.visible)
local controlSequence = calls[#calls][2]
for i=1,5 do tile.scripts.OnUpdate(tile, 0.1) end
assert(#calls == before+1, "redrawing a control must not issue a fresh command")
now = now + 1
shiftDown = true
button.scripts.OnClick(button, "LeftButton")
shiftDown = false
assert(calls[#calls][3] == 7 and calls[#calls][2] == controlSequence+1)
now = now + 1
ForeverDubbed_SkipAudio()
assert(calls[#calls][3] == 8)
now = now + 1
ForeverDubbed_StopAudio()
assert(calls[#calls][3] == 7)
-- Dragging saves the angle and does not dispatch a command on release.
button.scripts.OnDragStart(button)
button.scripts.OnUpdate(button)
local angle = ForeverDubbedDB.minimapAngle
assert(type(angle) == "number" and angle ~= 225)
button.scripts.OnDragStop(button)
before = #calls
button.scripts.OnClick(button, "LeftButton")
assert(#calls == before and not button.scripts.OnUpdate)
ns.Controls.Init(ForeverDubbedDB)
assert(ForeverDubbedDB.minimapAngle == angle)
-- Fresh dialogue waits until a control has had a chance to be captured.
GetQuestText = function() return "Deferred quest body" end
SlashCmdList.FOREVERDUBBED("on")
now = now + 1
SlashCmdList.FOREVERDUBBED("skip")
before = #calls
events.scripts.OnEvent(events, "QUEST_DETAIL")
assert(#calls == before and calls[#calls][3] == 8)
now = now + 1.6
tile.scripts.OnUpdate(tile, 0.1)
assert(#calls == before+1 and calls[#calls][3] == 2)
assert(calls[#calls][5] == "Quest" and calls[#calls][6] == "Deferred quest body" and calls[#calls][13] == "Quest objectives",
    "deferred quest lost separate fields")
before = #calls
GetObjectiveText = function() return "Updated objectives" end
events.scripts.OnEvent(events, "QUEST_DETAIL")
assert(#calls == before+1 and calls[#calls][13] == "Updated objectives", "objective-only update was deduplicated")
-- A late identity resolution cannot replace an explicit stop.
pending = {}
ns.Speakers.Resolve = function(info, cb) pending[#pending+1] = function() cb(info) end end
events.scripts.OnEvent(events, "GOSSIP_SHOW")
now = now + 1
SlashCmdList.FOREVERDUBBED("stop")
before = #calls
pending[1]()
assert(#calls == before and calls[#calls][3] == 7)
-- Every invocation has a fresh ID, including identical commands in one tick.
local sequence = calls[#calls][2]
ForeverDubbed_StopAudio()
assert(calls[#calls][2] == sequence+1 and calls[#calls][3] == 7)
ForeverDubbed_StopAudio()
assert(calls[#calls][2] == sequence+2 and calls[#calls][3] == 7)
ForeverDubbed_SkipAudio()
assert(calls[#calls][2] == sequence+3 and calls[#calls][3] == 8)
local function assertMinimapIcon(paused)
    local expected = "Interface\\AddOns\\ForeverDubbed\\" .. (paused and "IconPaused" or "Icon")
    assert(ForeverDubbedMinimapButton.textures[1].texture == expected, "minimap icon does not match pause state")
end
assertMinimapIcon(false)
-- Both menu APIs expose the current pause state and preserve the click shortcuts.
SlashCmdList.FOREVERDUBBED("on")
for _, modern in ipairs({true, false}) do
    MenuUtil = modern and modernMenu or nil
    before = #calls
    button.scripts.OnClick(button, "RightButton")
    assert(#calls == before and #menuItems == 3, "opening the menu sent a control")
    assert(menuItems[1].text == "Pause")
    menuItems[1].func()
    assert(ForeverDubbedDB.paused and tile.visible and #calls == before+1 and calls[#calls][3] == 7,
        "pause must send Stop")
    assertMinimapIcon(true)
    before = #calls
    now = now + 1
    tile.scripts.OnUpdate(tile, 1)
    assert(tile.visible and #calls == before, "Stop must remain visible without being resent")
    now = now + 15
    tile.scripts.OnUpdate(tile, 15)
    assert(not tile.visible)
    button.scripts.OnClick(button, "RightButton")
    assert(menuItems[1].text == "Resume" and not menuItems[2].disabled and not menuItems[3].disabled)
    for _, cmd in ipairs({"test", "unlock", "on", "cell 3", "reset"}) do
        SlashCmdList.FOREVERDUBBED(cmd)
        assert(not tile.visible and #calls == before, "paused command exposed the square: " .. cmd)
    end
    for _, event in ipairs({"GOSSIP_SHOW", "QUEST_DETAIL", "QUEST_PROGRESS", "QUEST_COMPLETE", "UI_SCALE_CHANGED"}) do
        events.scripts.OnEvent(events, event)
        assert(not tile.visible and #calls == before, "paused event transmitted: " .. event)
    end
    SlashCmdList.FOREVERDUBBED("lock")
    now = now + 20
    tile.scripts.OnUpdate(tile, 20)
    assert(not tile.visible and #calls == before)
    -- Controls remain usable while paused, without admitting new dialogue.
    menuItems[2].func()
    assert(calls[#calls][3] == 8 and tile.visible and ForeverDubbedDB.paused)
    menuItems[3].func()
    assert(calls[#calls][3] == 7 and tile.visible and ForeverDubbedDB.paused)
    before = #calls
    events.scripts.OnEvent(events, "QUEST_DETAIL")
    assert(#calls == before and calls[#calls][3] == 7)
    SlashCmdList.FOREVERDUBBED("lock")
    menuItems[1].func()
    assert(not ForeverDubbedDB.paused and #calls == before, "resume should wait for new dialogue")
    assertMinimapIcon(false)
    button.scripts.OnClick(button, "RightButton")
    assert(menuItems[1].text == "Pause" and not menuItems[2].disabled and not menuItems[3].disabled)
    menuItems[2].func()
    assert(calls[#calls][3] == 8)
    menuItems[3].func()
    assert(calls[#calls][3] == 7)
end
-- Late identity callbacks cannot resurrect pre-pause dialogue, even after resume.
pending = {}
events.scripts.OnEvent(events, "QUEST_DETAIL")
assert(#pending == 1)
SlashCmdList.FOREVERDUBBED("pause")
assertMinimapIcon(true)
SlashCmdList.FOREVERDUBBED("resume")
assertMinimapIcon(false)
before = #calls
pending[1]()
assert(#calls == before)
-- Drop dialogue deferred behind a control when pausing.
ns.Speakers.Resolve = function(info, cb) cb(info) end
ForeverDubbed_StopAudio()
events.scripts.OnEvent(events, "QUEST_DETAIL")
ForeverDubbed_TogglePause()
assert(ForeverDubbedDB.paused and calls[#calls][3] == 7)
now = now + 16
tile.scripts.OnUpdate(tile, 16)
assert(not tile.visible)
ForeverDubbed_TogglePause()
before = #calls
now = now + 2
tile.scripts.OnUpdate(tile, 2)
assert(#calls == before, "resume replayed deferred dialogue")
events.scripts.OnEvent(events, "QUEST_DETAIL")
assert(#calls == before+1 and calls[#calls][3] == 2, "new dialogue failed after resume")
-- Persisted pause also suppresses startup version publication.
SlashCmdList.FOREVERDUBBED("pause")
now = now + 16
tile.scripts.OnUpdate(tile, 16)
before = #calls
events.scripts.OnEvent(events, "ADDON_LOADED", "ForeverDubbed")
assert(ForeverDubbedDB.paused and not tile.visible and #calls == before)
assertMinimapIcon(true)
print("Addon smoke tests passed")
