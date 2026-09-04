/**
 * ============================================================
 * Browser-Based Chat Test Page
 * ============================================================
 * Trang HTML đơn giản để test chat trên browser
 * 
 * Cách sử dụng:
 * 1. Mở file này trên browser (hoặc serve qua HTTP server)
 * 2. Điền tên và nhấn "Bắt đầu chat"
 * 3. Gửi message và xem AI response
 */

export const BROWSER_TEST_HTML = `
<!DOCTYPE html>
<html lang="vi">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>CSKH Test - Đông Đô Partners</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      background: linear-gradient(135deg, #1e3a5f 0%, #0d1b2a 100%);
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 20px;
    }
    
    .container {
      background: white;
      border-radius: 16px;
      box-shadow: 0 20px 60px rgba(0,0,0,0.3);
      width: 100%;
      max-width: 500px;
      overflow: hidden;
    }
    
    .header {
      background: linear-gradient(135deg, #0d1b2a 0%, #1e3a5f 100%);
      color: white;
      padding: 24px;
      text-align: center;
    }
    
    .header h1 {
      font-size: 24px;
      margin-bottom: 8px;
    }
    
    .header p {
      font-size: 14px;
      opacity: 0.8;
    }
    
    .content {
      padding: 24px;
    }
    
    .form-group {
      margin-bottom: 16px;
    }
    
    .form-group label {
      display: block;
      margin-bottom: 8px;
      font-weight: 600;
      color: #333;
    }
    
    .form-group input {
      width: 100%;
      padding: 12px 16px;
      border: 2px solid #e0e0e0;
      border-radius: 8px;
      font-size: 16px;
      transition: border-color 0.3s;
    }
    
    .form-group input:focus {
      outline: none;
      border-color: #1e3a5f;
    }
    
    .btn {
      width: 100%;
      padding: 14px;
      background: linear-gradient(135deg, #1e3a5f 0%, #0d1b2a 100%);
      color: white;
      border: none;
      border-radius: 8px;
      font-size: 16px;
      font-weight: 600;
      cursor: pointer;
      transition: transform 0.2s, box-shadow 0.2s;
    }
    
    .btn:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 12px rgba(0,0,0,0.2);
    }
    
    .btn:disabled {
      opacity: 0.6;
      cursor: not-allowed;
      transform: none;
    }
    
    .chat-container {
      display: none;
      height: 500px;
      flex-direction: column;
    }
    
    .chat-container.active {
      display: flex;
    }
    
    .register-form.hidden {
      display: none;
    }
    
    .messages {
      flex: 1;
      overflow-y: auto;
      padding: 16px;
      background: #f5f5f5;
    }
    
    .message {
      margin-bottom: 12px;
      padding: 12px 16px;
      border-radius: 12px;
      max-width: 85%;
    }
    
    .message.guest {
      background: #e3f2fd;
      margin-left: auto;
    }
    
    .message.ai {
      background: white;
      border: 1px solid #e0e0e0;
    }
    
    .message.cs {
      background: #fff3e0;
    }
    
    .message .sender {
      font-size: 12px;
      font-weight: 600;
      margin-bottom: 4px;
      color: #666;
    }
    
    .message .content {
      padding: 0;
    }
    
    .message.guest .sender {
      text-align: right;
    }
    
    .typing {
      display: none;
      padding: 12px 16px;
      color: #666;
      font-style: italic;
    }
    
    .typing.active {
      display: block;
    }
    
    .input-area {
      display: flex;
      gap: 8px;
      padding: 16px;
      background: white;
      border-top: 1px solid #e0e0e0;
    }
    
    .input-area input {
      flex: 1;
      padding: 12px 16px;
      border: 2px solid #e0e0e0;
      border-radius: 8px;
      font-size: 16px;
    }
    
    .input-area input:focus {
      outline: none;
      border-color: #1e3a5f;
    }
    
    .input-area button {
      padding: 12px 24px;
      background: #1e3a5f;
      color: white;
      border: none;
      border-radius: 8px;
      font-weight: 600;
      cursor: pointer;
    }
    
    .input-area button:disabled {
      opacity: 0.6;
      cursor: not-allowed;
    }
    
    .status {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 16px;
      background: #f5f5f5;
      font-size: 12px;
      color: #666;
    }
    
    .status-dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background: #ccc;
    }
    
    .status-dot.connected {
      background: #4caf50;
    }
    
    .status-dot.disconnected {
      background: #f44336;
    }
    
    .metrics {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 8px;
      padding: 12px 16px;
      background: #f5f5f5;
      font-size: 12px;
    }
    
    .metric {
      text-align: center;
    }
    
    .metric .value {
      font-size: 20px;
      font-weight: 700;
      color: #1e3a5f;
    }
    
    .metric .label {
      color: #666;
    }
    
    .test-controls {
      display: flex;
      gap: 8px;
      padding: 12px 16px;
      background: #f5f5f5;
    }
    
    .test-controls button {
      flex: 1;
      padding: 8px;
      background: #e0e0e0;
      border: none;
      border-radius: 4px;
      font-size: 12px;
      cursor: pointer;
      transition: background 0.2s;
    }
    
    .test-controls button:hover {
      background: #d0d0d0;
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>💬 CSKH Test</h1>
      <p>Đông Đô Partners - Customer Service</p>
    </div>
    
    <div class="content">
      <!-- Registration Form -->
      <div id="registerForm" class="register-form">
        <div class="form-group">
          <label for="displayName">Tên của bạn</label>
          <input type="text" id="displayName" placeholder="Nhập tên của bạn..." />
        </div>
        <div class="form-group">
          <label for="phone">Số điện thoại (tùy chọn)</label>
          <input type="tel" id="phone" placeholder="0909xxxxxx" />
        </div>
        <button id="startBtn" class="btn">Bắt đầu Chat</button>
      </div>
      
      <!-- Chat Container -->
      <div id="chatContainer" class="chat-container">
        <div class="status">
          <span id="statusDot" class="status-dot disconnected"></span>
          <span id="statusText">Đang kết nối...</span>
        </div>
        
        <div class="metrics">
          <div class="metric">
            <div class="value" id="msgCount">0</div>
            <div class="label">Tin nhắn</div>
          </div>
          <div class="metric">
            <div class="value" id="aiCount">0</div>
            <div class="label">AI Response</div>
          </div>
          <div class="metric">
            <div class="value" id="avgTime">0ms</div>
            <div class="label">Avg Response</div>
          </div>
        </div>
        
        <div id="messages" class="messages">
          <div class="message ai">
            <div class="sender">🤖 Trợ lý AI</div>
            <div class="content">Xin chào! Em là trợ lý AI của Đông Đô Partners. Em có thể giúp gì cho bạn hôm nay?</div>
          </div>
        </div>
        
        <div id="typing" class="typing">🤖 AI đang gõ...</div>
        
        <div class="test-controls">
          <button onclick="sendTestMessage('Hàng hóa phái sinh là gì?')">Test 1</button>
          <button onclick="sendTestMessage('Cách nạp tiền?')">Test 2</button>
          <button onclick="sendTestMessage('tôi cần hỗ trợ')">Test Human</button>
        </div>
        
        <div class="input-area">
          <input type="text" id="messageInput" placeholder="Nhập tin nhắn..." />
          <button id="sendBtn">Gửi</button>
        </div>
      </div>
    </div>
  </div>

  <script>
    // Configuration
    const API_BASE = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
      ? 'http://localhost:8080'
      : window.location.origin;
    
    const WS_BASE = API_BASE.replace('http', 'ws');
    
    // State
    let session = null;
    let ws = null;
    let msgCount = 0;
    let aiCount = 0;
    let responseTimes = [];
    
    // DOM Elements
    const registerForm = document.getElementById('registerForm');
    const chatContainer = document.getElementById('chatContainer');
    const startBtn = document.getElementById('startBtn');
    const displayNameInput = document.getElementById('displayName');
    const phoneInput = document.getElementById('phone');
    const messagesDiv = document.getElementById('messages');
    const messageInput = document.getElementById('messageInput');
    const sendBtn = document.getElementById('sendBtn');
    const typingDiv = document.getElementById('typing');
    const statusDot = document.getElementById('statusDot');
    const statusText = document.getElementById('statusText');
    
    // Event Listeners
    startBtn.addEventListener('click', startChat);
    sendBtn.addEventListener('click', sendMessage);
    messageInput.addEventListener('keypress', (e) => {
      if (e.key === 'Enter') sendMessage();
    });
    
    // Functions
    async function startChat() {
      const displayName = displayNameInput.value.trim();
      const phone = phoneInput.value.trim();
      
      if (!displayName) {
        alert('Vui lòng nhập tên của bạn');
        return;
      }
      
      startBtn.disabled = true;
      startBtn.textContent = 'Đang kết nối...';
      
      try {
        // Register guest
        const response = await fetch(\`\${API_BASE}/guest/register\`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ display_name: displayName, phone }),
        });
        
        if (!response.ok) throw new Error('Failed to register');
        
        session = await response.json();
        console.log('Session created:', session);
        
        // Show chat
        registerForm.classList.add('hidden');
        chatContainer.classList.add('active');
        
        // Connect WebSocket
        connectWebSocket();
        
      } catch (error) {
        console.error('Error:', error);
        alert('Lỗi kết nối: ' + error.message);
        startBtn.disabled = false;
        startBtn.textContent = 'Bắt đầu Chat';
      }
    }
    
    function connectWebSocket() {
      const wsUrl = \`\${WS_BASE}/ws?session_id=\${encodeURIComponent(session.session_id)}&user_id=\${encodeURIComponent(session.display_name)}&role=guest\`;
      
      ws = new WebSocket(wsUrl);
      
      ws.onopen = () => {
        statusDot.classList.add('connected');
        statusDot.classList.remove('disconnected');
        statusText.textContent = 'Đã kết nối';
        console.log('WebSocket connected');
      };
      
      ws.onclose = () => {
        statusDot.classList.remove('connected');
        statusDot.classList.add('disconnected');
        statusText.textContent = 'Mất kết nối';
        console.log('WebSocket disconnected');
      };
      
      ws.onerror = (error) => {
        console.error('WebSocket error:', error);
      };
      
      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          handleMessage(data);
        } catch (error) {
          console.error('Parse error:', error);
        }
      };
    }
    
    function handleMessage(event) {
      console.log('WS Event:', event);
      
      switch (event.type) {
        case 'message':
          typingDiv.classList.remove('active');
          
          const msg = event.payload?.message || event.payload;
          if (msg && msg.sender_type === 'ai') {
            aiCount++;
            updateMetrics();
            addMessage(msg.content, 'ai', msg.sender_id || 'AI');
          }
          break;
          
        case 'typing':
          if (event.payload?.typing) {
            typingDiv.classList.add('active');
          } else {
            typingDiv.classList.remove('active');
          }
          break;
      }
    }
    
    async function sendMessage() {
      const content = messageInput.value.trim();
      if (!content || !session) return;
      
      messageInput.value = '';
      const startTime = Date.now();
      
      // Add to UI
      addMessage(content, 'guest', session.display_name);
      msgCount++;
      updateMetrics();
      
      try {
        const response = await fetch(\`\${API_BASE}/chat\`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            session_id: session.session_id,
            customer_name: session.display_name,
            message: content,
          }),
        });
        
        if (!response.ok) throw new Error('Failed to send');
        
        responseTimes.push(Date.now() - startTime);
        updateMetrics();
        
      } catch (error) {
        console.error('Send error:', error);
        addMessage('Lỗi gửi tin nhắn', 'ai', 'System');
      }
    }
    
    function sendTestMessage(content) {
      messageInput.value = content;
      sendMessage();
    }
    
    function addMessage(content, type, sender) {
      const div = document.createElement('div');
      div.className = \`message \${type}\`;
      
      const senderEmoji = type === 'guest' ? '👤' : type === 'cs' ? '👨‍💼' : '🤖';
      const senderName = type === 'guest' ? sender : type === 'cs' ? \`CS \${sender}\` : sender;
      
      div.innerHTML = \`
        <div class="sender">\${senderEmoji} \${senderName}</div>
        <div class="content">\${content}</div>
      \`;
      
      messagesDiv.appendChild(div);
      messagesDiv.scrollTop = messagesDiv.scrollHeight;
    }
    
    function updateMetrics() {
      document.getElementById('msgCount').textContent = msgCount;
      document.getElementById('aiCount').textContent = aiCount;
      
      if (responseTimes.length > 0) {
        const avg = Math.round(responseTimes.reduce((a, b) => a + b, 0) / responseTimes.length);
        document.getElementById('avgTime').textContent = avg + 'ms';
      }
    }
    
    // Initialize
    console.log('CSKH Test Page Loaded');
    console.log('API Base:', API_BASE);
  </script>
</body>
</html>
`;

// Export for use
export default BROWSER_TEST_HTML;
