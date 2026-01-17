// FleetCtrl API Client

const api = {
    getToken() {
        return localStorage.getItem('token');
    },

    async request(method, url, data = null) {
        const headers = {
            'Content-Type': 'application/json',
        };

        const token = this.getToken();
        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        const options = {
            method,
            headers,
        };

        if (data && (method === 'POST' || method === 'PUT' || method === 'PATCH')) {
            options.body = JSON.stringify(data);
        }

        const response = await fetch(url, options);

        if (response.status === 401) {
            localStorage.removeItem('token');
            localStorage.removeItem('tokenExpiry');
            window.location.href = '/login';
            throw new Error('Unauthorized');
        }

        if (response.status === 204) {
            return null;
        }

        // Get raw text first for better error diagnostics
        const text = await response.text();

        let result;
        try {
            result = JSON.parse(text);
        } catch (parseError) {
            console.error('JSON parse error. Response text:', text.substring(0, 200));
            throw new Error(`Invalid JSON response: ${text.substring(0, 50)}...`);
        }

        if (!response.ok) {
            throw new Error(result.error || 'Request failed');
        }

        return result;
    },

    get(url) {
        return this.request('GET', url);
    },

    post(url, data) {
        return this.request('POST', url, data);
    },

    put(url, data) {
        return this.request('PUT', url, data);
    },

    delete(url) {
        return this.request('DELETE', url);
    }
};

// SSO: Check for token in URL (cross-host authentication)
(function() {
    const urlParams = new URLSearchParams(window.location.search);
    const ssoToken = urlParams.get('token');

    if (ssoToken) {
        // Store the SSO token
        localStorage.setItem('token', ssoToken);
        // Set expiry to 24 hours from now (will be refreshed by API)
        localStorage.setItem('tokenExpiry', new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString());

        // Remove token from URL (clean up)
        urlParams.delete('token');
        const newUrl = window.location.pathname + (urlParams.toString() ? '?' + urlParams.toString() : '');
        window.history.replaceState({}, '', newUrl);
    }
})();

// Hide auth overlay after authentication is verified
function hideAuthOverlay() {
    function doHide() {
        const overlay = document.getElementById('auth-overlay');
        if (overlay) {
            overlay.classList.add('auth-verified');
        }
    }
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', doHide);
    } else {
        doHide();
    }
}

// Check token validity on page load
(function() {
    const token = localStorage.getItem('token');
    const expiry = localStorage.getItem('tokenExpiry');
    const isLoginPage = window.location.pathname.includes('/login');

    if (token && expiry) {
        const expiryDate = new Date(expiry);
        if (expiryDate < new Date()) {
            // Token expired
            localStorage.removeItem('token');
            localStorage.removeItem('tokenExpiry');
            if (!isLoginPage) {
                window.location.href = '/login';
                // Keep overlay visible during redirect
            } else {
                hideAuthOverlay();
            }
        } else {
            // Token valid - hide overlay
            hideAuthOverlay();
        }
    } else if (!isLoginPage) {
        // No token - redirect to login
        window.location.href = '/login';
        // Keep overlay visible during redirect
    } else {
        // On login page without token - that's fine
        hideAuthOverlay();
    }
})();
