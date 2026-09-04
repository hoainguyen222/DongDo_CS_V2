/**
 * ============================================================
 * Guest Concurrency Load Test
 * ============================================================
 * Test số lượng guest có thể chat đồng thời
 * 
 * Mục tiêu:
 * - Đo lường số lượng guest tối đa có thể kết nối cùng lúc
 * - Test tốc độ response khi có nhiều guest online
 * - Phát hiện bottleneck trong hệ thống
 */

import { TEST_CONFIG, TEST_DATA, testLogger, TestType, TestResult } from './config';

// Types
interface GuestSession {
  session_id: string;
  display_name: string;
  guest_id: string;
  created_at: string;
}

interface GuestTestMetrics {
  totalGuests: number;
  successfulConnections: number;
  failedConnections: number;
  averageConnectionTime: number;
  maxConnectionTime: number;
  minConnectionTime: number;
  sessionDetails: Array<{
    name: string;
    sessionId: string;
    success: boolean;
    connectionTime: number;
    error?: string;
  }>;
}

/**
 * Đăng ký một guest mới
 */
async function registerGuest(displayName: string): Promise<{ session: GuestSession; duration: number } | null> {
  const startTime = Date.now();
  
  try {
    const response = await fetch(`${TEST_CONFIG.API_BASE}/guest/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ 
        display_name: displayName, 
        phone: `0909000${Math.floor(Math.random() * 10000).toString().padStart(4, '0')}` 
      }),
    });

    const duration = Date.now() - startTime;

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    const data = await response.json();
    return { session: data, duration };
  } catch (error: any) {
    testLogger.error(`Failed to register guest ${displayName}:`, error.message);
    return null;
  }
}

/**
 * Test đăng ký nhiều guest đồng thời
 */
async function testConcurrentGuestRegistration(count: number): Promise<GuestTestMetrics> {
  testLogger.info(`Starting concurrent guest registration test with ${count} guests...`);
  
  const startTime = Date.now();
  const results: Array<{ name: string; sessionId: string; success: boolean; connectionTime: number; error?: string }> = [];
  
  // Tạo danh sách guest names
  const guestNames = Array.from({ length: count }, (_, i) => `LoadTest_Guest_${Date.now()}_${i}`);
  
  // Đăng ký tất cả guest đồng thời
  const promises = guestNames.map(async (name, index) => {
    const result = await registerGuest(name);
    
    if (result) {
      return {
        name,
        sessionId: result.session.session_id,
        success: true,
        connectionTime: result.duration,
      };
    } else {
      return {
        name,
        sessionId: '',
        success: false,
        connectionTime: Date.now() - startTime,
        error: 'Registration failed',
      };
    }
  });

  const settledResults = await Promise.all(promises);
  results.push(...settledResults);

  const totalDuration = Date.now() - startTime;
  
  // Tính toán metrics
  const successfulConnections = results.filter(r => r.success).length;
  const failedConnections = results.filter(r => !r.success).length;
  const connectionTimes = results.filter(r => r.success).map(r => r.connectionTime);
  
  const metrics: GuestTestMetrics = {
    totalGuests: count,
    successfulConnections,
    failedConnections,
    averageConnectionTime: connectionTimes.length > 0 
      ? connectionTimes.reduce((a, b) => a + b, 0) / connectionTimes.length 
      : 0,
    maxConnectionTime: connectionTimes.length > 0 ? Math.max(...connectionTimes) : 0,
    minConnectionTime: connectionTimes.length > 0 ? Math.min(...connectionTimes) : 0,
    sessionDetails: results,
  };

  return metrics;
}

/**
 * Test gradual increase để tìm điểm giới hạn
 */
async function testGradualGuestIncrease(): Promise<{
  maxGuestsReached: number;
  steps: Array<{ count: number; success: number; failed: number; avgTime: number }>;
}> {
  testLogger.info('Starting gradual guest increase test...');
  
  const steps = [];
  const stepSizes = [5, 10, 20, 30, 40, 50];
  
  for (const count of stepSizes) {
    if (count > TEST_CONFIG.MAX_CONCURRENT_GUESTS) break;
    
    const metrics = await testConcurrentGuestRegistration(count);
    
    steps.push({
      count,
      success: metrics.successfulConnections,
      failed: metrics.failedConnections,
      avgTime: Math.round(metrics.averageConnectionTime),
    });
    
    testLogger.info(`Step ${count} guests: ${metrics.successfulConnections} success, ${metrics.failedConnections} failed`);
    
    // Nếu có quá nhiều failed, dừng lại
    if (metrics.failedConnections > metrics.successfulConnections * 0.5) {
      testLogger.warn(`High failure rate detected at ${count} guests, stopping test`);
      break;
    }
  }

  return {
    maxGuestsReached: steps[steps.length - 1]?.count || 0,
    steps,
  };
}

/**
 * Chạy guest concurrency test chính
 */
export async function runGuestConcurrencyTest(options?: {
  count?: number;
  gradual?: boolean;
}): Promise<TestResult> {
  const startTime = Date.now();
  const { count = 10, gradual = false } = options || {};

  try {
    let metrics: GuestTestMetrics | { maxGuestsReached: number; steps: any[] };
    
    if (gradual) {
      metrics = await testGradualGuestIncrease();
    } else {
      metrics = await testConcurrentGuestRegistration(count);
    }

    const duration = Date.now() - startTime;
    const success = (metrics as GuestTestMetrics).successfulConnections > 0 || 
                    (metrics as any).maxGuestsReached > 0;

    return {
      testType: TestType.GUEST_CONCURRENCY,
      success,
      duration,
      metrics: {
        total: (metrics as GuestTestMetrics).totalGuests || (metrics as any).steps?.reduce((a: number, s: any) => a + s.count, 0) || 0,
        success: (metrics as GuestTestMetrics).successfulConnections || (metrics as any).steps?.reduce((a: number, s: any) => a + s.success, 0) || 0,
        failed: (metrics as GuestTestMetrics).failedConnections || (metrics as any).steps?.reduce((a: number, s: any) => a + s.failed, 0) || 0,
        errors: [],
      },
      details: metrics,
    };
  } catch (error: any) {
    testLogger.error('Guest concurrency test failed:', error);
    return {
      testType: TestType.GUEST_CONCURRENCY,
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
export { registerGuest, testConcurrentGuestRegistration };
