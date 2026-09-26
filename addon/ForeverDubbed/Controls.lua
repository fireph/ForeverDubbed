local _, NS = ...
NS.Controls = {}

BINDING_HEADER_FOREVERDUBBED = "ForeverDubbed"
BINDING_NAME_FOREVERDUBBED_STOP = "Stop all audio"
BINDING_NAME_FOREVERDUBBED_SKIP = "Skip current audio"

-- Bindings.xml calls these globals; the transport is initialized at ADDON_LOADED.
function ForeverDubbed_StopAudio() if NS.Control then NS.Control("stop") end end
function ForeverDubbed_SkipAudio() if NS.Control then NS.Control("skip") end end

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
    icon:SetTexture("Interface\\Icons\\INV_Misc_Book_09")
    icon:SetSize(20, 20)
    icon:SetPoint("CENTER", button, "CENTER", 0, 0)
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
        if mouseButton == "RightButton" then ForeverDubbed_StopAudio()
        else ForeverDubbed_SkipAudio() end
    end)
    button:SetScript("OnEnter", function(self)
        if self.dragging then return end
        GameTooltip:SetOwner(self, "ANCHOR_LEFT")
        GameTooltip:AddLine("ForeverDubbed", 1, 0.82, 0)
        GameTooltip:AddLine("Left-click: Skip current audio", 1, 1, 1)
        GameTooltip:AddLine("Right-click: Stop all audio", 1, 1, 1)
        GameTooltip:AddLine("Drag to move around the minimap.", 0.8, 0.8, 0.8)
        GameTooltip:AddLine("Requires the companion and visible data square.", 0.8, 0.8, 0.8)
        GameTooltip:Show()
    end)
    button:SetScript("OnLeave", function() GameTooltip:Hide() end)
    button:Show()
end
