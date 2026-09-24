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
assert(loadfile("addon/ForeverDubbed/Speakers.lua"))("ForeverDubbed",ns)
local S=ns.Speakers
local result
-- Addon collects raw metadata; Skyborne/NPC/display/model lookups live on desktop.
npc(254100);name="Zephras Citizen";modelID=7478494
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.race=="" and result.npcID=="254100" and result.modelID==7478494)
assert(result.gender=="male") -- Public API wins over model appearance.
local citizenGUID=guid;guid=""
assert(S.Chat(citizenGUID).modelID==7478494)
-- Missing model data can load asynchronously.
npc(123);result=nil
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result==nil and #queue==1)
modelID=119376;tick()
assert(result.race=="" and result.modelID==119376)
-- Saved race and public API race remain separate for desktop precedence.
race="Orc"
S.SetRace(S.Dialog(),"Skyborne")
local info=S.Dialog()
assert(info.race=="Skyborne" and info.apiRace=="Orc" and info.raceOverride=="Skyborne")
S.SetRace(info,nil)
assert(S.Dialog().race=="Orc" and S.Dialog().raceOverride=="")
-- A public race must not skip appearance collection.
modelID=7478487
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.apiRace=="Orc" and result.modelID==7478487)
-- Restricted API values cannot trigger appearance probing.
npc(131);race="secret";modelID=119940
issecretvalue=function(v) return v=="secret" end
local before=loads
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.race=="" and loads==before)
issecretvalue=nil
-- Retiring a request never attaches the next unit's model.
npc(125);local old,fresh
S.Resolve(S.Dialog(),function(v) old=v end)
npc(126);modelID=121287
S.Resolve(S.Dialog(),function(v) fresh=v end)
tick()
assert(old.npcID=="125" and old.modelID==nil)
assert(fresh.npcID=="126" and fresh.modelID==121287)
-- Same token changed occupant while loading.
npc(127);result=nil
S.Resolve(S.Dialog(),function(v) result=v end)
npc(128);modelID=119940;tick()
assert(result.npcID=="127" and result.modelID==nil)
-- Bounded timeout and failed binding.
npc(129);result=nil
S.Resolve(S.Dialog(),function(v) result=v end)
for i=1,12 do tick() end
assert(result and result.modelID==nil and #queue==0)
npc(130);fail=true;modelID=119940
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.modelID==nil)
-- Display and model IDs are observations only, and cached by GUID for chat.
local displayID
function model:GetDisplayInfo() return displayID end
npc(140);displayID=176;modelID=1100258;sex=nil
S.Resolve(S.Dialog(),function(v) result=v end,true)
assert(result.race=="" and result.gender=="" and result.displayID==176)
local cachedGUID=guid;guid=""
assert(S.Chat(cachedGUID).displayID==176)
-- Late display IDs are collected without replacing observed race/gender.
npc(142);modelID=119376;displayID=nil;result=nil
S.Resolve(S.Dialog(),function(v) result=v end,true)
assert(result==nil)
displayID=115;tick()
assert(result.displayID==115 and result.modelID==119376 and result.race=="")
-- Capture evidence on manual marking, including existing assignments.
npc(254100);displayID=176;modelID=7478494
S.SetRace(S.Dialog(),"Skyborne")
local evidence=ForeverDubbedDB.npcRaceEvidence["254100"]
assert(evidence.name==name and evidence.race=="Skyborne" and evidence.gender=="male")
assert(evidence.displayID==176 and evidence.modelID==7478494 and evidence.displayStatus=="available")
-- Other NPCs sharing a display have no inherited manual override.
npc(151);modelID=7478494
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.raceOverride=="" and result.race=="")
-- Assignment is immediate; evidence can arrive later.
npc(152);displayID=nil
S.SetRace(S.Dialog(),"Skyborne")
assert(ForeverDubbedDB.npcRaces["152"]=="Skyborne")
displayID=115;modelID=119376;tick()
assert(ForeverDubbedDB.npcRaceEvidence["152"].displayID==115)
-- Clearing a pending mark cannot resurrect override or evidence.
npc(153);displayID=nil
S.SetRace(S.Dialog(),"Skyborne")
S.SetRace(S.Dialog(),nil)
displayID=176;tick()
assert(ForeverDubbedDB.npcRaces["153"]==nil)
assert(ForeverDubbedDB.npcRaceEvidence["153"]==nil and S.Dialog().race=="")
-- Switching target while collecting never saves the new target's appearance.
npc(154);displayID=nil
S.SetRace(S.Dialog(),"Skyborne")
npc(155);displayID=115;modelID=119376;tick()
assert(ForeverDubbedDB.npcRaceEvidence["154"].displayID==nil)
assert(ForeverDubbedDB.npcRaceEvidence["154"].modelID==nil)
-- Failed probes must not reuse previously cached appearance evidence.
npc(156);displayID=176;modelID=1100258
S.Resolve(S.Dialog(),function(v) result=v end)
fail=true
S.SetRace(S.Dialog(),"Skyborne")
assert(ForeverDubbedDB.npcRaceEvidence["156"].displayID==nil)
assert(ForeverDubbedDB.npcRaceEvidence["156"].modelID==nil)
assert(ForeverDubbedDB.npcRaceEvidence["156"].displayStatus=="not read")
-- Preserve missing-display reasons without storing restricted values.
for _, case in ipairs({
    {get=function() return 0 end, status="API returned 0"},
    {get=function() return nil end, status="API returned nil"},
    {get=function() error("failed") end, status="API call failed"},
    {get=function() return "secret" end, status="restricted value"},
    {status="API unavailable"},
}) do
    npc(157);modelID=7478487
    model.GetDisplayInfo=case.get
    issecretvalue=function(v) return v=="secret" end
    S.SetRace(S.Dialog(),"Skyborne")
    for i=1,12 do tick() end
    local saved=ForeverDubbedDB.npcRaceEvidence["157"]
    assert(saved.displayID==nil and saved.displayStatus==case.status)
    assert(saved.modelID==7478487)
    issecretvalue=nil
end
-- Normal dialogue never calls or waits on the display API.
local displayReads=0
model.GetDisplayInfo=function() displayReads=displayReads+1; return 0 end
npc(160);modelID=7478487;result=nil
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result and result.modelID==7478487 and displayReads==0 and #queue==0)
-- Explicit inspection can report zero immediately once a model is available.
S.Resolve(S.Dialog(),function(v) result=v end,true)
assert(result.displayStatus=="API returned 0" and displayReads==1 and #queue==0)
-- Display-only and missing-model APIs remain supported.
model.GetModelFileID=nil
model.GetDisplayInfo=function() return 176 end
npc(158)
S.Resolve(S.Dialog(),function(v) result=v end,true)
assert(result.displayID==176 and result.modelID==nil)
model.GetDisplayInfo=nil
npc(159)
S.Resolve(S.Dialog(),function(v) result=v end)
assert(result.displayID==nil and result.modelID==nil)
print("Speaker observation tests passed")
