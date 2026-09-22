-- Minimal UI/API doubles exercise event routing, configuration and page lifetime.
local now, objects, messages = 100, {}, {}
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
function methods:SetSize(w,h) self.width, self.height = w,h end
function methods:SetScale(scale) self.scale=scale end
function methods:SetColorTexture(r,g,b,a) assert(r==0 or r==128/255 or r==1); assert(g==0 or g==128/255 or g==1); assert(b==0 or b==128/255 or b==1); assert(a==1) end
function methods:Show() self.visible = true end
function methods:Hide() self.visible = false end
function methods:GetEffectiveScale()
    if self == UIParent then return uiScale end
    local parent=rawget(self,"parent")
    return (rawget(self,"scale") or 1) * (parent and parent:GetEffectiveScale() or 1)
end
function methods:GetHeight() return 768/uiScale end
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
assert(loadfile("addon/ForeverDubbed/Races.lua"))("ForeverDubbed", ns)
assert(loadfile("addon/ForeverDubbed/DisplayRaces.lua"))("ForeverDubbed", ns)
assert(loadfile("addon/ForeverDubbed/Speakers.lua"))("ForeverDubbed", ns)
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
assert(ForeverDubbedDB.encodingVersion==3 and ForeverDubbedDB.mode==nil)
assert(tile.width==100 and tile.height==100 and #tile.textures==2500)
ForeverDubbedDB.chat=true
for _, event in ipairs({"GOSSIP_SHOW","QUEST_GREETING","QUEST_DETAIL","QUEST_PROGRESS","QUEST_COMPLETE"}) do
    now = now + 1
    events.scripts.OnEvent(events,event)
    assert(tile.visible)
end
assert(calls[3][7]=="Orc" and calls[3][8]=="male" and calls[3][9]=="4949")
assert(calls[3][6] == "Quest body\n\nQuest objectives")
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
SlashCmdList.FOREVERDUBBED("cell 3")
assert(tile.width==150 and tile.height==150)
assert(loadfile("addon/ForeverDubbed/ForeverDubbed.lua"))("ForeverDubbed", ns)
events, tile = objects[#objects], ForeverDubbedTile
events.scripts.OnEvent(events, "ADDON_LOADED", "ForeverDubbed")
assert(ForeverDubbedDB.cell==3 and ForeverDubbedDB.encodingVersion==3)
assert(tile.width==150 and tile.height==150)
now = now + 1
GetQuestText = function() return string.rep("A long quest. ", 200) end
events.scripts.OnEvent(events, "QUEST_DETAIL")
tile.scripts.OnUpdate(tile, 0.3)
assert(tile.visible)
assert(#actual(unpack(calls[#calls]))>1)
-- GetEffectiveScale alone is not a physical-pixel conversion. Exercise both
-- PixelUtil and its fallback across resolutions and user-selected UI scales.
for _, config in ipairs({{768,0.75},{1080,0.8},{1440,0.64},{2160,0.9}}) do
    physicalHeight,uiScale=config[1],config[2]
    for _, usePixelUtil in ipairs({false,true}) do
        PixelUtil=usePixelUtil and {GetPixelToUIUnitFactor=function() return 768/physicalHeight end} or nil
        events.scripts.OnEvent(events,"UI_SCALE_CHANGED")
        local pixelsPerUnit=tile:GetEffectiveScale()/(768/physicalHeight)
        assert(math.abs(pixelsPerUnit-1)<0.000001)
        assert(math.abs(tile.width*pixelsPerUnit-150)<0.000001)
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
SlashCmdList.FOREVERDUBBED("race Skyborne")
assert(ForeverDubbedDB.npcRaces["4949"]=="Skyborne")
SlashCmdList.FOREVERDUBBED("race clear")
assert(ForeverDubbedDB.npcRaces["4949"]==nil)
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
print("Addon smoke tests passed")
