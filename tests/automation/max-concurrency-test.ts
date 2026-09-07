/**
 * ============================================================
 * Max Concurrency Stress Test
 * ============================================================
 * Test để tìm số lượng chat đồng thời tối đa mà hệ thống chịu được
 * 
 * Chiến lược:
 * 1. Tăng dần số lượng guests từ thấp đến cao
 * 2. Mỗi guest gửi messages đồng thời
 * 3. Đo metrics: success rate, latency, errors
 * 4. Dừng khi success rate < 95% hoặc quá nhiều errors
 */

import { TEST_CONFIG, testLogger } from './config';
import { registerGuest } from './guest-load-test';
import { sendMessage, getChatHistory } from './message-load-test';

interface LoadLevel {
  level: number;
  guests: number;
  messagesPerGuest: number;
  totalMessages: number;
  successCount: number;
  failedCount: number;
  successRate: number;
  avgLatency: number;
  maxLatency: number;
  errors: string[];
  status: 'PASS' | 'WARNING' | 'FAIL';
}

interface StressTestResult {
  maxConcurrentChats: number;
  maxSuccessRate: number;
  levels: LoadLevel[];
  recommendations: string[];
}

/**
 * Test một load level cụ thể
 */
async function testLoadLevel(
  level: number,
  guestCount: number,
  messagesPerGuest: number
): Promise<LoadLevel> {
  testLogger.info(`\n📊 LEVEL ${level}: ${guestCount} guests × ${messagesPerGuest} messages = ${guestCount * messagesPerGuest} total messages`);
  
  const startTime = Date.now();
  const errors: string[] = [];
  const latencies: number[] = [];
  let successCount = 0;
  let failedCount = 0;

  // Bước 1: Đăng ký tất cả guests đồng thời
  testLogger.info('  📝 Registering guests...');
  const registerStart = Date.now();
  const guests = await Promise.all(
    Array.from({ length: guestCount }, async (_, i) => {
      const name = `StressTest_${Date.now()}_${level}_${i}`;
      const result = await registerGuest(name);
      if (result) {
        return { ...result.session, duration: result.duration };
      } else {
        errors.push(`Guest ${i} registration failed`);
        return null;
      }
    })
  );
  const registerTime = Date.now() - registerStart;
  testLogger.info(`  ✅ Registered ${guests.filter(g => g !== null).length}/${guestCount} guests in ${registerTime}ms`);

  // Bước 2: Tất cả guests gửi messages đồng thời
  testLogger.info('  💬 Sending messages concurrently...');
  const messagePromises = guests
    .filter(g => g !== null)
    .flatMap((guest) => {
      const guestResults: Promise<{ success: boolean; latency: number }>[] = [];
      for (let i = 0; i < messagesPerGuest; i++) {
        const question = getTestQuestion(i);
        const promise = (async () => {
          const msgStart = Date.now();
          const result = await sendMessage(guest!.session_id, guest!.display_name, question);
          const latency = Date.now() - msgStart;
          latencies.push(latency);
          return { success: result.success, latency };
        })();
        guestResults.push(promise);
      }
      return guestResults;
    });

  const messageResults = await Promise.all(messagePromises);
  successCount = messageResults.filter(r => r.success).length;
  failedCount = messageResults.filter(r => !r.success).length;

  const totalTime = Date.now() - startTime;
  const successRate = (successCount / messageResults.length) * 100;
  const avgLatency = latencies.length > 0 
    ? latencies.reduce((a, b) => a + b, 0) / latencies.length 
    : 0;
  const maxLatency = latencies.length > 0 ? Math.max(...latencies) : 0;

  // Xác định status
  let status: 'PASS' | 'WARNING' | 'FAIL' = 'PASS';
  if (successRate < 80) status = 'FAIL';
  else if (successRate < 95) status = 'WARNING';

  const levelResult: LoadLevel = {
    level,
    guests: guestCount,
    messagesPerGuest,
    totalMessages: messageResults.length,
    successCount,
    failedCount,
    successRate,
    avgLatency: Math.round(avgLatency),
    maxLatency,
    errors,
    status,
  };

  // Log kết quả
  const statusIcon = status === 'PASS' ? '✅' : status === 'WARNING' ? '⚠️' : '❌';
  testLogger.info(`  ${statusIcon} Level ${level} Results:`);
  testLogger.info(`     - Success: ${successCount}/${messageResults.length} (${successRate.toFixed(1)}%)`);
  testLogger.info(`     - Avg Latency: ${avgLatency.toFixed(0)}ms`);
  testLogger.info(`     - Max Latency: ${maxLatency}ms`);
  testLogger.info(`     - Total Time: ${totalTime}ms`);

  if (errors.length > 0) {
    testLogger.warn(`  ⚠️ Errors: ${errors.slice(0, 3).join(', ')}${errors.length > 3 ? '...' : ''}`);
  }

  return levelResult;
}

/**
 * Lấy câu hỏi test
 */
function getTestQuestion(index: number): string {
  const questions = [
    'Hàng hóa phái sinh là gì?',
    'Cách nạp tiền vào tài khoản?',
    'Giới thiệu về DDP Invest',
    'Quản lý rủi ro khi giao dịch',
    'Phí giao dịch là bao nhiêu?',
    'Làm sao để rút tiền?',
    'Thời gian giao dịch là khi nào?',
    'Có ứng dụng mobile không?',
  ];
  return questions[index % questions.length];
}

/**
 * Chạy stress test để tìm max concurrency
 */
async function runMaxConcurrencyStressTest(options?: {
  startGuests?: number;
  startMessages?: number;
  maxLevels?: number;
  successThreshold?: number;
}): Promise<StressTestResult> {
  const {
    startGuests = 5,
    startMessages = 3,
    maxLevels = 12,
    successThreshold = 95,
  } = options || {};

  console.log(`
╔════════════════════════════════════════════════════════════════╗
║        MAX CONCURRENCY STRESS TEST                              ║
║        Tìm số chat đồng thời tối đa của hệ thống               ║
╠════════════════════════════════════════════════════════════════╣
║  Server: ${TEST_CONFIG.API_BASE}
║  Start:  ${startGuests} guests × ${startMessages} messages
║  Max Levels: ${maxLevels}
║  Success Threshold: ${successThreshold}%
╚════════════════════════════════════════════════════════════════╝
  `);

  const levels: LoadLevel[] = [];
  const recommendations: string[] = [];
  
  let currentGuests = startGuests;
  let currentMessages = startMessages;
  let maxReached = false;
  let lastPassLevel = 0;

  for (let level = 1; level <= maxLevels && !maxReached; level++) {
    const result = await testLoadLevel(level, currentGuests, currentMessages);
    levels.push(result);

    if (result.status === 'PASS') {
      lastPassLevel = level;
      recommendations.push(`Level ${level}: ${currentGuests} guests × ${currentMessages} messages = ${result.successCount} msgs @ ${result.avgLatency}ms avg`);
    }

    if (result.status === 'FAIL') {
      testLogger.warn(`\n❌ System failed at Level ${level} with ${currentGuests} guests`);
      maxReached = true;
    } else if (result.status === 'WARNING') {
      testLogger.warn(`\n⚠️ Warning at Level ${level} - success rate below ${successThreshold}%`);
    }

    // Tăng load cho level tiếp theo
    if (level < 6) {
      // Tăng nhanh ban đầu
      currentGuests = Math.round(currentGuests * 1.5);
    } else {
      // Tăng chậm hơn
      currentGuests = Math.round(currentGuests * 1.3);
    }
  }

  // Tính toán kết quả
  const maxConcurrentChats = lastPassLevel > 0 
    ? levels[lastPassLevel - 1].guests * levels[lastPassLevel - 1].messagesPerGuest 
    : 0;
  
  const maxSuccessRate = Math.max(...levels.map(l => l.successRate));

  // Tạo recommendations
  if (maxConcurrentChats > 0) {
    recommendations.push(`\n📌 KẾT LUẬN:`);
    recommendations.push(`   Max concurrent messages: ~${maxConcurrentChats}`);
    recommendations.push(`   (${levels[lastPassLevel - 1].guests} guests × ${levels[lastPassLevel - 1].messagesPerGuest} messages)`);
    recommendations.push(`   Success rate at max: ${levels[lastPassLevel - 1].successRate.toFixed(1)}%`);
    
    if (lastPassLevel < maxLevels) {
      recommendations.push(`\n⚠️ System bắt đầu có vấn đề ở Level ${lastPassLevel + 1}`);
      recommendations.push(`   Với ${levels[lastPassLevel]?.guests || '?'} guests × ${levels[lastPassLevel]?.messagesPerGuest || '?'} messages`);
      recommendations.push(`   Success rate chỉ còn: ${levels[lastPassLevel]?.successRate.toFixed(1) || '?'}%`);
    }
  }

  // In bảng tổng kết
  printSummaryTable(levels);
  printRecommendations(recommendations);

  // Lưu report
  const report = {
    timestamp: new Date().toISOString(),
    config: { startGuests, startMessages, maxLevels, successThreshold },
    result: { maxConcurrentChats, maxSuccessRate, levels, recommendations },
  };
  
  const fs = await import('fs');
  const reportPath = `reports/max-concurrency-${Date.now()}.json`;
  if (!fs.existsSync('reports')) {
    fs.mkdirSync('reports', { recursive: true });
  }
  fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
  testLogger.success(`\n📄 Report saved to: ${reportPath}`);

  return { maxConcurrentChats, maxSuccessRate, levels, recommendations };
}

/**
 * In bảng tổng kết
 */
function printSummaryTable(levels: LoadLevel[]): void {
  console.log(`
╔════════════════════════════════════════════════════════════════════════════════════════════╗
║                              STRESS TEST SUMMARY                                           ║
╠═════╦══════════╦══════╦═════════════╦═══════════╦════════╦══════════╦══════════╦══════════╣
║ Lvl ║  Guests  ║ Msgs  ║ Total Msgs  ║  Success  ║ Failed ║ Success ║ Avg(ms)  ║  Status  ║
╠═════╬══════════╬══════╬═════════════╬═══════════╬════════╬══════════╬══════════╬══════════╣`);

  levels.forEach(l => {
    const statusIcon = l.status === 'PASS' ? '✅' : l.status === 'WARNING' ? '⚠️' : '❌';
    console.log(`║  ${l.level.toString().padStart(2)}  ║  ${l.guests.toString().padStart(5)}  ║  ${l.messagesPerGuest.toString().padStart(4)}  ║   ${l.totalMessages.toString().padStart(7)}   ║   ${l.successCount.toString().padStart(4)}   ║  ${l.failedCount.toString().padStart(4)}  ║  ${l.successRate.toFixed(1).padStart(5)}%  ║  ${l.avgLatency.toString().padStart(6)}  ║  ${statusIcon}   ║`);
  });

  console.log(`╚═════╩══════════╩══════╩═════════════╩═══════════╩════════╩══════════╩══════════╩══════════╝`);
}

/**
 * In recommendations
 */
function printRecommendations(recs: string[]): void {
  console.log(`
╔════════════════════════════════════════════════════════════════╗
║                          RECOMMENDATIONS                        ║
╠════════════════════════════════════════════════════════════════╣`);
  recs.forEach(r => {
    console.log(`║  ${r.padEnd(62)}  ║`);
  });
  console.log(`╚════════════════════════════════════════════════════════════════╝`);
}

// CLI
async function main() {
  const args = process.argv.slice(2);
  const options: any = {};

  for (let i = 0; i < args.length; i++) {
    switch (args[i]) {
      case '--start-guests':
      case '-g':
        options.startGuests = parseInt(args[++i]) || 5;
        break;
      case '--messages':
      case '-m':
        options.startMessages = parseInt(args[++i]) || 3;
        break;
      case '--max-levels':
      case '-l':
        options.maxLevels = parseInt(args[++i]) || 12;
        break;
      case '--help':
      case '-h':
        console.log(`
Max Concurrency Stress Test
===========================
Options:
  --start-guests, -g  Số guests ban đầu (default: 5)
  --messages, -m     Số messages/guest ban đầu (default: 3)
  --max-levels, -l   Số levels tối đa (default: 12)
  --help, -h         Hiển thị help

Examples:
  ts-node max-concurrency-test.ts
  ts-node max-concurrency-test.ts --start-guests 10 --messages 5
  ts-node max-concurrency-test.ts -g 5 -m 3 -l 15
        `);
        process.exit(0);
    }
  }

  await runMaxConcurrencyStressTest(options);
}

main().catch(console.error);
