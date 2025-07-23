/**
 * Common JavaScript utilities for Airdancer web interfaces
 */

class AirdancerUI {
    constructor() {
        this.messageTimeout = null;
        this.connectionStatus = 'connecting';
    }

    /**
     * Show a temporary message to the user
     * @param {string} message - The message to display
     * @param {string} type - Message type: 'success', 'error', 'warning', 'info'
     * @param {number} duration - Duration in ms (default: 4000 for success/info, 8000 for error/warning)
     */
    showMessage(message, type = 'info', duration = null) {
        const container = document.getElementById('message-container');
        if (!container) return;

        // Clear existing timeout
        if (this.messageTimeout) {
            clearTimeout(this.messageTimeout);
        }

        // Set default duration based on type
        if (duration === null) {
            duration = (type === 'error' || type === 'warning') ? 8000 : 4000;
        }

        // Create message element
        container.innerHTML = `<div class="message ${type}">${this.escapeHtml(message)}</div>`;

        // Auto-hide message after duration
        this.messageTimeout = setTimeout(() => {
            container.innerHTML = '';
        }, duration);
    }

    /**
     * Hide any currently displayed message
     */
    hideMessage() {
        const container = document.getElementById('message-container');
        if (container) {
            container.innerHTML = '';
        }
        if (this.messageTimeout) {
            clearTimeout(this.messageTimeout);
        }
    }

    /**
     * Update the connection status indicator
     * @param {string} status - Status text ('Connected', 'Connection Error', etc.)
     */
    updateConnectionStatus(status) {
        this.connectionStatus = status.toLowerCase();
        const statusElement = document.getElementById('connection-status');
        if (!statusElement) return;

        statusElement.textContent = status;
        
        // Remove existing status classes
        statusElement.classList.remove('connected', 'error');
        
        // Add appropriate status class
        if (status === 'Connected') {
            statusElement.classList.add('connected');
        } else if (status.includes('Error') || status.includes('Failed')) {
            statusElement.classList.add('error');
        }
    }

    /**
     * Set loading state for the entire interface
     * @param {boolean} loading - Whether to show loading state
     */
    setLoading(loading) {
        const container = document.querySelector('.container');
        if (!container) return;

        if (loading) {
            container.classList.add('loading');
        } else {
            container.classList.remove('loading');
        }
    }

    /**
     * Escape HTML to prevent XSS
     * @param {string} text - Text to escape
     * @returns {string} Escaped text
     */
    escapeHtml(text) {
        const map = {
            '&': '&amp;',
            '<': '&lt;',
            '>': '&gt;',
            '"': '&quot;',
            "'": '&#039;'
        };
        return String(text).replace(/[&<>"']/g, function(m) { return map[m]; });
    }

    /**
     * Create a safe DOM ID from arbitrary text
     * @param {string} text - Text to convert to safe ID
     * @returns {string} Safe DOM ID
     */
    getSafeId(text) {
        return String(text).replace(/[^a-zA-Z0-9-_]/g, '_');
    }

    /**
     * Debounce function calls
     * @param {Function} func - Function to debounce
     * @param {number} wait - Wait time in milliseconds
     * @param {boolean} immediate - Execute immediately on first call
     * @returns {Function} Debounced function
     */
    debounce(func, wait, immediate = false) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                timeout = null;
                if (!immediate) func.apply(this, args);
            };
            const callNow = immediate && !timeout;
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
            if (callNow) func.apply(this, args);
        };
    }

    /**
     * Make an API request with error handling
     * @param {string} url - URL to fetch
     * @param {Object} options - Fetch options
     * @returns {Promise<Object>} Response data
     */
    async apiRequest(url, options = {}) {
        try {
            const response = await fetch(url, {
                headers: {
                    'Content-Type': 'application/json',
                    ...options.headers
                },
                ...options
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            this.updateConnectionStatus('Connected');
            return await response.json();
        } catch (error) {
            this.updateConnectionStatus('Connection Error');
            throw error;
        }
    }

    /**
     * Format a timestamp for display
     * @param {Date|number} timestamp - Timestamp to format
     * @returns {string} Formatted time string
     */
    formatTime(timestamp) {
        const date = timestamp instanceof Date ? timestamp : new Date(timestamp);
        return date.toLocaleTimeString();
    }

    /**
     * Check if device is mobile
     * @returns {boolean} True if mobile device
     */
    isMobile() {
        return window.innerWidth <= 767;
    }

    /**
     * Add event listener that's automatically removed on mobile for hover events
     * @param {Element} element - Element to add listener to
     * @param {string} event - Event type ('hover' for mouseenter/mouseleave)
     * @param {Function} handler - Event handler
     */
    addResponsiveEventListener(element, event, handler) {
        if (event === 'hover' && this.isMobile()) {
            // Skip hover events on mobile
            return;
        }

        if (event === 'hover') {
            element.addEventListener('mouseenter', handler);
            element.addEventListener('mouseleave', handler);
        } else {
            element.addEventListener(event, handler);
        }
    }

    /**
     * Create a toggle switch element
     * @param {string} id - Unique ID for the toggle
     * @param {boolean} checked - Initial checked state
     * @param {Function} onChange - Change handler
     * @param {boolean} disabled - Whether the toggle should be disabled
     * @returns {HTMLElement} Toggle switch element
     */
    createToggleSwitch(id, checked = false, onChange = null, disabled = false) {
        const label = document.createElement('label');
        label.className = 'toggle-switch';

        if (disabled) {
            label.classList.add('disabled');
        }

        const input = document.createElement('input');
        input.type = 'checkbox';
        input.id = id;
        input.checked = checked;
        input.disabled = disabled;

        const slider = document.createElement('span');
        slider.className = 'slider';

        label.appendChild(input);
        label.appendChild(slider);

        if (onChange && !disabled) {
            input.addEventListener('change', onChange);
        }

        return label;
    }

    /**
     * Create a card element with header
     * @param {string} title - Card title
     * @param {string} content - Card content (HTML)
     * @param {string} state - Optional state indicator
     * @returns {HTMLElement} Card element
     */
    createCard(title, content, state = null) {
        const card = document.createElement('div');
        card.className = 'card';

        const header = document.createElement('div');
        header.className = 'card-header';

        const titleElement = document.createElement('div');
        titleElement.className = 'card-title';
        titleElement.textContent = title;
        header.appendChild(titleElement);

        if (state) {
            const stateElement = document.createElement('div');
            stateElement.className = `state-indicator ${state.toLowerCase()}`;
            stateElement.textContent = state.toUpperCase();
            header.appendChild(stateElement);
        }

        card.appendChild(header);

        if (content) {
            const contentElement = document.createElement('div');
            contentElement.innerHTML = content;
            card.appendChild(contentElement);
        }

        return card;
    }
}

// CSS for toggle switches (to be included in common styles)
const toggleSwitchCSS = `
.toggle-switch {
    position: relative;
    display: inline-block;
    width: 60px;
    height: 34px;
}

.toggle-switch input {
    opacity: 0;
    width: 0;
    height: 0;
}

.slider {
    position: absolute;
    cursor: pointer;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: #ccc;
    transition: .4s;
    border-radius: 34px;
}

.slider:before {
    position: absolute;
    content: "";
    height: 26px;
    width: 26px;
    left: 4px;
    bottom: 4px;
    background-color: white;
    transition: .4s;
    border-radius: 50%;
}

input:checked + .slider {
    background-color: #4CAF50;
}

input:focus + .slider {
    box-shadow: 0 0 1px #4CAF50;
}

input:checked + .slider:before {
    transform: translateX(26px);
}

@media (max-width: 767px) {
    .slider:before {
        height: 28px;
        width: 28px;
    }
}
`;

// Export for use in modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = AirdancerUI;
}