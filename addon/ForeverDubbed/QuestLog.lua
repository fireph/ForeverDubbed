local _, NS = ...
NS.QuestLog = {}
local buttons = {}

-- Read only the quest currently displayed; never change the player's selection.
local function selectedQuest(parent, modern)
    if not GetQuestLogQuestText then return end
    local index, title
    if modern then
        local questID = parent.questID
        if not questID or not C_QuestLog or not C_QuestLog.GetLogIndexForQuestID or not C_QuestLog.GetInfo then return end
        index = C_QuestLog.GetLogIndexForQuestID(questID)
        if not index or index <= 0 then return end
        local info = C_QuestLog.GetInfo(index)
        if not info or info.isHeader then return end
        title = info.title
    else
        if not GetQuestLogSelection or not GetQuestLogTitle then return end
        index = GetQuestLogSelection()
        if not index or index <= 0 then return end
        local level, tag, isHeader
        title, level, tag, isHeader = GetQuestLogTitle(index)
        if isHeader then return end
    end
    if not title or title == "" then return end
    local description, objectives
    if modern then description, objectives = GetQuestLogQuestText(index)
    else description, objectives = GetQuestLogQuestText() end
    if (not description or description == "") and (not objectives or objectives == "") then return end
    return title, description or "", objectives or ""
end

local function getQuest(parent, modern)
    -- Quest selection can disappear during a log refresh or quest abandonment.
    local ok, title, description, objectives = pcall(selectedQuest, parent, modern)
    if ok then return title, description, objectives end
end

function NS.QuestLog.Refresh()
    for parent, entry in pairs(buttons) do
        local db = ForeverDubbedDB
        entry.button:SetEnabled(db and db.enabled and getQuest(parent, entry.modern) ~= nil)
    end
end

function NS.QuestLog.Init()
    local modern = QuestMapFrame and QuestMapFrame.DetailsFrame ~= nil
    local parent = modern and QuestMapFrame.DetailsFrame or QuestLogFrame
    if not parent or buttons[parent] then return end
    local button = CreateFrame("Button", "ForeverDubbedQuestLogPlayButton", parent)
    buttons[parent] = {button=button, modern=modern}
    button:SetSize(24, 24)
    -- Use the fixed header so long quest titles and scrolling never cover Play.
    if modern and parent.BackFrame and parent.BackFrame.BackButton then
        button:SetPoint("LEFT", parent.BackFrame.BackButton, "RIGHT", 8, 0)
    elseif not modern and QuestLogFrameCloseButton then
        button:SetPoint("RIGHT", QuestLogFrameCloseButton, "LEFT", -4, 0)
    else
        button:SetPoint("TOPRIGHT", parent, "TOPRIGHT", -42, -12)
    end
    button:SetFrameLevel(parent:GetFrameLevel() + 5)
    button:SetNormalTexture("Interface\\Buttons\\UI-SpellbookIcon-NextPage-Up")
    button:SetPushedTexture("Interface\\Buttons\\UI-SpellbookIcon-NextPage-Down")
    button:SetDisabledTexture("Interface\\Buttons\\UI-SpellbookIcon-NextPage-Disabled")
    button:SetHighlightTexture("Interface\\Buttons\\UI-Common-MouseHilight", "ADD")
    button:RegisterForClicks("LeftButtonUp")
    button:SetScript("OnClick", function()
        local title, description, objectives = getQuest(parent, modern)
        if title and NS.ReadQuestLog then NS.ReadQuestLog(title, description, objectives) end
    end)
    button:SetScript("OnEnter", function(self)
        GameTooltip:SetOwner(self, "ANCHOR_RIGHT")
        GameTooltip:AddLine("Read quest aloud", 1, 0.82, 0)
        GameTooltip:AddLine("Read the selected quest with the narrator voice.", 1, 1, 1)
        if not ForeverDubbedDB.enabled then
            GameTooltip:AddLine("Enable ForeverDubbed to read this quest.", 1, 0.82, 0)
        end
        GameTooltip:Show()
    end)
    button:SetScript("OnLeave", function() GameTooltip:Hide() end)
    parent:HookScript("OnShow", NS.QuestLog.Refresh)
    local hooks = modern and {"QuestMapFrame_ShowQuestDetails"} or {"QuestLog_SetSelection", "QuestLog_Update"}
    for _, name in ipairs(hooks) do
        if type(_G[name]) == "function" then hooksecurefunc(name, NS.QuestLog.Refresh) end
    end
    NS.QuestLog.Refresh()
    button:Show()
end
