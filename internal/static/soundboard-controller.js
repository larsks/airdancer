/**
 * Soundboard Controller for Airdancer Soundboard Interface
 */

class SoundboardController extends AirdancerUI {
    constructor(baseURL) {
        super();
        this.baseURL = baseURL;
        this.currentPage = 1;
        this.totalPages = 1;
        this.perPage = 20;
        this.sounds = [];
        this.currentAudio = null;
        this.currentPlayingButton = null;
        this.serverAvailable = false;
        this.statusCheckInterval = null;
        this.volume = 70;
        this.userAdjustingVolume = false;
        this.lastSoundCount = 0;
        this.soundUpdateInterval = null;
        this.playbackMode = 'browser';
        
        this.initializePlaybackMode();
        this.setupEventListeners();
        this.checkServerStatus();
        this.loadSounds();
        this.startSoundUpdateChecking();
    }

    buildURL(path) {
        if (this.baseURL) {
            return this.baseURL + path;
        }
        return path;
    }

    initializePlaybackMode() {
        const playbackModeSelect = document.getElementById('playbackMode');
        if (!playbackModeSelect) return;
        
        const savedMode = localStorage.getItem('soundboard-playback-mode');
        if (savedMode && (savedMode === 'browser' || savedMode === 'server')) {
            this.playbackMode = savedMode;
            playbackModeSelect.value = savedMode;
        } else {
            this.playbackMode = playbackModeSelect.value || 'browser';
        }
        
        this.initializeModeFeatures();
    }
    
    initializeModeFeatures() {
        if (this.playbackMode === 'server') {
            setTimeout(() => {
                if (this.serverAvailable) {
                    this.startStatusPolling();
                    this.syncServerVolume();
                }
            }, 100);
        }
    }

    setupEventListeners() {
        // Pagination
        const prevBtn = document.getElementById('prevPage');
        const nextBtn = document.getElementById('nextPage');
        const perPageSelect = document.getElementById('perPage');
        const playbackModeSelect = document.getElementById('playbackMode');
        const volumeSlider = document.getElementById('volumeSlider');

        if (prevBtn) {
            prevBtn.addEventListener('click', () => {
                if (this.currentPage > 1) {
                    this.currentPage--;
                    this.loadSounds();
                }
            });
        }

        if (nextBtn) {
            nextBtn.addEventListener('click', () => {
                if (this.currentPage < this.totalPages) {
                    this.currentPage++;
                    this.loadSounds();
                }
            });
        }

        if (perPageSelect) {
            perPageSelect.addEventListener('change', (e) => {
                this.perPage = parseInt(e.target.value);
                this.currentPage = 1;
                this.loadSounds();
            });
        }

        if (playbackModeSelect) {
            playbackModeSelect.addEventListener('change', (e) => {
                this.playbackMode = e.target.value;
                localStorage.setItem('soundboard-playback-mode', this.playbackMode);
                this.stopCurrentSound();
                
                if (this.playbackMode === 'server') {
                    this.startStatusPolling();
                    this.syncServerVolume();
                } else {
                    this.stopStatusPolling();
                }
            });
        }

        if (volumeSlider) {
            volumeSlider.addEventListener('input', (e) => {
                this.userAdjustingVolume = true;
                this.volume = parseInt(e.target.value);
                this.updateVolumeDisplay();
                this.applyVolumeChange();
                
                if (this.volumeDebounceTimeout) {
                    clearTimeout(this.volumeDebounceTimeout);
                }
                
                this.volumeDebounceTimeout = setTimeout(() => {
                    this.userAdjustingVolume = false;
                }, 500);
            });
        }
    }

    async checkServerStatus() {
        try {
            const data = await this.apiRequest(this.buildURL('/api/audio/info'));
            this.serverAvailable = data.serverAvailable;
            this.updateServerStatus(data);
        } catch (error) {
            console.warn('Failed to check server audio status:', error);
            this.serverAvailable = false;
            this.updateServerStatus(null);
            this.showMessage('Unable to connect to server: ' + error.message, 'error');
        }
    }

    updateServerStatus(audioInfo) {
        const statusElement = document.getElementById('serverStatus');
        const playbackModeSelect = document.getElementById('playbackMode');
        
        if (!statusElement || !playbackModeSelect) return;
        
        if (this.serverAvailable) {
            statusElement.textContent = '✓ Available';
            statusElement.className = 'server-status available';
            playbackModeSelect.disabled = false;
        } else {
            statusElement.textContent = '✗ Unavailable';
            statusElement.className = 'server-status unavailable';
            playbackModeSelect.disabled = false;
            if (this.playbackMode === 'server') {
                this.playbackMode = 'browser';
                playbackModeSelect.value = 'browser';
            }
        }

        if (audioInfo && audioInfo.availablePlayers) {
            const players = Object.entries(audioInfo.availablePlayers)
                .filter(([, available]) => available)
                .map(([name]) => name);
            
            if (players.length > 0) {
                statusElement.title = `Available players: ${players.join(', ')}`;
            }
        }
    }

    startStatusPolling() {
        this.stopStatusPolling();
        this.statusCheckInterval = setInterval(() => {
            this.checkServerPlaybackStatus();
            this.syncServerVolume();
        }, 1000);
    }

    stopStatusPolling() {
        if (this.statusCheckInterval) {
            clearInterval(this.statusCheckInterval);
            this.statusCheckInterval = null;
        }
    }

    async syncServerVolume() {
        if (this.playbackMode === 'server' && !this.userAdjustingVolume) {
            try {
                const data = await this.apiRequest(this.buildURL('/api/audio/info'));
                if (data.volumeSuccess && data.volume !== undefined) {
                    if (this.volume !== data.volume) {
                        this.volume = data.volume;
                        const volumeSlider = document.getElementById('volumeSlider');
                        if (volumeSlider) {
                            volumeSlider.value = this.volume;
                        }
                        this.updateVolumeDisplay();
                    }
                }
            } catch (error) {
                console.warn('Failed to sync server volume:', error);
            }
        }
    }

    updateVolumeDisplay() {
        const volumeValue = document.getElementById('volumeValue');
        if (volumeValue) {
            volumeValue.textContent = this.volume + '%';
        }
    }

    async applyVolumeChange() {
        if (this.playbackMode === 'browser') {
            if (this.currentAudio) {
                this.currentAudio.volume = this.volume / 100;
            }
        } else {
            try {
                await this.apiRequest(this.buildURL('/api/audio/volume'), {
                    method: 'POST',
                    body: JSON.stringify({ volume: this.volume })
                });
            } catch (error) {
                console.warn('Failed to set server volume:', error);
            }
        }
    }

    async checkServerPlaybackStatus() {
        if (this.playbackMode !== 'server') {
            return;
        }
        
        try {
            const data = await this.apiRequest(this.buildURL('/api/audio/info'));
            
            if (!data.isPlaying && this.currentPlayingButton) {
                this.currentPlayingButton.classList.remove('playing');
                this.currentPlayingButton = null;
            }
            
            if (data.lastError) {
                console.warn('Server audio error:', data.lastError);
                if (this.currentPlayingButton) {
                    this.currentPlayingButton.classList.remove('playing');
                    this.currentPlayingButton = null;
                }
            }
        } catch (error) {
            console.warn('Failed to check server playback status:', error);
        }
    }

    async loadSounds() {
        try {
            this.stopCurrentSound();
            this.setLoading(true);
            this.hideMessage();
            
            const data = await this.apiRequest(this.buildURL(`/api/sounds?page=${this.currentPage}&per_page=${this.perPage}`));
            
            this.sounds = data.sounds;
            this.currentPage = data.currentPage;
            this.totalPages = data.totalPages;
            this.perPage = data.itemsPerPage;
            
            this.renderSounds();
            this.updatePagination();
            
        } catch (error) {
            console.error('Error loading sounds:', error);
            this.showMessage(`Failed to load sounds: ${error.message}`, 'error');
            this.renderSounds();
        } finally {
            this.setLoading(false);
        }
    }

    renderSounds() {
        const soundboard = document.getElementById('soundboard');
        if (!soundboard) return;
        
        soundboard.innerHTML = '';

        if (this.sounds.length === 0) {
            this.showMessage('No sounds found. Add some audio files to the sounds directory.', 'warning');
            return;
        }

        this.sounds.forEach(sound => {
            const button = document.createElement('button');
            button.className = 'sound-button';
            button.textContent = sound.displayName;
            button.title = `Play/Stop ${sound.displayName}`;
            
            button.addEventListener('click', () => {
                this.playSound(sound, button);
            });
            
            soundboard.appendChild(button);
        });
    }

    stopCurrentSound() {
        if (this.currentAudio) {
            this.currentAudio.pause();
            this.currentAudio.currentTime = 0;
            this.currentAudio = null;
        }
        
        fetch(this.buildURL('/api/sounds/stop'), {
            method: 'POST'
        }).catch(err => console.warn('Failed to stop server audio:', err));
        
        if (this.currentPlayingButton) {
            this.currentPlayingButton.classList.remove('playing');
            this.currentPlayingButton = null;
        }
    }

    async playSound(sound, button) {
        try {
            if (this.currentPlayingButton === button) {
                this.stopCurrentSound();
                return;
            }
            
            this.stopCurrentSound();
            
            button.classList.add('playing');
            this.currentPlayingButton = button;
            
            if (this.playbackMode === 'server') {
                const result = await this.apiRequest(this.buildURL(`/api/sounds/${sound.fileName}/play?mode=server`), {
                    method: 'POST'
                });
                
                console.log('Server playback result:', result);
                
                if (result.error || !result.serverPlayback) {
                    throw new Error(result.error || 'Server playback failed to start');
                }
                
                if (!this.statusCheckInterval) {
                    this.startStatusPolling();
                }
                
            } else {
                const audio = new Audio(this.buildURL(`/sounds/${sound.fileName}`));
                audio.volume = this.volume / 100;
                this.currentAudio = audio;
                
                audio.addEventListener('ended', () => {
                    this.stopCurrentSound();
                });
                
                audio.addEventListener('error', (e) => {
                    console.error('Audio error:', e);
                    this.stopCurrentSound();
                    this.showMessage('Failed to play sound: ' + sound.displayName, 'error');
                });
                
                await audio.play();
                
                fetch(this.buildURL(`/api/sounds/${sound.fileName}/play?mode=browser`), {
                    method: 'POST'
                }).catch(err => console.warn('Failed to log play event:', err));
            }
            
        } catch (error) {
            console.error('Error playing sound:', error);
            this.stopCurrentSound();
            this.showMessage(`Failed to play sound '${sound.displayName}': ${error.message}`, 'error');
        }
    }

    updatePagination() {
        const prevButton = document.getElementById('prevPage');
        const nextButton = document.getElementById('nextPage');
        const pageInfo = document.getElementById('pageInfo');
        const perPageSelect = document.getElementById('perPage');

        if (prevButton) prevButton.disabled = this.currentPage <= 1;
        if (nextButton) nextButton.disabled = this.currentPage >= this.totalPages;
        if (pageInfo) pageInfo.textContent = `Page ${this.currentPage} of ${this.totalPages}`;
        if (perPageSelect) perPageSelect.value = this.perPage.toString();
    }

    startSoundUpdateChecking() {
        this.soundUpdateInterval = setInterval(() => {
            this.checkForSoundUpdates();
        }, 5000);
    }

    stopSoundUpdateChecking() {
        if (this.soundUpdateInterval) {
            clearInterval(this.soundUpdateInterval);
            this.soundUpdateInterval = null;
        }
    }

    async checkForSoundUpdates() {
        try {
            const data = await this.apiRequest(this.buildURL('/api/sounds/status'));
            
            if (this.lastSoundCount > 0 && this.lastSoundCount !== data.soundCount) {
                console.log('Sound directory updated: ' + this.lastSoundCount + ' -> ' + data.soundCount + ' sounds');
                this.showMessage('Sound directory updated: ' + data.soundCount + ' sounds available', 'success');
                this.loadSounds();
            }
            
            this.lastSoundCount = data.soundCount;
        } catch (error) {
            console.warn('Failed to check for sound updates:', error);
        }
    }
}