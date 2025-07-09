// Portier CLI Setup Wizard JavaScript

class PortierWizard {
    constructor() {
        this.currentStep = 'welcome';
        this.completedSteps = new Set();
        this.ws = null;
        this.qrCode = null;
        
        this.initializeWebSocket();
        this.initializeEventListeners();
        this.initializeQRCode();
    }
    
    initializeWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        this.ws = new WebSocket(wsUrl);
        
        this.ws.onopen = () => {
            console.log('WebSocket connected');
        };
        
        this.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            this.handleWebSocketMessage(message);
        };
        
        this.ws.onclose = () => {
            console.log('WebSocket disconnected');
            // Try to reconnect after a delay
            setTimeout(() => this.initializeWebSocket(), 3000);
        };
        
        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };
    }
    
    initializeEventListeners() {
        // Welcome step
        document.getElementById('btn-start-setup').addEventListener('click', () => {
            this.goToStep('login');
        });
        
        // Login step
        document.getElementById('btn-start-login').addEventListener('click', () => {
            this.startLogin();
        });
        
        document.getElementById('btn-retry-login').addEventListener('click', () => {
            this.resetLoginStep();
            this.startLogin();
        });
        
        document.getElementById('btn-copy-url').addEventListener('click', () => {
            this.copyVerificationUrl();
        });
        
        document.getElementById('btn-prev-login').addEventListener('click', () => {
            this.goToStep('welcome');
        });
        
        document.getElementById('btn-next-login').addEventListener('click', () => {
            this.goToStep('register');
        });
        
        // Register step
        document.getElementById('btn-register-device').addEventListener('click', () => {
            this.registerDevice();
        });
        
        document.getElementById('btn-retry-register').addEventListener('click', () => {
            this.resetRegisterStep();
        });
        
        document.getElementById('btn-prev-register').addEventListener('click', () => {
            this.goToStep('login');
        });
        
        document.getElementById('btn-next-register').addEventListener('click', () => {
            this.goToStep('service');
        });
        
        // Service step
        document.getElementById('btn-install-service').addEventListener('click', () => {
            this.installService();
        });
        
        document.getElementById('btn-start-service').addEventListener('click', () => {
            this.startService();
        });
        
        document.getElementById('btn-retry-service').addEventListener('click', () => {
            this.resetServiceStep();
        });
        
        document.getElementById('btn-prev-service').addEventListener('click', () => {
            this.goToStep('register');
        });
        
        document.getElementById('btn-next-service').addEventListener('click', () => {
            this.goToStep('complete');
        });
        
        // Complete step
        document.getElementById('btn-close-wizard').addEventListener('click', () => {
            this.closeWizard();
        });
        
        // Console toggle
        document.getElementById('btn-toggle-console').addEventListener('click', () => {
            this.toggleConsole();
        });
        
        // Device name validation
        document.getElementById('device-name').addEventListener('input', () => {
            this.validateDeviceName();
        });
        
        // Enter key handling
        document.getElementById('device-name').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                this.registerDevice();
            }
        });
    }
    
    initializeQRCode() {
        // QR code will be initialized when needed
        this.qrCode = null;
    }
    
    handleWebSocketMessage(message) {
        console.log('Received message:', message);
        
        switch (message.type) {
            case 'step':
                this.goToStep(message.step);
                break;
                
            case 'loginResult':
                this.handleLoginResult(message);
                break;
                
            case 'registerResult':
                this.handleRegisterResult(message);
                break;
                
            case 'serviceInstallResult':
                this.handleServiceInstallResult(message);
                break;
                
            case 'serviceStartResult':
                this.handleServiceStartResult(message);
                break;
                
            case 'console':
                this.appendConsoleOutput(message.data);
                break;
                
            default:
                console.log('Unknown message type:', message.type);
        }
    }
    
    goToStep(stepName) {
        // Hide all steps
        document.querySelectorAll('.step').forEach(step => {
            step.classList.remove('active');
        });
        
        // Show target step
        document.getElementById(`step-${stepName}`).classList.add('active');
        
        // Update progress bar
        this.updateProgressBar(stepName);
        
        this.currentStep = stepName;
    }
    
    updateProgressBar(currentStep) {
        document.querySelectorAll('.progress-step').forEach(step => {
            step.classList.remove('active', 'completed');
        });
        
        const stepOrder = ['welcome', 'login', 'register', 'service', 'complete'];
        const currentIndex = stepOrder.indexOf(currentStep);
        
        stepOrder.forEach((stepName, index) => {
            const stepElement = document.querySelector(`[data-step="${stepName}"]`);
            if (index < currentIndex) {
                stepElement.classList.add('completed');
            } else if (index === currentIndex) {
                stepElement.classList.add('active');
            }
        });
    }
    
    startLogin() {
        this.showLoginProgress();
        
        fetch('/api/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            }
        })
        .then(response => response.json())
        .then(data => {
            console.log('Login started:', data);
            // The actual login process will be handled via WebSocket messages
        })
        .catch(error => {
            console.error('Failed to start login:', error);
            this.handleLoginResult({ error: 'Failed to start login process' });
        });
    }
    
    showLoginProgress() {
        document.getElementById('login-form').classList.add('hidden');
        document.getElementById('login-qr').classList.add('hidden');
        document.getElementById('login-success').classList.add('hidden');
        document.getElementById('login-error').classList.add('hidden');
        document.getElementById('login-progress').classList.remove('hidden');
    }
    
    showLoginQR(loginData) {
        document.getElementById('login-progress').classList.add('hidden');
        
        // Generate QR code
        const qrContainer = document.getElementById('qrcode');
        qrContainer.innerHTML = ''; // Clear existing QR code
        
        if (typeof QRCode !== 'undefined') {
            this.qrCode = new QRCode(qrContainer, {
                text: loginData.verificationUrlComplete,
                width: 200,
                height: 200,
                colorDark: '#000000',
                colorLight: '#ffffff',
            });
        }
        
        // Set verification URL
        document.getElementById('verification-url').value = loginData.verificationUrlComplete;
        
        document.getElementById('login-qr').classList.remove('hidden');
    }
    
    handleLoginResult(message) {
        if (message.success) {
            document.getElementById('login-qr').classList.add('hidden');
            document.getElementById('login-success').classList.remove('hidden');
            document.getElementById('btn-next-login').disabled = false;
            this.completedSteps.add('login');
        } else if (message.error) {
            document.getElementById('login-progress').classList.add('hidden');
            document.getElementById('login-qr').classList.add('hidden');
            document.getElementById('login-error-text').textContent = message.error;
            document.getElementById('login-error').classList.remove('hidden');
        } else if (message.data) {
            // Show QR code with login data
            this.showLoginQR(message.data);
        }
    }
    
    resetLoginStep() {
        document.getElementById('login-progress').classList.add('hidden');
        document.getElementById('login-qr').classList.add('hidden');
        document.getElementById('login-success').classList.add('hidden');
        document.getElementById('login-error').classList.add('hidden');
        document.getElementById('login-form').classList.remove('hidden');
        document.getElementById('btn-next-login').disabled = true;
    }
    
    copyVerificationUrl() {
        const urlInput = document.getElementById('verification-url');
        urlInput.select();
        urlInput.setSelectionRange(0, 99999); // For mobile devices
        
        try {
            document.execCommand('copy');
            // Show brief feedback
            const btn = document.getElementById('btn-copy-url');
            const originalText = btn.textContent;
            btn.textContent = 'Copied!';
            setTimeout(() => {
                btn.textContent = originalText;
            }, 2000);
        } catch (err) {
            console.error('Failed to copy URL:', err);
        }
    }
    
    validateDeviceName() {
        const input = document.getElementById('device-name');
        const btn = document.getElementById('btn-register-device');
        
        if (input.value.trim().length > 0) {
            btn.disabled = false;
        } else {
            btn.disabled = true;
        }
    }
    
    registerDevice() {
        const deviceName = document.getElementById('device-name').value.trim();
        
        if (!deviceName) {
            alert('Please enter a device name');
            return;
        }
        
        this.showRegisterProgress();
        
        fetch('/api/register', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ deviceName })
        })
        .then(response => response.json())
        .then(data => {
            console.log('Registration started:', data);
            // The actual registration process will be handled via WebSocket messages
        })
        .catch(error => {
            console.error('Failed to start registration:', error);
            this.handleRegisterResult({ error: 'Failed to start registration process' });
        });
    }
    
    showRegisterProgress() {
        document.getElementById('register-form').classList.add('hidden');
        document.getElementById('register-success').classList.add('hidden');
        document.getElementById('register-error').classList.add('hidden');
        document.getElementById('register-progress').classList.remove('hidden');
    }
    
    handleRegisterResult(message) {
        document.getElementById('register-progress').classList.add('hidden');
        
        if (message.success) {
            document.getElementById('register-success').classList.remove('hidden');
            document.getElementById('btn-next-register').disabled = false;
            this.completedSteps.add('register');
        } else if (message.error) {
            document.getElementById('register-error-text').textContent = message.error;
            document.getElementById('register-error').classList.remove('hidden');
        }
    }
    
    resetRegisterStep() {
        document.getElementById('register-progress').classList.add('hidden');
        document.getElementById('register-success').classList.add('hidden');
        document.getElementById('register-error').classList.add('hidden');
        document.getElementById('register-form').classList.remove('hidden');
        document.getElementById('btn-next-register').disabled = true;
    }
    
    installService() {
        this.showServiceInstallProgress();
        
        fetch('/api/service/install', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            }
        })
        .then(response => response.json())
        .then(data => {
            console.log('Service installation started:', data);
        })
        .catch(error => {
            console.error('Failed to start service installation:', error);
            this.handleServiceInstallResult({ error: 'Failed to start service installation' });
        });
    }
    
    startService() {
        this.showServiceStartProgress();
        
        fetch('/api/service/start', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            }
        })
        .then(response => response.json())
        .then(data => {
            console.log('Service start initiated:', data);
        })
        .catch(error => {
            console.error('Failed to start service:', error);
            this.handleServiceStartResult({ error: 'Failed to start service' });
        });
    }
    
    showServiceInstallProgress() {
        document.getElementById('service-form').classList.add('hidden');
        document.getElementById('service-success').classList.add('hidden');
        document.getElementById('service-error').classList.add('hidden');
        document.getElementById('service-start-progress').classList.add('hidden');
        document.getElementById('service-install-progress').classList.remove('hidden');
    }
    
    showServiceStartProgress() {
        document.getElementById('service-install-progress').classList.add('hidden');
        document.getElementById('service-start-progress').classList.remove('hidden');
    }
    
    handleServiceInstallResult(message) {
        document.getElementById('service-install-progress').classList.add('hidden');
        
        if (message.success) {
            // Enable start service button
            document.getElementById('btn-start-service').disabled = false;
            // Auto-start the service
            this.startService();
        } else if (message.error) {
            document.getElementById('service-error-text').textContent = message.error;
            document.getElementById('service-error').classList.remove('hidden');
        }
    }
    
    handleServiceStartResult(message) {
        document.getElementById('service-start-progress').classList.add('hidden');
        
        if (message.success) {
            document.getElementById('service-success').classList.remove('hidden');
            document.getElementById('btn-next-service').disabled = false;
            this.completedSteps.add('service');
        } else if (message.error) {
            document.getElementById('service-error-text').textContent = message.error;
            document.getElementById('service-error').classList.remove('hidden');
        }
    }
    
    resetServiceStep() {
        document.getElementById('service-install-progress').classList.add('hidden');
        document.getElementById('service-start-progress').classList.add('hidden');
        document.getElementById('service-success').classList.add('hidden');
        document.getElementById('service-error').classList.add('hidden');
        document.getElementById('service-form').classList.remove('hidden');
        document.getElementById('btn-install-service').disabled = false;
        document.getElementById('btn-start-service').disabled = true;
        document.getElementById('btn-next-service').disabled = true;
    }
    
    closeWizard() {
        if (this.ws) {
            this.ws.send(JSON.stringify({ type: 'exit' }));
        }
        
        // Show a brief message before closing
        alert('Setup wizard completed! You can now close this window.');
        
        // Try to close the window
        if (window.opener) {
            window.close();
        } else {
            // If we can't close the window, show instructions
            document.body.innerHTML = `
                <div style="text-align: center; padding: 50px; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;">
                    <h2 style="color: #28a745;">✅ Setup Complete!</h2>
                    <p>You can now safely close this browser window.</p>
                    <p style="color: #6c757d; font-size: 0.9rem;">The Portier CLI service is running in the background.</p>
                </div>
            `;
        }
    }
    
    toggleConsole() {
        const console = document.getElementById('console-output');
        const btn = document.getElementById('btn-toggle-console');
        
        if (console.classList.contains('hidden')) {
            console.classList.remove('hidden');
            btn.textContent = 'Hide';
        } else {
            console.classList.add('hidden');
            btn.textContent = 'Show Console';
        }
    }
    
    appendConsoleOutput(text) {
        const consoleText = document.getElementById('console-text');
        consoleText.textContent += text + '\n';
        
        // Auto-scroll to bottom
        const consoleContent = document.querySelector('.console-content');
        consoleContent.scrollTop = consoleContent.scrollHeight;
        
        // Show console if it's hidden and there's new output
        const console = document.getElementById('console-output');
        if (console.classList.contains('hidden')) {
            console.classList.remove('hidden');
            document.getElementById('btn-toggle-console').textContent = 'Hide';
        }
    }
}

// Initialize the wizard when the page loads
document.addEventListener('DOMContentLoaded', () => {
    new PortierWizard();
});