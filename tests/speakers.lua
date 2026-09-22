-- NPC UnitRace normally returns nil. Exercise the real missing-race path,
-- asynchronous model loading, overrides, and identity changes explicitly.
local ns, queue = {}, {}
local guid, name, sex, race, modelID, loads, fail = "", "", 2, nil, nil, 0, false
UIParent = {}
ForeverDubbedDB = {}
function UnitGUID() return guid end
function UnitName() return name end
function UnitRace() return nil, race end
function UnitSex() return sex end
local model = {}
function model:SetSize() end
function model:SetPoint() end
function model:SetAlpha() end
function model:EnableMouse() end
function model:Hide() end
function model:Show() end
function model:ClearModel() end
function model:SetUnit() loads=loads+1; return not fail end
function model:GetModelFileID() return modelID end
function CreateFrame() return model end
C_Timer = {After=function(_, fn) queue[#queue+1]=fn end}
local function tick()
    local current=queue; queue={}
    for _,fn in ipairs(current) do fn() end
end
local function npc(id)
    guid="Creature-0-1-2-3-" .. id .. "-12345"
    name="NPC " .. id
    race,sex,modelID,fail=nil,2,nil,false
end
assert(loadfile("addon/ForeverDubbed/Races.lua"))("ForeverDubbed",ns)
assert(loadfile("addon/ForeverDubbed/DisplayRaces.lua"))("ForeverDubbed",ns)
assert(loadfile("addon/ForeverDubbed/Speakers.lua"))("ForeverDubbed",ns)
local S=ns.Speakers
-- User's exact report: no UnitRace, but public gender and NPC ID 254100.
npc(254100); name="Zephras Citizen"
local info=S.Dialog()
assert(info.race=="Skyborne" and info.gender=="male" and info.npcID=="254100")
local result
S.Resolve(info,function(v) result=v end)
assert(result.race=="Skyborne" and loads==0 and result.raceSource=="NPC ID lookup")
-- Static identity also works for distant chat with only a public GUID.
local citizenGUID=guid
guid=""
assert(S.Chat(citizenGUID).race=="Skyborne")
-- Missing NPC API race, then delayed Goblin model becomes available.
npc(123)
result=nil
S.Resolve(S.Dialog(),function(v) result=v end)
assert(not result and #queue==1)
modelID=119376;tick()
assert(result.race=="Goblin" and result.modelID==119376 and result.gender=="male")
assert(result.raceSource=="model appearance")
-- A partial API read must not erase the previously resolved race.
assert(S.Dialog().race=="Goblin")
-- Explicit NPC assignment wins over shared models and survives later lookups.
S.SetRace(S.Dialog(),"Skyborne")
assert(S.Dialog().race=="Skyborne")
S.SetRace(S.Dialog(),nil)
assert(S.Dialog().race=="")
-- A native API race has priority over an inferred model.
race="Human";modelID=119376
S.Resolve(S.Dialog(),function(v) result=v end,true)
assert(result.race=="Human" and result.raceSource=="UnitRace")
-- Unknown models stay unknown.
npc(124);modelID=987654321
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.race=="" and result.modelID==987654321)
-- Retiring an old request does not attach its late model to another NPC.
npc(125);local old, fresh
S.Resolve(S.Dialog(),function(v) old=v end)
npc(126);modelID=121287
S.Resolve(S.Dialog(),function(v) fresh=v end)
tick()
assert(old.npcID=="125" and old.race=="")
assert(fresh.npcID=="126" and fresh.race=="Orc")
-- Same unit token changed occupant while a request was loading.
npc(127);result=nil
S.Resolve(S.Dialog(),function(v) result=v end)
npc(128);modelID=119940;tick()
assert(result.npcID=="127" and result.race=="")
-- Timeout is bounded even if a model never loads.
npc(129);result=nil
S.Resolve(S.Dialog(),function(v) result=v end)
for i=1,12 do tick() end
assert(result and result.race=="" and #queue==0)
-- Failed model binds never inspect a stale previous model.
npc(130);fail=true;modelID=119940
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.race=="")
-- Restricted API values must not be bypassed using model probing/cache.
npc(131);race="secret";modelID=119940
issecretvalue=function(v) return v=="secret" end
local before=loads
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.race=="" and loads==before)
issecretvalue=nil
-- Missing model APIs degrade to unknown instead of breaking dialog.
npc(132);model.GetModelFileID=nil
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.race=="")
-- Pinned database uses display IDs (not NPC IDs or model FileDataIDs), sex 0/1.
assert(ns.DisplayRaceCount==15444)
assert(ns.DisplayIdentity(115).race=="Dwarf" and ns.DisplayIdentity(115).gender=="male")
assert(ns.DisplayIdentity(176).race=="Human" and ns.DisplayIdentity(176).gender=="female")
assert(ns.DisplayIdentity(6882).race=="Goblin")
assert(ns.DisplayIdentity(4)==nil and ns.DisplayIdentity(999999999)==nil)
local displayID
function model:GetDisplayInfo() return displayID end
-- Display lookup works even without the model-file API.
npc(140);displayID=176;sex=nil
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.race=="Human" and result.gender=="female" and result.displayID==176)
assert(result.raceSource=="display lookup (VoiceOver)")
local cachedGUID=guid;guid=""
assert(S.Chat(cachedGUID).displayID==176 and S.Chat(cachedGUID).gender=="female")
function model:GetModelFileID() return modelID end
-- Display race wins over shared model appearance; public UnitSex still wins.
npc(141);displayID=115;modelID=119376;sex=3
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.race=="Dwarf" and result.gender=="female" and result.modelID==119376)
-- Native API, custom Skyborne identity, and saved assignments retain priority.
race="Orc"
S.Resolve(S.Dialog(),function(v) result=v end,true)
assert(result.race=="Orc" and result.raceSource=="UnitRace")
S.SetRace(S.Dialog(),"Skyborne")
S.Resolve(S.Dialog(),function(v) result=v end,true)
assert(result.race=="Skyborne" and result.raceSource=="saved NPC override")
npc(254100)
S.Resolve(S.Dialog(),function(v) result=v end,true)
assert(result.race=="Skyborne" and result.raceSource=="NPC ID lookup")
-- Model may arrive first; do not publish its weaker identity prematurely.
npc(142);modelID=119376;displayID=nil;result=nil
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result==nil)
displayID=115;tick()
assert(result.race=="Dwarf")
-- Failed display lookup still falls back, including after the bounded timeout.
npc(143);modelID=119376;displayID=999999999
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.race=="Goblin" and result.raceSource=="model appearance")
npc(144);modelID=119376;displayID=0;result=nil;sex=nil
S.Resolve(S.Dialog(),function(v) result=v end)
for i=1,12 do tick() end
assert(result.race=="Goblin" and result.gender=="male" and #queue==0)
-- A cached model guess can later be upgraded by a display record, including sex.
displayID=176
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.race=="Human" and result.gender=="female")
-- Secret or throwing display APIs must not break or leak into lookup keys.
npc(145);modelID=119376;displayID="secret"
issecretvalue=function(v) return v=="secret" end
S.Resolve(S.Dialog(),function(v) result=v end)
for i=1,12 do tick() end
assert(result.race=="Goblin" and result.displayID==nil)
issecretvalue=nil
function model:GetDisplayInfo() error("unavailable") end
npc(146);modelID=119376
S.Resolve(S.Dialog(),function(v) result=v end)
for i=1,12 do tick() end
assert(result.race=="Goblin")
function model:GetDisplayInfo() return displayID end
-- Changed unit and cancelled requests cannot pick up a late display identity.
npc(147);displayID=nil;local stale
S.Resolve(S.Dialog(),function(v) stale=v end)
npc(148);displayID=115;tick()
assert(stale.npcID=="147" and stale.race=="")
npc(149);displayID=nil;stale=nil
S.Resolve(S.Dialog(),function(v) stale=v end)
npc(150);displayID=176
S.Resolve(S.Dialog(),function(v) result=v end)
tick()
assert(stale.npcID=="149" and stale.race=="" and result.race=="Human")
print("Speaker race tests passed")
