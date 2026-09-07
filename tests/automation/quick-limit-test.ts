/**
 * ============================================================
 * QUICK SYSTEM LIMITS TEST
 * ============================================================
 * Test nhanh để tìm giới hạn của hệ thống
 * Chạy trong 5-10 phút
 * 
 * Usage:
 *   npx ts-node quick-limit-test.ts
 */

import * as fs from 'fs';
import * as path from 'path';

const API_BASE = process.env.TEST_API_BASE || 'http://localhost:8080';

// ============== COLORS ==============

const colors = {
  green: '\x1b[32m',
  red: '\x1b[31m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  cyan: '\x1b[36m',
  reset: '\x1b[0m',
};

function log(color: keyof typeof colors, icon: string, message: string): void {
  console.log(`${colors[color]}${icon}${colors.reset} ${message}`);
}

// ============== UTILITIES ==============

async function registerGuest(name: string): Promise<{ success: boolean; sessionId?: string; error?: string }> {
  try {
    const response = await fetch(`${API_BASE}/guest/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        display_name: name,
        phone: `090${Math.floor(Math.random() * 10000000).toString().padStart(7, '0')}`,
      }),
    });

    if (!response.ok) {
      return { success: false, error: `HTTP ${response.status}` };
    }

    const data: any = await response.json();
    return { success: true, sessionId: data.session_id };
  } catch (error: any) {
    return { success: false, error: error.message };
  }
}

async function sendMessage(sessionId: string, message: string): Promise<{ success: boolean; duration: number }> {
  const start = Date.now();
  try {
    const response = await fetch(`${API_BASE}/chat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        session_id: sessionId,
        customer_name: 'TestUser',
        message,
      }),
    });
    return { success: response.ok, duration: Date.now() - start };
  } catch {
    return { success: false, duration: Date.now() - start };
  }
}

async function getHistory(sessionId: string): Promise<number> {
  try {
    const response = await fetch(`${API_BASE}/history/${sessionId}`);
    if (!response.ok) return 0;
    const data: any = await response.json();
    return (data.messages || []).length;
  } catch {
    return 0;
  }
}

// ============== TESTS ==============

interface TestResult {
  name: string;
  passed: boolean;
  value: number;
  target: number;
  unit: string;
  details: string;
}

async function testConcurrentConnections(target: number): Promise<TestResult> {
  log('blue', '🔵', `Testing ${target} concurrent connections...`);
  
  const start = Date.now();
  const promises = Array.from({ length: target }, (_, i) => 
    registerGuest(`LimitTest_${Date.now()}_${i}`)
  );
  
  const results = await Promise.all(promises);
  const duration = Date.now() - start;
  
  const successful = results.filter(r => r.success).length;
  const rate = (successful / (duration / 1000)).toFixed(1);
  
  log(successful === target ? 'green' : 'yellow', 
    successful === target ? '✅' : '⚠️',
    `${successful}/${target} connections in ${duration}ms (${rate}/sec)`);
  
  return {
    name: 'Concurrent Connections',
    passed: successful >= target * 0.9,
    value: successful,
    target,
    unit: 'connections',
    details: `${duration}ms total, ${rate} conn/sec`,
  };
}

async function testMessageThroughput(sessionId: string, count: number): Promise<TestResult> {
  log('blue', '🔵', `Testing ${count} message throughput...`);
  
  const messages = [
    'Tôi muốn biết về sản phẩm',
    'Làm sao để đăng ký?',
    'Chi phí là bao nhiêu?',
    'Thời gian làm việc?',
    'Tôi cần hỗ trợ kỹ thuật',
  ];
  
  const start = Date.now();
  const promises = Array.from({ length: count }, (_, i) => 
    sendMessage(sessionId, `${messages[i % messages.length]} [${i + 1}]`)
  );
  
  const results = await Promise.all(promises);
  const duration = Date.now() - start;
  
  const successful = results.filter(r => r.success).length;
  const avgTime = results.filter(r => r.success).reduce((a, b) => a + b.duration, 0) / successful || 0;
  const rate = (successful / (duration / 1000)).toFixed(1);
  
  log(successful === count ? 'green' : 'yellow',
    successful === count ? '✅' : '⚠️',
    `${successful}/${count} messages in ${duration}ms (${rate} msg/sec), avg ${avgTime.toFixed(0)}ms`);
  
  return {
    name: 'Message Throughput',
    passed: successful >= count * 0.9,
    value: successful,
    target: count,
    unit: 'messages',
    details: `${rate} msg/sec, ${avgTime.toFixed(0)}ms avg`,
  };
}

async function testBurstTraffic(sessionId: string, size: number): Promise<TestResult> {
  log('blue', '🔵', `Testing burst of ${size} messages...`);
  
  const start = Date.now();
  const promises = Array.from({ length: size }, (_, i) => 
    sendMessage(sessionId, `Burst message ${i + 1}`)
  );
  
  const results = await Promise.all(promises);
  const duration = Date.now() - start;
  
  const successful = results.filter(r => r.success).length;
  const rate = (successful / (duration / 1000)).toFixed(1);
  
  log(successful === size ? 'green' : 'yellow',
    successful === size ? '✅' : '⚠️',
    `${successful}/${size} burst messages in ${duration}ms (${rate}/sec)`);
  
  return {
    name: 'Burst Traffic',
    passed: successful >= size * 0.95,
    value: successful,
    target: size,
    unit: 'messages',
    details: `${rate} msg/sec`,
  };
}

async function testAIResponseTime(sessionId: string): Promise<TestResult> {
  log('blue', '🔵', 'Testing AI response time...');
  
  // Send message
  const msgStart = Date.now();
  const msgResult = await sendMessage(sessionId, ' Xin chào, bạn có thể giới thiệu về công ty được không?');
  
  if (!msgResult.success) {
    return {
      name: 'AI Response',
      passed: false,
      value: 0,
      target: 5000,
      unit: 'ms',
      details: 'Failed to send message',
    };
  }
  
  // Wait for AI
  const aiStart = Date.now();
  await new Promise(resolve => setTimeout(resolve, 3000));
  
  // Check history
  const historyCount = await getHistory(sessionId);
  
  const totalTime = Date.now() - msgStart;
  const hasAI = historyCount > 1; // More than just the sent message
  
  log(hasAI ? 'green' : 'yellow',
    hasAI ? '✅' : '⏳',
    `AI response in ${totalTime}ms${hasAI ? '' : ' (still waiting)'}`);
  
  return {
    name: 'AI Response Time',
    passed: hasAI && totalTime < 10000,
    value: totalTime,
    target: 5000,
    unit: 'ms',
    details: hasAI ? 'Response received' : 'No response yet',
  };
}

async function testSustainedLoad(users: number, messagesPerUser: number): Promise<TestResult> {
  log('blue', '🔵', `Testing sustained load: ${users} users × ${messagesPerUser} messages...`);
  
  const start = Date.now();
  let totalSent = 0;
  let totalFailed = 0;
  
  // Create users
  const userPromises = Array.from({ length: users }, (_, i) => 
    registerGuest(`Sustained_${Date.now()}_${i}`)
  );
  
  const userResults = await Promise.all(userPromises);
  const sessions = userResults.filter(r => r.success).map((r, i) => ({
    sessionId: r.sessionId!,
    index: i,
  }));
  
  log('cyan', 'ℹ️', `Created ${sessions.length} sessions, sending messages...`);
  
  // Send messages
  for (let round = 0; round < messagesPerUser; round++) {
    const msgPromises = sessions.map(s => 
      sendMessage(s.sessionId, `Round ${round + 1} message from user ${s.index}`)
    );
    
    const msgResults = await Promise.all(msgPromises);
    totalSent += msgResults.filter(r => r.success).length;
    totalFailed += msgResults.filter(r => !r.success).length;
    
    // Small delay between rounds
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  
  const duration = Date.now() - start;
  const rate = (totalSent / (duration / 1000)).toFixed(1);
  
  log(totalSent > 0 ? 'green' : 'red',
    totalSent > 0 ? '✅' : '❌',
    `${totalSent}/${totalSent + totalFailed} messages in ${(duration / 1000).toFixed(1)}s (${rate}/sec)`);
  
  return {
    name: 'Sustained Load',
    passed: totalSent > 0 && totalFailed < totalSent * 0.1,
    value: totalSent,
    target: users * messagesPerUser,
    unit: 'messages',
    details: `${rate} msg/sec, ${totalFailed} failed`,
  };
}

async function testDatabaseLoad(): Promise<TestResult> {
  log('blue', '🔵', 'Testing database load with multiple history queries...');
  
  // First create a session and send some messages
  const guest = await registerGuest(`DBTest_${Date.now()}`);
  if (!guest.success || !guest.sessionId) {
    return {
      name: 'Database Load',
      passed: false,
      value: 0,
      target: 50,
      unit: 'queries',
      details: 'Failed to create session',
    };
  }
  
  const sessionId = guest.sessionId;
  
  // Send some messages
  for (let i = 0; i < 5; i++) {
    await sendMessage(sessionId, `DB Test message ${i + 1}`);
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  
  // Now query history multiple times
  const start = Date.now();
  const promises = Array.from({ length: 50 }, () => getHistory(sessionId));
  const results = await Promise.all(promises);
  const duration = Date.now() - start;
  
  const avgTime = duration / 50;
  const rate = (50 / (duration / 1000)).toFixed(1);
  
  log('green', '✅', `${results.length} history queries in ${duration}ms (${rate}/sec), avg ${avgTime.toFixed(0)}ms`);
  
  return {
    name: 'Database Load',
    passed: avgTime < 100,
    value: avgTime,
    target: 100,
    unit: 'ms',
    details: `${rate} queries/sec`,
  };
}

async function findMaxConnections(): Promise<number> {
  log('cyan', '🔍', 'Finding maximum concurrent connections...');
  
  // Binary search style
  const steps = [50, 100, 150, 200, 250, 300, 400, 500];
  
  for (const target of steps) {
    log('blue', '⏳', `Testing ${target} connections...`);
    
    const start = Date.now();
    const promises = Array.from({ length: target }, (_, i) => 
      registerGuest(`MaxTest_${Date.now()}_${i}`)
    );
    
    const results = await Promise.all(promises);
    const duration = Date.now() - start;
    const successful = results.filter(r => r.success).length;
    
    const rate = (successful / (duration / 1000)).toFixed(1);
    
    if (successful < target * 0.8) {
      log('yellow', '⚠️', `Failed at ${target} (${successful}/${target})`);
      return target - 50; // Return last successful level
    }
    
    log('green', '✅', `${successful}/${target} in ${duration}ms (${rate}/sec)`);
    
    // Wait before next test
    await new Promise(resolve => setTimeout(resolve, 2000));
  }
  
  return 500; // Reached max tested
}

// ============== MAIN ==============

async function main() {
  console.log('\n' + '🔷'.repeat(30));
  console.log('  QUICK SYSTEM LIMITS TEST');
  console.log('🔷'.repeat(30));
  console.log(`\n🌐 API: ${API_BASE}\n`);
  
  const results: TestResult[] = [];
  
  // 1. Test concurrent connections
  results.push(await testConcurrentConnections(50));
  
  // Create a session for subsequent tests
  const testGuest = await registerGuest(`MainTest_${Date.now()}`);
  if (!testGuest.success || !testGuest.sessionId) {
    log('red', '❌', 'Failed to create test session, aborting');
    process.exit(1);
  }
  
  const sessionId = testGuest.sessionId;
  
  // 2. Test message throughput
  results.push(await testMessageThroughput(sessionId, 50));
  
  // 3. Test burst traffic
  results.push(await testBurstTraffic(sessionId, 100));
  
  // 4. Test AI response time
  results.push(await testAIResponseTime(sessionId));
  
  // 5. Test sustained load
  results.push(await testSustainedLoad(20, 5));
  
  // 6. Test database load
  results.push(await testDatabaseLoad());
  
  // 7. Find max connections
  const maxConn = await findMaxConnections();
  results.push({
    name: 'Max Connections',
    passed: maxConn >= 100,
    value: maxConn,
    target: 100,
    unit: 'connections',
    details: 'Maximum stable concurrent connections',
  });
  
  // ============== SUMMARY ==============
  
  console.log('\n' + '='.repeat(70));
  console.log('  📊 TEST SUMMARY');
  console.log('='.repeat(70));
  
  results.forEach((r, i) => {
    const status = r.passed ? '✅' : '❌';
    console.log(`\n${i + 1}. ${r.name}`);
    console.log(`   ${status} ${r.value}/${r.target} ${r.unit}`);
    console.log(`   ${r.details}`);
  });
  
  const passed = results.filter(r => r.passed).length;
  const total = results.length;
  
  console.log('\n' + '-'.repeat(70));
  console.log(`  Result: ${passed}/${total} tests passed`);
  
  if (passed === total) {
    console.log('  🎉 System is ready for production load!');
  } else if (passed >= total * 0.7) {
    console.log('  ⚠️  System has some limitations but is acceptable');
  } else {
    console.log('  ❌ System needs optimization before production');
  }
  
  console.log('='.repeat(70));
  
  // Save report
  const report = {
    timestamp: new Date().toISOString(),
    api: API_BASE,
    results,
    maxConnections: maxConn,
    overallPass: passed === total,
  };
  
  const reportDir = path.join(__dirname, 'reports');
  if (!fs.existsSync(reportDir)) {
    fs.mkdirSync(reportDir, { recursive: true });
  }
  
  const filename = `quick-limit-test-${Date.now()}.json`;
  const filepath = path.join(reportDir, filename);
  fs.writeFileSync(filepath, JSON.stringify(report, null, 2));
  
  console.log(`\n📄 Report: ${filepath}\n`);
}

main().catch(console.error);
