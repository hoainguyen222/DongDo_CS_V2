#!/usr/bin/env node

/**
 * ============================================================
 * Quick Smoke Test
 * ============================================================
 * Chạy nhanh một số test cơ bản để kiểm tra hệ thống hoạt động
 * 
 * Usage:
 *   node quick-smoke-test.js
 */

const API_BASE = process.env.TEST_API_BASE || 'http://localhost:8080';

async function fetchJSON(url, options = {}) {
  try {
    const response = await fetch(url, {
      headers: { 'Content-Type': 'application/json' },
      ...options,
    });
    const data = await response.json();
    return { ok: response.ok, status: response.status, data };
  } catch (error) {
    return { ok: false, error: error.message };
  }
}

async function testAPI() {
  console.log('\n🔍 Testing API Endpoints...\n');
  
  const tests = [
    { name: 'Health Check', fn: () => fetchJSON(`${API_BASE}/health`) },
    { name: 'Guest Register', fn: () => fetchJSON(`${API_BASE}/guest/register`, {
      method: 'POST',
      body: JSON.stringify({ display_name: 'QuickTest_' + Date.now(), phone: '0909000000' }),
    })},
  ];

  const results = [];

  for (const test of tests) {
    const start = Date.now();
    const result = await test.fn();
    const duration = Date.now() - start;

    results.push({
      name: test.name,
      success: result.ok,
      duration,
      error: result.error || (result.ok ? null : `HTTP ${result.status}`),
    });

    const status = result.ok ? '✅' : '❌';
    console.log(`${status} ${test.name}: ${duration}ms${result.error ? ` - ${result.error}` : ''}`);
  }

  return results;
}

async function testWebSocket() {
  console.log('\n🔍 Testing WebSocket Connection...\n');

  return new Promise((resolve) => {
    try {
      const wsUrl = `${API_BASE.replace('http', 'ws')}/ws?session_id=test&user_id=quick_test&role=guest`;
      const ws = new WebSocket(wsUrl);

      const start = Date.now();
      let connected = false;

      ws.onopen = () => {
        connected = true;
        const duration = Date.now() - start;
        console.log(`✅ WebSocket Connected: ${duration}ms`);
        ws.close();
        resolve([{ name: 'WebSocket Connection', success: true, duration }]);
      };

      ws.onerror = (error) => {
        const duration = Date.now() - start;
        console.log(`❌ WebSocket Error: ${duration}ms`);
        resolve([{ name: 'WebSocket Connection', success: false, duration, error: 'Connection failed' }]);
      };

      // Timeout
      setTimeout(() => {
        if (!connected) {
          ws.close();
          console.log(`❌ WebSocket Timeout`);
          resolve([{ name: 'WebSocket Connection', success: false, duration: 5000, error: 'Timeout' }]);
        }
      }, 5000);
    } catch (error) {
      console.log(`❌ WebSocket Exception: ${error.message}`);
      resolve([{ name: 'WebSocket Connection', success: false, duration: 0, error: error.message }]);
    }
  });
}

async function testEndToEnd() {
  console.log('\n🔍 Testing End-to-End Flow...\n');

  // 1. Register guest
  console.log('1️⃣  Registering guest...');
  const registerResult = await fetchJSON(`${API_BASE}/guest/register`, {
    method: 'POST',
    body: JSON.stringify({ display_name: 'E2ETest_' + Date.now(), phone: '0909000001' }),
  });

  if (!registerResult.ok) {
    console.log('❌ Failed to register guest');
    return [{ name: 'E2E Flow', success: false, error: 'Failed to register guest' }];
  }

  const session = registerResult.data;
  console.log(`   ✅ Guest registered: ${session.display_name}`);
  console.log(`   Session ID: ${session.session_id}`);

  // 2. Send message
  console.log('\n2️⃣  Sending message...');
  const sendResult = await fetchJSON(`${API_BASE}/chat`, {
    method: 'POST',
    body: JSON.stringify({
      session_id: session.session_id,
      customer_name: session.display_name,
      message: 'E2E Test: Hàng hóa phái sinh là gì?',
    }),
  });

  if (!sendResult.ok) {
    console.log('❌ Failed to send message');
    return [{ name: 'E2E Flow', success: false, error: 'Failed to send message' }];
  }
  console.log('   ✅ Message sent');

  // 3. Wait and get history
  console.log('\n3️⃣  Waiting for AI response (5s)...');
  await new Promise(resolve => setTimeout(resolve, 5000));

  console.log('\n4️⃣  Getting chat history...');
  const historyResult = await fetchJSON(`${API_BASE}/history/${session.session_id}`);

  if (!historyResult.ok) {
    console.log('❌ Failed to get history');
    return [{ name: 'E2E Flow', success: false, error: 'Failed to get history' }];
  }

  const messages = historyResult.data.messages || [];
  const aiMessages = messages.filter(m => m.sender_type === 'ai' || m.sender_type === 'AI');
  
  console.log(`   ✅ History retrieved: ${messages.length} messages`);
  console.log(`   AI Responses: ${aiMessages.length}`);

  if (aiMessages.length > 0) {
    console.log(`   Latest AI Response: ${aiMessages[aiMessages.length - 1].content?.substring(0, 100)}...`);
  }

  return [{ name: 'E2E Flow', success: true, messages: messages.length, aiResponses: aiMessages.length }];
}

async function main() {
  console.log('\n' + '='.repeat(60));
  console.log('  🔷 CSKH Quick Smoke Test 🔷');
  console.log('='.repeat(60));
  console.log(`\n  API Base: ${API_BASE}`);
  console.log(`  Time: ${new Date().toISOString()}`);

  const allResults = [];

  try {
    // Test API
    const apiResults = await testAPI();
    allResults.push(...apiResults);
  } catch (e) {
    console.error('API test error:', e.message);
  }

  try {
    // Test WebSocket
    const wsResults = await testWebSocket();
    allResults.push(...wsResults);
  } catch (e) {
    console.error('WebSocket test error:', e.message);
  }

  try {
    // Test E2E
    const e2eResults = await testEndToEnd();
    allResults.push(...e2eResults);
  } catch (e) {
    console.error('E2E test error:', e.message);
  }

  // Summary
  console.log('\n' + '='.repeat(60));
  console.log('  Summary');
  console.log('='.repeat(60));

  const passed = allResults.filter(r => r.success).length;
  const failed = allResults.filter(r => !r.success).length;

  console.log(`\n  Total Tests: ${allResults.length}`);
  console.log(`  ✅ Passed: ${passed}`);
  console.log(`  ❌ Failed: ${failed}`);
  
  console.log('\n' + '='.repeat(60) + '\n');

  process.exit(failed > 0 ? 1 : 0);
}

main();
