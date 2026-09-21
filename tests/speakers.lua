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
print("Speaker race tests passed")
