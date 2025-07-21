/**
 * Switch Controller for Airdancer Switch Control Interface
 */

class SwitchController extends AirdancerUI {
    constructor(apiBaseURL) {
        super();
        this.apiBaseURL = apiBaseURL;
        this.switches = {};
        this.switchNames = [];
        this.groups = {};
        this.groupNames = [];
        this.updateInterval = null;
        this.switchCount = 0;
        this.lastUpdated = null;
        this.init();
    }

    async init() {
        await this.loadSwitches();
        this.setupEventListeners();
        this.startAutoUpdate();
        this.updateConnectionStatus('Connected');
    }

    async loadSwitches() {
        try {
            this.setLoading(true);
            const data = await this.apiRequest(`${this.apiBaseURL}/switch/all`);
            
            if (data.status !== 'ok') {
                throw new Error(data.message || 'Failed to load switches');
            }

            this.switchCount = data.data.count;
            this.switches = {};
            this.switchNames = [];
            this.groups = {};
            this.groupNames = [];
            
            // Convert new map-based response to our internal structure
            for (const [switchName, switchData] of Object.entries(data.data.switches)) {
                this.switches[switchName] = switchData;
                this.switchNames.push(switchName);
            }
            
            // Process groups if they exist
            if (data.data.groups) {
                for (const [groupName, groupData] of Object.entries(data.data.groups)) {
                    this.groups[groupName] = groupData;
                    this.groupNames.push(groupName);
                }
            }
            
            this.renderSwitches();
            this.renderGroups();
            this.updateLastUpdated();
            this.showMessage('Switches loaded successfully', 'success');
        } catch (error) {
            console.error('Failed to load switches:', error);
            this.showMessage(`Failed to load switches: ${error.message}`, 'error');
        } finally {
            this.setLoading(false);
        }
    }

    async updateSwitches() {
        try {
            const data = await this.apiRequest(`${this.apiBaseURL}/switch/all`);
            if (data.status !== 'ok') {
                throw new Error(data.message || 'Failed to update switches');
            }

            const newCount = data.data.count;
            const newSwitchNames = Object.keys(data.data.switches);
            const newGroupNames = data.data.groups ? Object.keys(data.data.groups) : [];
            
            // Check if the switch or group configuration has changed
            if (newCount !== this.switchCount || 
                !this.arraysEqual(newSwitchNames.sort(), this.switchNames.sort()) ||
                !this.arraysEqual(newGroupNames.sort(), this.groupNames.sort())) {
                
                this.switchCount = newCount;
                this.switches = {};
                this.switchNames = [];
                this.groups = {};
                this.groupNames = [];
                
                // Reload all switches
                for (const [switchName, switchData] of Object.entries(data.data.switches)) {
                    this.switches[switchName] = switchData;
                    this.switchNames.push(switchName);
                }
                
                // Reload all groups
                if (data.data.groups) {
                    for (const [groupName, groupData] of Object.entries(data.data.groups)) {
                        this.groups[groupName] = groupData;
                        this.groupNames.push(groupName);
                    }
                }
                
                this.renderSwitches();
                this.renderGroups();
                this.showMessage(`Configuration changed: ${this.switchCount} switches, ${this.groupNames.length} groups`, 'success');
            } else {
                // Update individual switch states
                for (const [switchName, switchData] of Object.entries(data.data.switches)) {
                    if (JSON.stringify(this.switches[switchName]) !== JSON.stringify(switchData)) {
                        this.switches[switchName] = switchData;
                        this.updateSwitchUI(switchName, switchData);
                    }
                }
                
                // Update group states
                if (data.data.groups) {
                    for (const [groupName, groupData] of Object.entries(data.data.groups)) {
                        if (JSON.stringify(this.groups[groupName]) !== JSON.stringify(groupData)) {
                            this.groups[groupName] = groupData;
                            this.updateGroupUI(groupName, groupData);
                        }
                    }
                }
            }
            this.updateLastUpdated();
        } catch (error) {
            console.error('Failed to update switches:', error);
        }
    }

    renderSwitches() {
        const container = document.getElementById('switches-container');
        if (!container) return;
        
        container.innerHTML = '';

        for (const switchName of this.switchNames) {
            const switchCard = this.createSwitchCard(switchName, this.switches[switchName]);
            container.appendChild(switchCard);
        }
    }

    renderGroups() {
        const groupsSection = document.getElementById('groups-section');
        const container = document.getElementById('groups-container');
        
        if (!groupsSection || !container) return;
        
        if (this.groupNames.length === 0) {
            groupsSection.style.display = 'none';
            return;
        }

        groupsSection.style.display = 'block';
        container.innerHTML = '';

        for (const groupName of this.groupNames) {
            const groupCard = this.createGroupCard(groupName, this.groups[groupName]);
            container.appendChild(groupCard);
        }
    }
    
    arraysEqual(a, b) {
        return a.length === b.length && a.every((val, index) => val === b[index]);
    }

    createSwitchCard(switchName, switchData) {
        const card = document.createElement('div');
        card.className = 'card';
        const safeId = this.getSafeId(switchName);
        const stateText = switchData.state.toUpperCase();
        
        const toggle = this.createToggleSwitch(`switch-${safeId}`, switchData.currentState, (e) => {
            this.toggleSwitch(switchName, e.target.checked);
        });
        
        card.innerHTML = `
            <div class="card-header">
                <div class="card-title">${this.escapeHtml(switchName)}</div>
                <div class="state-indicator ${switchData.state}" id="state-${safeId}">
                    ${stateText}
                </div>
            </div>
        `;
        
        card.appendChild(toggle);
        return card;
    }

    createGroupCard(groupName, groupData) {
        const card = document.createElement('div');
        card.className = 'card';
        const safeId = this.getSafeId(groupName);
        
        const stateText = groupData.state.toUpperCase();
        const switchesList = groupData.switches.join(', ');
        const isGroupOn = groupData.summary;
        
        const toggle = this.createToggleSwitch(`group-toggle-${safeId}`, isGroupOn, (e) => {
            this.controlGroup(groupName, e.target.checked);
        });
        
        card.innerHTML = `
            <div class="card-header">
                <div class="card-title">${this.escapeHtml(groupName)}</div>
                <div class="state-indicator ${groupData.state}" id="group-state-${safeId}">
                    ${stateText}
                </div>
            </div>
            <div style="margin-bottom: 15px;">
                <div style="font-size: 0.9em; color: #6c757d; margin-bottom: 5px;">Switches:</div>
                <div style="font-size: 0.9em; color: #495057; font-style: italic;" id="group-switches-${safeId}">
                    ${this.escapeHtml(switchesList)}
                </div>
            </div>
        `;
        
        card.appendChild(toggle);
        return card;
    }

    async toggleSwitch(switchName, state) {
        try {
            this.setLoading(true);
            const data = await this.apiRequest(`${this.apiBaseURL}/switch/${encodeURIComponent(switchName)}`, {
                method: 'POST',
                body: JSON.stringify({
                    state: state ? 'on' : 'off'
                })
            });
            
            if (data.status !== 'ok') {
                throw new Error(data.message || 'Failed to toggle switch');
            }

            // Create a switch data object for UI update
            const switchData = {
                state: state ? 'on' : 'off',
                currentState: state
            };
            this.switches[switchName] = switchData;
            this.updateSwitchUI(switchName, switchData);
            this.updateLastUpdated();
            
        } catch (error) {
            console.error(`Failed to toggle switch ${switchName}:`, error);
            this.showMessage(`Failed to toggle switch ${switchName}: ${error.message}`, 'error');
            const safeId = this.getSafeId(switchName);
            const checkbox = document.getElementById(`switch-${safeId}`);
            if (checkbox) {
                checkbox.checked = !state;
            }
        } finally {
            this.setLoading(false);
        }
    }

    async controlAllSwitches(state) {
        try {
            this.setLoading(true);
            const data = await this.apiRequest(`${this.apiBaseURL}/switch/all`, {
                method: 'POST',
                body: JSON.stringify({
                    state: state ? 'on' : 'off'
                })
            });
            
            if (data.status !== 'ok') {
                throw new Error(data.message || 'Failed to control all switches');
            }

            for (const switchName of this.switchNames) {
                const switchData = {
                    state: state ? 'on' : 'off',
                    currentState: state
                };
                this.switches[switchName] = switchData;
                this.updateSwitchUI(switchName, switchData);
            }
            
            this.updateLastUpdated();
            this.showMessage(`All switches turned ${state ? 'on' : 'off'}`, 'success');
            
        } catch (error) {
            console.error('Failed to control all switches:', error);
            this.showMessage(`Failed to control all switches: ${error.message}`, 'error');
        } finally {
            this.setLoading(false);
        }
    }

    async controlGroup(groupName, state) {
        try {
            this.setLoading(true);
            const data = await this.apiRequest(`${this.apiBaseURL}/switch/${encodeURIComponent(groupName)}`, {
                method: 'POST',
                body: JSON.stringify({
                    state: state ? 'on' : 'off'
                })
            });
            
            if (data.status !== 'ok') {
                throw new Error(data.message || 'Failed to control group');
            }

            this.updateLastUpdated();
            this.showMessage(`Group ${groupName} turned ${state ? 'on' : 'off'}`, 'success');
            
            // Update the affected switches in the UI
            for (const switchName of this.groups[groupName].switches) {
                const switchData = {
                    state: state ? 'on' : 'off',
                    currentState: state
                };
                if (JSON.stringify(this.switches[switchName]) !== JSON.stringify(switchData)) {
                    this.switches[switchName] = switchData;
                    this.updateSwitchUI(switchName, switchData);
                }
            }
            
        } catch (error) {
            console.error(`Failed to control group ${groupName}:`, error);
            this.showMessage(`Failed to control group ${groupName}: ${error.message}`, 'error');
            // Revert the toggle switch on error
            const safeId = this.getSafeId(groupName);
            const toggleCheckbox = document.getElementById(`group-toggle-${safeId}`);
            if (toggleCheckbox) {
                toggleCheckbox.checked = !state;
            }
        } finally {
            this.setLoading(false);
        }
    }

    updateGroupUI(groupName, groupData) {
        const safeId = this.getSafeId(groupName);
        const stateLabel = document.getElementById(`group-state-${safeId}`);
        const switchesList = document.getElementById(`group-switches-${safeId}`);
        const toggleCheckbox = document.getElementById(`group-toggle-${safeId}`);
        
        if (stateLabel) {
            const stateText = groupData.state.toUpperCase();
            stateLabel.textContent = stateText;
            stateLabel.className = `state-indicator ${groupData.state}`;
        }
        
        if (switchesList) {
            switchesList.textContent = groupData.switches.join(', ');
        }
        
        if (toggleCheckbox) {
            toggleCheckbox.checked = groupData.summary;
        }
    }

    updateSwitchUI(switchName, switchData) {
        const safeId = this.getSafeId(switchName);
        const checkbox = document.getElementById(`switch-${safeId}`);
        const stateLabel = document.getElementById(`state-${safeId}`);
        
        if (checkbox) {
            checkbox.checked = switchData.currentState;
        }
        
        if (stateLabel) {
            const stateText = switchData.state.toUpperCase();
            stateLabel.textContent = stateText;
            stateLabel.className = `state-indicator ${switchData.state}`;
        }
    }

    setupEventListeners() {
        const allOnBtn = document.getElementById('all-on-btn');
        const allOffBtn = document.getElementById('all-off-btn');
        
        if (allOnBtn) {
            allOnBtn.addEventListener('click', () => {
                this.controlAllSwitches(true);
            });
        }

        if (allOffBtn) {
            allOffBtn.addEventListener('click', () => {
                this.controlAllSwitches(false);
            });
        }
    }

    startAutoUpdate() {
        this.updateInterval = setInterval(() => {
            this.updateSwitches();
        }, 2000);
    }

    updateLastUpdated() {
        this.lastUpdated = new Date();
        const element = document.getElementById('last-updated');
        if (element) {
            element.textContent = `Last updated: ${this.formatTime(this.lastUpdated)}`;
        }
    }
}