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
    local f = setmetatable({scripts={}, events={}, parent=parent}, {__index=methods})
    objects[#objects+1] = f
    if name then _G[name] = f end
    return f
end
UIParent = CreateFrame("Frame")
Minimap = CreateFrame("Frame", nil, UIParent)
GameTooltip = setmetatable({}, {__index=methods})
local cursorX, cursorY = 200, 100
function GetCursorPosition() return cursorX, cursorY end
DEFAULT_CHAT_FRAME = {AddMessage=function(_, text) messages[#messages+1]=text end}
function time() return 12345 end
function GetTime() return now end
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
assert(calls[3][7]=="Orc" and calls[3][8]=="male" and calls[3][9]=="4949")
assert(calls[3][6] == "Quest body" and calls[3][13] == "Quest objectives")
events.scripts.OnEvent(events,"CHAT_MSG_MONSTER_SAY","|cffffffffHello|r |Hitem:1|hfriend|h |Ticon:16|t","NPC")
assert(calls[#calls][6] == "Hello friend")
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
assert(not tile.visible)
SlashCmdList.FOREVERDUBBED("off")
local before = #calls
events.scripts.OnEvent(events,"QUEST_DETAIL")
assert(#calls==before and not tile.visible)
SlashCmdList.FOREVERDUBBED("on")
SlashCmdList.FOREVERDUBBED("test")
assert(tile.visible)
SlashCmdList.FOREVERDUBBED("status")
-- A custom cell size survives reload after the one-time encoding upgrade.
ForeverDubbedDB.encodingVersion=3 -- upgrade preserves a valid custom cell size
SlashCmdList.FOREVERDUBBED("cell 3")
assert(tile.width==154 and tile.height==154)
assert(loadfile("addon/ForeverDubbed/ForeverDubbed.lua"))("ForeverDubbed", ns)
events, tile = objects[#objects], ForeverDubbedTile
events.scripts.OnEvent(events, "ADDON_LOADED", "ForeverDubbed")
assert(ForeverDubbedDB.cell==3 and ForeverDubbedDB.encodingVersion==5)
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
button.scripts.OnClick(button, "RightButton")
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
print("Addon smoke tests passed")
