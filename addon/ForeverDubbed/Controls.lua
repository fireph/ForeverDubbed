local _, NS = ...
NS.Controls = {}
local minimapIcon

function NS.Controls.UpdateIcon(paused)
    if minimapIcon then
        minimapIcon:SetTexture("Interface\\AddOns\\ForeverDubbed\\" .. (paused and "IconPaused" or "Icon"))
    end
end

BINDING_HEADER_FOREVERDUBBED = "ForeverDubbed"
BINDING_NAME_FOREVERDUBBED_STOP = "Stop all audio"
BINDING_NAME_FOREVERDUBBED_SKIP = "Skip current audio"
BINDING_NAME_FOREVERDUBBED_PAUSE = "Pause / resume"

-- Bindings.xml calls these globals; the transport is initialized at ADDON_LOADED.
function ForeverDubbed_StopAudio() if NS.Control then NS.Control("stop") end end
function ForeverDubbed_SkipAudio() if NS.Control then NS.Control("skip") end end

function ForeverDubbed_TogglePause()
    if NS.SetPaused then NS.SetPaused(not ForeverDubbedDB.paused) end
end

local function openMenu(button, db)
    GameTooltip:Hide()
    local entries = {
        {db.paused and "Resume" or "Pause", ForeverDubbed_TogglePause},
        {"Skip current audio", ForeverDubbed_SkipAudio},
        {"Stop all audio", ForeverDubbed_StopAudio},
    }
    if MenuUtil and MenuUtil.CreateContextMenu then
        MenuUtil.CreateContextMenu(button, function(_, root)
            root:CreateTitle("ForeverDubbed")
            for _, entry in ipairs(entries) do root:CreateButton(entry[1], entry[2]) end
        end)
    else
        -- Classic clients use the legacy dropdown API.
        button.menu = button.menu or CreateFrame("Frame", nil, UIParent, "UIDropDownMenuTemplate")
        UIDropDownMenu_Initialize(button.menu, function()
            for _, entry in ipairs(entries) do
                local info = UIDropDownMenu_CreateInfo()
                info.text, info.func, info.notCheckable = entry[1], entry[2], true
                UIDropDownMenu_AddButton(info)
            end
        end, "MENU")
        ToggleDropDownMenu(1, nil, button.menu, "cursor", 0, 0)
    end
end

function NS.Controls.Init(db)
    if not Minimap then return end
    db.minimapAngle = tonumber(db.minimapAngle) or 225
    local button = CreateFrame("Button", "ForeverDubbedMinimapButton", Minimap)
    button.dragging, button.ignoreClickUntil = false, 0
    button:SetSize(32, 32)
    button:SetFrameStrata("MEDIUM")
    button:SetFrameLevel(Minimap:GetFrameLevel() + 8)
    button:RegisterForClicks("LeftButtonUp", "RightButtonUp")
    button:RegisterForDrag("LeftButton")

    local icon = button:CreateTexture(nil, "BACKGROUND")
    minimapIcon = icon
    NS.Controls.UpdateIcon(db.paused)
    -- Inset the artwork beneath the tracking border, whose opening is offset.
    icon:SetSize(24, 24)
    icon:SetPoint("CENTER", button, "CENTER", 1, 0)
    local mask = button:CreateMaskTexture()
    mask:SetTexture("Interface\\CHARACTERFRAME\\TempPortraitAlphaMask", "CLAMPTOBLACKADDITIVE", "CLAMPTOBLACKADDITIVE")
    mask:SetAllPoints(icon)
    icon:AddMaskTexture(mask)
    local border = button:CreateTexture(nil, "OVERLAY")
    border:SetTexture("Interface\\Minimap\\MiniMap-TrackingBorder")
    border:SetSize(54, 54)
    border:SetPoint("TOPLEFT", button, "TOPLEFT", 0, 0)
    button:SetHighlightTexture("Interface\\Minimap\\UI-Minimap-ZoomButton-Highlight")

    local function position()
        local angle = math.rad(db.minimapAngle)
        button:ClearAllPoints()
        button:SetPoint("CENTER", Minimap, "CENTER",
            math.cos(angle) * (Minimap:GetWidth()/2 + 8),
            math.sin(angle) * (Minimap:GetHeight()/2 + 8))
    end
    position()
    button:SetScript("OnDragStart", function(self)
        self.dragging = true
        GameTooltip:Hide()
        self:SetScript("OnUpdate", function()
            local x, y = GetCursorPosition()
            local cx, cy = Minimap:GetCenter()
            if not cx or not cy then return end
            local scale = Minimap:GetEffectiveScale()
            db.minimapAngle = math.deg(math.atan2(y/scale-cy, x/scale-cx))
            position()
        end)
    end)
    button:SetScript("OnDragStop", function(self)
        self:SetScript("OnUpdate", nil)
        self.dragging = false
        self.ignoreClickUntil = GetTime() + 0.1
    end)
    button:SetScript("OnClick", function(self, mouseButton)
        if self.dragging or (self.ignoreClickUntil and GetTime() < self.ignoreClickUntil) then return end
        if mouseButton == "RightButton" then openMenu(self, db)
        elseif mouseButton == "LeftButton" then
            if IsShiftKeyDown() then ForeverDubbed_StopAudio()
            else ForeverDubbed_SkipAudio() end
        end
    end)
    button:SetScript("OnEnter", function(self)
        if self.dragging then return end
        GameTooltip:SetOwner(self, "ANCHOR_LEFT")
        GameTooltip:AddLine("ForeverDubbed", 1, 0.82, 0)
        GameTooltip:AddLine("Left-click: Skip current audio", 1, 1, 1)
        GameTooltip:AddLine("Shift + left-click: Stop all audio", 1, 1, 1)
        GameTooltip:AddLine("Right-click: Menu", 1, 1, 1)
        if db.paused then GameTooltip:AddLine("Paused: new dialogue will not be read.", 1, 0.82, 0) end
        GameTooltip:AddLine("Drag to move around the minimap.", 0.6, 0.6, 0.6)
        GameTooltip:AddLine("Requires the companion app.", 0.6, 0.6, 0.6)
        GameTooltip:Show()
    end)
    button:SetScript("OnLeave", function() GameTooltip:Hide() end)
    button:Show()
end
