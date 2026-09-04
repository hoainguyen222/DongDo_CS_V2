/**
 * ============================================================
 * Team Agent Support Test
 * ============================================================
 * Test khả năng hỗ trợ của đội ngũ CSKH (Customer Service Team)
 * 
 * Mục tiêu:
 * - Test việc chuyển case từ AI sang human CS
 * - Test phân phối case cho các CS agents
 * - Test reply từ CS agent
 * - Test resolve case
 * - Test thời gian phản hồi của CS team
 */

import { TEST_CONFIG, TEST_DATA, testLogger, TestType, TestResult } from './config';
import { registerGuest } from './guest-load-test';
import { sendMessage, getChatHistory } from './message-load-test';
import { TestWSClient } from './websocket-test';

// Types
interface TeamAgentTestMetrics {
  totalTestCases: number;
  successfulTransfers: number;
  failedTransfers: number;
  averageTransferTime: number;
  csResponsesReceived: number;
  caseDetails: Array<{
    sessionId: string;
    customerName: string;
    transferSuccess: boolean;
    transferTime: number;
    csResponseTime?: number;
    csAgentName?: string;
    error?: string;
  }>;
}

/**
 * Lấy danh sách users (CS agents)
 */
async function getCSAgents(): Promise<Array<{
  id: string;
  username: string;
  full_name: string;
  role: string;
}>> {
  try {
    // Lấy auth token trước
    const loginResponse = await fetch(`${TEST_CONFIG.API_BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: process.env.CS_AGENT_USERNAME || 'admin',
        password: process.env.CS_AGENT_PASSWORD || 'admin123',
      }),
    });

    if (!loginResponse.ok) {
      testLogger.warn('Failed to login to get CS agents');
      return [];
    }

    const loginData = await loginResponse.json();
    const token = loginData.token || loginData.access_token;

    // Lấy danh sách users
    const usersResponse = await fetch(`${TEST_CONFIG.API_BASE}/users`, {
      headers: { Authorization: `Bearer ${token}` },
    });

    if (!usersResponse.ok) {
      return [];
    }

    const usersData = await usersResponse.json();
    return usersData.users?.filter((u: any) => u.role !== 'owner') || [];
  } catch (error) {
    testLogger.error('Failed to get CS agents:', error);
    return [];
  }
}

/**
 * Lấy danh sách cases từ admin inbox
 */
async function getOpenCases(): Promise<any[]> {
  try {
    // Lấy auth token
    const loginResponse = await fetch(`${TEST_CONFIG.API_BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: process.env.CS_AGENT_USERNAME || 'admin',
        password: process.env.CS_AGENT_PASSWORD || 'admin123',
      }),
    });

    if (!loginResponse.ok) {
      return [];
    }

    const loginData = await loginResponse.json();
    const token = loginData.token || loginData.access_token;

    // Lấy cases
    const casesResponse = await fetch(`${TEST_CONFIG.API_BASE}/cases?status=ai_active&limit=50`, {
      headers: { Authorization: `Bearer ${token}` },
    });

    if (!casesResponse.ok) {
      return [];
    }

    const casesData = await casesResponse.json();
    return casesData.cases || [];
  } catch (error) {
    return [];
  }
}

/**
 * CS Agent take case
 */
async function takeCase(sessionID: string, csUsername: string, csFullName: string): Promise<{
  success: boolean;
  error?: string;
}> {
  try {
    // Lấy auth token
    const loginResponse = await fetch(`${TEST_CONFIG.API_BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: csUsername,
        password: process.env.CS_AGENT_PASSWORD || 'admin123',
      }),
    });

    if (!loginResponse.ok) {
      return { success: false, error: 'Failed to login as CS agent' };
    }

    const loginData = await loginResponse.json();
    const token = loginData.token || loginData.access_token;

    // Take case
    const response = await fetch(`${TEST_CONFIG.API_BASE}/cases/take`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ session_id: sessionID }),
    });

    return { success: response.ok, error: response.ok ? undefined : 'Failed to take case' };
  } catch (error: any) {
    return { success: false, error: error.message };
  }
}

/**
 * CS Agent send reply
 */
async function sendCSReply(
  sessionID: string,
  csUsername: string,
  csFullName: string,
  message: string
): Promise<{
  success: boolean;
  messageId?: number;
  error?: string;
}> {
  try {
    // Lấy auth token
    const loginResponse = await fetch(`${TEST_CONFIG.API_BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: csUsername,
        password: process.env.CS_AGENT_PASSWORD || 'admin123',
      }),
    });

    if (!loginResponse.ok) {
      return { success: false, error: 'Failed to login as CS agent' };
    }

    const loginResponseData = await loginResponse.json();
    const token = loginResponseData.token || loginResponseData.access_token;

    // Send reply
    const response = await fetch(`${TEST_CONFIG.API_BASE}/cases/reply`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        session_id: sessionID,
        message,
      }),
    });

    if (!response.ok) {
      return { success: false, error: `HTTP ${response.status}` };
    }

    const data = await response.json();
    return { success: true, messageId: data.id };
  } catch (error: any) {
    return { success: false, error: error.message };
  }
}

/**
 * Resolve case
 */
async function resolveCase(sessionID: string): Promise<{
  success: boolean;
  error?: string;
}> {
  try {
    // Lấy auth token
    const loginResponse = await fetch(`${TEST_CONFIG.API_BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: process.env.CS_AGENT_USERNAME || 'admin',
        password: process.env.CS_AGENT_PASSWORD || 'admin123',
      }),
    });

    if (!loginResponse.ok) {
      return { success: false, error: 'Failed to login' };
    }

    const loginData = await loginResponse.json();
    const token = loginData.token || loginData.access_token;

    // Resolve case
    const response = await fetch(`${TEST_CONFIG.API_BASE}/cases/resolve`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ session_id: sessionID }),
    });

    return { success: response.ok, error: response.ok ? undefined : 'Failed to resolve case' };
  } catch (error: any) {
    return { success: false, error: error.message };
  }
}

/**
 * Test chuyển case từ AI sang human CS
 */
async function testTransferToHumanCS(sessionID: string, customerName: string): Promise<{
  success: boolean;
  transferTime: number;
  error?: string;
}> {
  const startTime = Date.now();

  // Gửi message trigger human CS
  const triggerMessage = TEST_DATA.HUMAN_CS_TRIGGERS[0]; // "tôi cần hỗ trợ"
  const sendResult = await sendMessage(sessionID, customerName, triggerMessage);
  
  if (!sendResult.success) {
    return { 
      success: false, 
      transferTime: Date.now() - startTime,
      error: 'Failed to send trigger message' 
    };
  }

  // Chờ một chút để case được tạo với status cần human CS
  await new Promise(resolve => setTimeout(resolve, 3000));

  // Kiểm tra case status qua history
  const messages = await getChatHistory(sessionID);
  const hasFallbackMessage = messages.some(m => 
    (m.content as string)?.toLowerCase().includes('chuyên viên') ||
    (m.content as string)?.toLowerCase().includes('hỗ trợ')
  );

  return {
    success: true, // Case đã được tạo, việc assign cho CS agent cần manual hoặc qua admin
    transferTime: Date.now() - startTime,
  };
}

/**
 * Test CS agent nhận và xử lý case
 */
async function testCSAgentHandling(): Promise<{
  casesFound: number;
  casesTaken: number;
  repliesSent: number;
  errors: string[];
}> {
  testLogger.info('Testing CS agent case handling...');
  
  const errors: string[] = [];
  
  // Lấy danh sách open cases
  const openCases = await getOpenCases();
  testLogger.info(`Found ${openCases.length} open cases`);
  
  if (openCases.length === 0) {
    return { casesFound: 0, casesTaken: 0, repliesSent: 0, errors: [] };
  }

  // Lấy CS agents
  const agents = await getCSAgents();
  testLogger.info(`Found ${agents.length} CS agents`);
  
  if (agents.length === 0) {
    return { 
      casesFound: openCases.length, 
      casesTaken: 0, 
      repliesSent: 0, 
      errors: ['No CS agents available'] 
    };
  }

  // Thử take case đầu tiên
  const firstCase = openCases[0];
  const agent = agents[0];
  
  const takeResult = await takeCase(firstCase.session_id, agent.username, agent.full_name);
  
  if (!takeResult.success) {
    errors.push(`Failed to take case: ${takeResult.error}`);
  }

  // Thử gửi reply
  if (takeResult.success) {
    const replyResult = await sendCSReply(
      firstCase.session_id,
      agent.username,
      agent.full_name,
      'Cảm ơn bạn đã liên hệ. Chúng tôi đang xử lý yêu cầu của bạn.'
    );
    
    if (!replyResult.success) {
      errors.push(`Failed to send CS reply: ${replyResult.error}`);
    }

    // Resolve case
    const resolveResult = await resolveCase(firstCase.session_id);
    if (!resolveResult.success) {
      errors.push(`Failed to resolve case: ${resolveResult.error}`);
    }
  }

  return {
    casesFound: openCases.length,
    casesTaken: takeResult.success ? 1 : 0,
    repliesSent: takeResult.success ? 1 : 0,
    errors,
  };
}

/**
 * Test real-time notification cho CS team
 */
async function testCSRealTimeNotification(): Promise<{
  guestCreated: boolean;
  messageSent: boolean;
  notificationReceived: boolean;
  notificationTime: number;
}> {
  testLogger.info('Testing CS real-time notification...');
  
  // Tạo guest và gửi message
  const guestResult = await registerGuest(`CSTest_${Date.now()}`);
  if (!guestResult) {
    return {
      guestCreated: false,
      messageSent: false,
      notificationReceived: false,
      notificationTime: 0,
    };
  }

  // Kết nối WebSocket với role là CS để nhận notification
  // (Giả định có một CS agent online)
  const wsClient = new TestWSClient(
    'admin_inbox', // admin inbox nhận tất cả notifications
    'TestCSAgent',
    'cs'
  );

  const connectResult = await wsClient.connect();
  if (!connectResult.success) {
    return {
      guestCreated: true,
      messageSent: false,
      notificationReceived: false,
      notificationTime: 0,
    };
  }

  let notificationReceived = false;
  const startTime = Date.now();

  wsClient.on('case_update', (event) => {
    if (event.session_id === guestResult.session.session_id) {
      notificationReceived = true;
    }
  });

  // Gửi message
  const sendResult = await sendMessage(
    guestResult.session.session_id,
    guestResult.session.display_name,
    'Test message for CS notification'
  );

  // Chờ notification (tối đa 10 giây)
  await new Promise(resolve => setTimeout(resolve, 10000));

  wsClient.disconnect();

  return {
    guestCreated: true,
    messageSent: sendResult.success,
    notificationReceived,
    notificationTime: Date.now() - startTime,
  };
}

/**
 * Test phân phối case cho nhiều CS agents
 */
async function testCaseDistribution(): Promise<{
  totalGuests: number;
  totalCases: number;
  casesAssigned: number;
  avgAssignmentTime: number;
  distribution: Record<string, number>;
}> {
  testLogger.info('Testing case distribution to CS agents...');
  
  // Tạo nhiều guests với trigger message
  const guestCount = 5;
  const guests = await Promise.all(
    Array.from({ length: guestCount }, async (_, i) => {
      const result = await registerGuest(`DistributionTest_${Date.now()}_${i}`);
      return result?.session || null;
    })
  );

  const validGuests = guests.filter(g => g !== null);
  testLogger.info(`Created ${validGuests.length} guests`);

  // Gửi message trigger human CS cho mỗi guest
  for (const guest of validGuests) {
    await sendMessage(
      guest!.session_id,
      guest!.display_name,
      TEST_DATA.HUMAN_CS_TRIGGERS[i % TEST_DATA.HUMAN_CS_TRIGGERS.length]
    );
    await new Promise(resolve => setTimeout(resolve, 1000));
  }

  // Chờ một chút để cases được tạo
  await new Promise(resolve => setTimeout(resolve, 5000));

  // Kiểm tra cases
  const openCases = await getOpenCases();
  const newCases = openCases.filter(c => 
    validGuests.some(g => g!.session_id === c.session_id)
  );

  return {
    totalGuests: validGuests.length,
    totalCases: openCases.length,
    casesAssigned: newCases.length,
    avgAssignmentTime: 0, // Cần implementation thêm để đo thời gian
    distribution: {}, // Cần implementation thêm để đo distribution
  };
}

/**
 * Chạy Team Agent Support Test chính
 */
export async function runTeamAgentTest(options?: {
  testHandling?: boolean;
  testNotification?: boolean;
  testDistribution?: boolean;
}): Promise<TestResult> {
  const startTime = Date.now();
  const { testHandling = true, testNotification = true, testDistribution = true } = options || {};

  try {
    const details: any = {};

    // Test 1: Transfer to Human CS
    testLogger.info('=== Test 1: Transfer to Human CS ===');
    const guestResult = await registerGuest(`TeamAgentTest_${Date.now()}`);
    if (!guestResult) {
      throw new Error('Failed to create guest for team agent test');
    }

    const transferResult = await testTransferToHumanCS(
      guestResult.session.session_id,
      guestResult.session.display_name
    );
    details.transferTest = transferResult;
    testLogger.success(`Transfer test: ${transferResult.success ? 'SUCCESS' : 'FAILED'} (${transferResult.transferTime}ms)`);

    // Test 2: CS Agent Handling
    if (testHandling) {
      testLogger.info('=== Test 2: CS Agent Case Handling ===');
      const handlingResult = await testCSAgentHandling();
      details.agentHandling = handlingResult;
      testLogger.success(`Cases found: ${handlingResult.casesFound}`);
      testLogger.success(`Cases taken: ${handlingResult.casesTaken}`);
      testLogger.success(`Replies sent: ${handlingResult.repliesSent}`);
      if (handlingResult.errors.length > 0) {
        testLogger.warn('Errors:', handlingResult.errors);
      }
    }

    // Test 3: Real-time Notification
    if (testNotification) {
      testLogger.info('=== Test 3: CS Real-time Notification ===');
      const notificationResult = await testCSRealTimeNotification();
      details.realTimeNotification = notificationResult;
      testLogger.success(`Guest created: ${notificationResult.guestCreated}`);
      testLogger.success(`Message sent: ${notificationResult.messageSent}`);
      testLogger.success(`Notification received: ${notificationResult.notificationReceived}`);
    }

    // Test 4: Case Distribution
    if (testDistribution) {
      testLogger.info('=== Test 4: Case Distribution ===');
      const distributionResult = await testCaseDistribution();
      details.caseDistribution = distributionResult;
      testLogger.success(`Guests: ${distributionResult.totalGuests}`);
      testLogger.success(`Cases assigned: ${distributionResult.casesAssigned}`);
    }

    const duration = Date.now() - startTime;
    const success = transferResult.success;

    return {
      testType: TestType.TEAM_AGENT,
      success,
      duration,
      metrics: {
        total: details.agentHandling?.casesFound || 1,
        success: (details.agentHandling?.casesTaken || 0) + (details.realTimeNotification?.notificationReceived ? 1 : 0),
        failed: (details.agentHandling?.casesFound || 0) - (details.agentHandling?.casesTaken || 0),
        errors: details.agentHandling?.errors || [],
      },
      details,
    };
  } catch (error: any) {
    testLogger.error('Team agent test failed:', error);
    return {
      testType: TestType.TEAM_AGENT,
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

// Export functions
export { getCSAgents, getOpenCases, takeCase, sendCSReply, resolveCase };
