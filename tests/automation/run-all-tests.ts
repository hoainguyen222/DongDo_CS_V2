/**
 * ============================================================
 * Automation Test Runner
 * ============================================================
 * Chạy tất cả các bài test automation và tạo báo cáo tổng hợp
 * 
 * Usage:
 *   npx ts-node tests/automation/run-all-tests.ts
 *   npx ts-node tests/automation/run-all-tests.ts --guest
 *   npx ts-node tests/automation/run-all-tests.ts --message
 *   npx ts-node tests/automation/run-all-tests.ts --ws
 *   npx ts-node tests/automation/run-all-tests.ts --ai
 *   npx ts-node tests/automation/run-all-tests.ts --team
 */

import { TEST_CONFIG, TestType, testLogger } from './config';
import { runGuestConcurrencyTest } from './guest-load-test';
import { runMessageConcurrencyTest } from './message-load-test';
import { runWebSocketTest } from './websocket-test';
import { runAIResponseTest } from './ai-response-test';
import { runTeamAgentTest } from './team-agent-test';

// Types
interface TestReport {
  timestamp: string;
  environment: {
    apiBase: string;
    wsBase: string;
    maxConcurrentGuests: number;
    maxConcurrentMessages: number;
  };
  summary: {
    totalTests: number;
    passed: number;
    failed: number;
    totalDuration: number;
  };
  results: {
    testType: TestType;
    success: boolean;
    duration: number;
    metrics: {
      total: number;
      success: number;
      failed: number;
    };
    details?: any;
  }[];
}

/**
 * In kết quả test ra console với format đẹp
 */
function printTestResult(result: {
  testType: TestType;
  success: boolean;
  duration: number;
  metrics: {
    total: number;
    success: number;
    failed: number;
    errors: string[];
  };
  details?: any;
}) {
  const testName = {
    [TestType.GUEST_CONCURRENCY]: 'Guest Concurrency',
    [TestType.MESSAGE_CONCURRENCY]: 'Message Concurrency',
    [TestType.WEBSOCKET]: 'WebSocket',
    [TestType.AI_RESPONSE]: 'AI Response',
    [TestType.TEAM_AGENT]: 'Team Agent Support',
    [TestType.ALL]: 'All Tests',
  }[result.testType] || result.testType;

  console.log('\n' + '='.repeat(60));
  console.log(`  ${testName}`);
  console.log('='.repeat(60));
  console.log(`  Status: ${result.success ? '✅ PASSED' : '❌ FAILED'}`);
  console.log(`  Duration: ${(result.duration / 1000).toFixed(2)}s`);
  console.log(`  Metrics:`);
  console.log(`    - Total: ${result.metrics.total}`);
  console.log(`    - Success: ${result.metrics.success}`);
  console.log(`    - Failed: ${result.metrics.failed}`);
  
  if (result.metrics.errors.length > 0) {
    console.log(`  Errors:`);
    result.metrics.errors.forEach(err => console.log(`    - ${err}`));
  }

  // In thêm chi tiết nếu có
  if (result.details) {
    console.log('\n  Details:');
    if (result.details.successfulConnections !== undefined) {
      console.log(`    - Avg connection time: ${result.details.averageConnectionTime?.toFixed(2) || 0}ms`);
    }
    if (result.details.totalMessages !== undefined) {
      console.log(`    - Avg send time: ${result.details.averageSendTime?.toFixed(2) || 0}ms`);
    }
    if (result.details.connectionTest) {
      console.log(`    - WS connection: ${result.details.connectionTest.success ? 'OK' : 'FAILED'}`);
    }
    if (result.details.totalQuestions !== undefined) {
      console.log(`    - Avg AI response: ${result.details.averageResponseTime?.toFixed(0) || 0}ms`);
      console.log(`    - Fallbacks: ${result.details.fallbackCount || 0}`);
    }
    if (result.details.transferTest) {
      console.log(`    - Transfer success: ${result.details.transferTest.success}`);
    }
  }
}

/**
 * In báo cáo tổng kết
 */
function printSummaryReport(report: TestReport) {
  console.log('\n' + '#'.repeat(60));
  console.log('  AUTOMATION TEST SUMMARY REPORT');
  console.log('#'.repeat(60));
  console.log(`\n  Timestamp: ${report.timestamp}`);
  console.log(`\n  Environment:`);
  console.log(`    - API Base: ${report.environment.apiBase}`);
  console.log(`    - WS Base: ${report.environment.wsBase}`);
  console.log(`    - Max Concurrent Guests: ${report.environment.maxConcurrentGuests}`);
  console.log(`    - Max Concurrent Messages: ${report.environment.maxConcurrentMessages}`);
  console.log(`\n  Summary:`);
  console.log(`    - Total Tests: ${report.summary.totalTests}`);
  console.log(`    - Passed: ${report.summary.passed} ✅`);
  console.log(`    - Failed: ${report.summary.failed} ❌`);
  console.log(`    - Total Duration: ${(report.summary.totalDuration / 1000).toFixed(2)}s`);

  console.log(`\n  Test Results:`);
  report.results.forEach((result, index) => {
    const testName = {
      [TestType.GUEST_CONCURRENCY]: 'Guest Concurrency',
      [TestType.MESSAGE_CONCURRENCY]: 'Message Concurrency',
      [TestType.WEBSOCKET]: 'WebSocket',
      [TestType.AI_RESPONSE]: 'AI Response',
      [TestType.TEAM_AGENT]: 'Team Agent Support',
    }[result.testType] || result.testType;

    console.log(`    ${index + 1}. ${testName}: ${result.success ? '✅' : '❌'} (${result.metrics.success}/${result.metrics.total})`);
  });

  console.log('\n' + '#'.repeat(60));
}

/**
 * Lưu báo cáo ra file JSON
 */
function saveReportToFile(report: TestReport) {
  const fs = require('fs');
  const path = require('path');
  
  const reportDir = path.join(__dirname, 'reports');
  if (!fs.existsSync(reportDir)) {
    fs.mkdirSync(reportDir, { recursive: true });
  }
  
  const filename = `test-report-${Date.now()}.json`;
  const filepath = path.join(reportDir, filename);
  
  fs.writeFileSync(filepath, JSON.stringify(report, null, 2));
  testLogger.success(`Report saved to: ${filepath}`);
}

/**
 * Parse command line arguments
 */
function parseArgs(): {
  testTypes: TestType[];
  options: Record<string, any>;
} {
  const args = process.argv.slice(2);
  const testTypes: TestType[] = [];
  const options: Record<string, any> = {};

  for (const arg of args) {
    switch (arg) {
      case '--guest':
        testTypes.push(TestType.GUEST_CONCURRENCY);
        break;
      case '--message':
        testTypes.push(TestType.MESSAGE_CONCURRENCY);
        break;
      case '--ws':
        testTypes.push(TestType.WEBSOCKET);
        break;
      case '--ai':
        testTypes.push(TestType.AI_RESPONSE);
        break;
      case '--team':
        testTypes.push(TestType.TEAM_AGENT);
        break;
      case '--all':
        testTypes.push(TestType.ALL);
        break;
      case '--count':
        const nextArg = args[args.indexOf(arg) + 1];
        if (nextArg && !nextArg.startsWith('--')) {
          options.count = parseInt(nextArg);
        }
        break;
      case '--help':
        console.log(`
Usage: npx ts-node run-all-tests.ts [options]

Options:
  --guest         Run guest concurrency test
  --message       Run message concurrency test
  --ws            Run WebSocket test
  --ai            Run AI response test
  --team          Run team agent support test
  --all           Run all tests (default)
  --count N       Set test count (default: 10)
  --help          Show this help message

Examples:
  npx ts-node run-all-tests.ts --all
  npx ts-node run-all-tests.ts --guest --count 20
  npx ts-node run-all-tests.ts --ai --ws
        `);
        process.exit(0);
        break;
    }
  }

  // Default to all tests if none specified
  if (testTypes.length === 0) {
    testTypes.push(TestType.ALL);
  }

  return { testTypes, options };
}

/**
 * Main test runner
 */
async function main() {
  console.log('\n🔷🔷🔷  CSKH Automation Test Suite  🔷🔷🔷\n');
  
  const { testTypes, options } = parseArgs();
  const results: TestReport['results'] = [];
  const overallStartTime = Date.now();

  testLogger.info('Starting automation tests...');
  testLogger.info(`API Base: ${TEST_CONFIG.API_BASE}`);
  testLogger.info(`WS Base: ${TEST_CONFIG.WS_BASE}`);

  // Run tests based on selection
  for (const testType of testTypes) {
    if (testType === TestType.ALL) {
      // Run all tests
      console.log('\n📋 Running ALL tests...\n');

      try {
        testLogger.info('Running Guest Concurrency Test...');
        const guestResult = await runGuestConcurrencyTest({ count: options.count || 10 });
        results.push(guestResult);
        printTestResult(guestResult);
      } catch (e) {
        testLogger.error('Guest test error:', e);
      }

      try {
        testLogger.info('Running Message Concurrency Test...');
        const messageResult = await runMessageConcurrencyTest({ count: options.count || 10 });
        results.push(messageResult);
        printTestResult(messageResult);
      } catch (e) {
        testLogger.error('Message test error:', e);
      }

      try {
        testLogger.info('Running WebSocket Test...');
        const wsResult = await runWebSocketTest({ connectionCount: 5 });
        results.push(wsResult);
        printTestResult(wsResult);
      } catch (e) {
        testLogger.error('WebSocket test error:', e);
      }

      try {
        testLogger.info('Running AI Response Test...');
        const aiResult = await runAIResponseTest({ 
          testConsistency: true,
          testHumanCS: true,
          testConcurrency: true,
          concurrencyCount: 5,
        });
        results.push(aiResult);
        printTestResult(aiResult);
      } catch (e) {
        testLogger.error('AI test error:', e);
      }

      try {
        testLogger.info('Running Team Agent Test...');
        const teamResult = await runTeamAgentTest({
          testHandling: true,
          testNotification: true,
          testDistribution: true,
        });
        results.push(teamResult);
        printTestResult(teamResult);
      } catch (e) {
        testLogger.error('Team agent test error:', e);
      }

      break;
    } else {
      // Run specific test
      switch (testType) {
        case TestType.GUEST_CONCURRENCY:
          testLogger.info('Running Guest Concurrency Test...');
          const guestResult = await runGuestConcurrencyTest({ count: options.count || 10 });
          results.push(guestResult);
          printTestResult(guestResult);
          break;

        case TestType.MESSAGE_CONCURRENCY:
          testLogger.info('Running Message Concurrency Test...');
          const messageResult = await runMessageConcurrencyTest({ count: options.count || 10 });
          results.push(messageResult);
          printTestResult(messageResult);
          break;

        case TestType.WEBSOCKET:
          testLogger.info('Running WebSocket Test...');
          const wsResult = await runWebSocketTest({ connectionCount: 5 });
          results.push(wsResult);
          printTestResult(wsResult);
          break;

        case TestType.AI_RESPONSE:
          testLogger.info('Running AI Response Test...');
          const aiResult = await runAIResponseTest({ 
            testConsistency: true,
            testHumanCS: true,
            testConcurrency: true,
            concurrencyCount: 5,
          });
          results.push(aiResult);
          printTestResult(aiResult);
          break;

        case TestType.TEAM_AGENT:
          testLogger.info('Running Team Agent Test...');
          const teamResult = await runTeamAgentTest({
            testHandling: true,
            testNotification: true,
            testDistribution: true,
          });
          results.push(teamResult);
          printTestResult(teamResult);
          break;
      }
    }
  }

  // Create and print summary report
  const report: TestReport = {
    timestamp: new Date().toISOString(),
    environment: {
      apiBase: TEST_CONFIG.API_BASE,
      wsBase: TEST_CONFIG.WS_BASE,
      maxConcurrentGuests: TEST_CONFIG.MAX_CONCURRENT_GUESTS,
      maxConcurrentMessages: TEST_CONFIG.MAX_CONCURRENT_MESSAGES,
    },
    summary: {
      totalTests: results.length,
      passed: results.filter(r => r.success).length,
      failed: results.filter(r => !r.success).length,
      totalDuration: Date.now() - overallStartTime,
    },
    results,
  };

  printSummaryReport(report);
  saveReportToFile(report);

  // Exit with appropriate code
  process.exit(report.summary.failed > 0 ? 1 : 0);
}

// Run
main().catch((error) => {
  testLogger.error('Fatal error:', error);
  process.exit(1);
});
