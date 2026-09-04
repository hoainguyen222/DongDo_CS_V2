/**
 * ============================================================
 * AI Response Test
 * ============================================================
 * Test khả năng trả lời của AI và chất lượng response
 * 
 * Mục tiêu:
 * - Test thời gian phản hồi của AI
 * - Test độ chính xác của RAG
 * - Test fallback khi AI fail
 * - Test với các loại câu hỏi khác nhau
 */

import { TEST_CONFIG, TEST_DATA, testLogger, TestType, TestResult } from './config';
import { registerGuest } from './guest-load-test';
import { sendMessage, getChatHistory } from './message-load-test';
import { TestWSClient } from './websocket-test';

// Types
interface AIResponseTestMetrics {
  totalQuestions: number;
  successfulResponses: number;
  failedResponses: number;
  averageResponseTime: number;
  maxResponseTime: number;
  minResponseTime: number;
  responseTimeByCategory: {
    product: number[];
    trading: number[];
    account: number[];
    other: number[];
  };
  fallbackCount: number;
  questions: Array<{
    question: string;
    category: string;
    responseTime: number;
    success: boolean;
    hasContent: boolean;
    contentLength: number;
    isFallback: boolean;
    error?: string;
  }>;
}

/**
 * Phân loại câu hỏi theo category
 */
function categorizeQuestion(question: string): string {
  const lower = question.toLowerCase();
  
  if (lower.includes('phái sinh') || lower.includes('hàng hóa') || lower.includes('commodity')) {
    return 'product';
  }
  if (lower.includes('nạp tiền') || lower.includes('rút tiền') || lower.includes('tài khoản') || lower.includes('đăng ký')) {
    return 'account';
  }
  if (lower.includes('giao dịch') || lower.includes('trade') || lower.includes('mua') || lower.includes('bán')) {
    return 'trading';
  }
  if (lower.includes('ddp') || lower.includes('invest') || lower.includes('ứng dụng') || lower.includes('app')) {
    return 'product';
  }
  if (lower.includes('phí') || lower.includes('giá')) {
    return 'trading';
  }
  if (lower.includes('rủi ro') || lower.includes('quản lý') || lower.includes('an toàn')) {
    return 'trading';
  }
  
  return 'other';
}

/**
 * Chờ và lấy AI response
 */
async function waitForAIResponse(
  sessionID: string,
  timeout: number = TEST_CONFIG.AI_RESPONSE_TIMEOUT
): Promise<{
  received: boolean;
  responseTime: number;
  content: string;
  isFallback: boolean;
  message?: any;
}> {
  const startTime = Date.now();
  const pollInterval = 500;

  return new Promise((resolve) => {
    const checkInterval = setInterval(async () => {
      const elapsed = Date.now() - startTime;
      
      if (elapsed > timeout) {
        clearInterval(checkInterval);
        resolve({
          received: false,
          responseTime: elapsed,
          content: '',
          isFallback: false,
        });
        return;
      }

      const messages = await getChatHistory(sessionID);
      
      // Tìm message mới nhất từ AI
      const aiMessages = messages
        .filter(m => m.sender_type === 'ai' || m.sender_type === 'AI')
        .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());

      if (aiMessages.length > 0) {
        const latestAI = aiMessages[0];
        const isFallback = (latestAI.content as string)?.includes('chuyên viên') || 
                          (latestAI.content as string)?.includes('xin lỗi') ||
                          (latestAI.content as string)?.includes('gián đoạn');
        
        clearInterval(checkInterval);
        resolve({
          received: true,
          responseTime: Date.now() - startTime,
          content: latestAI.content,
          isFallback,
          message: latestAI,
        });
      }
    }, pollInterval);
  });
}

/**
 * Test một câu hỏi cụ thể
 */
async function testSingleQuestion(
  sessionID: string,
  customerName: string,
  question: string
): Promise<{
  question: string;
  category: string;
  responseTime: number;
  success: boolean;
  hasContent: boolean;
  contentLength: number;
  isFallback: boolean;
  error?: string;
}> {
  const category = categorizeQuestion(question);
  const startTime = Date.now();

  // Gửi câu hỏi
  const sendResult = await sendMessage(sessionID, customerName, question);
  
  if (!sendResult.success) {
    return {
      question,
      category,
      responseTime: Date.now() - startTime,
      success: false,
      hasContent: false,
      contentLength: 0,
      isFallback: false,
      error: sendResult.error || 'Failed to send message',
    };
  }

  // Chờ AI response
  const aiResponse = await waitForAIResponse(sessionID);

  return {
    question,
    category,
    responseTime: aiResponse.responseTime,
    success: aiResponse.received && aiResponse.hasContent,
    hasContent: aiResponse.received && aiResponse.content.length > 0,
    contentLength: aiResponse.content.length,
    isFallback: aiResponse.isFallback,
  };
}

/**
 * Test AI với nhiều câu hỏi khác nhau
 */
async function testAIWithVariousQuestions(): Promise<AIResponseTestMetrics> {
  testLogger.info('Testing AI with various questions...');
  
  // Tạo guest mới
  const guestResult = await registerGuest(`AITest_${Date.now()}`);
  if (!guestResult) {
    throw new Error('Failed to create guest for AI test');
  }

  const sessionID = guestResult.session.session_id;
  const customerName = guestResult.session.display_name;

  const questions = TEST_DATA.AI_TEST_QUESTIONS;
  const results: AIResponseTestMetrics['questions'] = [];
  
  // Response times by category
  const responseTimeByCategory = {
    product: [] as number[],
    trading: [] as number[],
    account: [] as number[],
    other: [] as number[],
  };

  // Test từng câu hỏi với delay
  for (const question of questions) {
    testLogger.info(`Testing question: "${question}"`);
    
    const result = await testSingleQuestion(sessionID, customerName, question);
    results.push(result);
    
    // Cập nhật response times theo category
    if (result.success) {
      responseTimeByCategory[result.category as keyof typeof responseTimeByCategory]?.push(result.responseTime);
    }
    
    // Delay giữa các câu hỏi
    await new Promise(resolve => setTimeout(resolve, TEST_CONFIG.MESSAGE_DELAY * 2));
  }

  // Tính toán metrics
  const successfulResponses = results.filter(r => r.success).length;
  const failedResponses = results.filter(r => !r.success).length;
  const successfulTimes = results.filter(r => r.success).map(r => r.responseTime);
  const fallbackCount = results.filter(r => r.isFallback).length;

  return {
    totalQuestions: questions.length,
    successfulResponses,
    failedResponses,
    averageResponseTime: successfulTimes.length > 0 
      ? successfulTimes.reduce((a, b) => a + b, 0) / successfulTimes.length 
      : 0,
    maxResponseTime: successfulTimes.length > 0 ? Math.max(...successfulTimes) : 0,
    minResponseTime: successfulTimes.length > 0 ? Math.min(...successfulTimes) : 0,
    responseTimeByCategory,
    fallbackCount,
    questions: results,
  };
}

/**
 * Test AI response time consistency
 */
async function testAIResponseTimeConsistency(): Promise<{
  samples: number[];
  avg: number;
  stdDev: number;
  min: number;
  max: number;
  p50: number;
  p95: number;
  p99: number;
}> {
  testLogger.info('Testing AI response time consistency...');
  
  // Tạo guest mới
  const guestResult = await registerGuest(`AITimeTest_${Date.now()}`);
  if (!guestResult) {
    throw new Error('Failed to create guest for AI time test');
  }

  const sessionID = guestResult.session.session_id;
  const customerName = guestResult.session.display_name;

  const samples: number[] = [];
  const testQuestion = 'Hàng hóa phái sinh là gì?';

  // Test 10 lần với cùng một câu hỏi
  for (let i = 0; i < 10; i++) {
    const startTime = Date.now();
    const sendResult = await sendMessage(sessionID, customerName, `${testQuestion} (test ${i + 1})`);
    
    if (sendResult.success) {
      const response = await waitForAIResponse(sessionID);
      samples.push(response.responseTime);
    }
    
    // Delay giữa các tests
    await new Promise(resolve => setTimeout(resolve, 3000));
  }

  // Tính toán statistics
  const sorted = [...samples].sort((a, b) => a - b);
  const avg = samples.reduce((a, b) => a + b, 0) / samples.length;
  const variance = samples.reduce((a, b) => a + Math.pow(b - avg, 2), 0) / samples.length;
  const stdDev = Math.sqrt(variance);

  return {
    samples,
    avg,
    stdDev,
    min: sorted[0],
    max: sorted[sorted.length - 1],
    p50: sorted[Math.floor(sorted.length * 0.5)],
    p95: sorted[Math.floor(sorted.length * 0.95)],
    p99: sorted[Math.floor(sorted.length * 0.99)] || sorted[sorted.length - 1],
  };
}

/**
 * Test AI với câu hỏi trigger human CS
 */
async function testAIWithHumanCSTriggers(): Promise<{
  totalTriggers: number;
  fallbacksTriggered: number;
  results: Array<{
    trigger: string;
    responseTime: number;
    isFallback: boolean;
    content: string;
  }>;
}> {
  testLogger.info('Testing AI with human CS triggers...');
  
  // Tạo guest mới
  const guestResult = await registerGuest(`HumanCSTest_${Date.now()}`);
  if (!guestResult) {
    throw new Error('Failed to create guest for human CS trigger test');
  }

  const sessionID = guestResult.session.session_id;
  const customerName = guestResult.session.display_name;

  const results: Array<{
    trigger: string;
    responseTime: number;
    isFallback: boolean;
    content: string;
  }> = [];

  for (const trigger of TEST_DATA.HUMAN_CS_TRIGGERS) {
    const sendResult = await sendMessage(sessionID, customerName, trigger);
    
    if (sendResult.success) {
      const response = await waitForAIResponse(sessionID);
      results.push({
        trigger,
        responseTime: response.responseTime,
        isFallback: response.isFallback,
        content: response.content,
      });
    }
    
    await new Promise(resolve => setTimeout(resolve, 2000));
  }

  const fallbacksTriggered = results.filter(r => r.isFallback).length;

  return {
    totalTriggers: TEST_DATA.HUMAN_CS_TRIGGERS.length,
    fallbacksTriggered,
    results,
  };
}

/**
 * Test AI với concurrent requests
 */
async function testAIConcurrentRequests(requestCount: number): Promise<{
  totalRequests: number;
  successfulResponses: number;
  failedResponses: number;
  averageResponseTime: number;
  longestResponseTime: number;
  shortestResponseTime: number;
}> {
  testLogger.info(`Testing AI with ${requestCount} concurrent requests...`);
  
  // Tạo nhiều guests
  const guests = await Promise.all(
    Array.from({ length: requestCount }, async (_, i) => {
      const result = await registerGuest(`AIConcurrent_${Date.now()}_${i}`);
      return result?.session || null;
    })
  );

  const validGuests = guests.filter(g => g !== null);
  testLogger.info(`Created ${validGuests.length} guests for concurrent AI test`);

  // Gửi câu hỏi đồng thời từ tất cả guests
  const testQuestion = TEST_DATA.AI_TEST_QUESTIONS[0];
  
  const promises = validGuests.map(async (guest) => {
    const startTime = Date.now();
    const sendResult = await sendMessage(guest!.session_id, guest!.display_name, testQuestion);
    
    if (!sendResult.success) {
      return { success: false, responseTime: 0 };
    }

    const response = await waitForAIResponse(guest!.session_id);
    return { 
      success: response.received, 
      responseTime: response.responseTime 
    };
  });

  const results = await Promise.all(promises);

  const successfulResponses = results.filter(r => r.success).length;
  const responseTimes = results.filter(r => r.success).map(r => r.responseTime);

  return {
    totalRequests: requestCount,
    successfulResponses,
    failedResponses: results.filter(r => !r.success).length,
    averageResponseTime: responseTimes.length > 0 
      ? responseTimes.reduce((a, b) => a + b, 0) / responseTimes.length 
      : 0,
    longestResponseTime: responseTimes.length > 0 ? Math.max(...responseTimes) : 0,
    shortestResponseTime: responseTimes.length > 0 ? Math.min(...responseTimes) : 0,
  };
}

/**
 * Chạy AI response test chính
 */
export async function runAIResponseTest(options?: {
  testConsistency?: boolean;
  testHumanCS?: boolean;
  testConcurrency?: boolean;
  concurrencyCount?: number;
}): Promise<TestResult> {
  const startTime = Date.now();
  const { 
    testConsistency = true, 
    testHumanCS = true, 
    testConcurrency = true,
    concurrencyCount = 10,
  } = options || {};

  try {
    const details: any = {};

    // Test 1: Various questions
    testLogger.info('=== Test 1: AI with various questions ===');
    const metrics = await testAIWithVariousQuestions();
    details.variousQuestions = metrics;
    testLogger.success(`Questions: ${metrics.successfulResponses}/${metrics.totalQuestions} successful`);
    testLogger.success(`Avg response time: ${metrics.averageResponseTime.toFixed(0)}ms`);
    testLogger.success(`Fallback triggered: ${metrics.fallbackCount} times`);

    // Test 2: Response time consistency
    if (testConsistency) {
      testLogger.info('=== Test 2: AI response time consistency ===');
      const consistency = await testAIResponseTimeConsistency();
      details.consistency = consistency;
      testLogger.success(`Avg: ${consistency.avg.toFixed(0)}ms, StdDev: ${consistency.stdDev.toFixed(0)}ms`);
      testLogger.success(`P50: ${consistency.p50}ms, P95: ${consistency.p95}ms, P99: ${consistency.p99}ms`);
    }

    // Test 3: Human CS triggers
    if (testHumanCS) {
      testLogger.info('=== Test 3: AI with human CS triggers ===');
      const humanCS = await testAIWithHumanCSTriggers();
      details.humanCSTriggers = humanCS;
      testLogger.success(`Fallbacks triggered: ${humanCS.fallbacksTriggered}/${humanCS.totalTriggers}`);
    }

    // Test 4: Concurrent requests
    if (testConcurrency) {
      testLogger.info('=== Test 4: AI with concurrent requests ===');
      const concurrent = await testAIConcurrentRequests(concurrencyCount);
      details.concurrentRequests = concurrent;
      testLogger.success(`${concurrent.successfulResponses}/${concurrent.totalRequests} successful`);
      testLogger.success(`Avg response time: ${concurrent.averageResponseTime.toFixed(0)}ms`);
    }

    const duration = Date.now() - startTime;
    const success = metrics.successfulResponses > 0;

    return {
      testType: TestType.AI_RESPONSE,
      success,
      duration,
      metrics: {
        total: metrics.totalQuestions,
        success: metrics.successfulResponses,
        failed: metrics.failedResponses,
        errors: [],
      },
      details,
    };
  } catch (error: any) {
    testLogger.error('AI response test failed:', error);
    return {
      testType: TestType.AI_RESPONSE,
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
export { testSingleQuestion, waitForAIResponse };
