// AetherFlow E2E Demo - Web Client

// Configuration
const CONFIG = {
    gatewayUrl: 'ws://localhost:8000/ws',
    reconnectDelay: 3000,
    heartbeatInterval: 30000,
};

// Global State
let ws = null;
let currentUser = null;
let currentDocument = null;
let isLocked = false;
let stats = {
    operations: 0,
    latencies: [],
    conflicts: 0,
    syncs: 0,
};

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    setupEditorListeners();
});

// Connection Management
function connect() {
    const username = document.getElementById('usernameInput').value.trim();
    const documentId = document.getElementById('documentInput').value.trim();

    if (!username) {
        showAlert('请输入用户名', 'error');
        return;
    }

    currentUser = {
        name: username,
        id: 'user-' + Date.now(),
    };

    showAlert('正在连接...', 'info');

    ws = new WebSocket(CONFIG.gatewayUrl);

    ws.onopen = () => {
        console.log('WebSocket connected');
        showAlert('连接成功！', 'info');

        // Authenticate
        sendMessage({
            type: 'auth',
            user_id: currentUser.id,
            username: currentUser.name,
        });

        // Join or create document
        if (documentId) {
            joinDocument(documentId);
        } else {
            createDocument();
        }

        // Start heartbeat
        startHeartbeat();

        // Update UI
        updateConnectionStatus(true);
    };

    ws.onmessage = (event) => {
        handleMessage(JSON.parse(event.data));
    };

    ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        showAlert('连接错误', 'error');
    };

    ws.onclose = () => {
        console.log('WebSocket closed');
        showAlert('连接已断开', 'warning');
        updateConnectionStatus(false);

        // Auto reconnect
        setTimeout(() => {
            if (currentUser) {
                showAlert('尝试重新连接...', 'info');
                connect();
            }
        }, CONFIG.reconnectDelay);
    };
}

function sendMessage(message) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify(message));
    }
}

function startHeartbeat() {
    setInterval(() => {
        sendMessage({ type: 'heartbeat' });
    }, CONFIG.heartbeatInterval);
}

// Document Management
function createDocument() {
    const docId = generateUUID();
    sendMessage({
        type: 'create_document',
        document_id: docId,
        name: `${currentUser.name}'s Document`,
        doc_type: 'text',
        content: '# Welcome to AetherFlow\n\nStart typing...\n',
    });
}

function joinDocument(docId) {
    sendMessage({
        type: 'join_document',
        document_id: docId,
        user_id: currentUser.id,
    });
}

// Message Handlers
function handleMessage(message) {
    console.log('Received:', message);

    switch (message.type) {
        case 'auth_success':
            handleAuthSuccess(message);
            break;

        case 'document_created':
        case 'document_joined':
            handleDocumentJoined(message);
            break;

        case 'document_updated':
            handleDocumentUpdated(message);
            break;

        case 'operation_applied':
            handleOperationApplied(message);
            break;

        case 'user_joined':
            handleUserJoined(message);
            break;

        case 'user_left':
            handleUserLeft(message);
            break;

        case 'conflict_detected':
            handleConflictDetected(message);
            break;

        case 'lock_acquired':
        case 'lock_released':
            handleLockChange(message);
            break;

        case 'error':
            handleError(message);
            break;
    }
}

function handleAuthSuccess(message) {
    console.log('Auth successful');
}

function handleDocumentJoined(message) {
    currentDocument = {
        id: message.document_id,
        version: message.version || 0,
        content: message.content || '',
    };

    // Update UI
    document.getElementById('docIdDisplay').textContent = currentDocument.id.substring(0, 8);
    document.getElementById('versionDisplay').textContent = currentDocument.version;
    document.getElementById('editor').value = currentDocument.content;
    updateCharCount();

    // Show main content
    document.getElementById('loginPanel').style.display = 'none';
    document.getElementById('mainContent').style.display = 'grid';

    showAlert('已加入文档！', 'info');

    // Update users list
    if (message.active_users) {
        updateUsersList(message.active_users);
    }
}

function handleDocumentUpdated(message) {
    if (message.content) {
        currentDocument.content = message.content;
        currentDocument.version = message.version;

        // Update editor if not typing
        const editor = document.getElementById('editor');
        if (document.activeElement !== editor) {
            editor.value = message.content;
        }

        document.getElementById('versionDisplay').textContent = message.version;
        updateCharCount();

        stats.syncs++;
        document.getElementById('syncCount').textContent = stats.syncs;
    }
}

function handleOperationApplied(message) {
    stats.operations++;
    document.getElementById('operationCount').textContent = stats.operations;

    // Update latency
    if (message.latency_ms) {
        stats.latencies.push(message.latency_ms);
        if (stats.latencies.length > 100) {
            stats.latencies.shift();
        }
        updateLatencyDisplay();
    }

    // Add to operations list
    addOperationToList({
        type: message.operation_type,
        user: message.user_name || 'Unknown',
        time: new Date().toLocaleTimeString(),
        version: message.version,
    });
}

function handleUserJoined(message) {
    addUserToList({
        id: message.user_id,
        name: message.user_name,
        status: 'active',
    });

    showNotification(`${message.user_name} 加入了文档`);
}

function handleUserLeft(message) {
    removeUserFromList(message.user_id);
    showNotification(`${message.user_name} 离开了文档`);
}

function handleConflictDetected(message) {
    stats.conflicts++;
    document.getElementById('conflictCount').textContent = stats.conflicts;

    showAlert('检测到冲突，正在解决...', 'warning');

    addOperationToList({
        type: 'CONFLICT',
        user: 'System',
        time: new Date().toLocaleTimeString(),
        details: message.description || '版本冲突',
    });
}

function handleLockChange(message) {
    isLocked = message.type === 'lock_acquired';
    updateLockUI();

    const lockHolder = message.user_name || 'Unknown';
    if (isLocked) {
        showNotification(`${lockHolder} 锁定了文档`);
    } else {
        showNotification(`${lockHolder} 释放了锁`);
    }
}

function handleError(message) {
    showAlert(message.error || '操作失败', 'error');
}

// Editor Interaction
function setupEditorListeners() {
    const editor = document.getElementById('editor');

    let typingTimer;
    const typingDelay = 500; // 500ms after typing stops

    editor.addEventListener('input', () => {
        clearTimeout(typingTimer);
        updateCharCount();

        typingTimer = setTimeout(() => {
            sendOperation('update');
        }, typingDelay);
    });
}

function sendOperation(type) {
    if (!currentDocument) return;

    const editor = document.getElementById('editor');
    const content = editor.value;

    const startTime = performance.now();

    sendMessage({
        type: 'apply_operation',
        document_id: currentDocument.id,
        operation: {
            id: generateUUID(),
            type: type,
            data: content,
            timestamp: Date.now(),
            client_id: currentUser.name,
        },
    });

    // Record latency start time
    currentDocument.lastOperationTime = startTime;
}

function insertText(text) {
    const editor = document.getElementById('editor');
    const start = editor.selectionStart;
    const end = editor.selectionEnd;
    const value = editor.value;

    editor.value = value.substring(0, start) + text + value.substring(end);
    editor.selectionStart = editor.selectionEnd = start + text.length;

    updateCharCount();
    sendOperation('insert');
}

function clearEditor() {
    if (confirm('确定要清空文档内容吗？')) {
        document.getElementById('editor').value = '';
        updateCharCount();
        sendOperation('delete');
    }
}

function toggleLock() {
    if (!currentDocument) return;

    if (isLocked) {
        sendMessage({
            type: 'release_lock',
            document_id: currentDocument.id,
            user_id: currentUser.id,
        });
    } else {
        sendMessage({
            type: 'acquire_lock',
            document_id: currentDocument.id,
            user_id: currentUser.id,
        });
    }
}

// UI Updates
function updateConnectionStatus(connected) {
    const dot = document.querySelector('.status-dot');
    if (connected) {
        dot.classList.remove('status-disconnected');
        dot.classList.add('status-connected');
    } else {
        dot.classList.remove('status-connected');
        dot.classList.add('status-disconnected');
    }
}

function updateCharCount() {
    const content = document.getElementById('editor').value;
    document.getElementById('charCount').textContent = content.length;
}

function updateLatencyDisplay() {
    if (stats.latencies.length === 0) return;

    const avg = stats.latencies.reduce((a, b) => a + b, 0) / stats.latencies.length;
    document.getElementById('latencyDisplay').textContent = Math.round(avg);
}

function updateLockUI() {
    const indicator = document.getElementById('lockIndicator');
    const button = document.getElementById('lockButton');
    const editor = document.getElementById('editor');

    if (isLocked) {
        indicator.textContent = '🔒 已锁定';
        indicator.classList.remove('unlocked');
        button.textContent = '解锁';
        editor.disabled = true;
    } else {
        indicator.textContent = '🔓 未锁定';
        indicator.classList.add('unlocked');
        button.textContent = '锁定';
        editor.disabled = false;
    }
}

function updateUsersList(users) {
    const list = document.getElementById('usersList');
    list.innerHTML = '';

    users.forEach(user => {
        addUserToList(user);
    });

    document.getElementById('userCount').textContent = users.length;
}

function addUserToList(user) {
    const list = document.getElementById('usersList');

    // Check if user already exists
    const existing = document.getElementById(`user-${user.id}`);
    if (existing) return;

    const li = document.createElement('li');
    li.className = 'user-item';
    li.id = `user-${user.id}`;

    const initial = user.name.charAt(0).toUpperCase();
    const color = stringToColor(user.name);

    li.innerHTML = `
        <div class="user-info">
            <div class="user-avatar" style="background: ${color};">${initial}</div>
            <div>
                <div class="user-name">${user.name}</div>
                <div class="user-status">● 在线</div>
            </div>
        </div>
    `;

    list.appendChild(li);

    const count = list.children.length;
    document.getElementById('userCount').textContent = count;
}

function removeUserFromList(userId) {
    const item = document.getElementById(`user-${userId}`);
    if (item) {
        item.remove();
        const count = document.getElementById('usersList').children.length;
        document.getElementById('userCount').textContent = count;
    }
}

function addOperationToList(operation) {
    const list = document.getElementById('operationsList');

    const div = document.createElement('div');
    div.className = 'operation-item';

    div.innerHTML = `
        <div class="operation-header">
            <span class="operation-type">${operation.type}</span>
            <span class="operation-time">${operation.time}</span>
        </div>
        <div class="operation-details">
            ${operation.user}${operation.version ? ` • v${operation.version}` : ''}
            ${operation.details ? ` • ${operation.details}` : ''}
        </div>
    `;

    list.insertBefore(div, list.firstChild);

    // Keep only last 20 operations
    while (list.children.length > 20) {
        list.removeChild(list.lastChild);
    }
}

function showAlert(message, type = 'info') {
    const alert = document.getElementById('connectionAlert');
    alert.textContent = message;
    alert.className = `alert alert-${type}`;
}

function showNotification(message) {
    // Simple notification (could be enhanced with a toast library)
    console.log('Notification:', message);
}

// Utilities
function generateUUID() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        const r = Math.random() * 16 | 0;
        const v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });
}

function stringToColor(str) {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
        hash = str.charCodeAt(i) + ((hash << 5) - hash);
    }

    const colors = [
        '#667eea', '#764ba2', '#f093fb', '#f5576c',
        '#4facfe', '#00f2fe', '#43e97b', '#38f9d7',
        '#fa709a', '#fee140', '#30cfd0', '#330867',
    ];

    return colors[Math.abs(hash) % colors.length];
}
