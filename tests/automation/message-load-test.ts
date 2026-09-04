/**
 * ============================================================
 * Message Concurrency Load Test
 * ============================================================
 * Test số lượng message có thể gửi đồng thời
 * 
 * Mục tiêu:
 * - Đo lường throughput của hệ thống chat
 * - Test tốc độ AI response khi có nhiều message
 * - Kiểm tra độ trễ (latency) của hệ thống
 */

import { TEST_CONFIG, TEST_DATA, testLogger, TestType, TestResult } from './config';
import { registerGuest } from './guest-load-test';

// Types
interface Message {
  id: number;
  session_id: string;
  sender_type: string;
  sender_id: string;
  content: string;
  client_msg_id?: string;
  created_at: string;
}

interface MessageTestMetrics {
  totalMessages: number;
  successfulSends: number;
  failedSends: number;
  averageSendTime: number;
  maxSendTime: number;
  minSendTime: number;
  averageAIReplyTime: number;
  maxAIReplyTime: number;
  messageDetails: Array<{
    content: string;
    sessionId: string;
    sendSuccess: boolean;
    sendTime: number;
    aiReplyTime?: number;
    hasAIReply: boolean;
    error?: string;
  }>;
}

/**
 * Gửi một message qua API
 */
async function sendMessage(
  sessionID: string,
  customerName: string,
  content: string,
  clientMsgID?: string
): Promise<{ success: boolean; duration: number; error?: string }> {
  const startTime = Date.now();

  try {
    const response = await fetch(`${TEST_CONFIG.API_BASE}/chat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        session_id: sessionID,
        customer_name: customerName,
        message: content,
        client_msg_id: clientMsgID || `msg_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
      }),
    });

    const duration = Date.now() - startTime;

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    return { success: true, duration };
  } catch (error: any) {
    return { 
      success: false, 
      duration: Date.now() - startTime,
      error: error.message 
    };
  }
}

/**
 * Lấy lịch sử chat
 */
async function getChatHistory(sessionID: string): Promise<Message[]> {
  try {
    const response = await fetch(`${TEST_CONFIG.API_BASE}/history/${sessionID}`);
    if (!response.ok) return [];
    
    const data = await response.json();
    return data.messages || [];
  } catch {
    return [];
  }
}

/**
 * Chờ và kiểm tra AI response
 */
async function waitForAIResponse(
  sessionID: string,
  timeout: number = TEST_CONFIG.AI_RESPONSE_TIMEOUT
): Promise<{ received: boolean; replyTime: number; message?: Message }> {
  const startTime = Date.now();
  const pollInterval = 500;
  
  // Lấy số message ban đầu
  const initialMessages = await getChatHistory(sessionID);
  const initialCount = initialMessages.length;

  return new Promise((resolve) => {
    const checkInterval = setInterval(async () => {
      const elapsed = Date.now() - startTime;
      
      if (elapsed > timeout) {
        clearInterval(checkInterval);
        resolve({ received: false, replyTime: elapsed });
        return;
      }

      const currentMessages = await getChatHistory(sessionID);
      
      // Kiểm tra có message mới từ AI không
      if (currentMessages.length > initialCount) {
        const newMessages = currentMessages.slice(initialCount);
        const aiMessage = newMessages.find(m => m.sender_type === 'ai' || m.sender_type === 'AI');
        
        if (aiMessage) {
          clearInterval(checkInterval);
          resolve({ 
            received: true, 
            replyTime: Date.now() - startTime,
            message: aiMessage,
          });
        }
      }
    }, pollInterval);
  });
}

/**
 * Test gửi nhiều message đồng thời từ một guest
 */
async function testConcurrentMessagesFromOneGuest(
  sessionID: string,
  customerName: string,
  count: number
): Promise<MessageTestMetrics> {
  testLogger.info(`Testing ${count} concurrent messages from one guest...`);
  
  const messages = TEST_DATA.AI_TEST_QUESTIONS.slice(0, count);
  const results: MessageTestMetrics['messageDetails'] = [];

  // Gửi tất cả message đồng thời
  const promises = messages.map(async (content) => {
    const sendResult = await sendMessage(sessionID, customerName, content);
    
    return {
      content,
      sessionId: sessionID,
      sendSuccess: sendResult.success,
      sendTime: sendResult.duration,
      hasAIReply: false,
      error: sendResult.error,
    };
  });

  const sendResults = await Promise.all(promises);
  results.push(...sendResults);

  // Tính toán metrics
  const successfulSends = sendResults.filter(r => r.sendSuccess).length;
  const failedSends = sendResults.filter(r => !r.sendSuccess).length;
  const sendTimes = sendResults.filter(r => r.sendSuccess).map(r => r.sendTime);

  // Chờ một chút để AI xử lý
  await new Promise(resolve => setTimeout(resolve, 2000));

  // Kiểm tra AI responses
  const finalMessages = await getChatHistory(sessionID);
  const aiMessages = finalMessages.filter(m => m.sender_type === 'ai' || m.sender_type === 'AI');

  return {
    totalMessages: count,
    successfulSends,
    failedSends,
    averageSendTime: sendTimes.length > 0 ? sendTimes.reduce((a, b) => a + b, 0) / sendTimes.length : 0,
    maxSendTime: sendTimes.length > 0 ? Math.max(...sendTimes) : 0,
    minSendTime: sendTimes.length > 0 ? Math.min(...sendTimes) : 0,
    averageAIReplyTime: 0, // Sẽ được tính sau nếu cần
    maxAIReplyTime: 0,
    messageDetails: results.map(r => ({ ...r, hasAIReply: aiMessages.length > 0 })),
  };
}

/**
 * Test nhiều guest gửi message đồng thời
 */
async function testMultiGuestConcurrentMessages(guestCount: number, messagesPerGuest: number): Promise<{
  totalMessages: number;
  successfulSends: number;
  failedSends: number;
  guestResults: Array<{
    guestName: string;
    sessionId: string;
    messages: number;
    success: number;
    failed: number;
  }>;
}> {
  testLogger.info(`Testing ${guestCount} guests sending ${messagesPerGuest} messages each...`);

  // Đăng ký nhiều guest
  const guests = await Promise.all(
    Array.from({ length: guestCount }, async (_, i) => {
      const result = await registerGuest(`MultiGuest_${Date.now()}_${i}`);
      return result?.session || null;
    })
  );

  const validGuests = guests.filter(g => g !== null);
  testLogger.info(`Registered ${validGuests.length} guests successfully`);

  // Mỗi guest gửi message
  const guestPromises = validGuests.map(async (guest) => {
    const results: Array<{ success: boolean; duration: number }> = [];
    
    for (let i = 0; i < messagesPerGuest; i++) {
      const content = TEST_DATA.AI_TEST_QUESTIONS[i % TEST_DATA.AI_TEST_QUESTIONS.length];
      const result = await sendMessage(guest!.session_id, guest!.display_name, `${content} [Guest ${guest!.display_name}]`);
      results.push(result);
      
      // Small delay between messages
      await new Promise(resolve => setTimeout(resolve, TEST_CONFIG.MESSAGE_DELAY));
    }

    return {
      guestName: guest!.display_name,
      sessionId: guest!.session_id,
      messages: messagesPerGuest,
      success: results.filter(r => r.success).length,
      failed: results.filter(r => !r.success).length,
    };
  });

  const guestResults = await Promise.all(guestPromises);

  // Tổng hợp kết quả
  return {
    totalMessages: guestCount * messagesPerGuest,
    successfulSends: guestResults.reduce((a, g) => a + g.success, 0),
    failedSends: guestResults.reduce((a, g) => a + g.failed, 0),
    guestResults,
  };
}

/**
 * Test burst message (gửi liên tục không delay)
 */
async function testBurstMessages(sessionID: string, customerName: string, count: number): Promise<{
  totalSent: number;
  successfulSends: number;
  failedSends: number;
  avgTime: number;
}> {
  testLogger.info(`Testing burst of ${count} messages...`);
  
  const promises = Array.from({ length: count }, async (_, i) => {
    const content = `Burst message ${i + 1}: ${TEST_DATA.AI_TEST_QUESTIONS[i % TEST_DATA.AI_TEST_QUESTIONS.length]}`;
    return sendMessage(sessionID, customerName, content);
  });

  const results = await Promise.all(promises);
  
  return {
    totalSent: count,
    successfulSends: results.filter(r => r.success).length,
    failedSends: results.filter(r => !r.success).length,
    avgTime: results.filter(r => r.success).reduce((a, b) => a + b.duration, 0) / results.filter(r => r.success).length,
  };
}

/**
 * Chạy message concurrency test chính
 */
export async function runMessageConcurrencyTest(options?: {
  count?: number;
  multiGuest?: boolean;
  guestCount?: number;
  burst?: boolean;
}): Promise<TestResult> {
  const startTime = Date.now();
  const { count = 10, multiGuest = false, guestCount = 5, burst = false } = options || {};

  try {
    let metrics: any;

    if (multiGuest) {
      metrics = await testMultiGuestConcurrentMessages(guestCount, Math.ceil(count / guestCount));
    } else if (burst) {
      // Tạo guest mới cho burst test
      const guestResult = await registerGuest(`BurstTest_${Date.now()}`);
      if (!guestResult) {
        throw new Error('Failed to create guest for burst test');
      }
      metrics = await testBurstMessages(guestResult.session.session_id, guestResult.session.display_name, count);
    } else {
      // Tạo guest mới cho test
      const guestResult = await registerGuest(`MsgTest_${Date.now()}`);
      if (!guestResult) {
        throw new Error('Failed to create guest for message test');
      }
      metrics = await testConcurrentMessagesFromOneGuest(
        guestResult.session.session_id,
        guestResult.session.display_name,
        count
      );
    }

    const duration = Date.now() - startTime;
    const success = metrics.successfulSends > 0;

    return {
      testType: TestType.MESSAGE_CONCURRENCY,
      success,
      duration,
      metrics: {
        total: metrics.totalMessages || metrics.totalSent || count,
        success: metrics.successfulSends,
        failed: metrics.failedSends,
        errors: [],
      },
      details: metrics,
    };
  } catch (error: any) {
    testLogger.error('Message concurrency test failed:', error);
    return {
      testType: TestType.MESSAGE_CONCURRENCY,
      success: false,
      duration: Date.now() - startTime,
      metrics: {
        total: count,
        success: 0,
        failed: count,
        errors: [error.message],
      },
    };
  }
}

// Export functions for use in other tests
export { sendMessage, getChatHistory, waitForAIResponse };
