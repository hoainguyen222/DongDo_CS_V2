/**
 * ============================================================
 * REALISTIC LOAD TEST
 * ============================================================
 * Test load thực tế với think time hợp lý
 * 
 * Usage:
 *   npx ts-node realistic-load-test.ts --users 20 --minutes 5
 */

const API_BASE = process.env.TEST_API_BASE || 'http://localhost:8080';

interface Stats {
  startTime: number;
  users: number;
  messagesSent: number;
  messagesFailed: number;
  aiResponses: number;
  errors: string[];
}

const stats: Stats = {
  startTime: Date.now(),
  users: 0,
  messagesSent: 0,
  messagesFailed: 0,
  aiResponses: 0,
  errors: [],
};

const questions = [
  "Tôi muốn biết về sản phẩm của công ty",
  "Làm sao để đăng ký dịch vụ?",
  "Chi phí dịch vụ là bao nhiêu?",
  "Thời gian làm việc của công ty?",
  "Tôi cần hỗ trợ về kỹ thuật",
];

async function registerGuest(name: string): Promise<{ success: boolean; sessionId?: string }> {
  try {
    const res = await fetch(`${API_BASE}/guest/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        display_name: name,
        phone: `090${Math.floor(Math.random() * 10000000)}`,
      }),
    });
    
    if (!res.ok) return { success: false };
    const data: any = await res.json();
    return { success: true, sessionId: data.session_id };
  } catch {
    return { success: false };
  }
}

async function sendMessage(sessionId: string, name: string, msg: string): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/chat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        session_id: sessionId,
        customer_name: name,
        message: msg,
      }),
    });
    return res.ok;
  } catch {
    return false;
  }
}

async function getHistory(sessionId: string): Promise<number> {
  try {
    const res = await fetch(`${API_BASE}/history/${sessionId}`);
    if (!res.ok) return 0;
    const data: any = await res.json();
    return (data.messages || []).length;
  } catch {
    return 0;
  }
}

function parseArgs() {
  const args = process.argv.slice(2);
  let users = 10;
  let minutes = 3;
  
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--users' || args[i] === '-u') {
      users = parseInt(args[++i]) || 10;
    }
    if (args[i] === '--minutes' || args[i] === '-m') {
      minutes = parseInt(args[++i]) || 3;
    }
    if (args[i] === '--help' || args[i] === '-h') {
      console.log(`
REALISTIC LOAD TEST
===================

Usage: npx ts-node realistic-load-test.ts [options]

Options:
  --users, -u      Number of concurrent users (default: 10)
  --minutes, -m    Test duration in minutes (default: 3)
  --help, -h       Show this help
      `);
      process.exit(0);
    }
  }
  
  return { users, minutes };
}

async function runUser(
  userId: number,
  sessionId: string,
  userName: string,
  durationMs: number,
  messagesPerUser: number
): Promise<void> {
  const endTime = Date.now() + durationMs;
  let msgCount = 0;
  
  while (Date.now() < endTime && msgCount < messagesPerUser) {
    const question = questions[msgCount % questions.length];
    const success = await sendMessage(sessionId, userName, `[U${userId}] ${question}`);
    
    if (success) {
      stats.messagesSent++;
      msgCount++;
      
      // Wait for AI response
      await new Promise(resolve => setTimeout(resolve, 3000));
      
      // Check if AI responded
      const historyCount = await getHistory(sessionId);
      if (historyCount > msgCount) {
        stats.aiResponses++;
      }
    } else {
      stats.messagesFailed++;
    }
    
    // Think time (realistic 2-5 seconds)
    const thinkTime = 2000 + Math.random() * 3000;
    await new Promise(resolve => setTimeout(resolve, thinkTime));
  }
}

async function main() {
  const { users, minutes } = parseArgs();
  const durationMs = minutes * 60 * 1000;
  
  console.log('\n🔷 REALISTIC LOAD TEST 🔷\n');
  console.log(`Users: ${users}`);
  console.log(`Duration: ${minutes} minutes`);
  console.log(`API: ${API_BASE}\n`);
  
  stats.startTime = Date.now();
  
  // Create users
  console.log('📝 Creating users...');
  const userPromises = Array.from({ length: users }, async (_, i) => {
    const name = `LoadUser_${Date.now()}_${i + 1}`;
    const result = await registerGuest(name);
    if (result.success) {
      return {
        id: i + 1,
        sessionId: result.sessionId!,
        name: `User${i + 1}`,
      };
    }
    return null;
  });
  
  const userSessions = (await Promise.all(userPromises)).filter(u => u !== null);
  stats.users = userSessions.length;
  
  console.log(`✅ Created ${stats.users} users\n`);
  
  if (stats.users === 0) {
    console.log('❌ Failed to create any users. Check server connectivity.');
    process.exit(1);
  }
  
  // Calculate messages per user based on duration
  const messagesPerUser = Math.ceil((minutes * 60) / 5); // ~1 message per 5 seconds
  
  // Start monitoring
  const monitorInterval = setInterval(() => {
    const elapsed = (Date.now() - stats.startTime) / 1000;
    const mins = Math.floor(elapsed / 60);
    const secs = Math.floor(elapsed % 60);
    
    process.stdout.write(
      `\r⏱️  ${mins}:${secs.toString().padStart(2, '0')} | Users: ${stats.users} | ` +
      `Messages: ${stats.messagesSent}/${stats.messagesSent + stats.messagesFailed} | ` +
      `AI: ${stats.aiResponses} | Errors: ${stats.errors.length}    `
    );
  }, 1000);
  
  // Run all users concurrently
  const runPromises = userSessions.map(user => 
    runUser(user.id, user.sessionId, user.name, durationMs, messagesPerUser)
  );
  
  await Promise.all(runPromises);
  
  clearInterval(monitorInterval);
  console.log('\n');
  
  // Final report
  const totalDuration = (Date.now() - stats.startTime) / 1000;
  const successRate = stats.messagesSent / (stats.messagesSent + stats.messagesFailed) * 100;
  
  console.log('='.repeat(60));
  console.log('  📊 LOAD TEST RESULTS');
  console.log('='.repeat(60));
  console.log(`\n  Duration: ${(totalDuration / 60).toFixed(1)} minutes`);
  console.log(`  Concurrent Users: ${stats.users}`);
  console.log(`\n  📬 Messages:`);
  console.log(`     Sent: ${stats.messagesSent}`);
  console.log(`     Failed: ${stats.messagesFailed}`);
  console.log(`     Success Rate: ${successRate.toFixed(1)}%`);
  console.log(`\n  🤖 AI Responses: ${stats.aiResponses}`);
  console.log(`     AI Rate: ${(stats.aiResponses / stats.messagesSent * 100).toFixed(1)}%`);
  console.log(`\n  ⚡ Throughput: ${(stats.messagesSent / totalDuration).toFixed(2)} msg/sec`);
  
  if (stats.errors.length > 0) {
    console.log(`\n  ❌ Errors:`);
    const uniqueErrors = [...new Set(stats.errors)].slice(0, 5);
    uniqueErrors.forEach((err, i) => {
      const count = stats.errors.filter(e => e === err).length;
      console.log(`     ${i + 1}. ${err} (${count}x)`);
    });
  }
  
  console.log('\n' + '='.repeat(60));
  
  const verdict = successRate >= 95 ? '✅ PASS' : successRate >= 80 ? '⚠️  CONDITIONAL PASS' : '❌ FAIL';
  console.log(`  ${verdict} - System ${successRate >= 95 ? 'ready' : 'needs attention'}`);
  console.log('='.repeat(60) + '\n');
}

main().catch(console.error);
