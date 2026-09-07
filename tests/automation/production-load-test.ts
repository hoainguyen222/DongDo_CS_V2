/**
 * ============================================================
 * PRODUCTION LOAD TEST SUITE
 * ============================================================
 * Test load thực tế cho production deployment
 * 
 * Scenarios:
 * - smoke: Quick validation (1-2 min)
 * - light: Light load (5 min)
 * - medium: Medium load (10-15 min)
 * - heavy: Heavy load (15-20 min)
 * - stress: Stress test to find limits
 * - full: Full production simulation (30-60 min)
 * 
 * Usage:
 *   npx ts-node production-load-test.ts --scenario medium --verbose
 *   npx ts-node production-load-test.ts --users 100 --duration 10m
 *   npx ts-node production-load-test.ts --scenario stress --progressive
 */

import * as fs from 'fs';
import * as path from 'path';

// ============== CONFIGURATION ==============

interface TestConfig {
  scenario: string;
  users: number;
  durationMinutes: number;
  rampUpSeconds: number;
  messagesPerUser: number;
  thinkTimeMs: number;
  verbose: boolean;
  report: boolean;
  progressive: boolean;
}

interface LoadStep {
  users: number;
  durationSeconds: number;
}

interface TestMetrics {
  // Timing
  startTime: number;
  endTime: number;
  totalDuration: number;
  
  // Connection metrics
  totalConnections: number;
  successfulConnections: number;
  failedConnections: number;
  connectionErrors: string[];
  
  // Message metrics
  totalMessages: number;
  sentMessages: number;
  failedMessages: number;
  averageSendTime: number;
  
  // AI metrics
  aiResponses: number;
  aiFailures: number;
  averageAIResponseTime: number;
  p95AIResponseTime: number;
  
  // API metrics
  apiCalls: number;
  apiErrors: number;
  averageAPIResponseTime: number;
  p95APIResponseTime: number;
  
  // Real-time data
  connectionTimes: number[];
  sendTimes: number[];
  aiResponseTimes: number[];
  apiResponseTimes: number[];
  
  // System state
  peakConcurrentUsers: number;
  currentConcurrentUsers: number;
  messageQueueDepth: number;
  
  // Errors
  errors: Array<{ timestamp: number; type: string; message: string }>;
}

interface ScenarioConfig {
  name: string;
  description: string;
  users: number;
  durationMinutes: number;
  rampUpSeconds: number;
  messagesPerUser: number;
  thinkTimeMs: number;
  burstSize: number;
  aiDelayMs: number;
}

// ============== SCENARIO DEFINITIONS ==============

const SCENARIOS: Record<string, ScenarioConfig> = {
  smoke: {
    name: 'Smoke Test',
    description: 'Quick validation - 5 users, 2 minutes',
    users: 5,
    durationMinutes: 2,
    rampUpSeconds: 5,
    messagesPerUser: 3,
    thinkTimeMs: 1000,
    burstSize: 10,
    aiDelayMs: 3000,
  },
  light: {
    name: 'Light Load',
    description: 'Light production load - 20 users, 5 minutes',
    users: 20,
    durationMinutes: 5,
    rampUpSeconds: 15,
    messagesPerUser: 5,
    thinkTimeMs: 2000,
    burstSize: 20,
    aiDelayMs: 4000,
  },
  medium: {
    name: 'Medium Load',
    description: 'Normal peak hours - 50 users, 10 minutes',
    users: 50,
    durationMinutes: 10,
    rampUpSeconds: 30,
    messagesPerUser: 10,
    thinkTimeMs: 3000,
    burstSize: 50,
    aiDelayMs: 5000,
  },
  heavy: {
    name: 'Heavy Load',
    description: 'High traffic - 100 users, 15 minutes',
    users: 100,
    durationMinutes: 15,
    rampUpSeconds: 60,
    messagesPerUser: 15,
    thinkTimeMs: 3000,
    burstSize: 100,
    aiDelayMs: 6000,
  },
  stress: {
    name: 'Stress Test',
    description: 'Find system limits - progressive increase',
    users: 200,
    durationMinutes: 20,
    rampUpSeconds: 120,
    messagesPerUser: 20,
    thinkTimeMs: 2000,
    burstSize: 200,
    aiDelayMs: 8000,
  },
  peak_hours: {
    name: 'Peak Hours Simulation',
    description: 'Realistic peak hours - 80 users, 30 minutes',
    users: 80,
    durationMinutes: 30,
    rampUpSeconds: 45,
    messagesPerUser: 12,
    thinkTimeMs: 2500,
    burstSize: 80,
    aiDelayMs: 5000,
  },
  rush_hour: {
    name: 'Rush Hour',
    description: 'Sudden traffic spike - 150 users burst',
    users: 150,
    durationMinutes: 5,
    rampUpSeconds: 10,
    messagesPerUser: 8,
    thinkTimeMs: 1000,
    burstSize: 150,
    aiDelayMs: 4000,
  },
  full: {
    name: 'Full Production Simulation',
    description: 'Complete production scenario - 200 users, 60 minutes',
    users: 200,
    durationMinutes: 60,
    rampUpSeconds: 180,
    messagesPerUser: 25,
    thinkTimeMs: 3000,
    burstSize: 200,
    aiDelayMs: 6000,
  },
};

// ============== TEST DATA ==============

const TEST_QUESTIONS = [
  "Tôi muốn biết về các sản phẩm của công ty",
  "Làm sao để đăng ký dịch vụ?",
  "Chi phí dịch vụ là bao nhiêu?",
  "Thời gian làm việc của công ty?",
  "Tôi cần hỗ trợ về kỹ thuật",
  "Cách liên hệ với bộ phận chăm sóc khách hàng?",
  "Làm sao để thanh toán?",
  "Có chương trình khuyến mãi nào không?",
  "Tôi muốn nâng cấp dịch vụ",
  "Làm sao để hủy dịch vụ?",
  "Sản phẩm A có tính năng gì?",
  "Bảo hành như thế nào?",
  "Tôi gặp lỗi khi sử dụng",
  "Cần bao lâu để xử lý?",
  "Tôi muốn phản hồi về chất lượng dịch vụ",
];

const CS_QUESTIONS = [
  "Chào bạn, tôi cần hỗ trợ",
  "Cảm ơn bạn đã hỗ trợ",
  "Làm ơn giải thích thêm",
  "Tôi vẫn chưa hiểu",
  "Bạn có thể gửi thông tin qua email không?",
];

// ============== UTILITIES ==============

function parseArgs(): TestConfig {
  const args = process.argv.slice(2);
  
  let scenario = 'medium';
  let users = 50;
  let durationMinutes = 10;
  let rampUpSeconds = 30;
  let messagesPerUser = 10;
  let thinkTimeMs = 3000;
  let verbose = false;
  let report = false;
  let progressive = false;

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    switch (arg) {
      case '--scenario':
      case '-s':
        scenario = args[++i] || 'medium';
        break;
      case '--users':
      case '-u':
        users = parseInt(args[++i]) || 50;
        break;
      case '--duration':
        durationMinutes = parseInt(args[++i]) || 10;
        break;
      case '--ramp':
        rampUpSeconds = parseInt(args[++i]) || 30;
        break;
      case '--messages':
        messagesPerUser = parseInt(args[++i]) || 10;
        break;
      case '--think':
        thinkTimeMs = parseInt(args[++i]) || 3000;
        break;
      case '--verbose':
      case '-v':
        verbose = true;
        break;
      case '--report':
        report = true;
        break;
      case '--progressive':
        progressive = true;
        break;
      case '--help':
      case '-h':
        printHelp();
        process.exit(0);
        break;
    }
  }

  return {
    scenario,
    users,
    durationMinutes,
    rampUpSeconds,
    messagesPerUser,
    thinkTimeMs,
    verbose,
    report,
    progressive,
  };
}

function printHelp(): void {
  console.log(`
PRODUCTION LOAD TEST SUITE
===========================

Usage:
  npx ts-node production-load-test.ts [options]

Options:
  --scenario, -s    Test scenario (smoke|light|medium|heavy|stress|peak_hours|rush_hour|full)
  --users, -u       Number of concurrent users
  --duration        Test duration in minutes
  --ramp            Ramp-up time in seconds
  --messages        Messages per user
  --think           Think time between messages (ms)
  --verbose, -v     Verbose output
  --report          Generate JSON report
  --progressive     Progressive stress test
  --help, -h        Show this help

Scenarios:
  smoke       - Quick validation (5 users, 2 min)
  light       - Light load (20 users, 5 min)
  medium      - Medium load (50 users, 10 min) [DEFAULT]
  heavy       - Heavy load (100 users, 15 min)
  stress      - Stress test to find limits
  peak_hours  - Peak hours simulation (80 users, 30 min)
  rush_hour   - Rush hour spike (150 users, 5 min)
  full        - Full production (200 users, 60 min)

Examples:
  npx ts-node production-load-test.ts --scenario medium
  npx ts-node production-load-test.ts --users 100 --duration 15
  npx ts-node production-load-test.ts --scenario stress --progressive
  `);
}

function getAPIConfig(): { apiBase: string; wsBase: string } {
  // Check environment variables
  const apiBase = process.env.TEST_API_BASE || 'http://localhost:8080';
  const wsBase = process.env.TEST_WS_BASE || 'ws://localhost:8080';
  return { apiBase, wsBase };
}

function randomPhone(): string {
  return `090${Math.floor(Math.random() * 10000000).toString().padStart(7, '0')}`;
}

function randomName(): string {
  const names = [
    'Nguyễn Văn An', 'Trần Thị Bình', 'Lê Hoàng Cường', 'Phạm Thu Dung',
    'Hoàng Văn Em', 'Vũ Thị Hoa', 'Đặng Minh Hùng', 'Bùi Thị Lan',
    'Trương Văn Minh', 'Ngô Thị Ngọc', 'Dương Văn Quang', 'Đỗ Thu Trang',
    'Lý Văn Sơn', 'Phan Thị Uyên', 'Cao Văn Vũ',
  ];
  return names[Math.floor(Math.random() * names.length)];
}

// ============== API FUNCTIONS ==============

async function registerGuest(apiBase: string, displayName: string): Promise<{
  success: boolean;
  session?: { session_id: string; display_name: string; guest_id: string; token: string };
  error?: string;
  duration: number;
}> {
  const startTime = Date.now();
  
  try {
    const response = await fetch(`${apiBase}/guest/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        display_name: displayName,
        phone: randomPhone(),
      }),
    });

    const duration = Date.now() - startTime;

    if (!response.ok) {
      return { success: false, error: `HTTP ${response.status}`, duration };
    }

    const data: any = await response.json();
    if (data && data.session_id) {
      return {
        success: true,
        session: {
          session_id: data.session_id,
          display_name: data.display_name || displayName,
          guest_id: data.guest_id || '',
          token: data.token || '',
        },
        duration,
      };
    }

    return { success: false, error: 'Invalid response format', duration };
  } catch (error: any) {
    return { success: false, error: error.message, duration: Date.now() - startTime };
  }
}

async function sendMessage(
  apiBase: string,
  sessionId: string,
  customerName: string,
  message: string
): Promise<{ success: boolean; messageId?: string; error?: string; duration: number }> {
  const startTime = Date.now();

  try {
    const response = await fetch(`${apiBase}/chat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        session_id: sessionId,
        customer_name: customerName,
        message: message,
        client_msg_id: `msg_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
      }),
    });

    const duration = Date.now() - startTime;

    if (!response.ok) {
      return { success: false, error: `HTTP ${response.status}`, duration };
    }

    const data: any = await response.json();
    return { success: true, messageId: data.message_id, duration };
  } catch (error: any) {
    return { success: false, error: error.message, duration: Date.now() - startTime };
  }
}

async function getChatHistory(
  apiBase: string,
  sessionId: string
): Promise<{ messages: any[]; duration: number }> {
  const startTime = Date.now();

  try {
    const response = await fetch(`${apiBase}/history/${sessionId}`);
    const duration = Date.now() - startTime;

    if (!response.ok) {
      return { messages: [], duration };
    }

    const data: any = await response.json();
    return { messages: data.messages || [], duration };
  } catch {
    return { messages: [], duration: Date.now() - startTime };
  }
}

async function agentLogin(
  apiBase: string,
  username: string,
  password: string
): Promise<{ success: boolean; token?: string; error?: string; duration: number }> {
  const startTime = Date.now();

  try {
    const response = await fetch(`${apiBase}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });

    const duration = Date.now() - startTime;

    if (!response.ok) {
      return { success: false, error: `HTTP ${response.status}`, duration };
    }

    const data: any = await response.json();
    return { success: true, token: data.token, duration };
  } catch (error: any) {
    return { success: false, error: error.message, duration: Date.now() - startTime };
  }
}

async function getCases(
  apiBase: string,
  token: string,
  status?: string
): Promise<{ success: boolean; cases: any[]; duration: number }> {
  const startTime = Date.now();
  const url = status ? `${apiBase}/cases?status=${status}` : `${apiBase}/cases`;

  try {
    const response = await fetch(url, {
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
    });

    const duration = Date.now() - startTime;

    if (!response.ok) {
      return { success: false, cases: [], duration };
    }

    const data: any = await response.json();
    return { success: true, cases: data.cases || [], duration };
  } catch {
    return { success: false, cases: [], duration: Date.now() - startTime };
  }
}

async function takeCase(
  apiBase: string,
  token: string,
  sessionId: string
): Promise<{ success: boolean; duration: number }> {
  const startTime = Date.now();

  try {
    const response = await fetch(`${apiBase}/cases/take`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ session_id: sessionId }),
    });

    return { success: response.ok, duration: Date.now() - startTime };
  } catch {
    return { success: false, duration: Date.now() - startTime };
  }
}

async function replyCase(
  apiBase: string,
  token: string,
  sessionId: string,
  message: string
): Promise<{ success: boolean; duration: number }> {
  const startTime = Date.now();

  try {
    const response = await fetch(`${apiBase}/cases/reply`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ session_id: sessionId, message }),
    });

    return { success: response.ok, duration: Date.now() - startTime };
  } catch {
    return { success: false, duration: Date.now() - startTime };
  }
}

// ============== METRICS ==============

function createMetrics(): TestMetrics {
  return {
    startTime: Date.now(),
    endTime: 0,
    totalDuration: 0,
    totalConnections: 0,
    successfulConnections: 0,
    failedConnections: 0,
    connectionErrors: [],
    totalMessages: 0,
    sentMessages: 0,
    failedMessages: 0,
    averageSendTime: 0,
    aiResponses: 0,
    aiFailures: 0,
    averageAIResponseTime: 0,
    p95AIResponseTime: 0,
    apiCalls: 0,
    apiErrors: 0,
    averageAPIResponseTime: 0,
    p95APIResponseTime: 0,
    connectionTimes: [],
    sendTimes: [],
    aiResponseTimes: [],
    apiResponseTimes: [],
    peakConcurrentUsers: 0,
    currentConcurrentUsers: 0,
    messageQueueDepth: 0,
    errors: [],
  };
}

function updateMetrics(metrics: TestMetrics, updates: Partial<TestMetrics>): void {
  Object.assign(metrics, updates);
  
  // Recalculate aggregates
  if (metrics.connectionTimes.length > 0) {
    metrics.averageAPIResponseTime = 
      metrics.connectionTimes.reduce((a, b) => a + b, 0) / metrics.connectionTimes.length;
  }
  
  if (metrics.sendTimes.length > 0) {
    metrics.averageSendTime = 
      metrics.sendTimes.reduce((a, b) => a + b, 0) / metrics.sendTimes.length;
  }
  
  if (metrics.aiResponseTimes.length > 0) {
    metrics.averageAIResponseTime = 
      metrics.aiResponseTimes.reduce((a, b) => a + b, 0) / metrics.aiResponseTimes.length;
    
    const sorted = [...metrics.aiResponseTimes].sort((a, b) => a - b);
    const p95Index = Math.floor(sorted.length * 0.95);
    metrics.p95AIResponseTime = sorted[p95Index] || sorted[sorted.length - 1];
  }
  
  if (metrics.apiResponseTimes.length > 0) {
    const sorted = [...metrics.apiResponseTimes].sort((a, b) => a - b);
    const p95Index = Math.floor(sorted.length * 0.95);
    metrics.p95APIResponseTime = sorted[p95Index] || sorted[sorted.length - 1];
  }
}

// ============== USER SIMULATION ==============

interface SimulatedUser {
  id: number;
  name: string;
  sessionId: string;
  messagesSent: number;
  startTime: number;
  isActive: boolean;
}

async function simulateUser(
  user: SimulatedUser,
  config: ScenarioConfig,
  apiBase: string,
  metrics: TestMetrics,
  onMessage: (userId: number, duration: number) => void
): Promise<void> {
  const endTime = metrics.startTime + (config.durationMinutes * 60 * 1000);
  
  while (Date.now() < endTime && user.isActive) {
    const question = TEST_QUESTIONS[user.messagesSent % TEST_QUESTIONS.length];
    
    // Send message
    const result = await sendMessage(apiBase, user.sessionId, user.name, question);
    
    if (result.success) {
      metrics.sentMessages++;
      metrics.sendTimes.push(result.duration);
      user.messagesSent++;
      onMessage(user.id, result.duration);
      
      // Wait for AI response
      const aiStart = Date.now();
      await new Promise(resolve => setTimeout(resolve, config.aiDelayMs));
      
      // Check history for AI response
      const history = await getChatHistory(apiBase, user.sessionId);
      const aiResponseTime = Date.now() - aiStart;
      
      if (history.messages.length > 0) {
        const lastMessage = history.messages[history.messages.length - 1];
        if (lastMessage.sender_type === 'ai' || lastMessage.sender_type === 'AI') {
          metrics.aiResponses++;
          metrics.aiResponseTimes.push(aiResponseTime);
        }
      }
    } else {
      metrics.failedMessages++;
      metrics.errors.push({
        timestamp: Date.now(),
        type: 'send_message',
        message: result.error || 'Unknown error',
      });
    }
    
    // Think time before next message
    const thinkVariation = Math.random() * 0.5 + 0.75; // 75-125% of thinkTime
    await new Promise(resolve => setTimeout(resolve, config.thinkTimeMs * thinkVariation));
  }
}

// ============== PROGRESSIVE STRESS TEST ==============

async function runProgressiveStressTest(
  config: ScenarioConfig,
  apiBase: string,
  onProgress: (step: number, users: number, metrics: TestMetrics) => void
): Promise<TestMetrics[]> {
  const metrics: TestMetrics[] = [];
  
  // Progressive load steps
  const steps: LoadStep[] = [
    { users: 10, durationSeconds: 30 },
    { users: 25, durationSeconds: 60 },
    { users: 50, durationSeconds: 60 },
    { users: 75, durationSeconds: 60 },
    { users: 100, durationSeconds: 90 },
    { users: 150, durationSeconds: 90 },
    { users: 200, durationSeconds: 120 },
  ];

  for (let i = 0; i < steps.length; i++) {
    const step = steps[i];
    console.log(`\n📊 STEP ${i + 1}/${steps.length}: ${step.users} users for ${step.durationSeconds}s`);
    
    const stepMetrics = await runLoadTest(
      { ...config, users: step.users, durationMinutes: step.durationSeconds / 60 },
      apiBase,
      false
    );
    
    metrics.push(stepMetrics);
    onProgress(i + 1, step.users, stepMetrics);
    
    // Brief pause between steps
    await new Promise(resolve => setTimeout(resolve, 5000));
  }
  
  return metrics;
}

// ============== BURST TEST ==============

async function runBurstTest(
  config: ScenarioConfig,
  apiBase: string
): Promise<{ success: number; failed: number; duration: number }> {
  console.log(`\n💥 BURST TEST: ${config.burstSize} messages in rapid succession`);
  
  const startTime = Date.now();
  let success = 0;
  let failed = 0;
  
  // Register one guest for burst test
  const guestResult = await registerGuest(apiBase, `BurstTest_${Date.now()}`);
  if (!guestResult.success || !guestResult.session) {
    console.log('❌ Failed to register guest for burst test');
    return { success: 0, failed: 1, duration: Date.now() - startTime };
  }
  
  const sessionId = guestResult.session.session_id;
  const displayName = guestResult.session.display_name;
  
  // Send all messages immediately
  const promises = Array.from({ length: config.burstSize }, async (_, i) => {
    const question = TEST_QUESTIONS[i % TEST_QUESTIONS.length];
    const result = await sendMessage(apiBase, sessionId, displayName, `Burst ${i + 1}: ${question}`);
    
    if (result.success) {
      return 1;
    }
    return 0;
  });
  
  const results = await Promise.all(promises);
  const successArray = results.map(r => r ? 1 : 0);
  success = successArray.reduce<number>((a, b) => a + b, 0);
  failed = results.length - success;
  
  const duration = Date.now() - startTime;
  
  console.log(`   Completed in ${duration}ms`);
  console.log(`   Success: ${success}/${config.burstSize}`);
  console.log(`   Rate: ${(success / (duration / 1000)).toFixed(1)} msg/sec`);
  
  return { success, failed, duration };
}

// ============== MAIN LOAD TEST ==============

async function runLoadTest(
  config: ScenarioConfig,
  apiBase: string,
  showProgress: boolean = true
): Promise<TestMetrics> {
  const metrics = createMetrics();
  const users: SimulatedUser[] = [];
  const userPromises: Promise<void>[] = [];
  
  if (showProgress) {
    console.log(`\n🚀 Starting load test: ${config.name}`);
    console.log(`   Users: ${config.users}`);
    console.log(`   Duration: ${config.durationMinutes} minutes`);
    console.log(`   Ramp-up: ${config.rampUpSeconds}s`);
    console.log(`   Messages/user: ${config.messagesPerUser}`);
  }
  
  // Calculate ramp-up interval
  const rampUpInterval = config.rampUpSeconds * 1000 / config.users;
  
  // Register users with ramp-up
  for (let i = 0; i < config.users; i++) {
    const userId = i + 1;
    const displayName = `LoadUser_${Date.now()}_${userId}`;
    
    // Ramp-up delay
    if (i > 0) {
      await new Promise(resolve => setTimeout(resolve, rampUpInterval));
    }
    
    const result = await registerGuest(apiBase, displayName);
    
    if (result.success && result.session) {
      const user: SimulatedUser = {
        id: userId,
        name: result.session.display_name,
        sessionId: result.session.session_id,
        messagesSent: 0,
        startTime: Date.now(),
        isActive: true,
      };
      
      users.push(user);
      metrics.successfulConnections++;
      metrics.connectionTimes.push(result.duration);
      
      if (showProgress && userId % 10 === 0) {
        console.log(`   Registered ${userId}/${config.users} users...`);
      }
    } else {
      metrics.failedConnections++;
      metrics.connectionErrors.push(result.error || 'Unknown');
    }
    
    metrics.totalConnections++;
  }
  
  if (showProgress) {
    console.log(`\n✅ Connected ${metrics.successfulConnections}/${metrics.totalConnections} users`);
  }
  
  // Update peak concurrent users
  metrics.peakConcurrentUsers = users.length;
  metrics.currentConcurrentUsers = users.length;
  
  // Start message sending
  const messageStartTime = Date.now();
  const endTime = messageStartTime + (config.durationMinutes * 60 * 1000);
  
  const sendPromises = users.map(user => simulateUser(
    user,
    config,
    apiBase,
    metrics,
    (userId, duration) => {
      metrics.apiResponseTimes.push(duration);
      metrics.apiCalls++;
    }
  ));
  
  // Show progress
  let lastProgressUpdate = Date.now();
  
  while (Date.now() < endTime) {
    await new Promise(resolve => setTimeout(resolve, 5000));
    
    const elapsed = (Date.now() - messageStartTime) / 1000;
    const totalExpected = metrics.successfulConnections * config.messagesPerUser;
    const progress = (metrics.sentMessages / totalExpected * 100).toFixed(1);
    
    if (showProgress) {
      process.stdout.write(`\r   Progress: ${progress}% | Messages: ${metrics.sentMessages}/${totalExpected} | AI Responses: ${metrics.aiResponses} | Errors: ${metrics.errors.length}     `);
    }
    
    lastProgressUpdate = Date.now();
  }
  
  if (showProgress) {
    console.log('\n');
  }
  
  // Stop all users
  users.forEach(u => u.isActive = false);
  
  // Wait for pending operations
  await Promise.race([
    Promise.all(sendPromises),
    new Promise(resolve => setTimeout(resolve, 10000)),
  ]);
  
  // Finalize metrics
  metrics.endTime = Date.now();
  metrics.totalDuration = metrics.endTime - metrics.startTime;
  
  return metrics;
}

// ============== CS AGENT SIMULATION ==============

async function runCSAgentTest(
  config: ScenarioConfig,
  apiBase: string,
  verbose: boolean
): Promise<{ casesTaken: number; replies: number; errors: number }> {
  console.log('\n👥 CS Agent Simulation Test');
  
  const agentCount = Math.min(5, Math.ceil(config.users / 20));
  const results = { casesTaken: 0, replies: 0, errors: 0 };
  
  // Simulate CS agents taking and replying to cases
  const agentPromises = Array.from({ length: agentCount }, async (_, i) => {
    const username = `cskh0${i + 1}`;
    const password = '12345678';
    
    const loginResult = await agentLogin(apiBase, username, password);
    if (!loginResult.success || !loginResult.token) {
      if (verbose) console.log(`   Agent ${username} login failed`);
      return;
    }
    
    const token = loginResult.token;
    
    // Get AI_ACTIVE cases
    const casesResult = await getCases(apiBase, token, 'ai_active');
    if (!casesResult.success || casesResult.cases.length === 0) {
      if (verbose) console.log(`   No cases available for ${username}`);
      return;
    }
    
    // Take a case
    const sessionId = casesResult.cases[0].session_id;
    const takeResult = await takeCase(apiBase, token, sessionId);
    
    if (takeResult.success) {
      results.casesTaken++;
      
      // Reply to the case
      const replyResult = await replyCase(
        apiBase,
        token,
        sessionId,
        `Xin chào, tôi là ${username}. Tôi đang hỗ trợ bạn. Vui lòng cho biết vấn đề bạn cần hỗ trợ.`
      );
      
      if (replyResult.success) {
        results.replies++;
      } else {
        results.errors++;
      }
    } else {
      results.errors++;
    }
  });
  
  await Promise.all(agentPromises);
  
  console.log(`   Cases taken: ${results.casesTaken}`);
  console.log(`   Replies sent: ${results.replies}`);
  console.log(`   Errors: ${results.errors}`);
  
  return results;
}

// ============== REPORTING ==============

function printMetricsReport(metrics: TestMetrics, config: ScenarioConfig): void {
  console.log('\n' + '='.repeat(70));
  console.log('  📊 LOAD TEST RESULTS');
  console.log('='.repeat(70));
  
  console.log(`\n  📅 Test: ${config.name}`);
  console.log(`  ⏱️  Duration: ${(metrics.totalDuration / 1000 / 60).toFixed(2)} minutes`);
  console.log(`  🕐 Started: ${new Date(metrics.startTime).toISOString()}`);
  
  console.log('\n  ──────────────────────────────────────');
  console.log('  CONNECTION METRICS');
  console.log('  ──────────────────────────────────────');
  console.log(`  Total Connections: ${metrics.totalConnections}`);
  console.log(`  ✅ Successful: ${metrics.successfulConnections} (${(metrics.successfulConnections / metrics.totalConnections * 100).toFixed(1)}%)`);
  console.log(`  ❌ Failed: ${metrics.failedConnections}`);
  console.log(`  Peak Concurrent Users: ${metrics.peakConcurrentUsers}`);
  
  if (metrics.connectionTimes.length > 0) {
    const avgConn = metrics.connectionTimes.reduce((a, b) => a + b, 0) / metrics.connectionTimes.length;
    const maxConn = Math.max(...metrics.connectionTimes);
    console.log(`  Avg Connection Time: ${avgConn.toFixed(0)}ms`);
    console.log(`  Max Connection Time: ${maxConn}ms`);
  }
  
  console.log('\n  ──────────────────────────────────────');
  console.log('  MESSAGE METRICS');
  console.log('  ──────────────────────────────────────');
  console.log(`  Total Messages: ${metrics.totalMessages}`);
  console.log(`  ✅ Sent: ${metrics.sentMessages}`);
  console.log(`  ❌ Failed: ${metrics.failedMessages}`);
  
  if (metrics.sendTimes.length > 0) {
    const avgSend = metrics.sendTimes.reduce((a, b) => a + b, 0) / metrics.sendTimes.length;
    const maxSend = Math.max(...metrics.sendTimes);
    console.log(`  Avg Send Time: ${avgSend.toFixed(0)}ms`);
    console.log(`  Max Send Time: ${maxSend}ms`);
  }
  
  console.log('\n  ──────────────────────────────────────');
  console.log('  AI RESPONSE METRICS');
  console.log('  ──────────────────────────────────────');
  console.log(`  AI Responses: ${metrics.aiResponses}`);
  console.log(`  AI Failures: ${metrics.aiFailures}`);
  
  if (metrics.aiResponseTimes.length > 0) {
    const avgAI = metrics.aiResponseTimes.reduce((a, b) => a + b, 0) / metrics.aiResponseTimes.length;
    console.log(`  Avg AI Response: ${avgAI.toFixed(0)}ms`);
    console.log(`  P95 AI Response: ${metrics.p95AIResponseTime}ms`);
  }
  
  console.log('\n  ──────────────────────────────────────');
  console.log('  API PERFORMANCE');
  console.log('  ──────────────────────────────────────');
  console.log(`  Total API Calls: ${metrics.apiCalls}`);
  console.log(`  API Errors: ${metrics.apiErrors}`);
  
  if (metrics.apiResponseTimes.length > 0) {
    const avgAPI = metrics.apiResponseTimes.reduce((a, b) => a + b, 0) / metrics.apiResponseTimes.length;
    console.log(`  Avg API Response: ${avgAPI.toFixed(0)}ms`);
    console.log(`  P95 API Response: ${metrics.p95APIResponseTime}ms`);
  }
  
  const msgPerSec = (metrics.sentMessages / (metrics.totalDuration / 1000)).toFixed(2);
  console.log(`  Throughput: ${msgPerSec} msg/sec`);
  
  if (metrics.errors.length > 0) {
    console.log('\n  ──────────────────────────────────────');
    console.log('  ERRORS');
    console.log('  ──────────────────────────────────────');
    const uniqueErrors = [...new Set(metrics.errors.map(e => e.message))];
    uniqueErrors.slice(0, 10).forEach((err, i) => {
      const count = metrics.errors.filter(e => e.message === err).length;
      console.log(`  ${i + 1}. ${err} (${count} occurrences)`);
    });
  }
  
  // Verdict
  console.log('\n  ' + '='.repeat(70));
  const successRate = (metrics.sentMessages / metrics.totalMessages * 100);
  const errorRate = (metrics.failedMessages / metrics.totalMessages * 100);
  
  let verdict: string;
  let verdictIcon: string;
  
  if (successRate >= 99 && errorRate < 1) {
    verdict = '✅ PASS - System ready for production';
    verdictIcon = '✅';
  } else if (successRate >= 95 && errorRate < 5) {
    verdict = '⚠️  CONDITIONAL PASS - Minor issues detected';
    verdictIcon = '⚠️';
  } else {
    verdict = '❌ FAIL - Issues need to be addressed';
    verdictIcon = '❌';
  }
  
  console.log(`  ${verdictIcon} VERDICT: ${verdict}`);
  console.log('='.repeat(70) + '\n');
}

function saveReport(metrics: TestMetrics, config: ScenarioConfig, csResults: any): void {
  const report = {
    test_run: {
      scenario: config.name,
      description: config.description,
      timestamp: new Date().toISOString(),
      duration_minutes: metrics.totalDuration / 1000 / 60,
    },
    load_profile: {
      concurrent_users: config.users,
      messages_per_user: config.messagesPerUser,
      think_time_ms: config.thinkTimeMs,
      ramp_up_seconds: config.rampUpSeconds,
    },
    results: {
      connections: {
        total: metrics.totalConnections,
        successful: metrics.successfulConnections,
        failed: metrics.failedConnections,
        avg_time_ms: metrics.connectionTimes.length > 0 
          ? metrics.connectionTimes.reduce((a, b) => a + b, 0) / metrics.connectionTimes.length 
          : 0,
      },
      messages: {
        total: metrics.totalMessages,
        sent: metrics.sentMessages,
        failed: metrics.failedMessages,
        avg_send_time_ms: metrics.averageSendTime,
      },
      ai: {
        responses: metrics.aiResponses,
        failures: metrics.aiFailures,
        avg_response_ms: metrics.averageAIResponseTime,
        p95_response_ms: metrics.p95AIResponseTime,
      },
      api: {
        calls: metrics.apiCalls,
        errors: metrics.apiErrors,
        avg_response_ms: metrics.averageAPIResponseTime,
        p95_response_ms: metrics.p95APIResponseTime,
      },
    },
    cs_agent_results: csResults,
    errors: metrics.errors,
    verdict: metrics.sentMessages / metrics.totalMessages >= 0.95 ? 'PASS' : 'FAIL',
  };
  
  const reportDir = path.join(__dirname, 'reports');
  if (!fs.existsSync(reportDir)) {
    fs.mkdirSync(reportDir, { recursive: true });
  }
  
  const filename = `production-load-test-${Date.now()}.json`;
  const filepath = path.join(reportDir, filename);
  
  fs.writeFileSync(filepath, JSON.stringify(report, null, 2));
  console.log(`📄 Report saved: ${filepath}`);
}

// ============== MAIN ==============

async function main() {
  console.log('\n' + '🔷'.repeat(20));
  console.log('  PRODUCTION LOAD TEST SUITE');
  console.log('🔷'.repeat(20));
  
  const args = parseArgs();
  const { apiBase, wsBase } = getAPIConfig();
  
  // Get scenario config
  const scenarioConfig = SCENARIOS[args.scenario] || SCENARIOS.medium;
  
  console.log(`\n📋 Scenario: ${scenarioConfig.name}`);
  console.log(`📝 ${scenarioConfig.description}`);
  console.log(`🌐 API: ${apiBase}`);
  console.log(`🔌 WS: ${wsBase}`);
  
  // Check server connectivity
  console.log('\n🔍 Checking server connectivity...');
  try {
    const testResponse = await fetch(`${apiBase}/health`, { method: 'GET' });
    if (testResponse.ok) {
      console.log('   ✅ Server is reachable');
    } else {
      console.log(`   ⚠️  Server returned ${testResponse.status}`);
    }
  } catch (error: any) {
    console.log(`   ❌ Cannot reach server: ${error.message}`);
    console.log('   Please ensure the server is running at', apiBase);
    process.exit(1);
  }
  
  // Run burst test first
  const burstResult = await runBurstTest(scenarioConfig, apiBase);
  
  // Run main load test
  const metrics = await runLoadTest(scenarioConfig, apiBase, true);
  
  // Run CS agent test
  const csResults = await runCSAgentTest(scenarioConfig, apiBase, args.verbose);
  
  // Print report
  printMetricsReport(metrics, scenarioConfig);
  
  // Save report if requested
  if (args.report) {
    saveReport(metrics, scenarioConfig, csResults);
  }
  
  // Run progressive stress test if requested
  if (args.progressive) {
    console.log('\n' + '💪'.repeat(35));
    console.log('  PROGRESSIVE STRESS TEST');
    console.log('💪'.repeat(35));
    
    const stressMetrics = await runProgressiveStressTest(
      scenarioConfig,
      apiBase,
      (step, users, stepMetrics) => {
        console.log(`   Step ${step}: ${users} users | Success: ${stepMetrics.sentMessages} | Errors: ${stepMetrics.errors.length}`);
      }
    );
    
    // Print stress test summary
    console.log('\n' + '='.repeat(70));
    console.log('  💪 STRESS TEST SUMMARY');
    console.log('='.repeat(70));
    
    stressMetrics.forEach((m, i) => {
      const users = [10, 25, 50, 75, 100, 150, 200][i];
      console.log(`\n  ${users} users:`);
      console.log(`    Duration: ${(m.totalDuration / 1000).toFixed(1)}s`);
      console.log(`    Messages: ${m.sentMessages}`);
      console.log(`    Success Rate: ${(m.sentMessages / m.totalMessages * 100).toFixed(1)}%`);
      console.log(`    Avg Response: ${m.averageAPIResponseTime.toFixed(0)}ms`);
    });
  }
  
  console.log('\n✅ Load test completed!\n');
}

// Run
main().catch((error) => {
  console.error('\n❌ Fatal error:', error);
  process.exit(1);
});
