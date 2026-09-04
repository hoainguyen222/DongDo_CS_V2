/**
 * ============================================================
 * WebSocket Connection Test
 * ============================================================
 * Test kết nối WebSocket và các tính năng real-time
 * 
 * Mục tiêu:
 * - Test kết nối WebSocket ổn định
 * - Test nhận tin nhắn real-time
 * - Test typing indicator
 * - Test reconnect khi mất kết nối
 */

import { TEST_CONFIG, testLogger, TestType, TestResult } from './config';
import { registerGuest } from './guest-load-test';
import { sendMessage } from './message-load-test';

// Types
interface WSEvent {
  type: string;
  session_id?: string;
  sender_id?: string;
  content?: string;
  payload?: any;
  client_msg_id?: string;
}

interface WebSocketTestMetrics {
  connectionTest: {
    success: boolean;
    connectionTime: number;
    error?: string;
  };
  messageReception: {
    totalEvents: number;
    messageEvents: number;
    typingEvents: number;
    otherEvents: number;
  };
  reconnectionTest: {
    tested: boolean;
    success: boolean;
    reconnectTime: number;
  };
  latency: {
    min: number;
    max: number;
    avg: number;
  };
}

/**
 * WebSocket Client cho test
 */
class TestWSClient {
  private ws: WebSocket | null = null;
  private url: string;
  private sessionID: string;
  private userID: string;
  private role: string;
  private eventHandlers: Map<string, Set<(event: WSEvent) => void>> = new Map();
  private messageQueue: WSEvent[] = [];
  private isConnected = false;
  private connectionTime = 0;

  constructor(sessionID: string, userID: string, role: string = 'guest') {
    this.sessionID = sessionID;
    this.userID = userID;
    this.role = role;
    
    const wsProtocol = TEST_CONFIG.WS_BASE.startsWith('wss') ? 'wss:' : 'ws:';
    const host = TEST_CONFIG.WS_BASE.replace(/^https?:\/\//, '').replace(/^wss?:\/\//, '');
    this.url = `${wsProtocol}//${host}/ws?session_id=${encodeURIComponent(sessionID)}&user_id=${encodeURIComponent(userID)}&role=${encodeURIComponent(role)}`;
  }

  async connect(): Promise<{ success: boolean; connectionTime: number; error?: string }> {
    const startTime = Date.now();
    
    return new Promise((resolve) => {
      try {
        this.ws = new WebSocket(this.url);

        this.ws.onopen = () => {
          this.connectionTime = Date.now() - startTime;
          this.isConnected = true;
          resolve({ success: true, connectionTime: this.connectionTime });
        };

        this.ws.onmessage = (event) => {
          try {
            const parsed: WSEvent = JSON.parse(event.data);
            this.messageQueue.push(parsed);
            this.emit(parsed);
          } catch (err) {
            testLogger.warn('Failed to parse WS message:', event.data);
          }
        };

        this.ws.onclose = () => {
          this.isConnected = false;
        };

        this.ws.onerror = (error) => {
          resolve({ 
            success: false, 
            connectionTime: Date.now() - startTime,
            error: 'WebSocket connection failed'
          });
        };

        // Timeout
        setTimeout(() => {
          if (!this.isConnected) {
            this.ws?.close();
            resolve({ 
              success: false, 
              connectionTime: Date.now() - startTime,
              error: 'Connection timeout'
            });
          }
        }, TEST_CONFIG.REQUEST_TIMEOUT);
      } catch (error: any) {
        resolve({ 
          success: false, 
          connectionTime: Date.now() - startTime,
          error: error.message 
        });
      }
    });
  }

  send(type: string, content: string = '', payload?: any): boolean {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({
        type,
        session_id: this.sessionID,
        content,
        payload,
      }));
      return true;
    }
    return false;
  }

  on(eventType: string, handler: (event: WSEvent) => void): () => void {
    if (!this.eventHandlers.has(eventType)) {
      this.eventHandlers.set(eventType, new Set());
    }
    this.eventHandlers.get(eventType)!.add(handler);

    return () => {
      this.eventHandlers.get(eventType)?.delete(handler);
    };
  }

  private emit(event: WSEvent): void {
    const handlers = this.eventHandlers.get(event.type);
    if (handlers) {
      handlers.forEach(fn => fn(event));
    }
  }

  disconnect(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.isConnected = false;
  }

  getMessageQueue(): WSEvent[] {
    return [...this.messageQueue];
  }

  clearMessageQueue(): void {
    this.messageQueue = [];
  }

  getConnectionTime(): number {
    return this.connectionTime;
  }

  isSocketConnected(): boolean {
    return this.isConnected;
  }
}

/**
 * Test kết nối WebSocket cơ bản
 */
async function testBasicConnection(sessionID: string, userID: string): Promise<{
  success: boolean;
  connectionTime: number;
  error?: string;
}> {
  const client = new TestWSClient(sessionID, userID);
  const result = await client.connect();
  
  // Đợi một chút để nhận các event ban đầu
  await new Promise(resolve => setTimeout(resolve, 1000));
  
  client.disconnect();
  return result;
}

/**
 * Test nhận message qua WebSocket
 */
async function testMessageReception(sessionID: string, userID: string, customerName: string): Promise<{
  events: WSEvent[];
  messageCount: number;
  typingCount: number;
}> {
  const client = new TestWSClient(sessionID, userID);
  const connectResult = await client.connect();
  
  if (!connectResult.success) {
    return { events: [], messageCount: 0, typingCount: 0 };
  }

  const receivedEvents: WSEvent[] = [];
  let messageCount = 0;
  let typingCount = 0;

  // Subscribe to message events
  client.on('message', (event) => {
    receivedEvents.push(event);
    messageCount++;
    testLogger.info(`WS received message event:`, event.payload?.message?.content?.substring(0, 50));
  });

  client.on('typing', (event) => {
    receivedEvents.push(event);
    typingCount++;
    testLogger.info(`WS received typing event:`, event.payload);
  });

  client.on('case_update', (event) => {
    receivedEvents.push(event);
  });

  // Gửi một message để kích hoạt AI response
  const sendResult = await sendMessage(sessionID, customerName, 'Test message for WebSocket reception');
  
  if (sendResult.success) {
    // Chờ AI response (có thể mất vài giây)
    await new Promise(resolve => setTimeout(resolve, 5000));
  }

  client.disconnect();

  return { events: receivedEvents, messageCount, typingCount };
}

/**
 * Test reconnect khi mất kết nối
 */
async function testReconnection(sessionID: string, userID: string): Promise<{
  success: boolean;
  reconnectTime: number;
}> {
  // Kết nối lần đầu
  const client1 = new TestWSClient(sessionID, userID);
  const connect1 = await client1.connect();
  
  if (!connect1.success) {
    return { success: false, reconnectTime: 0 };
  }

  // Ngắt kết nối
  client1.disconnect();
  await new Promise(resolve => setTimeout(resolve, 1000));

  // Kết nối lại
  const client2 = new TestWSClient(sessionID, userID);
  const startTime = Date.now();
  const connect2 = await client2.connect();
  
  const reconnectTime = Date.now() - startTime;
  client2.disconnect();

  return {
    success: connect2.success,
    reconnectTime,
  };
}

/**
 * Test đo latency của WebSocket
 */
async function testLatency(sessionID: string, userID: string): Promise<{
  min: number;
  max: number;
  avg: number;
  samples: number[];
}> {
  const client = new TestWSClient(sessionID, userID);
  const connectResult = await client.connect();
  
  if (!connectResult.success) {
    return { min: 0, max: 0, avg: 0, samples: [] };
  }

  const samples: number[] = [];
  const pingInterval = 500; // ms

  // Send multiple pings and measure responses
  for (let i = 0; i < 10; i++) {
    const startTime = Date.now();
    
    client.on('pong', () => {
      const latency = Date.now() - startTime;
      samples.push(latency);
    });

    client.send('ping');
    await new Promise(resolve => setTimeout(resolve, pingInterval));
  }

  client.disconnect();

  if (samples.length === 0) {
    return { min: 0, max: 0, avg: 0, samples: [] };
  }

  return {
    min: Math.min(...samples),
    max: Math.max(...samples),
    avg: samples.reduce((a, b) => a + b, 0) / samples.length,
    samples,
  };
}

/**
 * Test nhiều WebSocket connections đồng thời
 */
async function testConcurrentConnections(guestCount: number): Promise<{
  totalConnections: number;
  successfulConnections: number;
  failedConnections: number;
  avgConnectionTime: number;
  connectionTimes: number[];
}> {
  testLogger.info(`Testing ${guestCount} concurrent WebSocket connections...`);

  // Đăng ký guests trước
  const guests = await Promise.all(
    Array.from({ length: guestCount }, async (_, i) => {
      const result = await registerGuest(`WSTest_${Date.now()}_${i}`);
      return result?.session || null;
    })
  );

  const validGuests = guests.filter(g => g !== null);
  testLogger.info(`Registered ${validGuests.length} guests, starting WS connections...`);

  // Kết nối WebSocket cho tất cả guests đồng thời
  const connectionResults = await Promise.all(
    validGuests.map(async (guest) => {
      const client = new TestWSClient(guest!.session_id, guest!.display_name);
      const result = await client.connect();
      client.disconnect();
      return result;
    })
  );

  const successfulConnections = connectionResults.filter(r => r.success).length;
  const failedConnections = connectionResults.filter(r => !r.success).length;
  const connectionTimes = connectionResults.filter(r => r.success).map(r => r.connectionTime);

  return {
    totalConnections: guestCount,
    successfulConnections,
    failedConnections,
    avgConnectionTime: connectionTimes.length > 0 
      ? connectionTimes.reduce((a, b) => a + b, 0) / connectionTimes.length 
      : 0,
    connectionTimes,
  };
}

/**
 * Chạy WebSocket test chính
 */
export async function runWebSocketTest(options?: {
  connectionCount?: number;
  testReconnect?: boolean;
  testLatency?: boolean;
}): Promise<TestResult> {
  const startTime = Date.now();
  const { connectionCount = 5, testReconnect = true, testLatency = true } = options || {};

  try {
    // Tạo một guest để test
    const guestResult = await registerGuest(`WSTest_Main_${Date.now()}`);
    if (!guestResult) {
      throw new Error('Failed to create guest for WebSocket test');
    }

    const metrics: WebSocketTestMetrics = {
      connectionTest: { success: false, connectionTime: 0 },
      messageReception: { totalEvents: 0, messageEvents: 0, typingEvents: 0, otherEvents: 0 },
      reconnectionTest: { tested: false, success: false, reconnectTime: 0 },
      latency: { min: 0, max: 0, avg: 0 },
    };

    // Test 1: Basic connection
    testLogger.info('Testing basic WebSocket connection...');
    metrics.connectionTest = await testBasicConnection(
      guestResult.session.session_id,
      guestResult.session.display_name
    );
    testLogger.success(`Connection test: ${metrics.connectionTest.success ? 'SUCCESS' : 'FAILED'} (${metrics.connectionTest.connectionTime}ms)`);

    // Test 2: Message reception
    if (metrics.connectionTest.success) {
      testLogger.info('Testing message reception...');
      const reception = await testMessageReception(
        guestResult.session.session_id,
        guestResult.session.display_name,
        guestResult.session.display_name
      );
      metrics.messageReception = {
        totalEvents: reception.events.length,
        messageEvents: reception.messageCount,
        typingEvents: reception.typingCount,
        otherEvents: reception.events.length - reception.messageCount - reception.typingCount,
      };
      testLogger.success(`Received ${metrics.messageReception.totalEvents} events (${metrics.messageReception.messageEvents} messages, ${metrics.messageReception.typingEvents} typing)`);
    }

    // Test 3: Reconnection
    if (testReconnect) {
      testLogger.info('Testing reconnection...');
      metrics.reconnectionTest = {
        tested: true,
        ...(await testReconnection(guestResult.session.session_id, guestResult.session.display_name)),
      };
      testLogger.success(`Reconnection test: ${metrics.reconnectionTest.success ? 'SUCCESS' : 'FAILED'} (${metrics.reconnectionTest.reconnectTime}ms)`);
    }

    // Test 4: Latency
    if (testLatency) {
      testLogger.info('Testing latency...');
      const latencyResult = await testLatency(guestResult.session.session_id, guestResult.session.display_name);
      metrics.latency = latencyResult;
      testLogger.success(`Latency: min=${latencyResult.min}ms, max=${latencyResult.max}ms, avg=${latencyResult.avg.toFixed(2)}ms`);
    }

    // Test 5: Concurrent connections
    testLogger.info(`Testing ${connectionCount} concurrent connections...`);
    const concurrentResult = await testConcurrentConnections(connectionCount);
    testLogger.success(`${concurrentResult.successfulConnections}/${concurrentResult.totalConnections} concurrent connections successful`);

    const duration = Date.now() - startTime;
    const success = metrics.connectionTest.success && concurrentResult.successfulConnections > 0;

    return {
      testType: TestType.WEBSOCKET,
      success,
      duration,
      metrics: {
        total: concurrentResult.totalConnections,
        success: concurrentResult.successfulConnections,
        failed: concurrentResult.failedConnections,
        errors: [],
      },
      details: {
        ...metrics,
        concurrentConnections: concurrentResult,
      },
    };
  } catch (error: any) {
    testLogger.error('WebSocket test failed:', error);
    return {
      testType: TestType.WEBSOCKET,
      success: false,
      duration: Date.now() - startTime,
      metrics: {
        total: 0,
        success: 0,
        failed: 0,
        errors: [error.message],
      },
    };
  }
}

// Export classes and functions
export { TestWSClient };
