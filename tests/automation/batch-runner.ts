/**
 * ============================================================
 * Batch Parallel Test Runner
 * ============================================================
 * Chạy test với nhiều agent users đồng thời (batch/parallel)
 * Sử dụng bearer tokens từ config.agents.json
 * 
 * Usage:
 *   npx ts-node batch-runner.ts --scenario smoke_test
 *   npx ts-node batch-runner.ts --agents 5 --parallel
 *   npx ts-node batch-runner.ts --scenario medium_load --agents 10
 */

import {
  getAgentConfigManager,
  createAgentConfigManager,
  AgentUser,
} from './agent-config';
import { testLogger, TestType } from './config';

// Types
interface BatchTestResult {
  agentId: string;
  agentName: string;
  success: boolean;
  duration: number;
  metrics: {
    total: number;
    success: number;
    failed: number;
    errors: string[];
  };
  details?: any;
}

interface BatchReport {
  timestamp: string;
  scenario: string;
  totalAgents: number;
  successfulAgents: number;
  failedAgents: number;
  totalDuration: number;
  results: BatchTestResult[];
  errors: string[];
  metrics: {
    avgDuration: number;
    minDuration: number;
    maxDuration: number;
    totalMessages: number;
    totalSuccess: number;
    totalFailed: number;
  };
}

// Test operations that each agent will perform
async function runAgentChatTest(
  agent: { token: string; user: AgentUser },
  config: {
    apiBase: string;
    guestCount: number;
    messagesPerGuest: number;
    questions: string[];
  }
): Promise<BatchTestResult> {
  const startTime = Date.now();
  const errors: string[] = [];

  try {
    // 1. Create guests
    const guests: Array<{ session_id: string; display_name: string }> = [];
    
    for (let i = 0; i < config.guestCount; i++) {
      try {
        const guestName = `Batch_${agent.user.username}_Guest_${Date.now()}_${i}`;
        const response = await fetch(`${config.apiBase}/guest/register`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ display_name: guestName }),
        });

        if (response.ok) {
          const data = await response.json();
          guests.push({ session_id: data.session_id, display_name: data.display_name });
        }
      } catch (e: any) {
        errors.push(`Guest ${i}: ${e.message}`);
      }
    }

    // 2. Send messages from each guest
    let totalMessages = 0;
    let successMessages = 0;

    for (const guest of guests) {
      for (let i = 0; i < config.messagesPerGuest; i++) {
        const question = config.questions[i % config.questions.length];
        
        try {
          const response = await fetch(`${config.apiBase}/chat`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              session_id: guest.session_id,
              customer_name: guest.display_name,
              message: `[${agent.user.username}] ${question}`,
            }),
          });

          totalMessages++;
          if (response.ok) {
            successMessages++;
          } else {
            errors.push(`Message failed: HTTP ${response.status}`);
          }

          // Small delay between messages
          await new Promise(resolve => setTimeout(resolve, 50));
        } catch (e: any) {
          errors.push(`Message error: ${e.message}`);
        }
      }
    }

    return {
      agentId: agent.user.id,
      agentName: agent.user.username,
      success: true,
      duration: Date.now() - startTime,
      metrics: {
        total: totalMessages,
        success: successMessages,
        failed: totalMessages - successMessages,
        errors: errors.slice(0, 5), // Limit errors
      },
      details: {
        guestsCreated: guests.length,
        messagesPerGuest: config.messagesPerGuest,
      },
    };
  } catch (error: any) {
    return {
      agentId: agent.user.id,
      agentName: agent.user.username,
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

async function runAgentCSTest(
  agent: { token: string; user: AgentUser },
  config: {
    apiBase: string;
  }
): Promise<BatchTestResult> {
  const startTime = Date.now();
  const errors: string[] = [];

  try {
    // Get open cases using agent's token
    const casesResponse = await fetch(`${config.apiBase}/cases?status=ai_active&limit=10`, {
      headers: {
        'Authorization': `Bearer ${agent.token}`,
        'Content-Type': 'application/json',
      },
    });

    if (!casesResponse.ok) {
      errors.push(`Failed to get cases: HTTP ${casesResponse.status}`);
    }

    // Try to take a case
    if (casesResponse.ok) {
      const casesData = await casesResponse.json();
      const cases = casesData.cases || [];

      if (cases.length > 0) {
        const takeResponse = await fetch(`${config.apiBase}/cases/take`, {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${agent.token}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({ session_id: cases[0].session_id }),
        });

        if (takeResponse.ok) {
          // Send a reply
          const replyResponse = await fetch(`${config.apiBase}/cases/reply`, {
            method: 'POST',
            headers: {
              'Authorization': `Bearer ${agent.token}`,
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({
              session_id: cases[0].session_id,
              message: `Xin chào, tôi là ${agent.user.full_name}. Tôi đang hỗ trợ bạn.`,
            }),
          });

          return {
            agentId: agent.user.id,
            agentName: agent.user.username,
            success: replyResponse.ok,
            duration: Date.now() - startTime,
            metrics: {
              total: 1,
              success: replyResponse.ok ? 1 : 0,
              failed: replyResponse.ok ? 0 : 1,
              errors: [],
            },
            details: {
              casesFound: cases.length,
              caseTaken: true,
            },
          };
        }
      }
    }

    return {
      agentId: agent.user.id,
      agentName: agent.user.username,
      success: true,
      duration: Date.now() - startTime,
      metrics: {
        total: 0,
        success: 0,
        failed: 0,
        errors: ['No cases to process'],
      },
      details: {
        casesFound: 0,
        caseTaken: false,
      },
    };
  } catch (error: any) {
    return {
      agentId: agent.user.id,
      agentName: agent.user.username,
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

/**
 * Run batch test with multiple agents
 */
async function runBatchTest(options: {
  agentCount: number;
  scenario?: string;
  testType: 'chat' | 'cs' | 'mixed';
  parallel?: boolean;
  verbose?: boolean;
}): Promise<BatchReport> {
  const startTime = Date.now();
  const { agentCount, scenario, testType, parallel = true, verbose = false } = options;

  // Load config
  const configManager = createAgentConfigManager();
  const fullConfig = configManager.getConfig();
  const serverConfig = configManager.getServerConfig();

  // Get scenario config or use defaults
  const scenarioConfig = scenario ? configManager.getTestScenario(scenario) : undefined;
  
  const guestCount = scenarioConfig?.guest_count || 3;
  const messagesPerGuest = scenarioConfig?.messages_per_guest || 3;
  const questions = scenarioConfig?.ai_questions || 
    configManager.getAIQuestions().slice(0, 5).map(q => q.question);

  testLogger.info(`Starting batch test with ${agentCount} agents...`);
  testLogger.info(`Server: ${serverConfig.api}`);
  testLogger.info(`Test type: ${testType}`);
  testLogger.info(`Mode: ${parallel ? 'PARALLEL' : 'SEQUENTIAL'}`);

  // Login agents
  testLogger.info(`Logging in ${agentCount} agents...`);
  const agents = await configManager.loginAgentsForBatch(agentCount, parallel);
  testLogger.success(`Logged in ${agents.length} agents`);

  if (verbose) {
    agents.forEach(a => {
      testLogger.info(`  - ${a.user.username} (${a.user.full_name})`);
    });
  }

  // Run tests
  const results: BatchTestResult[] = [];
  const errors: string[] = [];

  if (parallel) {
    // Parallel execution
    testLogger.info('Running tests in parallel...');
    
    const promises = agents.map(async (agent, index) => {
      try {
        let result: BatchTestResult;
        
        if (testType === 'chat') {
          result = await runAgentChatTest(agent, {
            apiBase: serverConfig.api,
            guestCount,
            messagesPerGuest,
            questions,
          });
        } else if (testType === 'cs') {
          result = await runAgentCSTest(agent, {
            apiBase: serverConfig.api,
          });
        } else {
          // Mixed: run both
          const [chatResult, csResult] = await Promise.all([
            runAgentChatTest(agent, {
              apiBase: serverConfig.api,
              guestCount,
              messagesPerGuest,
              questions,
            }),
            runAgentCSTest(agent, {
              apiBase: serverConfig.api,
            }),
          ]);
          
          result = {
            agentId: agent.user.id,
            agentName: agent.user.username,
            success: chatResult.success && csResult.success,
            duration: chatResult.duration + csResult.duration,
            metrics: {
              total: chatResult.metrics.total + csResult.metrics.total,
              success: chatResult.metrics.success + csResult.metrics.success,
              failed: chatResult.metrics.failed + csResult.metrics.failed,
              errors: [...chatResult.metrics.errors, ...csResult.metrics.errors],
            },
            details: {
              chat: chatResult.details,
              cs: csResult.details,
            },
          };
        }

        if (verbose) {
          testLogger.info(`  [${index + 1}/${agents.length}] ${agent.user.username}: ${result.success ? '✅' : '❌'} (${result.duration}ms)`);
        }

        return result;
      } catch (error: any) {
        const errorMsg = `${agent.user.username}: ${error.message}`;
        errors.push(errorMsg);
        
        return {
          agentId: agent.user.id,
          agentName: agent.user.username,
          success: false,
          duration: 0,
          metrics: {
            total: 0,
            success: 0,
            failed: 0,
            errors: [error.message],
          },
        };
      }
    });

    const settledResults = await Promise.all(promises);
    results.push(...settledResults);
  } else {
    // Sequential execution
    testLogger.info('Running tests sequentially...');
    
    for (let i = 0; i < agents.length; i++) {
      const agent = agents[i];
      testLogger.info(`  [${i + 1}/${agents.length}] Testing ${agent.user.username}...`);
      
      let result: BatchTestResult;
      
      if (testType === 'chat') {
        result = await runAgentChatTest(agent, {
          apiBase: serverConfig.api,
          guestCount,
          messagesPerGuest,
          questions,
        });
      } else if (testType === 'cs') {
        result = await runAgentCSTest(agent, {
          apiBase: serverConfig.api,
        });
      } else {
        result = await runAgentChatTest(agent, {
          apiBase: serverConfig.api,
          guestCount,
          messagesPerGuest,
          questions,
        });
      }

      results.push(result);
      testLogger.info(`    ${result.success ? '✅' : '❌'} Duration: ${result.duration}ms`);
    }
  }

  // Calculate metrics
  const successfulAgents = results.filter(r => r.success).length;
  const failedAgents = results.filter(r => !r.success).length;
  const durations = results.map(r => r.duration);
  const totalMessages = results.reduce((a, r) => a + r.metrics.total, 0);
  const totalSuccess = results.reduce((a, r) => a + r.metrics.success, 0);
  const totalFailed = results.reduce((a, r) => a + r.metrics.failed, 0);

  // Create report
  const report: BatchReport = {
    timestamp: new Date().toISOString(),
    scenario: scenario || 'custom',
    totalAgents: agents.length,
    successfulAgents,
    failedAgents,
    totalDuration: Date.now() - startTime,
    results,
    errors,
    metrics: {
      avgDuration: durations.length > 0 ? durations.reduce((a, b) => a + b, 0) / durations.length : 0,
      minDuration: durations.length > 0 ? Math.min(...durations) : 0,
      maxDuration: durations.length > 0 ? Math.max(...durations) : 0,
      totalMessages,
      totalSuccess,
      totalFailed,
    },
  };

  return report;
}

/**
 * Print batch report
 */
function printBatchReport(report: BatchReport): void {
  console.log('\n' + '#'.repeat(70));
  console.log('  🔷 BATCH TEST REPORT 🔷');
  console.log('#'.repeat(70));
  
  console.log(`\n  Timestamp: ${report.timestamp}`);
  console.log(`  Scenario: ${report.scenario}`);
  console.log(`  Duration: ${(report.totalDuration / 1000).toFixed(2)}s`);
  
  console.log('\n  ──────────────────────────────────────');
  console.log('  AGENTS SUMMARY');
  console.log('  ──────────────────────────────────────');
  console.log(`  Total Agents: ${report.totalAgents}`);
  console.log(`  ✅ Successful: ${report.successfulAgents}`);
  console.log(`  ❌ Failed: ${report.failedAgents}`);
  
  console.log('\n  ──────────────────────────────────────');
  console.log('  PERFORMANCE METRICS');
  console.log('  ──────────────────────────────────────');
  console.log(`  Avg Duration: ${report.metrics.avgDuration.toFixed(0)}ms`);
  console.log(`  Min Duration: ${report.metrics.minDuration}ms`);
  console.log(`  Max Duration: ${report.metrics.maxDuration}ms`);
  console.log(`  Total Messages: ${report.metrics.totalMessages}`);
  console.log(`  Successful: ${report.metrics.totalSuccess}`);
  console.log(`  Failed: ${report.metrics.totalFailed}`);
  
  if (report.results.length > 0 && report.results[0].details) {
    console.log('\n  ──────────────────────────────────────');
    console.log('  AGENT DETAILS');
    console.log('  ──────────────────────────────────────');
    report.results.forEach((r, i) => {
      console.log(`  ${i + 1}. ${r.agentName}`);
      console.log(`     Status: ${r.success ? '✅' : '❌'}`);
      console.log(`     Duration: ${r.duration}ms`);
      console.log(`     Messages: ${r.metrics.success}/${r.metrics.total}`);
      if (r.details?.guestsCreated) {
        console.log(`     Guests Created: ${r.details.guestsCreated}`);
      }
    });
  }
  
  if (report.errors.length > 0) {
    console.log('\n  ──────────────────────────────────────');
    console.log('  ERRORS');
    console.log('  ──────────────────────────────────────');
    report.errors.forEach((err, i) => {
      console.log(`  ${i + 1}. ${err}`);
    });
  }
  
  console.log('\n' + '#'.repeat(70) + '\n');
}

/**
 * Save report to file
 */
function saveReportToFile(report: BatchReport): void {
  const fs = require('fs');
  const path = require('path');
  
  const reportDir = path.join(__dirname, 'reports');
  if (!fs.existsSync(reportDir)) {
    fs.mkdirSync(reportDir, { recursive: true });
  }
  
  const filename = `batch-report-${Date.now()}.json`;
  const filepath = path.join(reportDir, filename);
  
  fs.writeFileSync(filepath, JSON.stringify(report, null, 2));
  testLogger.success(`Report saved to: ${filepath}`);
}

/**
 * Parse command line arguments
 */
function parseArgs(): {
  agentCount: number;
  scenario: string | undefined;
  testType: 'chat' | 'cs' | 'mixed';
  parallel: boolean;
  verbose: boolean;
} {
  const args = process.argv.slice(2);
  
  let agentCount = 3;
  let scenario: string | undefined;
  let testType: 'chat' | 'cs' | 'mixed' = 'chat';
  let parallel = true;
  let verbose = false;

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    
    switch (arg) {
      case '--agents':
      case '-n':
        agentCount = parseInt(args[++i]) || 3;
        break;
      case '--scenario':
      case '-s':
        scenario = args[++i];
        break;
      case '--test':
      case '-t':
        testType = args[++i] as 'chat' | 'cs' | 'mixed';
        break;
      case '--parallel':
        parallel = true;
        break;
      case '--sequential':
        parallel = false;
        break;
      case '--verbose':
      case '-v':
        verbose = true;
        break;
      case '--help':
      case '-h':
        console.log(`
Batch Parallel Test Runner
==========================

Usage: npx ts-node batch-runner.ts [options]

Options:
  --agents, -n N     Number of agents to use (default: 3)
  --scenario, -s     Test scenario name (smoke_test, light_load, medium_load, heavy_load, stress_test)
  --test, -t         Test type: chat, cs, mixed (default: chat)
  --parallel         Run tests in parallel (default)
  --sequential       Run tests sequentially
  --verbose, -v     Verbose output
  --help, -h         Show this help

Examples:
  npx ts-node batch-runner.ts --agents 5 --parallel
  npx ts-node batch-runner.ts --scenario medium_load
  npx ts-node batch-runner.ts --test cs --agents 3
  npx ts-node batch-runner.ts -n 10 -s heavy_load -v
        `);
        process.exit(0);
        break;
    }
  }

  return { agentCount, scenario, testType, parallel, verbose };
}

/**
 * Main entry point
 */
async function main() {
  console.log('\n🔷🔷🔷  BATCH PARALLEL TEST RUNNER  🔷🔷🔷\n');
  
  const args = parseArgs();
  
  testLogger.info('Configuration:');
  testLogger.info(`  Agents: ${args.agentCount}`);
  testLogger.info(`  Scenario: ${args.scenario || 'custom'}`);
  testLogger.info(`  Test Type: ${args.testType}`);
  testLogger.info(`  Mode: ${args.parallel ? 'PARALLEL' : 'SEQUENTIAL'}`);
  
  try {
    const report = await runBatchTest({
      agentCount: args.agentCount,
      scenario: args.scenario,
      testType: args.testType,
      parallel: args.parallel,
      verbose: args.verbose,
    });
    
    printBatchReport(report);
    saveReportToFile(report);
    
    process.exit(report.failedAgents === report.totalAgents ? 1 : 0);
  } catch (error: any) {
    testLogger.error('Batch test failed:', error);
    process.exit(1);
  }
}

// Run
main().catch((error) => {
  testLogger.error('Fatal error:', error);
  process.exit(1);
});
